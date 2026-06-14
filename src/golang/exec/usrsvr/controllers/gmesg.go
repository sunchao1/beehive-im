package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/garyburd/redigo/redis"
	"github.com/golang/protobuf/proto"

	"beehive-im/lib/chat"
	"beehive-im/lib/comm"
	"beehive-im/lib/im"
	"beehive-im/lib/mesg"
)

// 群聊 0x03xx — 用户/群生命周期由 usrsvr 处理；GROUP-CHAT 由 msgsvr 处理。

func (ctx *UsrSvrCntx) groupParseHead(data []byte) (*comm.MesgHeader, uint32, error) {
	head := comm.MesgHeadNtoh(data)
	if !head.IsValid(1) {
		return nil, comm.ERR_SVR_HEAD_INVALID, errors.New("header invalid")
	}
	return head, 0, nil
}

func (ctx *UsrSvrCntx) groupOperatorUID(head *comm.MesgHeader) (uint64, error) {
	attr, err := im.GetSidAttr(ctx.redis, head.GetSid())
	if err != nil {
		return 0, err
	}
	if attr.GetUid() == 0 {
		return 0, errors.New("sid not online")
	}
	return attr.GetUid(), nil
}

func (ctx *UsrSvrCntx) groupRequireManager(gid, operatorUID uint64) error {
	role, ok, err := chat.GroupGetRole(ctx.redis, gid, operatorUID)
	if err != nil {
		return err
	}
	if !ok || !chatGroupIsManager(role) {
		return chat.ErrGroupNoPerm
	}
	return nil
}

func chatGroupIsManager(role int) bool {
	return role == chat.GROUP_ROLE_OWNER || role == chat.GROUP_ROLE_MANAGER
}

////////////////////////////////////////////////////////////////////////////////
// GROUP-CREAT

func (ctx *UsrSvrCntx) groupCreatParse(data []byte) (
	head *comm.MesgHeader, req *mesg.MesgGroupCreat, code uint32, err error) {
	head, code, err = ctx.groupParseHead(data)
	if err != nil {
		return nil, nil, code, err
	}
	req = &mesg.MesgGroupCreat{}
	if err = proto.Unmarshal(data[comm.MESG_HEAD_SIZE:], req); err != nil {
		return head, nil, comm.ERR_SVR_BODY_INVALID, err
	}
	if req.GetUid() == 0 || req.GetName() == "" {
		return head, req, comm.ERR_SVR_INVALID_PARAM, errors.New("uid or name invalid")
	}
	return head, req, 0, nil
}

func (ctx *UsrSvrCntx) allocGid() (uint64, error) {
	rds := ctx.redis.Get()
	defer rds.Close()
	gidStr, err := redis.String(rds.Do("INCR", comm.CHAT_KEY_GID_INCR))
	if err != nil {
		return 0, err
	}
	gid, _ := strconv.ParseUint(gidStr, 10, 64)
	return gid, nil
}

func UsrSvrGroupCreatHandler(cmd uint32, nid uint32, data []byte, length uint32, param interface{}) int {
	ctx, ok := param.(*UsrSvrCntx)
	if !ok {
		return -1
	}

	head, req, code, err := ctx.groupCreatParse(data)
	if err != nil {
		ctx.groupSendSimpleAck(comm.CMD_GROUP_CREAT_ACK, head, code, err.Error())
		return -1
	}

	gid, err := ctx.allocGid()
	if err != nil {
		ctx.groupSendSimpleAck(comm.CMD_GROUP_CREAT_ACK, head, comm.ERR_SYS_SYSTEM, err.Error())
		return -1
	}

	if err = chat.GroupRegister(ctx.redis, gid, req.GetUid(), req.GetName(), req.GetDesc()); err != nil {
		ctx.groupSendSimpleAck(comm.CMD_GROUP_CREAT_ACK, head, comm.ERR_SYS_SYSTEM, err.Error())
		return -1
	}

	if err = chat.GroupJoinOnline(ctx.redis, gid, req.GetUid(), head.GetSid(), head.GetNid()); err != nil {
		ctx.log.Error("Auto join after creat failed! gid:%d %s", gid, err.Error())
	}

	// ACK 无 gid 字段：errmsg 携带 gid 供 demo/smoke 解析
	ctx.groupSendSimpleAck(comm.CMD_GROUP_CREAT_ACK, head, 0, fmt.Sprintf("Ok:%d", gid))
	return 0
}

////////////////////////////////////////////////////////////////////////////////
// GROUP-DISMISS

func UsrSvrGroupDismissHandler(cmd uint32, nid uint32, data []byte, length uint32, param interface{}) int {
	ctx, ok := param.(*UsrSvrCntx)
	if !ok {
		return -1
	}
	head, code, err := ctx.groupParseHead(data)
	if err != nil {
		ctx.groupSendSimpleAck(comm.CMD_GROUP_DISMISS_ACK, head, code, err.Error())
		return -1
	}
	req := &mesg.MesgGroupDismiss{}
	if err = proto.Unmarshal(data[comm.MESG_HEAD_SIZE:], req); err != nil {
		ctx.groupSendSimpleAck(comm.CMD_GROUP_DISMISS_ACK, head, comm.ERR_SVR_BODY_INVALID, err.Error())
		return -1
	}
	role, ok, err := chat.GroupGetRole(ctx.redis, req.GetGid(), req.GetUid())
	if err != nil {
		ctx.groupSendSimpleAck(comm.CMD_GROUP_DISMISS_ACK, head, comm.ERR_SYS_SYSTEM, err.Error())
		return -1
	}
	if !ok || role != chat.GROUP_ROLE_OWNER {
		ctx.groupSendSimpleAck(comm.CMD_GROUP_DISMISS_ACK, head, comm.ERR_SVR_AUTH_FAIL, "owner only")
		return -1
	}
	if err = chat.GroupDismiss(ctx.redis, req.GetGid()); err != nil {
		ctx.groupSendSimpleAck(comm.CMD_GROUP_DISMISS_ACK, head, comm.ERR_SYS_SYSTEM, err.Error())
		return -1
	}
	ctx.groupSendSimpleAck(comm.CMD_GROUP_DISMISS_ACK, head, 0, "Ok")
	return 0
}

////////////////////////////////////////////////////////////////////////////////
// GROUP-JOIN

func UsrSvrGroupJoinHandler(cmd uint32, nid uint32, data []byte, length uint32, param interface{}) int {
	ctx, ok := param.(*UsrSvrCntx)
	if !ok {
		return -1
	}
	head, code, err := ctx.groupParseHead(data)
	if err != nil {
		ctx.groupSendSimpleAck(comm.CMD_GROUP_JOIN_ACK, head, code, err.Error())
		return -1
	}
	req := &mesg.MesgGroupJoin{}
	if err = proto.Unmarshal(data[comm.MESG_HEAD_SIZE:], req); err != nil {
		ctx.groupSendSimpleAck(comm.CMD_GROUP_JOIN_ACK, head, comm.ERR_SVR_BODY_INVALID, err.Error())
		return -1
	}
	if err = chat.GroupJoinOnline(ctx.redis, req.GetGid(), req.GetUid(), head.GetSid(), head.GetNid()); err != nil {
		errCode := uint32(comm.ERR_SVR_CHECK_FAIL)
		if errors.Is(err, chat.ErrGroupNotFound) {
			errCode = comm.ERR_SVR_INVALID_PARAM
		}
		ctx.groupSendSimpleAck(comm.CMD_GROUP_JOIN_ACK, head, errCode, err.Error())
		return -1
	}
	ctx.groupSendSimpleAck(comm.CMD_GROUP_JOIN_ACK, head, 0, "Ok")
	ntf, _ := proto.Marshal(&mesg.MesgGroupJoinNtf{
		Uid: proto.Uint64(req.GetUid()),
		Gid: proto.Uint64(req.GetGid()),
	})
	ctx.groupBroadcast(comm.CMD_GROUP_JOIN_NTF, req.GetGid(), ntf)
	return 0
}

////////////////////////////////////////////////////////////////////////////////
// GROUP-QUIT

func UsrSvrGroupQuitHandler(cmd uint32, nid uint32, data []byte, length uint32, param interface{}) int {
	ctx, ok := param.(*UsrSvrCntx)
	if !ok {
		return -1
	}
	head, code, err := ctx.groupParseHead(data)
	if err != nil {
		ctx.groupSendSimpleAck(comm.CMD_GROUP_QUIT_ACK, head, code, err.Error())
		return -1
	}
	req := &mesg.MesgGroupQuit{}
	if err = proto.Unmarshal(data[comm.MESG_HEAD_SIZE:], req); err != nil {
		ctx.groupSendSimpleAck(comm.CMD_GROUP_QUIT_ACK, head, comm.ERR_SVR_BODY_INVALID, err.Error())
		return -1
	}
	if err = chat.GroupQuit(ctx.redis, req.GetGid(), req.GetUid(), head.GetSid()); err != nil {
		ctx.groupSendSimpleAck(comm.CMD_GROUP_QUIT_ACK, head, comm.ERR_SYS_SYSTEM, err.Error())
		return -1
	}
	ctx.groupSendSimpleAck(comm.CMD_GROUP_QUIT_ACK, head, 0, "Ok")
	ntf, _ := proto.Marshal(&mesg.MesgGroupQuitNtf{
		Uid: proto.Uint64(req.GetUid()),
		Gid: proto.Uint64(req.GetGid()),
	})
	ctx.groupBroadcast(comm.CMD_GROUP_QUIT_NTF, req.GetGid(), ntf)
	return 0
}

////////////////////////////////////////////////////////////////////////////////
// GROUP-INVITE

func UsrSvrGroupInviteHandler(cmd uint32, nid uint32, data []byte, length uint32, param interface{}) int {
	ctx, ok := param.(*UsrSvrCntx)
	if !ok {
		return -1
	}
	head, code, err := ctx.groupParseHead(data)
	if err != nil {
		ctx.groupSendSimpleAck(comm.CMD_GROUP_INVITE_ACK, head, code, err.Error())
		return -1
	}
	req := &mesg.MesgGroupInvite{}
	if err = proto.Unmarshal(data[comm.MESG_HEAD_SIZE:], req); err != nil {
		ctx.groupSendSimpleAck(comm.CMD_GROUP_INVITE_ACK, head, comm.ERR_SVR_BODY_INVALID, err.Error())
		return -1
	}
	if err = ctx.groupRequireManager(req.GetGid(), req.GetUid()); err != nil {
		ctx.groupSendSimpleAck(comm.CMD_GROUP_INVITE_ACK, head, comm.ERR_SVR_AUTH_FAIL, err.Error())
		return -1
	}
	if err = chat.GroupAddMember(ctx.redis, req.GetGid(), req.GetTo()); err != nil {
		errCode := uint32(comm.ERR_SVR_CHECK_FAIL)
		if errors.Is(err, chat.ErrGroupNotFound) {
			errCode = comm.ERR_SVR_INVALID_PARAM
		}
		ctx.groupSendSimpleAck(comm.CMD_GROUP_INVITE_ACK, head, errCode, err.Error())
		return -1
	}
	ctx.groupSendSimpleAck(comm.CMD_GROUP_INVITE_ACK, head, 0, "Ok")
	ntf, _ := proto.Marshal(&mesg.MesgGroupJoinNtf{
		Uid: proto.Uint64(req.GetTo()),
		Gid: proto.Uint64(req.GetGid()),
	})
	ctx.groupBroadcast(comm.CMD_GROUP_JOIN_NTF, req.GetGid(), ntf)
	return 0
}

////////////////////////////////////////////////////////////////////////////////
// GROUP-KICK

func UsrSvrGroupKickHandler(cmd uint32, nid uint32, data []byte, length uint32, param interface{}) int {
	ctx, ok := param.(*UsrSvrCntx)
	if !ok {
		return -1
	}
	head, code, err := ctx.groupParseHead(data)
	if err != nil {
		ctx.groupSendSimpleAck(comm.CMD_GROUP_KICK_ACK, head, code, err.Error())
		return -1
	}
	req := &mesg.MesgGroupKick{}
	if err = proto.Unmarshal(data[comm.MESG_HEAD_SIZE:], req); err != nil {
		ctx.groupSendSimpleAck(comm.CMD_GROUP_KICK_ACK, head, comm.ERR_SVR_BODY_INVALID, err.Error())
		return -1
	}
	operator, err := ctx.groupOperatorUID(head)
	if err != nil {
		ctx.groupSendSimpleAck(comm.CMD_GROUP_KICK_ACK, head, comm.ERR_SVR_AUTH_FAIL, err.Error())
		return -1
	}
	if err = ctx.groupRequireManager(req.GetGid(), operator); err != nil {
		ctx.groupSendSimpleAck(comm.CMD_GROUP_KICK_ACK, head, comm.ERR_SVR_AUTH_FAIL, err.Error())
		return -1
	}
	target := req.GetUid()
	if err = chat.GroupKick(ctx.redis, req.GetGid(), target); err != nil {
		ctx.groupSendSimpleAck(comm.CMD_GROUP_KICK_ACK, head, comm.ERR_SYS_SYSTEM, err.Error())
		return -1
	}
	ctx.groupSendSimpleAck(comm.CMD_GROUP_KICK_ACK, head, 0, "Ok")
	ntf, _ := proto.Marshal(&mesg.MesgGroupKickNtf{
		Uid: proto.Uint64(target),
		Gid: proto.Uint64(req.GetGid()),
	})
	ctx.groupBroadcast(comm.CMD_GROUP_KICK_NTF, req.GetGid(), ntf)
	return 0
}

////////////////////////////////////////////////////////////////////////////////
// 禁言 / 黑名单 / 管理员

func UsrSvrGroupGagAddHandler(cmd uint32, nid uint32, data []byte, length uint32, param interface{}) int {
	return ctxGroupGag(param, data, true)
}

func UsrSvrGroupGagDelHandler(cmd uint32, nid uint32, data []byte, length uint32, param interface{}) int {
	return ctxGroupGag(param, data, false)
}

func ctxGroupGag(param interface{}, data []byte, gag bool) int {
	ctx, ok := param.(*UsrSvrCntx)
	if !ok {
		return -1
	}
	head, code, err := ctx.groupParseHead(data)
	ackCmd := comm.CMD_GROUP_GAG_ADD_ACK
	if !gag {
		ackCmd = comm.CMD_GROUP_GAG_DEL_ACK
	}
	if err != nil {
		ctx.groupSendSimpleAck(ackCmd, head, code, err.Error())
		return -1
	}
	var gid, targetUID uint64
	if gag {
		req := &mesg.MesgGroupGagAdd{}
		if err = proto.Unmarshal(data[comm.MESG_HEAD_SIZE:], req); err != nil {
			ctx.groupSendSimpleAck(ackCmd, head, comm.ERR_SVR_BODY_INVALID, err.Error())
			return -1
		}
		gid, targetUID = req.GetGid(), req.GetUid()
	} else {
		req := &mesg.MesgGroupGagDel{}
		if err = proto.Unmarshal(data[comm.MESG_HEAD_SIZE:], req); err != nil {
			ctx.groupSendSimpleAck(ackCmd, head, comm.ERR_SVR_BODY_INVALID, err.Error())
			return -1
		}
		gid, targetUID = req.GetGid(), req.GetUid()
	}
	operator, err := ctx.groupOperatorUID(head)
	if err != nil || ctx.groupRequireManager(gid, operator) != nil {
		ctx.groupSendSimpleAck(ackCmd, head, comm.ERR_SVR_AUTH_FAIL, "permission denied")
		return -1
	}
	if err = chat.GroupSetGag(ctx.redis, gid, targetUID, gag); err != nil {
		ctx.groupSendSimpleAck(ackCmd, head, comm.ERR_SYS_SYSTEM, err.Error())
		return -1
	}
	ctx.groupSendSimpleAck(ackCmd, head, 0, "Ok")
	if gag {
		body, _ := proto.Marshal(&mesg.MesgGroupGagAddNtf{Uid: proto.Uint64(targetUID), Gid: proto.Uint64(gid)})
		ctx.groupBroadcast(comm.CMD_GROUP_GAG_ADD_NTF, gid, body)
	} else {
		body, _ := proto.Marshal(&mesg.MesgGroupGagDelNtf{Uid: proto.Uint64(targetUID), Gid: proto.Uint64(gid)})
		ctx.groupBroadcast(comm.CMD_GROUP_GAG_DEL_NTF, gid, body)
	}
	return 0
}

func UsrSvrGroupBlacklistAddHandler(cmd uint32, nid uint32, data []byte, length uint32, param interface{}) int {
	return ctxGroupBL(param, data, true)
}

func UsrSvrGroupBlacklistDelHandler(cmd uint32, nid uint32, data []byte, length uint32, param interface{}) int {
	return ctxGroupBL(param, data, false)
}

func ctxGroupBL(param interface{}, data []byte, add bool) int {
	ctx, ok := param.(*UsrSvrCntx)
	if !ok {
		return -1
	}
	head, code, err := ctx.groupParseHead(data)
	ackCmd := comm.CMD_GROUP_BL_ADD_ACK
	if !add {
		ackCmd = comm.CMD_GROUP_BL_DEL_ACK
	}
	if err != nil {
		ctx.groupSendSimpleAck(ackCmd, head, code, err.Error())
		return -1
	}
	var gid, targetUID uint64
	if add {
		req := &mesg.MesgGroupBlAdd{}
		if err = proto.Unmarshal(data[comm.MESG_HEAD_SIZE:], req); err != nil {
			ctx.groupSendSimpleAck(ackCmd, head, comm.ERR_SVR_BODY_INVALID, err.Error())
			return -1
		}
		gid, targetUID = req.GetGid(), req.GetUid()
	} else {
		req := &mesg.MesgGroupBlDel{}
		if err = proto.Unmarshal(data[comm.MESG_HEAD_SIZE:], req); err != nil {
			ctx.groupSendSimpleAck(ackCmd, head, comm.ERR_SVR_BODY_INVALID, err.Error())
			return -1
		}
		gid, targetUID = req.GetGid(), req.GetUid()
	}
	operator, err := ctx.groupOperatorUID(head)
	if err != nil || ctx.groupRequireManager(gid, operator) != nil {
		ctx.groupSendSimpleAck(ackCmd, head, comm.ERR_SVR_AUTH_FAIL, "permission denied")
		return -1
	}
	if err = chat.GroupSetBlacklist(ctx.redis, gid, targetUID, add); err != nil {
		ctx.groupSendSimpleAck(ackCmd, head, comm.ERR_SYS_SYSTEM, err.Error())
		return -1
	}
	ctx.groupSendSimpleAck(ackCmd, head, 0, "Ok")
	if add {
		body, _ := proto.Marshal(&mesg.MesgGroupBlAddNtf{Uid: proto.Uint64(targetUID), Gid: proto.Uint64(gid)})
		ctx.groupBroadcast(comm.CMD_GROUP_BL_ADD_NTF, gid, body)
	} else {
		body, _ := proto.Marshal(&mesg.MesgGroupBlDelNtf{Uid: proto.Uint64(targetUID), Gid: proto.Uint64(gid)})
		ctx.groupBroadcast(comm.CMD_GROUP_BL_DEL_NTF, gid, body)
	}
	return 0
}

func UsrSvrGroupMgrAddHandler(cmd uint32, nid uint32, data []byte, length uint32, param interface{}) int {
	return ctxGroupMgr(param, data, true)
}

func UsrSvrGroupMgrDelHandler(cmd uint32, nid uint32, data []byte, length uint32, param interface{}) int {
	return ctxGroupMgr(param, data, false)
}

func ctxGroupMgr(param interface{}, data []byte, add bool) int {
	ctx, ok := param.(*UsrSvrCntx)
	if !ok {
		return -1
	}
	head, code, err := ctx.groupParseHead(data)
	ackCmd := comm.CMD_GROUP_MGR_ADD_ACK
	if !add {
		ackCmd = comm.CMD_GROUP_MGR_DEL_ACK
	}
	if err != nil {
		ctx.groupSendSimpleAck(ackCmd, head, code, err.Error())
		return -1
	}
	req := &mesg.MesgGroupMgrAdd{}
	if add {
		if err = proto.Unmarshal(data[comm.MESG_HEAD_SIZE:], req); err != nil {
			ctx.groupSendSimpleAck(ackCmd, head, comm.ERR_SVR_BODY_INVALID, err.Error())
			return -1
		}
	} else {
		delReq := &mesg.MesgGroupMgrDel{}
		if err = proto.Unmarshal(data[comm.MESG_HEAD_SIZE:], delReq); err != nil {
			ctx.groupSendSimpleAck(ackCmd, head, comm.ERR_SVR_BODY_INVALID, err.Error())
			return -1
		}
		req = &mesg.MesgGroupMgrAdd{Uid: delReq.Uid, Gid: delReq.Gid}
	}
	operator, err := ctx.groupOperatorUID(head)
	if err != nil {
		ctx.groupSendSimpleAck(ackCmd, head, comm.ERR_SVR_AUTH_FAIL, err.Error())
		return -1
	}
	role, ok, _ := chat.GroupGetRole(ctx.redis, req.GetGid(), operator)
	if !ok || role != chat.GROUP_ROLE_OWNER {
		ctx.groupSendSimpleAck(ackCmd, head, comm.ERR_SVR_AUTH_FAIL, "owner only")
		return -1
	}
	if err = chat.GroupSetManager(ctx.redis, req.GetGid(), req.GetUid(), add); err != nil {
		ctx.groupSendSimpleAck(ackCmd, head, comm.ERR_SVR_CHECK_FAIL, err.Error())
		return -1
	}
	ctx.groupSendSimpleAck(ackCmd, head, 0, "Ok")
	if add {
		body, _ := proto.Marshal(&mesg.MesgGroupMgrAddNtf{Uid: proto.Uint64(req.GetUid()), Gid: proto.Uint64(req.GetGid())})
		ctx.groupBroadcast(comm.CMD_GROUP_MGR_ADD_NTF, req.GetGid(), body)
	} else {
		body, _ := proto.Marshal(&mesg.MesgGroupMgrDelNtf{Uid: proto.Uint64(req.GetUid()), Gid: proto.Uint64(req.GetGid())})
		ctx.groupBroadcast(comm.CMD_GROUP_MGR_DEL_NTF, req.GetGid(), body)
	}
	return 0
}

////////////////////////////////////////////////////////////////////////////////
// GROUP-USR-LIST

func UsrSvrGroupUsrListHandler(cmd uint32, nid uint32, data []byte, length uint32, param interface{}) int {
	ctx, ok := param.(*UsrSvrCntx)
	if !ok {
		return -1
	}
	head, _, err := ctx.groupParseHead(data)
	if err != nil {
		return -1
	}
	req := &mesg.MesgGroupUsrList{}
	if err = proto.Unmarshal(data[comm.MESG_HEAD_SIZE:], req); err != nil {
		ctx.groupSendSimpleAck(comm.CMD_GROUP_USR_LIST_ACK, head, comm.ERR_SVR_BODY_INVALID, err.Error())
		return -1
	}
	uids, err := chat.GroupListMembers(ctx.redis, req.GetGid())
	if err != nil {
		ctx.groupSendSimpleAck(comm.CMD_GROUP_USR_LIST_ACK, head, comm.ERR_SYS_SYSTEM, err.Error())
		return -1
	}
	num := int(req.GetNum())
	if num > 0 && num < len(uids) {
		uids = uids[:num]
	}
	listJSON, _ := json.Marshal(uids)
	ack := &mesg.MesgGroupUsrListAck{
		Gid:  proto.Uint64(req.GetGid()),
		List: proto.String(string(listJSON)),
	}
	body, err := proto.Marshal(ack)
	if err != nil {
		return -1
	}
	ctx.groupSendPacket(comm.CMD_GROUP_USR_LIST_ACK, head, body)
	return 0
}
