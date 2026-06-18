package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"
)

type Stats struct {
	mu sync.Mutex

	ConnectOK   int
	ConnectFail int
	OnlineOK    int
	OnlineFail  int
	JoinOK      int
	JoinFail    int
	ChatOK      int
	ChatFail    int
	PingOK      int
	PingFail    int

	latencies []time.Duration
	errors    map[string]int
}

func NewStats() *Stats {
	return &Stats{errors: make(map[string]int)}
}

func (s *Stats) RecordErr(kind string) {
	s.mu.Lock()
	s.errors[kind]++
	s.mu.Unlock()
}

func (s *Stats) RecordLatency(d time.Duration) {
	s.mu.Lock()
	s.latencies = append(s.latencies, d)
	s.mu.Unlock()
}

func (s *Stats) inc(fn func(*Stats)) {
	s.mu.Lock()
	fn(s)
	s.mu.Unlock()
}

type Report struct {
	Scenario   string    `json:"scenario"`
	Mode       string    `json:"mode"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	Duration   string    `json:"duration"`
	Conns      int       `json:"conns"`
	Rate       float64   `json:"rate_per_conn"`
	Rid        uint64    `json:"rid"`
	UsrSvr     string    `json:"usrsvr"`
	WSAddr     string    `json:"ws_addr"`

	ConnectOK   int `json:"connect_ok"`
	ConnectFail int `json:"connect_fail"`
	OnlineOK    int `json:"online_ok"`
	OnlineFail  int `json:"online_fail"`
	JoinOK      int `json:"join_ok"`
	JoinFail    int `json:"join_fail"`
	ChatOK      int `json:"chat_ok"`
	ChatFail    int `json:"chat_fail"`
	PingOK      int `json:"ping_ok"`
	PingFail    int `json:"ping_fail"`

	ChatQPS   float64        `json:"chat_qps"`
	PingQPS   float64        `json:"ping_qps"`
	LatencyMs *LatencyReport `json:"latency_ms,omitempty"`
	Errors    map[string]int `json:"errors"`

	Senders int `json:"senders,omitempty"`
	Rooms   int `json:"rooms,omitempty"`

	// EstDownstreamQPS ≈ chat_qps × 同房接收连接数（单房时 join_ok；多房时为 chat_qps×join_ok/rooms 粗估）
	EstDownstreamQPS float64            `json:"est_downstream_qps,omitempty"`
	Extrapolation    *ExtrapolationMeta `json:"extrapolation,omitempty"`

	Notes []string `json:"notes,omitempty"`
}

// ExtrapolationMeta 由 -conn-per-pod / -target-online 生成，供百万在线外推。
type ExtrapolationMeta struct {
	TargetOnlineUsers int64   `json:"target_online_users"`
	ConnPerPod        int     `json:"conn_per_pod_assumed"`
	PodsForTarget     int64   `json:"pods_for_target_online"`
	EstDownstreamQPS  float64 `json:"est_downstream_qps"`
}

type LatencyReport struct {
	Samples int     `json:"samples"`
	P50     float64 `json:"p50"`
	P95     float64 `json:"p95"`
	P99     float64 `json:"p99"`
	Max     float64 `json:"max"`
}

func (s *Stats) BuildReport(meta Report) Report {
	s.mu.Lock()
	defer s.mu.Unlock()

	meta.ConnectOK = s.ConnectOK
	meta.ConnectFail = s.ConnectFail
	meta.OnlineOK = s.OnlineOK
	meta.OnlineFail = s.OnlineFail
	meta.JoinOK = s.JoinOK
	meta.JoinFail = s.JoinFail
	meta.ChatOK = s.ChatOK
	meta.ChatFail = s.ChatFail
	meta.PingOK = s.PingOK
	meta.PingFail = s.PingFail
	meta.Errors = copyMap(s.errors)

	elapsed := meta.FinishedAt.Sub(meta.StartedAt).Seconds()
	if elapsed > 0 {
		meta.ChatQPS = float64(s.ChatOK) / elapsed
		meta.PingQPS = float64(s.PingOK) / elapsed
	}
	if len(s.latencies) > 0 {
		meta.LatencyMs = percentileMs(s.latencies)
	}
	if meta.Mode == "chat" && meta.ChatQPS > 0 && meta.JoinOK > 0 {
		if meta.Rooms <= 1 {
			meta.EstDownstreamQPS = meta.ChatQPS * float64(meta.JoinOK)
		} else {
			avg := float64(meta.JoinOK) / float64(meta.Rooms)
			meta.EstDownstreamQPS = meta.ChatQPS * avg
		}
	}
	if meta.Extrapolation != nil && meta.Extrapolation.ConnPerPod > 0 && meta.Extrapolation.TargetOnlineUsers > 0 {
		meta.Extrapolation.EstDownstreamQPS = meta.EstDownstreamQPS
		pods := (meta.Extrapolation.TargetOnlineUsers + int64(meta.Extrapolation.ConnPerPod) - 1) /
			int64(meta.Extrapolation.ConnPerPod)
		meta.Extrapolation.PodsForTarget = pods
	}
	return meta
}

func copyMap(m map[string]int) map[string]int {
	out := make(map[string]int, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func percentileMs(samples []time.Duration) *LatencyReport {
	cp := make([]time.Duration, len(samples))
	copy(cp, samples)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })

	pick := func(p float64) float64 {
		if len(cp) == 0 {
			return 0
		}
		idx := int(float64(len(cp)-1) * p)
		return float64(cp[idx].Microseconds()) / 1000.0
	}

	return &LatencyReport{
		Samples: len(cp),
		P50:     pick(0.50),
		P95:     pick(0.95),
		P99:     pick(0.99),
		Max:     float64(cp[len(cp)-1].Microseconds()) / 1000.0,
	}
}

func (r Report) WriteJSON(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

func (r Report) WriteCSV(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()

	rows := [][]string{
		{"scenario", r.Scenario},
		{"mode", r.Mode},
		{"duration", r.Duration},
		{"conns", fmt.Sprintf("%d", r.Conns)},
		{"rate_per_conn", fmt.Sprintf("%.2f", r.Rate)},
		{"chat_ok", fmt.Sprintf("%d", r.ChatOK)},
		{"chat_fail", fmt.Sprintf("%d", r.ChatFail)},
		{"chat_qps", fmt.Sprintf("%.2f", r.ChatQPS)},
		{"ping_ok", fmt.Sprintf("%d", r.PingOK)},
		{"ping_fail", fmt.Sprintf("%d", r.PingFail)},
	}
	if r.LatencyMs != nil {
		rows = append(rows,
			[]string{"latency_p50_ms", fmt.Sprintf("%.2f", r.LatencyMs.P50)},
			[]string{"latency_p95_ms", fmt.Sprintf("%.2f", r.LatencyMs.P95)},
			[]string{"latency_p99_ms", fmt.Sprintf("%.2f", r.LatencyMs.P99)},
		)
	}
	for _, row := range rows {
		if err := w.Write(row); err != nil {
			return err
		}
	}
	return nil
}

func (r Report) PrintSummary() {
	fmt.Println("=== loadtest summary ===")
	fmt.Printf("scenario=%s mode=%s duration=%s conns=%d\n", r.Scenario, r.Mode, r.Duration, r.Conns)
	if r.Mode == "chat" {
		fmt.Printf("chat ok=%d fail=%d qps=%.2f\n", r.ChatOK, r.ChatFail, r.ChatQPS)
		if r.EstDownstreamQPS > 0 {
			fmt.Printf("est downstream fan-out qps=%.2f (chat_qps×room_audience)\n", r.EstDownstreamQPS)
		}
		if r.Extrapolation != nil && r.Extrapolation.PodsForTarget > 0 {
			fmt.Printf("extrapolation: %d online -> ~%d ws pods @ %d conn/pod\n",
				r.Extrapolation.TargetOnlineUsers, r.Extrapolation.PodsForTarget, r.Extrapolation.ConnPerPod)
		}
		if r.LatencyMs != nil {
			fmt.Printf("latency ms: p50=%.2f p95=%.2f p99=%.2f max=%.2f (n=%d)\n",
				r.LatencyMs.P50, r.LatencyMs.P95, r.LatencyMs.P99, r.LatencyMs.Max, r.LatencyMs.Samples)
		}
	} else {
		fmt.Printf("keepalive ping ok=%d fail=%d qps=%.2f\n", r.PingOK, r.PingFail, r.PingQPS)
		if r.Extrapolation != nil && r.Extrapolation.PodsForTarget > 0 {
			fmt.Printf("extrapolation: %d online -> ~%d ws pods @ %d conn/pod (measured online=%d)\n",
				r.Extrapolation.TargetOnlineUsers, r.Extrapolation.PodsForTarget,
				r.Extrapolation.ConnPerPod, r.OnlineOK)
		}
	}
	fmt.Printf("connect ok=%d fail=%d online ok=%d fail=%d join ok=%d fail=%d\n",
		r.ConnectOK, r.ConnectFail, r.OnlineOK, r.OnlineFail, r.JoinOK, r.JoinFail)
	if len(r.Errors) > 0 {
		fmt.Println("errors:")
		for k, v := range r.Errors {
			fmt.Printf("  %s: %d\n", k, v)
		}
	}
}
