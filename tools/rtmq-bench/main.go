// RTMQ 单机压测：依赖已启动的 frwder（28888 FORWARD / 28889 BACKEND）。
//
// 用法（在 Linux 容器或本机 Linux 上）:
//
//	go run -mod=mod ./tools/rtmq-bench/main.go -mode publish -duration 10s
//	go run -mod=mod ./tools/rtmq-bench/main.go -mode unicast -duration 10s
package main

import (
	"flag"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/astaxie/beego/logs"

	"beehive-im/lib/comm"
	"beehive-im/lib/rtmq"
)

const benchCmd uint32 = 0xBEEF

type tally struct {
	sent uint64
	recv uint64
}

func main() {
	var (
		mode       = flag.String("mode", "publish", "publish (FORWARD→publish→BACKEND) | unicast (BACKEND→async_send→FORWARD nid)")
		duration   = flag.Duration("duration", 10*time.Second, "measure window after warmup")
		warmup     = flag.Duration("warmup", 2*time.Second, "warmup before measure")
		producers  = flag.Int("producers", 4, "concurrent producer goroutines")
		consumers  = flag.Int("consumers", 1, "BACKEND consumer proxies (publish mode)")
		payload    = flag.Int("payload", 256, "body bytes (excluding IM header for unicast)")
		forward    = flag.String("forward", env("BEEHIVE_RTMQ_FORWARD", "127.0.0.1:28888"), "FORWARD RTMQ addr")
		backend    = flag.String("backend", env("BEEHIVE_RTMQ_BACKEND", "127.0.0.1:28889"), "BACKEND RTMQ addr")
		chanLen    = flag.Uint("chan-len", 50000, "proxy send/recv channel length")
		workers    = flag.Uint("workers", 4, "proxy worker goroutines")
		drain      = flag.Duration("drain", 3*time.Second, "wait for in-flight after send stops")
	)
	flag.Parse()

	if *producers <= 0 || *consumers <= 0 || *payload < 0 {
		fmt.Fprintln(os.Stderr, "invalid producers/consumers/payload")
		os.Exit(2)
	}
	if *mode != "publish" && *mode != "unicast" {
		fmt.Fprintln(os.Stderr, "mode must be publish or unicast")
		os.Exit(2)
	}

	log := logs.NewLogger(1000)
	log.SetLevel(logs.LevelError)

	stats := &tally{}
	body := make([]byte, *payload)
	for i := range body {
		body[i] = byte(i)
	}

	switch *mode {
	case "publish":
		runPublish(log, stats, *forward, *backend, body, *consumers, *producers, *chanLen, *workers, *warmup, *duration, *drain)
	case "unicast":
		if *consumers != 1 {
			fmt.Println("note: unicast uses 1 FORWARD consumer; ignoring -consumers > 1")
		}
		runUnicast(log, stats, *forward, *backend, body, *producers, *chanLen, *workers, *warmup, *duration, *drain)
	}
}

func runPublish(log *logs.BeeLogger, stats *tally, forward, backend string, body []byte,
	consumers, producers int, chanLen, workers uint, warmup, duration, drain time.Duration) {

	var consumerProxies []*rtmq.Proxy
	for i := 0; i < consumers; i++ {
		nid := uint32(40001 + i)
		pxy := mustProxy(log, nid, 10, backend, chanLen, workers)
		pxy.Register(benchCmd, func(cmd, orig uint32, data []byte, length uint32, param interface{}) int {
			atomic.AddUint64(&stats.recv, 1)
			return 0
		}, nil)
		pxy.Launch()
		consumerProxies = append(consumerProxies, pxy)
	}

	prod := mustProxy(log, 50001, 10, forward, chanLen, workers)
	prod.Launch()

	fmt.Printf("rtmq-bench publish: forward=%s backend=%s consumers=%d producers=%d payload=%d\n",
		forward, backend, consumers, producers, len(body))
	waitConnect(warmup)

	runSendLoop(prod, body, producers, duration, stats)
	time.Sleep(drain)
	printReport("publish", stats, duration, consumers)
}

func runUnicast(log *logs.BeeLogger, stats *tally, forward, backend string, body []byte,
	producers int, chanLen, workers uint, warmup, duration, drain time.Duration) {

	const consumerNID uint32 = 40001

	pxy := mustProxy(log, consumerNID, 10, forward, chanLen, workers)
	pxy.Register(benchCmd, func(cmd, orig uint32, data []byte, length uint32, param interface{}) int {
		atomic.AddUint64(&stats.recv, 1)
		return 0
	}, nil)
	pxy.Launch()

	prod := mustProxy(log, 50001, 20, backend, chanLen, workers)
	prod.Launch()

	packet := packIM(benchCmd, consumerNID, body)

	fmt.Printf("rtmq-bench unicast: forward=%s backend=%s dest_nid=%d producers=%d payload=%d im_total=%d\n",
		forward, backend, consumerNID, producers, len(body), len(packet))
	waitConnect(warmup)

	runSendLoopBytes(prod, packet, producers, duration, stats)
	time.Sleep(drain)
	printReport("unicast", stats, duration, 1)
}

func runSendLoop(pxy *rtmq.Proxy, body []byte, producers int, duration time.Duration, stats *tally) {
	runSendLoopBytes(pxy, body, producers, duration, stats)
}

func runSendLoopBytes(pxy *rtmq.Proxy, packet []byte, producers int, duration time.Duration, stats *tally) {
	var wg sync.WaitGroup
	stop := make(chan struct{})
	deadline := time.Now().Add(duration)

	wg.Add(producers)
	for p := 0; p < producers; p++ {
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				if time.Now().After(deadline) {
					return
				}
				if pxy.AsyncSend(benchCmd, packet, uint32(len(packet))) == 0 {
					atomic.AddUint64(&stats.sent, 1)
				}
			}
		}()
	}
	time.Sleep(duration)
	close(stop)
	wg.Wait()
}

func packIM(cmd, nid uint32, body []byte) []byte {
	buf := make([]byte, comm.MESG_HEAD_SIZE+len(body))
	head := &comm.MesgHeader{
		Cmd:    cmd,
		Length: uint32(len(body)),
		Nid:    nid,
		Sid:    1,
	}
	p := &comm.MesgPacket{Buff: buf}
	comm.MesgHeadHton(head, p)
	copy(buf[comm.MESG_HEAD_SIZE:], body)
	return buf
}

func mustProxy(log *logs.BeeLogger, id, gid uint32, addr string, chanLen, workers uint) *rtmq.Proxy {
	conf := &rtmq.ProxyConf{
		Id:          id,
		Gid:         gid,
		Usr:         "qifeng",
		Passwd:      "111111",
		RemoteAddr:  addr,
		SendChanLen: uint32(chanLen),
		RecvChanLen: uint32(chanLen),
		WorkerNum:   uint32(workers),
	}
	pxy := rtmq.ProxyInit(conf, log)
	if pxy == nil {
		fmt.Fprintf(os.Stderr, "ProxyInit failed id=%d addr=%s\n", id, addr)
		os.Exit(1)
	}
	return pxy
}

func waitConnect(d time.Duration) {
	time.Sleep(d)
}

func printReport(mode string, stats *tally, window time.Duration, fanout int) {
	sent := atomic.LoadUint64(&stats.sent)
	recv := atomic.LoadUint64(&stats.recv)
	sec := window.Seconds()
	if sec <= 0 {
		sec = 1
	}
	sendQPS := float64(sent) / sec
	recvQPS := float64(recv) / sec

	fmt.Println("--- rtmq-bench result ---")
	fmt.Printf("mode:           %s\n", mode)
	fmt.Printf("measure:        %s\n", window)
	fmt.Printf("sent:           %d  (%.0f msg/s)\n", sent, sendQPS)
	fmt.Printf("recv:           %d  (%.0f msg/s)\n", recv, recvQPS)
	if mode == "publish" && fanout > 0 {
		fmt.Printf("fanout:         %d BACKEND consumers (expect recv ≈ sent × %d)\n", fanout, fanout)
		expected := sent * uint64(fanout)
		if expected > 0 {
			fmt.Printf("delivery:       %.2f%% (recv/expected)\n", 100*float64(recv)/float64(expected))
		}
	} else if sent > 0 {
		fmt.Printf("delivery:       %.2f%% (recv/sent)\n", 100*float64(recv)/float64(sent))
	}
	fmt.Println("note: RTMQ 无持久化；以上为进程内内存转发，不含业务逻辑。")
	fmt.Println("compare: Kafka 单机通常按 MB/s 与持久化 fsync 计；此测试为低延迟 fan-out 场景。")
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
