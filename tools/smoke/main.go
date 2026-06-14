package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/gorilla/websocket"

	"beehive-im/lib/comm"
	"beehive-im/lib/mesg"
)

const (
	defaultUsrSvr = "http://127.0.0.1:8000"
	defaultRid    = 10001
)

type registerResp struct {
	Uid    uint64 `json:"uid"`
	Sid    uint64 `json:"sid"`
	Code   int    `json:"code"`
	ErrMsg string `json:"errmsg"`
}

type iplistResp struct {
	Token  string   `json:"token"`
	List   []string `json:"list"`
	Len    int      `json:"len"`
	Code   int      `json:"code"`
	ErrMsg string   `json:"errmsg"`
}

type wsClient struct {
	uid   uint64
	sid   uint64
	token string
	wsURL string
	gid   uint32
	seq   uint64
	conn  *websocket.Conn
}

func main() {
	usrsvr := env("BEEHIVE_USRSVR_URL", defaultUsrSvr)
	if err := run(usrsvr, defaultRid); err != nil {
		fmt.Fprintf(os.Stderr, "\nsmoke-test FAILED: %v\n", err)
		printHints(err)
		os.Exit(1)
	}
	fmt.Println("smoke-test OK")
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func run(usrsvr string, rid uint64) error {
	fmt.Println("[1/6] register uid=100001,100002")
	a, err := register(usrsvr, 100001)
	if err != nil {
		return err
	}
	b, err := register(usrsvr, 100002)
	if err != nil {
		return err
	}

	fmt.Println("[2/6] iplist type=2")
	if err := fillIplist(usrsvr, a); err != nil {
		return err
	}
	if err := fillIplist(usrsvr, b); err != nil {
		return err
	}
	fmt.Printf("       ws: %s\n", a.wsURL)

	fmt.Println("[3/6] websocket connect + ONLINE")
	if err := a.connect(); err != nil {
		return fmt.Errorf("user A: %w", err)
	}
	defer a.close()
	if err := b.connect(); err != nil {
		return fmt.Errorf("user B: %w", err)
	}
	defer b.close()
	if err := a.online(); err != nil {
		return fmt.Errorf("user A online: %w", err)
	}
	if err := b.online(); err != nil {
		return fmt.Errorf("user B online: %w", err)
	}

	fmt.Printf("[4/6] ROOM-JOIN rid=%d\n", rid)
	if err := a.roomJoin(rid); err != nil {
		return fmt.Errorf("user A join: %w", err)
	}
	if err := b.roomJoin(rid); err != nil {
		return fmt.Errorf("user B join: %w", err)
	}

	fmt.Println("[5/6] ROOM-CHAT (B waits ROOM-CHAT)")
	bcCh := make(chan error, 1)
	go func() {
		_, _, err := b.waitCmd(comm.CMD_ROOM_CHAT, 20*time.Second)
		bcCh <- err
	}()
	time.Sleep(300 * time.Millisecond)
	if err := a.roomChat(rid, "smoke hello"); err != nil {
		return err
	}
	if err := <-bcCh; err != nil {
		return fmt.Errorf("user B recv: %w", err)
	}

	fmt.Println("[6/6] ROOM-QUIT / OFFLINE")
	if err := a.roomQuit(rid); err != nil {
		return err
	}
	if err := a.offline(); err != nil {
		return err
	}
	if err := b.roomQuit(rid); err != nil {
		return err
	}
	if err := b.offline(); err != nil {
		return err
	}
	return nil
}

func register(base string, uid uint64) (*wsClient, error) {
	u := fmt.Sprintf("%s/im/register?uid=%d&nation=1&city=1&town=1", base, uid)
	resp, err := http.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var out registerResp
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("register json: %w (%s)", err, string(body))
	}
	if out.Code != 0 || out.Sid == 0 {
		return nil, fmt.Errorf("register uid=%d code=%d errmsg=%s", uid, out.Code, out.ErrMsg)
	}
	return &wsClient{uid: uid, sid: out.Sid, seq: 1}, nil
}

func fillIplist(base string, c *wsClient) error {
	q := url.Values{}
	q.Set("type", "2")
	q.Set("uid", fmt.Sprintf("%d", c.uid))
	q.Set("sid", fmt.Sprintf("%d", c.sid))
	q.Set("clientip", "127.0.0.1")
	u := fmt.Sprintf("%s/im/iplist?%s", base, q.Encode())
	resp, err := http.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var out iplistResp
	if err := json.Unmarshal(body, &out); err != nil {
		return fmt.Errorf("iplist json: %w (%s)", err, string(body))
	}
	if out.Code != 0 || out.Len == 0 {
		return fmt.Errorf("iplist uid=%d code=%d len=%d errmsg=%s", c.uid, out.Code, out.Len, out.ErrMsg)
	}
	c.token = out.Token
	c.wsURL = "ws://" + out.List[0] + "/im"
	return nil
}

func (c *wsClient) connect() error {
	conn, _, err := websocket.DefaultDialer.Dial(c.wsURL, nil)
	if err != nil {
		return err
	}
	c.conn = conn
	return nil
}

func (c *wsClient) close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

func (c *wsClient) send(cmd uint32, msg proto.Message) error {
	var body []byte
	var err error
	if msg != nil {
		body, err = proto.Marshal(msg)
		if err != nil {
			return err
		}
	}
	return c.conn.WriteMessage(websocket.BinaryMessage, packMsg(cmd, c.sid, c.nextSeq(), body))
}

func (c *wsClient) waitCmd(want uint32, timeout time.Duration) (*comm.MesgHeader, []byte, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		c.conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			return nil, nil, err
		}
		head, payload, err := parseMsg(data)
		if err != nil {
			continue
		}
		if head.Cmd == want {
			return head, payload, nil
		}
	}
	return nil, nil, fmt.Errorf("timeout waiting cmd 0x%04X", want)
}

func (c *wsClient) online() error {
	if err := c.send(comm.CMD_ONLINE, &mesg.MesgOnline{
		Uid:      proto.Uint64(c.uid),
		Sid:      proto.Uint64(c.sid),
		Token:    proto.String(c.token),
		App:      proto.String("beehive-demo"),
		Version:  proto.String("1.0"),
		Terminal: proto.Uint32(1),
	}); err != nil {
		return err
	}
	_, payload, err := c.waitCmd(comm.CMD_ONLINE_ACK, 15*time.Second)
	if err != nil {
		return err
	}
	ack := &mesg.MesgOnlineAck{}
	if err := proto.Unmarshal(payload, ack); err != nil {
		return err
	}
	if ack.GetCode() != 0 {
		return fmt.Errorf("code=%d errmsg=%s", ack.GetCode(), ack.GetErrmsg())
	}
	c.seq = ack.GetSeq() + 1
	return nil
}

func (c *wsClient) roomJoin(rid uint64) error {
	if err := c.send(comm.CMD_ROOM_JOIN, &mesg.MesgRoomJoin{
		Uid: proto.Uint64(c.uid),
		Rid: proto.Uint64(rid),
	}); err != nil {
		return err
	}
	_, payload, err := c.waitCmd(comm.CMD_ROOM_JOIN_ACK, 15*time.Second)
	if err != nil {
		return err
	}
	ack := &mesg.MesgRoomJoinAck{}
	if err := proto.Unmarshal(payload, ack); err != nil {
		return err
	}
	if ack.GetCode() != 0 {
		return fmt.Errorf("code=%d errmsg=%s", ack.GetCode(), ack.GetErrmsg())
	}
	c.gid = ack.GetGid()
	return nil
}

func (c *wsClient) roomChat(rid uint64, text string) error {
	return c.send(comm.CMD_ROOM_CHAT, &mesg.MesgRoomChat{
		Uid:   proto.Uint64(c.uid),
		Rid:   proto.Uint64(rid),
		Gid:   proto.Uint32(c.gid),
		Level: proto.Uint32(0),
		Time:  proto.Uint64(uint64(time.Now().Unix())),
		Text:  proto.String(text),
	})
}

func (c *wsClient) roomQuit(rid uint64) error {
	return c.send(comm.CMD_ROOM_QUIT, &mesg.MesgRoomQuit{
		Uid: proto.Uint64(c.uid),
		Rid: proto.Uint64(rid),
	})
}

func (c *wsClient) offline() error {
	return c.send(comm.CMD_OFFLINE, nil)
}

func (c *wsClient) nextSeq() uint64 {
	seq := c.seq
	c.seq++
	return seq
}

func packMsg(cmd uint32, sid, seq uint64, body []byte) []byte {
	buf := make([]byte, comm.MESG_HEAD_SIZE+len(body))
	binary.BigEndian.PutUint32(buf[0:4], cmd)
	binary.BigEndian.PutUint32(buf[4:8], uint32(len(body)))
	binary.BigEndian.PutUint64(buf[8:16], sid)
	binary.BigEndian.PutUint64(buf[28:36], seq)
	copy(buf[comm.MESG_HEAD_SIZE:], body)
	return buf
}

func parseMsg(data []byte) (*comm.MesgHeader, []byte, error) {
	if len(data) < comm.MESG_HEAD_SIZE {
		return nil, nil, fmt.Errorf("short packet len=%d", len(data))
	}
	head := comm.MesgHeadNtoh(data)
	return head, data[comm.MESG_HEAD_SIZE:], nil
}

func printHints(err error) {
	msg := err.Error()
	fmt.Fprintln(os.Stderr, "\nHints:")
	switch {
	case contains(msg, "connection refused"):
		fmt.Fprintln(os.Stderr, "  - Run ./scripts/up-demo.sh and wait for ports 8000/8002")
	case contains(msg, "iplist"):
		fmt.Fprintln(os.Stderr, "  - Check log/monitor.log for websocket CMD_LSND_INFO")
		fmt.Fprintln(os.Stderr, "  - BEEHIVE_WS_IP should be 127.0.0.1 for host browser/smoke")
	case contains(msg, "ROOM-CHAT") || contains(msg, "ROOM-BC") || contains(msg, "join"):
		fmt.Fprintln(os.Stderr, "  - ./scripts/status.sh — frwder/chatroom/listend must be up")
		fmt.Fprintln(os.Stderr, "  - MySQL seed room rid=10001 (docker/mysql/init.sql)")
	default:
		fmt.Fprintln(os.Stderr, "  - ./scripts/status.sh")
		fmt.Fprintln(os.Stderr, "  - docker logs beehive-runner | tail -80")
	}
}

func contains(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
