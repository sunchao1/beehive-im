package push

import (
	"fmt"
	"strconv"
	"time"

	"github.com/garyburd/redigo/redis"

	"beehive-im/lib/comm"
	"beehive-im/lib/im"
)

const maxPassthroughLen = 4096

// Sender 下发 RTMQ 数据（通常为 frwder.AsyncSend）。
type Sender func(cmd uint32, data []byte, length uint32) int

// Deliver 向在线会话/侦听层推送透传消息（BC/P2P）。
type Deliver struct {
	Pool   *redis.Pool
	SendFn Sender
}

func (d *Deliver) sendToNid(cmd uint32, sid, cid uint64, nid uint32, body []byte) error {
	if d == nil || d.Pool == nil || d.SendFn == nil {
		return fmt.Errorf("push deliver not initialized")
	}
	if len(body) > maxPassthroughLen {
		return fmt.Errorf("payload too large")
	}
	p := &comm.MesgPacket{}
	p.Buff = make([]byte, comm.MESG_HEAD_SIZE+len(body))
	head := comm.MesgHeader{
		Cmd:    cmd,
		Sid:    sid,
		Cid:    cid,
		Nid:    nid,
		Length: uint32(len(body)),
	}
	comm.MesgHeadHton(&head, p)
	copy(p.Buff[comm.MESG_HEAD_SIZE:], body)
	if d.SendFn(cmd, p.Buff, uint32(len(p.Buff))) != 0 {
		return fmt.Errorf("AsyncSend failed")
	}
	return nil
}

// ToSid 向指定 SID 推送。
func (d *Deliver) ToSid(cmd uint32, sid uint64, body []byte) error {
	attr, err := im.GetSidAttr(d.Pool, sid)
	if err != nil {
		return err
	}
	if attr.GetUid() == 0 || attr.GetNid() == 0 {
		return fmt.Errorf("sid not online")
	}
	return d.sendToNid(cmd, sid, attr.GetCid(), attr.GetNid(), body)
}

// ToUid 向用户所有在线 SID 推送。
func (d *Deliver) ToUid(cmd uint32, uid uint64, body []byte) (int, error) {
	rds := d.Pool.Get()
	defer rds.Close()
	key := fmt.Sprintf(comm.IM_KEY_UID_TO_SID_SET, uid)
	sidStrs, err := redis.Strings(rds.Do("SMEMBERS", key))
	if err != nil {
		return 0, err
	}
	sent := 0
	for _, s := range sidStrs {
		sid, perr := strconv.ParseUint(s, 10, 64)
		if perr != nil || sid == 0 {
			continue
		}
		if err := d.ToSid(cmd, sid, body); err == nil {
			sent++
		}
	}
	if sent == 0 {
		return 0, fmt.Errorf("uid not online")
	}
	return sent, nil
}

// ToAllLsnd 向所有在线侦听层广播（客户端自行处理 BC 内容）。
func (d *Deliver) ToAllLsnd(cmd uint32, body []byte) (int, error) {
	rds := d.Pool.Get()
	defer rds.Close()
	ctm := time.Now().Unix()
	nids, err := redis.Ints(rds.Do("ZRANGEBYSCORE", comm.IM_KEY_LSND_NID_ZSET, ctm, "+inf"))
	if err != nil {
		return 0, err
	}
	for _, nid := range nids {
		_ = d.sendToNid(cmd, 0, 0, uint32(nid), body)
	}
	return len(nids), nil
}

// FanOutFromPacket 将已含 MesgHeader 的完整包按 sid 或全站广播。
func (d *Deliver) FanOutFromPacket(data []byte) error {
	if len(data) < comm.MESG_HEAD_SIZE {
		return fmt.Errorf("short packet")
	}
	head := comm.MesgHeadNtoh(data)
	body := data[comm.MESG_HEAD_SIZE:]
	if head.GetSid() != 0 {
		attr, err := im.GetSidAttr(d.Pool, head.GetSid())
		if err != nil {
			return err
		}
		return d.sendToNid(head.GetCmd(), head.GetSid(), attr.GetCid(), attr.GetNid(), body)
	}
	_, err := d.ToAllLsnd(head.GetCmd(), body)
	return err
}
