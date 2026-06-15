package main

import (
	"fmt"
	"os"

	"beehive-im/lib/comm"
	"beehive-im/lib/smokeclient"
)

func main() {
	base := smokeclient.Env("BEEHIVE_USRSVR_URL", "http://127.0.0.1:8000")
	if err := run(base); err != nil {
		fmt.Fprintf(os.Stderr, "smoke-fail FAILED: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("smoke-fail OK")
}

func run(base string) error {
	fmt.Println("[1/3] bad token ONLINE")
	c, err := smokeclient.Register(base, 100001)
	if err != nil {
		return err
	}
	if err := c.FillIplist(base); err != nil {
		return err
	}
	if err := c.ConnectURL(""); err != nil {
		return err
	}
	defer c.Close()
	code, err := c.OnlineBadToken()
	if err != nil {
		return err
	}
	if code == 0 {
		return fmt.Errorf("expected online failure, got code=0")
	}
	fmt.Printf("       online rejected code=%d\n", code)

	fmt.Println("[2/3] join invalid rid")
	if err := c.Online(); err != nil {
		return err
	}
	jcode, err := c.RoomJoin(99999999)
	if err != nil {
		return err
	}
	if jcode == 0 {
		return fmt.Errorf("expected join failure, got code=0")
	}
	fmt.Printf("       join rejected code=%d\n", jcode)

	fmt.Println("[3/3] chat without join")
	c2, err := smokeclient.Register(base, 100002)
	if err != nil {
		return err
	}
	if err := c2.FillIplist(base); err != nil {
		return err
	}
	if err := c2.ConnectURL(""); err != nil {
		return err
	}
	defer c2.Close()
	if err := c2.Online(); err != nil {
		return err
	}
	ccode, err := c2.RoomChat(10001, "should fail")
	if err != nil {
		return err
	}
	if ccode == 0 {
		return fmt.Errorf("expected chat failure, got code=0")
	}
	if ccode != comm.ERR_SVR_CHECK_FAIL {
		fmt.Printf("       chat rejected code=%d (expected %d ideally)\n", ccode, comm.ERR_SVR_CHECK_FAIL)
	}
	return nil
}
