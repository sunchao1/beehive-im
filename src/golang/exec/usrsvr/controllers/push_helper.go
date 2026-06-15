package controllers

import (
	"beehive-im/lib/comm"
	"beehive-im/lib/push"
)

func (ctx *UsrSvrCntx) pushDeliver() *push.Deliver {
	return &push.Deliver{
		Pool: ctx.redis,
		SendFn: func(cmd uint32, data []byte, length uint32) int {
			return ctx.frwder.AsyncSend(cmd, data, length)
		},
	}
}

func (ctx *UsrSvrCntx) pushBody(cmd uint32, sid, uid uint64, body []byte) (int, error) {
	d := ctx.pushDeliver()
	if sid != 0 {
		return 1, d.ToSid(cmd, sid, body)
	}
	return d.ToUid(cmd, uid, body)
}

func pushCmdFromParam(kind string) uint32 {
	if kind == "p2p" {
		return comm.CMD_P2P
	}
	return comm.CMD_BC
}
