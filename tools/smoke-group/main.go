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
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/gorilla/websocket"

	"beehive-im/lib/comm"
	"beehive-im/lib/mesg"
)

const defaultUsrSvr = "http://127.0.0.1:8000"

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
	seq   uint64
	conn  *websocket.Conn
}

func main() {
	usrsvr := env("BEEHIVE_USRSVR_URL", defaultUsrSvr)
	if err := run(usrsvr); err != nil {
		fmt.Fprintf(os.Stderr, "\nsmoke-group FAILED: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("smoke-group OK")
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func run(usrsvr string) error {
	fmt.Println("[1/5] register + online uid=100001,100002")
	a, err := setupClient(usrsvr, 100001)
	if err != nil {
		return err
	}
	defer a.close()
	b, err := setupClient(usrsvr, 100002)
	if err != nil {
		return err
	}
	defer b.close()

	fmt.Println("[2/5] GROUP-CREAT by A")
	gid, err := a.groupCreat("smoke-group", "task02 demo")
	if err != nil {
		return err
	}
	fmt.Printf("       gid=%d\n", gid)

	fmt.Println("[3/5] GROUP-JOIN B")
	if err := b.groupJoin(gid); err != nil {
		return err
	}

	fmt.Println("[4/5] GROUP-CHAT (B waits)")
	ch := make(chan error, 1)
	go func() {
		_, payload, err := b.waitCmd(comm.CMD_GROUP_CHAT, 20*time.Second)
		if err != nil {
			ch <- err
			return
		}
		chat := &mesg.MesgGroupChat{}
		if err := proto.Unmarshal(payload, chat); err != nil {
			ch <- err
			return
		}
		if chat.GetText() != "group hello" {
			ch <- fmt.Errorf("unexpected text %q", chat.GetText())
			return
		}
		ch <- nil
	}()
	time.Sleep(300 * time.Millisecond)
	if err := a.groupChat(gid, "group hello"); err != nil {
		return err
	}
	if err := <-ch; err != nil {
		return fmt.Errorf("user B recv: %w", err)
	}

	fmt.Println("[5/5] GROUP-QUIT + GROUP-DISMISS")
	if err := a.groupQuit(gid); err != nil {
		return err
	}
	if err := b.groupQuit(gid); err != nil {
		return err
	}
	return a.groupDismiss(gid)
}

func setupClient(base string, uid uint64) (*wsClient, error) {
	c, err := register(base, uid)
	if err != nil {
		return nil, err
	}
	if err := fillIplist(base, c); err != nil {
		return nil, err
	}
	if err := c.connect(); err != nil {
		return nil, err
	}
	if err := c.online(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *wsClient) groupCreat(name, desc string) (uint64, error) {
	if err := c.send(comm.CMD_GROUP_CREAT, &mesg.MesgGroupCreat{
		Uid:  proto.Uint64(c.uid),
		Gid:  proto.Uint64(0),
		Name: proto.String(name),
		Desc: proto.String(desc),
	}); err != nil {
		return 0, err
	}
	_, payload, err := c.waitCmd(comm.CMD_GROUP_CREAT_ACK, 15*time.Second)
	if err != nil {
		return 0, err
	}
	ack := &mesg.MesgGroupCreatAck{}
	if err := proto.Unmarshal(payload, ack); err != nil {
		return 0, err
	}
	if ack.GetCode() != 0 {
		return 0, fmt.Errorf("code=%d errmsg=%s", ack.GetCode(), ack.GetErrmsg())
	}
	return parseGidFromOk(ack.GetErrmsg())
}

func parseGidFromOk(errmsg string) (uint64, error) {
	if strings.HasPrefix(errmsg, "Ok:") {
		return strconv.ParseUint(strings.TrimPrefix(errmsg, "Ok:"), 10, 64)
	}
	return 0, fmt.Errorf("creat ack missing gid in errmsg: %q", errmsg)
}

func (c *wsClient) groupJoin(gid uint64) error {
	if err := c.send(comm.CMD_GROUP_JOIN, &mesg.MesgGroupJoin{
		Uid: proto.Uint64(c.uid),
		Gid: proto.Uint64(gid),
	}); err != nil {
		return err
	}
	_, payload, err := c.waitCmd(comm.CMD_GROUP_JOIN_ACK, 15*time.Second)
	if err != nil {
		return err
	}
	ack := &mesg.MesgGroupJoinAck{}
	if err := proto.Unmarshal(payload, ack); err != nil {
		return err
	}
	if ack.GetCode() != 0 {
		return fmt.Errorf("code=%d errmsg=%s", ack.GetCode(), ack.GetErrmsg())
	}
	return nil
}

func (c *wsClient) groupChat(gid uint64, text string) error {
	if err := c.send(comm.CMD_GROUP_CHAT, &mesg.MesgGroupChat{
		Uid:   proto.Uint64(c.uid),
		Gid:   proto.Uint64(gid),
		Level: proto.Uint32(0),
		Time:  proto.Uint64(uint64(time.Now().Unix())),
		Text:  proto.String(text),
	}); err != nil {
		return err
	}
	_, payload, err := c.waitCmd(comm.CMD_GROUP_CHAT_ACK, 15*time.Second)
	if err != nil {
		return err
	}
	ack := &mesg.MesgGroupChatAck{}
	if err := proto.Unmarshal(payload, ack); err != nil {
		return err
	}
	if ack.GetCode() != 0 {
		return fmt.Errorf("chat ack code=%d", ack.GetCode())
	}
	return nil
}

func (c *wsClient) groupQuit(gid uint64) error {
	return c.send(comm.CMD_GROUP_QUIT, &mesg.MesgGroupQuit{
		Uid: proto.Uint64(c.uid),
		Gid: proto.Uint64(gid),
	})
}

func (c *wsClient) groupDismiss(gid uint64) error {
	if err := c.send(comm.CMD_GROUP_DISMISS, &mesg.MesgGroupDismiss{
		Uid: proto.Uint64(c.uid),
		Gid: proto.Uint64(gid),
	}); err != nil {
		return err
	}
	_, payload, err := c.waitCmd(comm.CMD_GROUP_DISMISS_ACK, 15*time.Second)
	if err != nil {
		return err
	}
	ack := &mesg.MesgGroupDismissAck{}
	if err := proto.Unmarshal(payload, ack); err != nil {
		return err
	}
	if ack.GetCode() != 0 {
		return fmt.Errorf("dismiss code=%d", ack.GetCode())
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
		return nil, err
	}
	if out.Code != 0 || out.Sid == 0 {
		return nil, fmt.Errorf("register failed code=%d", out.Code)
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
		return err
	}
	if out.Code != 0 || out.Len == 0 {
		return fmt.Errorf("iplist empty")
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

func (c *wsClient) online() error {
	if err := c.send(comm.CMD_ONLINE, &mesg.MesgOnline{
		Uid: proto.Uint64(c.uid), Sid: proto.Uint64(c.sid), Token: proto.String(c.token),
		App: proto.String("beehive-demo"), Version: proto.String("1.0"), Terminal: proto.Uint32(1),
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
		return fmt.Errorf("online code=%d", ack.GetCode())
	}
	c.seq = ack.GetSeq() + 1
	return nil
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
	return nil, nil, fmt.Errorf("timeout cmd 0x%04X", want)
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
		return nil, nil, fmt.Errorf("short packet")
	}
	head := comm.MesgHeadNtoh(data)
	return head, data[comm.MESG_HEAD_SIZE:], nil
}
