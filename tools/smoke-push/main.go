package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"beehive-im/lib/comm"
	"beehive-im/lib/smokeclient"
)

func main() {
	usrsvr := smokeclient.Env("BEEHIVE_USRSVR_URL", "http://127.0.0.1:8000")
	if err := run(usrsvr); err != nil {
		fmt.Fprintf(os.Stderr, "smoke-push FAILED: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("smoke-push OK")
}

func run(base string) error {
	a, err := smokeclient.Register(base, 100001)
	if err != nil {
		return err
	}
	b, err := smokeclient.Register(base, 100002)
	if err != nil {
		return err
	}
	for _, c := range []*smokeclient.Client{a, b} {
		if err := c.FillIplist(base); err != nil {
			return err
		}
		if err := c.ConnectURL(""); err != nil {
			return err
		}
		defer c.Close()
		if err := c.Online(); err != nil {
			return err
		}
	}

	fmt.Println("[1/2] HTTP P2P push")
	payload := []byte("p2p-notify-smoke")
	go func() {
		_, _ = b.WaitCmd(comm.CMD_P2P, 20*time.Second)
	}()
	time.Sleep(300 * time.Millisecond)
	req, _ := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/im/push?dim=uid&uid=100002&kind=p2p", base), bytes.NewReader(payload))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("p2p push http %d body=%s", resp.StatusCode, string(body))
	}

	fmt.Println("[2/2] HTTP BC push (uid)")
	payload2 := []byte("bc-notify-smoke")
	go func() {
		_, _ = a.WaitCmd(comm.CMD_BC, 20*time.Second)
	}()
	time.Sleep(300 * time.Millisecond)
	req2, _ := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/im/push?dim=uid&uid=100001&kind=bc", base), bytes.NewReader(payload2))
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		return err
	}
	resp2.Body.Close()
	if resp2.StatusCode != 200 {
		return fmt.Errorf("bc push http %d", resp2.StatusCode)
	}
	return nil
}
