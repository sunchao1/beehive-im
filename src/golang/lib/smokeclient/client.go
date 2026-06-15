package smokeclient

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

type Client struct {
	UID   uint64
	SID   uint64
	Token string
	WSURL string
	Gid   uint32
	Seq   uint64
	conn  *websocket.Conn
}

func Env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func Register(base string, uid uint64) (*Client, error) {
	u := fmt.Sprintf("%s/im/register?uid=%d&nation=1&city=1&town=1", base, uid)
	resp, err := http.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var out struct {
		Uid    uint64 `json:"uid"`
		Sid    uint64 `json:"sid"`
		Code   int    `json:"code"`
		ErrMsg string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	if out.Code != 0 || out.Sid == 0 {
		return nil, fmt.Errorf("register failed code=%d", out.Code)
	}
	return &Client{UID: uid, SID: out.Sid, Seq: 1}, nil
}

func (c *Client) FillIplist(base string) error {
	q := url.Values{}
	q.Set("type", "2")
	q.Set("uid", fmt.Sprintf("%d", c.UID))
	q.Set("sid", fmt.Sprintf("%d", c.SID))
	q.Set("clientip", "127.0.0.1")
	resp, err := http.Get(fmt.Sprintf("%s/im/iplist?%s", base, q.Encode()))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var out struct {
		Token  string   `json:"token"`
		List   []string `json:"list"`
		Len    int      `json:"len"`
		Code   int      `json:"code"`
		ErrMsg string   `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return err
	}
	if out.Code != 0 || out.Len == 0 {
		return fmt.Errorf("iplist empty")
	}
	c.Token = out.Token
	c.WSURL = "ws://" + out.List[0] + "/im"
	return nil
}

func (c *Client) ConnectURL(wsURL string) error {
	if wsURL != "" {
		c.WSURL = wsURL
	}
	conn, _, err := websocket.DefaultDialer.Dial(c.WSURL, nil)
	if err != nil {
		return err
	}
	c.conn = conn
	return nil
}

func (c *Client) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

func (c *Client) Send(cmd uint32, msg proto.Message) error {
	var body []byte
	var err error
	if msg != nil {
		body, err = proto.Marshal(msg)
		if err != nil {
			return err
		}
	}
	return c.conn.WriteMessage(websocket.BinaryMessage, packMsg(cmd, c.SID, c.nextSeq(), body))
}

func (c *Client) WaitCmd(want uint32, timeout time.Duration) (*comm.MesgHeader, []byte, error) {
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
		if err != nil || head.Cmd != want {
			continue
		}
		return head, payload, nil
	}
	return nil, nil, fmt.Errorf("timeout cmd 0x%04X", want)
}

func (c *Client) Online() error {
	if err := c.Send(comm.CMD_ONLINE, &mesg.MesgOnline{
		Uid: proto.Uint64(c.UID), Sid: proto.Uint64(c.SID), Token: proto.String(c.Token),
		App: proto.String("beehive-demo"), Version: proto.String("1.0"), Terminal: proto.Uint32(1),
	}); err != nil {
		return err
	}
	_, payload, err := c.WaitCmd(comm.CMD_ONLINE_ACK, 15*time.Second)
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
	c.Seq = ack.GetSeq() + 1
	return nil
}

func (c *Client) OnlineBadToken() (uint32, error) {
	_ = c.Send(comm.CMD_ONLINE, &mesg.MesgOnline{
		Uid: proto.Uint64(c.UID), Sid: proto.Uint64(c.SID), Token: proto.String("bad-token"),
		App: proto.String("beehive-demo"), Version: proto.String("1.0"), Terminal: proto.Uint32(1),
	})
	_, payload, err := c.WaitCmd(comm.CMD_ONLINE_ACK, 15*time.Second)
	if err != nil {
		return 0, err
	}
	ack := &mesg.MesgOnlineAck{}
	_ = proto.Unmarshal(payload, ack)
	return ack.GetCode(), nil
}

func (c *Client) RoomJoin(rid uint64) (uint32, error) {
	_ = c.Send(comm.CMD_ROOM_JOIN, &mesg.MesgRoomJoin{Uid: proto.Uint64(c.UID), Rid: proto.Uint64(rid)})
	_, payload, err := c.WaitCmd(comm.CMD_ROOM_JOIN_ACK, 15*time.Second)
	if err != nil {
		return 0, err
	}
	ack := &mesg.MesgRoomJoinAck{}
	_ = proto.Unmarshal(payload, ack)
	if ack.GetCode() == 0 {
		c.Gid = ack.GetGid()
	}
	return ack.GetCode(), nil
}

func (c *Client) RoomChat(rid uint64, text string) (uint32, error) {
	_ = c.Send(comm.CMD_ROOM_CHAT, &mesg.MesgRoomChat{
		Uid: proto.Uint64(c.UID), Rid: proto.Uint64(rid), Gid: proto.Uint32(c.Gid),
		Level: proto.Uint32(0), Time: proto.Uint64(uint64(time.Now().Unix())), Text: proto.String(text),
	})
	_, payload, err := c.WaitCmd(comm.CMD_ROOM_CHAT_ACK, 15*time.Second)
	if err != nil {
		return 0, err
	}
	ack := &mesg.MesgRoomChatAck{}
	_ = proto.Unmarshal(payload, ack)
	return ack.GetCode(), nil
}

func (c *Client) RoomCreat(name, desc string) (uint64, error) {
	_ = c.Send(comm.CMD_ROOM_CREAT, &mesg.MesgRoomCreat{
		Uid: proto.Uint64(c.UID), Name: proto.String(name), Desc: proto.String(desc),
	})
	_, payload, err := c.WaitCmd(comm.CMD_ROOM_CREAT_ACK, 15*time.Second)
	if err != nil {
		return 0, err
	}
	ack := &mesg.MesgRoomCreatAck{}
	_ = proto.Unmarshal(payload, ack)
	if ack.GetCode() != 0 {
		return 0, fmt.Errorf("creat code=%d", ack.GetCode())
	}
	return ack.GetRid(), nil
}

func (c *Client) RoomDismiss(rid uint64) (uint32, error) {
	_ = c.Send(comm.CMD_ROOM_DISMISS, &mesg.MesgRoomDismiss{Uid: proto.Uint64(c.UID), Rid: proto.Uint64(rid)})
	_, payload, err := c.WaitCmd(comm.CMD_ROOM_DISMISS_ACK, 15*time.Second)
	if err != nil {
		return 0, err
	}
	ack := &mesg.MesgRoomDismissAck{}
	_ = proto.Unmarshal(payload, ack)
	return ack.GetCode(), nil
}

func (c *Client) nextSeq() uint64 {
	s := c.Seq
	c.Seq++
	return s
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
		return nil, nil, fmt.Errorf("short")
	}
	head := comm.MesgHeadNtoh(data)
	return head, data[comm.MESG_HEAD_SIZE:], nil
}
