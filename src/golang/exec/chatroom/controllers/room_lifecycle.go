package controllers

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/golang/protobuf/proto"

	"beehive-im/lib/comm"
	"beehive-im/lib/im"
	"beehive-im/lib/mesg"

	"beehive-im/exec/chatroom/models"
)

const roomChatTextMaxLen = 4096

func (ctx *ChatRoomCntx) validateRoomJoin(head *comm.MesgHeader, req *mesg.MesgRoomJoin) (uint32, error) {
	if head == nil || req == nil {
		return comm.ERR_SVR_JOIN_REQ, errors.New("Invalid join request")
	}
	if req.GetUid() == 0 || req.GetRid() == 0 {
		return comm.ERR_SVR_JOIN_REQ, errors.New("Invalid uid or rid")
	}

	attr, err := im.GetSidAttr(ctx.cache.Pool(), head.GetSid())
	if err != nil || attr.GetUid() == 0 {
		return comm.ERR_SVR_AUTH_FAIL, errors.New("User not online")
	}
	if attr.GetUid() != req.GetUid() {
		return comm.ERR_SVR_AUTH_FAIL, errors.New("Uid mismatch sid")
	}
	if attr.GetNid() != head.GetNid() {
		return comm.ERR_SVR_DATA_COLLISION, errors.New("Session collision")
	}

	open, err := ctx.userdb.RoomIsOpen(req.GetRid())
	if err == sql.ErrNoRows {
		return comm.ERR_SVR_JOIN_REQ, errors.New("Room not exists")
	}
	if err != nil {
		return comm.ERR_SYS_DB, err
	}
	if !open {
		return comm.ERR_SVR_JOIN_REQ, errors.New("Room closed")
	}

	rds := ctx.cache.Get()
	defer rds.Close()
	key := fmt.Sprintf(models.ROOM_KEY_SID_TO_RID_ZSET, head.GetSid())
	score, err := redis.Int64(rds.Do("ZSCORE", key, req.GetRid()))
	if err == nil && score > time.Now().Unix() {
		return comm.ERR_SVR_DATA_COLLISION, errors.New("Already in room")
	}

	return 0, nil
}

func (ctx *ChatRoomCntx) validateRoomChat(head *comm.MesgHeader, req *mesg.MesgRoomChat) (uint32, string) {
	if head == nil || req == nil {
		return comm.ERR_SVR_BODY_INVALID, "Invalid chat request"
	}
	if req.GetUid() == 0 || req.GetRid() == 0 {
		return comm.ERR_SVR_BODY_INVALID, "Invalid uid or rid"
	}
	text := req.GetText()
	if len(text) == 0 {
		return comm.ERR_SVR_BODY_INVALID, "Empty message"
	}
	if len(text) > roomChatTextMaxLen {
		return comm.ERR_SYS_BODY_OVER_LIMIT, "Message too long"
	}

	attr, err := im.GetSidAttr(ctx.cache.Pool(), head.GetSid())
	if err != nil || attr.GetUid() == 0 {
		return comm.ERR_SVR_AUTH_FAIL, "User not online"
	}
	if attr.GetUid() != req.GetUid() {
		return comm.ERR_SVR_AUTH_FAIL, "Uid mismatch sid"
	}

	inRoom, err := ctx.cache.RoomSidInRoom(req.GetRid(), head.GetSid())
	if err != nil {
		return comm.ERR_SYS_SYSTEM, err.Error()
	}
	if !inRoom {
		return comm.ERR_SVR_CHECK_FAIL, "Not in room"
	}

	open, err := ctx.userdb.RoomIsOpen(req.GetRid())
	if err == sql.ErrNoRows {
		return comm.ERR_SVR_JOIN_REQ, "Room not exists"
	}
	if err != nil {
		return comm.ERR_SYS_DB, err.Error()
	}
	if !open {
		return comm.ERR_SVR_JOIN_REQ, "Room closed"
	}

	return 0, ""
}

func (ctx *ChatRoomCntx) parseRoomDismissReq(data []byte) (
	head *comm.MesgHeader, req *mesg.MesgRoomDismiss, code uint32, err error) {
	head = comm.MesgHeadNtoh(data)
	if !head.IsValid(1) {
		return nil, nil, comm.ERR_SVR_HEAD_INVALID, errors.New("Header invalid")
	}
	req = &mesg.MesgRoomDismiss{}
	if err = proto.Unmarshal(data[comm.MESG_HEAD_SIZE:], req); err != nil {
		return head, nil, comm.ERR_SVR_BODY_INVALID, err
	}
	if req.GetUid() == 0 || req.GetRid() == 0 {
		return head, req, comm.ERR_SVR_INVALID_PARAM, errors.New("Invalid uid or rid")
	}
	return head, req, 0, nil
}

func (ctx *ChatRoomCntx) roomDismissAck(head *comm.MesgHeader, code uint32, errmsg string) {
	ack := &mesg.MesgRoomDismissAck{
		Code:   proto.Uint32(code),
		Errmsg: proto.String(errmsg),
	}
	body, err := proto.Marshal(ack)
	if err != nil {
		ctx.log.Error("Marshal dismiss ack failed! %s", err.Error())
		return
	}
	_ = ctx.sendData(comm.CMD_ROOM_DISMISS_ACK, head.GetSid(), head.GetCid(),
		head.GetNid(), head.GetSeq(), body, uint32(len(body)))
}

func (ctx *ChatRoomCntx) roomDismissHandler(head *comm.MesgHeader, req *mesg.MesgRoomDismiss) (uint32, error) {
	attr, err := im.GetSidAttr(ctx.cache.Pool(), head.GetSid())
	if err != nil || attr.GetUid() == 0 {
		return comm.ERR_SVR_AUTH_FAIL, errors.New("User not online")
	}
	if attr.GetUid() != req.GetUid() {
		return comm.ERR_SVR_AUTH_FAIL, errors.New("Uid mismatch sid")
	}

	owner, err := ctx.userdb.RoomIsOwner(req.GetRid(), req.GetUid())
	if err == sql.ErrNoRows {
		return comm.ERR_SVR_JOIN_REQ, errors.New("Room not exists")
	}
	if err != nil {
		return comm.ERR_SYS_DB, err
	}
	if !owner {
		role, rerr := ctx.cacheRoomRole(req.GetRid(), req.GetUid())
		if rerr != nil || role != models.ROOM_ROLE_OWNER {
			return comm.ERR_SYS_PERM_DENIED, errors.New("Permission denied")
		}
	}

	if err := ctx.userdb.RoomDismiss(req.GetRid()); err != nil {
		return comm.ERR_SYS_DB, err
	}
	if err := ctx.cache.RoomDismissRedis(req.GetRid()); err != nil {
		return comm.ERR_SYS_SYSTEM, err
	}
	return 0, nil
}

func (ctx *ChatRoomCntx) cacheRoomRole(rid, uid uint64) (int, error) {
	rds := ctx.cache.Get()
	defer rds.Close()
	key := fmt.Sprintf(models.ROOM_KEY_ROOM_ROLE_TAB, rid)
	return redis.Int(rds.Do("HGET", key, uid))
}

// DismissRoomByRid HTTP 关闭房间时复用。
func (ctx *ChatRoomCntx) DismissRoomByRid(rid uint64) error {
	if err := ctx.userdb.RoomDismiss(rid); err != nil {
		return err
	}
	return ctx.cache.RoomDismissRedis(rid)
}
