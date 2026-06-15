package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"beehive-im/lib/smokeclient"
)

func main() {
	usrsvr := smokeclient.Env("BEEHIVE_USRSVR_URL", "http://127.0.0.1:8000")
	chatroom := smokeclient.Env("BEEHIVE_CHATROOM_URL", "http://127.0.0.1:8004")
	if err := run(usrsvr, chatroom); err != nil {
		fmt.Fprintf(os.Stderr, "smoke-room FAILED: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("smoke-room OK")
}

func run(usrsvr, chatroom string) error {
	a, err := smokeclient.Register(usrsvr, 100001)
	if err != nil {
		return err
	}
	b, err := smokeclient.Register(usrsvr, 100002)
	if err != nil {
		return err
	}
	for _, c := range []*smokeclient.Client{a, b} {
		if err := c.FillIplist(usrsvr); err != nil {
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

	fmt.Println("[1/4] ROOM-CREAT")
	rid, err := a.RoomCreat("smoke-room-auto", "task02")
	if err != nil {
		return err
	}
	fmt.Printf("       rid=%d\n", rid)

	fmt.Println("[2/4] ROOM-JOIN + CHAT")
	if code, err := a.RoomJoin(rid); err != nil || code != 0 {
		return fmt.Errorf("A join code=%d err=%v", code, err)
	}
	if code, err := b.RoomJoin(rid); err != nil || code != 0 {
		return fmt.Errorf("B join code=%d err=%v", code, err)
	}
	go func() {
		_, _ = b.WaitCmd(comm.CMD_ROOM_CHAT, 20*time.Second)
	}()
	time.Sleep(300 * time.Millisecond)
	if code, err := a.RoomChat(rid, "room lifecycle hello"); err != nil || code != 0 {
		return fmt.Errorf("chat code=%d err=%v", code, err)
	}

	fmt.Println("[3/4] HTTP room-num")
	resp, err := http.Get(fmt.Sprintf("%s/room/query?option=room-num&rid=%d", chatroom, rid))
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("room-num http %d", resp.StatusCode)
	}

	fmt.Println("[4/4] ROOM-DISMISS")
	if code, err := a.RoomDismiss(rid); err != nil || code != 0 {
		return fmt.Errorf("dismiss code=%d err=%v", code, err)
	}

	// 回归种子房间
	fmt.Println("[regression] seed rid=10001 join")
	if code, err := a.RoomJoin(10001); err != nil || code != 0 {
		return fmt.Errorf("seed join code=%d", code)
	}
	return nil
}
