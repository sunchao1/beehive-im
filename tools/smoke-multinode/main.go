package main

import (
	"fmt"
	"os"
	"time"

	"beehive-im/lib/comm"
	"beehive-im/lib/smokeclient"
)

func main() {
	usrsvr := smokeclient.Env("BEEHIVE_USRSVR_URL", "http://127.0.0.1:8000")
	ws1 := smokeclient.Env("BEEHIVE_WS1", "ws://127.0.0.1:8002/im")
	ws2 := smokeclient.Env("BEEHIVE_WS2", "ws://127.0.0.1:8003/im")
	rid := uint64(10001)
	if err := run(usrsvr, ws1, ws2, rid); err != nil {
		fmt.Fprintf(os.Stderr, "smoke-multinode FAILED: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("smoke-multinode OK")
}

func run(usrsvr, ws1, ws2 string, rid uint64) error {
	a, err := smokeclient.Register(usrsvr, 100001)
	if err != nil {
		return err
	}
	b, err := smokeclient.Register(usrsvr, 100002)
	if err != nil {
		return err
	}
	if err := a.FillIplist(usrsvr); err != nil {
		return err
	}
	if err := b.FillIplist(usrsvr); err != nil {
		return err
	}
	if err := a.ConnectURL(ws1); err != nil {
		return err
	}
	defer a.Close()
	if err := b.ConnectURL(ws2); err != nil {
		return err
	}
	defer b.Close()
	if err := a.Online(); err != nil {
		return err
	}
	if err := b.Online(); err != nil {
		return err
	}

	fmt.Printf("[1/2] join rid=%d on ws1/ws2\n", rid)
	if code, err := a.RoomJoin(rid); err != nil || code != 0 {
		return fmt.Errorf("A join: %v code=%d", err, code)
	}
	if code, err := b.RoomJoin(rid); err != nil || code != 0 {
		return fmt.Errorf("B join: %v code=%d", err, code)
	}

	fmt.Println("[2/2] cross-node ROOM-CHAT")
	go func() {
		_, _ = b.WaitCmd(comm.CMD_ROOM_CHAT, 20*time.Second)
	}()
	time.Sleep(300 * time.Millisecond)
	if code, err := a.RoomChat(rid, "multinode hello"); err != nil || code != 0 {
		return fmt.Errorf("chat: %v code=%d", err, code)
	}
	return nil
}
