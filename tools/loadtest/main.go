package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"beehive-im/lib/smokeclient"
)

func main() {
	var (
		uidBase  = flag.Uint64("uid-base", 200000, "first virtual user uid")
		conns    = flag.Int("conns", 200, "number of concurrent WS clients")
		rid      = flag.Uint64("rid", 10001, "room id (first room when -rooms>1)")
		rooms    = flag.Int("rooms", 1, "spread clients across N rooms: rid+i%rooms")
		senders  = flag.Int("senders", 0, "chat: only first N clients send; rest join and watch (0=all send)")
		rate     = flag.Float64("rate", 1, "messages or pings per second per connection (chat mode)")
		duration = flag.Duration("duration", 60*time.Second, "test duration after all clients online")
		usrsvr   = flag.String("usrsvr", smokeclient.Env("BEEHIVE_USRSVR_URL", "http://127.0.0.1:8000"), "usrsvr base URL")
		wsAddr   = flag.String("ws-addr", smokeclient.Env("BEEHIVE_WS_ADDR", ""), "override ws URL (empty = iplist)")
		mode     = flag.String("mode", "chat", "chat | keepalive")
		ramp     = flag.Duration("ramp", 10*time.Second, "spread connection establishment over this duration")
		burst    = flag.Bool("burst", false, "chat: send as fast as rate allows without ticker coalescing")
		scenario      = flag.String("scenario", "custom", "label for report")
		targetOnline  = flag.Int64("target-online", 0, "extrapolation: target concurrent online users (e.g. 1000000)")
		connPerPod    = flag.Int("conn-per-pod", 0, "extrapolation: assumed stable WS connections per pod")
		report        = flag.String("report", "", "JSON report path (default reports/loadtest/latest.json)")
		csv           = flag.String("csv", "", "optional CSV summary path")
	)
	flag.Parse()

	if *conns <= 0 {
		fmt.Fprintln(os.Stderr, "conns must be > 0")
		os.Exit(2)
	}
	if *rooms <= 0 {
		fmt.Fprintln(os.Stderr, "rooms must be > 0")
		os.Exit(2)
	}
	if *senders < 0 || *senders > *conns {
		fmt.Fprintln(os.Stderr, "senders must be in [0, conns]")
		os.Exit(2)
	}
	if *mode != "chat" && *mode != "keepalive" {
		fmt.Fprintln(os.Stderr, "mode must be chat or keepalive")
		os.Exit(2)
	}

	if *report == "" {
		*report = "reports/loadtest/latest.json"
	}
	if err := os.MkdirAll(filepath.Dir(*report), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir report: %v\n", err)
		os.Exit(1)
	}

	stats := NewStats()
	started := time.Now()

	var wg sync.WaitGroup
	wg.Add(*conns)
	rampStep := time.Duration(0)
	if *conns > 1 && *ramp > 0 {
		rampStep = *ramp / time.Duration(*conns-1)
	}

	senderN := *senders
	if *mode == "chat" && senderN == 0 {
		senderN = *conns
	}
	fmt.Printf("loadtest start scenario=%s mode=%s conns=%d senders=%d rooms=%d rate=%.2f duration=%s rid=%d\n",
		*scenario, *mode, *conns, senderN, *rooms, *rate, *duration, *rid)

	for i := 0; i < *conns; i++ {
		if i > 0 && rampStep > 0 {
			time.Sleep(rampStep)
		}
		roomRid := *rid + uint64(i%*rooms)
		cfg := workerCfg{
			index:    i,
			uid:      *uidBase + uint64(i),
			usrsvr:   *usrsvr,
			wsAddr:   *wsAddr,
			rid:      roomRid,
			mode:     *mode,
			rate:     *rate,
			burst:    *burst,
			sender:   i < senderN,
			duration: *duration,
			stats:    stats,
			wg:       &wg,
		}
		go runWorker(cfg)
	}

	wg.Wait()
	finished := time.Now()

	meta := Report{
		Scenario:   *scenario,
		Mode:       *mode,
		StartedAt:  started,
		FinishedAt: finished,
		Duration:   finished.Sub(started).String(),
		Conns:      *conns,
		Rate:       *rate,
		Rid:        *rid,
		Senders:    senderN,
		Rooms:      *rooms,
		UsrSvr:     *usrsvr,
		WSAddr:     *wsAddr,
		Notes: []string{
			"本地标定系数；百万在线为同架构线性外推，见 doc/INTERVIEW_CAPACITY_DEMO.md",
		},
	}
	if *targetOnline > 0 && *connPerPod > 0 {
		meta.Extrapolation = &ExtrapolationMeta{
			TargetOnlineUsers: *targetOnline,
			ConnPerPod:        *connPerPod,
		}
	}
	rep := stats.BuildReport(meta)

	if err := rep.WriteJSON(*report); err != nil {
		fmt.Fprintf(os.Stderr, "write json: %v\n", err)
		os.Exit(1)
	}
	if *csv != "" {
		if err := os.MkdirAll(filepath.Dir(*csv), 0o755); err == nil {
			_ = rep.WriteCSV(*csv)
		}
	}

	rep.PrintSummary()
	fmt.Printf("report: %s\n", *report)

	if rep.ConnectFail > 0 || rep.OnlineFail > 0 {
		os.Exit(1)
	}
	if *mode == "chat" && rep.ChatOK == 0 {
		os.Exit(1)
	}
	if *mode == "keepalive" && rep.PingOK == 0 {
		os.Exit(1)
	}
}
