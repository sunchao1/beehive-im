package controllers

import (
	"errors"
	"os"

	"beehive-im/lib/comm"
	"beehive-im/lib/mesg"
)

const roomBroadcastChanLen = 100000

func chatroomAsyncBroadcast() bool {
	return os.Getenv("BEEHIVE_CHATROOM_ASYNC_BROADCAST") == "1"
}

func (ctx *ChatRoomCntx) roomChatEnqueueAsync(
	head *comm.MesgHeader, req *mesg.MesgRoomChat, data []byte) error {
	raw := make([]byte, len(data))
	copy(raw, data)

	store := &MesgRoomItem{head: head, req: req, raw: raw}
	select {
	case ctx.room_mesg_chan <- store:
	default:
		return errors.New("room message storage queue full")
	}

	bc := &MesgRoomItem{head: head, req: req, raw: raw}
	select {
	case ctx.room_broadcast_chan <- bc:
		return nil
	default:
		return errors.New("room broadcast queue full")
	}
}

func (ctx *ChatRoomCntx) roomBroadcastFanOut(item *MesgRoomItem) {
	head := item.head
	req := item.req
	data := item.raw

	ctx.room.node.RLock()
	defer ctx.room.node.RUnlock()

	nid_list, ok := ctx.room.node.m[req.GetRid()]
	if !ok {
		ctx.log.Error("Broadcast fan-out: no nid list! rid:%d", req.GetRid())
		return
	}

	for idx, nid := range nid_list {
		ctx.log.Debug("broadcast idx:%d rid:%d nid:%d", idx, req.GetRid(), nid)
		ctx.sendData(comm.CMD_ROOM_CHAT, head.GetSid(), 0, uint32(nid),
			head.GetSeq(), data[comm.MESG_HEAD_SIZE:], head.GetLength())
	}
}

func (ctx *ChatRoomCntx) taskRoomBroadcastPop() {
	for item := range ctx.room_broadcast_chan {
		ctx.roomBroadcastFanOut(item)
	}
}
