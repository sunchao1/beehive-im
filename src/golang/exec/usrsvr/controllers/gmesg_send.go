package controllers

import (
	"github.com/golang/protobuf/proto"

	"beehive-im/lib/chat"
	"beehive-im/lib/comm"
	"beehive-im/lib/mesg"
)

// groupSendPacket 向发起连接回复 ACK（保留 head 中 sid/cid/nid/seq）。
func (ctx *UsrSvrCntx) groupSendPacket(cmd int, head *comm.MesgHeader, body []byte) {
	if head == nil {
		return
	}
	p := &comm.MesgPacket{}
	p.Buff = make([]byte, comm.MESG_HEAD_SIZE+len(body))
	h := *head
	h.Cmd = uint32(cmd)
	h.Length = uint32(len(body))
	comm.MesgHeadHton(&h, p)
	copy(p.Buff[comm.MESG_HEAD_SIZE:], body)
	ctx.frwder.AsyncSend(uint32(cmd), p.Buff, uint32(len(p.Buff)))
}

func (ctx *UsrSvrCntx) groupSendCodeAck(cmd int, head *comm.MesgHeader, code uint32, errmsg string) {
	ack := &mesg.MesgGroupJoinAck{
		Code:   proto.Uint32(code),
		Errmsg: proto.String(errmsg),
	}
	body, err := proto.Marshal(ack)
	if err != nil {
		ctx.log.Error("Marshal group ack failed! errmsg:%s", err.Error())
		return
	}
	ctx.groupSendPacket(cmd, head, body)
}

// groupBroadcast 向群相关接入点广播通知（cmd + protobuf body）。
func (ctx *UsrSvrCntx) groupBroadcast(cmd int, gid uint64, body []byte) {
	nids, err := chatGroupNids(ctx, gid)
	if err != nil {
		ctx.log.Error("Broadcast gid:%d cmd:0x%04X failed! %s", gid, cmd, err.Error())
		return
	}
	for _, nid := range nids {
		p := &comm.MesgPacket{}
		p.Buff = make([]byte, comm.MESG_HEAD_SIZE+len(body))
		head := comm.MesgHeader{
			Cmd:    uint32(cmd),
			Length: uint32(len(body)),
			Nid:    nid,
		}
		comm.MesgHeadHton(&head, p)
		copy(p.Buff[comm.MESG_HEAD_SIZE:], body)
		ctx.frwder.AsyncSend(uint32(cmd), p.Buff, uint32(len(p.Buff)))
	}
}

func chatGroupNids(ctx *UsrSvrCntx, gid uint64) ([]uint32, error) {
	return chat.GroupGetGidToNidSet(ctx.redis, gid)
}

func groupAckBytes(code uint32, errmsg string) ([]byte, error) {
	return proto.Marshal(&mesg.MesgGroupJoinAck{
		Code:   proto.Uint32(code),
		Errmsg: proto.String(errmsg),
	})
}

func (ctx *UsrSvrCntx) groupSendSimpleAck(cmd int, head *comm.MesgHeader, code uint32, errmsg string) {
	body, err := groupAckBytes(code, errmsg)
	if err != nil {
		ctx.log.Error("Marshal group ack failed! errmsg:%s", err.Error())
		return
	}
	ctx.groupSendPacket(cmd, head, body)
}
