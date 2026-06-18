package main

import (
	"fmt"
	"sync"
	"time"

	"beehive-im/lib/smokeclient"
)

type workerCfg struct {
	index    int
	uid      uint64
	usrsvr   string
	wsAddr   string
	rid      uint64
	mode     string
	rate     float64
	burst    bool
	sender   bool
	duration time.Duration
	stats    *Stats
	wg       *sync.WaitGroup
}

func runWorker(cfg workerCfg) {
	defer cfg.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			cfg.stats.RecordErr("panic")
		}
	}()

	c, err := smokeclient.Register(cfg.usrsvr, cfg.uid)
	if err != nil {
		cfg.stats.inc(func(s *Stats) { s.ConnectFail++ })
		cfg.stats.RecordErr("register")
		return
	}
	if err := c.FillIplist(cfg.usrsvr); err != nil {
		cfg.stats.inc(func(s *Stats) { s.ConnectFail++ })
		cfg.stats.RecordErr("iplist")
		return
	}
	if err := c.ConnectURL(cfg.wsAddr); err != nil {
		cfg.stats.inc(func(s *Stats) { s.ConnectFail++ })
		cfg.stats.RecordErr("ws_connect")
		return
	}
	defer c.Close()
	cfg.stats.inc(func(s *Stats) { s.ConnectOK++ })

	if err := c.Online(); err != nil {
		cfg.stats.inc(func(s *Stats) { s.OnlineFail++ })
		cfg.stats.RecordErr("online")
		return
	}
	cfg.stats.inc(func(s *Stats) { s.OnlineOK++ })

	if cfg.mode == "chat" {
		code, err := c.RoomJoin(cfg.rid)
		if err != nil || code != 0 {
			cfg.stats.inc(func(s *Stats) { s.JoinFail++ })
			cfg.stats.RecordErr("room_join")
			return
		}
		cfg.stats.inc(func(s *Stats) { s.JoinOK++ })
		if cfg.sender {
			runChatLoop(c, cfg)
			return
		}
		runWatchLoop(c, cfg)
		return
	}

	runKeepaliveLoop(c, cfg)
}

func runChatLoop(c *smokeclient.Client, cfg workerCfg) {
	deadline := time.Now().Add(cfg.duration)
	text := fmt.Sprintf("load uid=%d", cfg.uid)

	if cfg.burst {
		for time.Now().Before(deadline) {
			lat, code, err := c.RoomChatTimed(cfg.rid, text, 15*time.Second)
			if err != nil || code != 0 {
				cfg.stats.inc(func(s *Stats) { s.ChatFail++ })
				cfg.stats.RecordErr("room_chat")
				time.Sleep(50 * time.Millisecond)
				continue
			}
			cfg.stats.inc(func(s *Stats) { s.ChatOK++ })
			cfg.stats.RecordLatency(lat)
			if cfg.rate > 0 {
				sleep := time.Duration(float64(time.Second) / cfg.rate)
				time.Sleep(sleep)
			}
		}
		return
	}

	if cfg.rate <= 0 {
		cfg.rate = 1
	}
	ticker := time.NewTicker(time.Duration(float64(time.Second) / cfg.rate))
	defer ticker.Stop()

	for time.Now().Before(deadline) {
		<-ticker.C
		lat, code, err := c.RoomChatTimed(cfg.rid, text, 15*time.Second)
		if err != nil || code != 0 {
			cfg.stats.inc(func(s *Stats) { s.ChatFail++ })
			cfg.stats.RecordErr("room_chat")
			continue
		}
		cfg.stats.inc(func(s *Stats) { s.ChatOK++ })
		cfg.stats.RecordLatency(lat)
	}
}

// runWatchLoop：已 JOIN 同房，仅心跳，用于 1 发 N 看 fan-out 压测。
func runWatchLoop(c *smokeclient.Client, cfg workerCfg) {
	runKeepaliveLoop(c, cfg)
}

func runKeepaliveLoop(c *smokeclient.Client, cfg workerCfg) {
	deadline := time.Now().Add(cfg.duration)
	interval := 10 * time.Second
	if cfg.rate > 0 {
		interval = time.Duration(float64(time.Second) / cfg.rate)
		if interval < 500*time.Millisecond {
			interval = 500 * time.Millisecond
		}
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for time.Now().Before(deadline) {
		<-ticker.C
		if err := c.Ping(5 * time.Second); err != nil {
			cfg.stats.inc(func(s *Stats) { s.PingFail++ })
			cfg.stats.RecordErr("ping")
			continue
		}
		cfg.stats.inc(func(s *Stats) { s.PingOK++ })
	}
}
