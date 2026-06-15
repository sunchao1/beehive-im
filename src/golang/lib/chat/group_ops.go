package chat

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/garyburd/redigo/redis"

	"beehive-im/lib/comm"
)

const (
	GROUP_ROLE_MEMBER = 3 // 普通成员
)

var (
	ErrGroupNotFound    = errors.New("group not found")
	ErrGroupNotMember   = errors.New("not a group member")
	ErrGroupNoPerm      = errors.New("permission denied")
	ErrGroupBlacklisted = errors.New("user is blacklisted")
	ErrGroupGagged      = errors.New("user is gagged")
	ErrGroupExists      = errors.New("already in group")
)

func groupRedisUIDs(reply interface{}, err error) ([]uint64, error) {
	strs, err := redis.Strings(reply, err)
	if err != nil {
		return nil, err
	}
	out := make([]uint64, 0, len(strs))
	for _, s := range strs {
		u, _ := strconv.ParseUint(s, 10, 64)
		if u > 0 {
			out = append(out, u)
		}
	}
	return out, nil
}

// GroupExists 判断群组是否已注册（chat:gid:zset）。
func GroupExists(pool *redis.Pool, gid uint64) (bool, error) {
	rds := pool.Get()
	defer rds.Close()

	score, err := redis.Int64(rds.Do("ZSCORE", comm.CHAT_KEY_GID_ZSET, gid))
	if err == redis.ErrNil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return score > time.Now().Unix(), nil
}

// GroupGetRole 返回成员角色；非成员返回 0, false。
func GroupGetRole(pool *redis.Pool, gid, uid uint64) (int, bool, error) {
	rds := pool.Get()
	defer rds.Close()

	key := fmt.Sprintf(comm.CHAT_KEY_GROUP_ROLE_TAB, gid)
	role, err := redis.Int(rds.Do("HGET", key, uid))
	if err == redis.ErrNil {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return role, true, nil
}

// GroupRegister 创建群：分配 gid 后写入 Redis（调用方已 INCR）。
func GroupRegister(pool *redis.Pool, gid, ownerUID uint64, name, desc string) error {
	rds := pool.Get()
	defer rds.Close()

	pl := pool.Get()
	defer func() {
		_, _ = pl.Do("")
		pl.Close()
	}()

	ttl := time.Now().Unix() + comm.CHAT_SID_TTL

	pl.Send("ZADD", comm.CHAT_KEY_GID_ZSET, ttl, gid)
	pl.Send("ZADD", comm.CHAT_KEY_GROUP_CAP_ZSET, 500, gid)

	key := fmt.Sprintf(comm.CHAT_KEY_GROUP_ROLE_TAB, gid)
	pl.Send("HSET", key, ownerUID, GROUP_ROLE_OWNER)

	key = fmt.Sprintf(comm.CHAT_KEY_GROUP_INFO_TAB, gid)
	pl.Send("HMSET", key, "NAME", name, "DESC", desc, "OWNER", ownerUID)

	key = fmt.Sprintf(comm.CHAT_KEY_UID_TO_GID, ownerUID)
	pl.Send("HSET", key, gid, 0)

	attrKey := fmt.Sprintf(comm.CHAT_KEY_GID_ATTR, gid)
	pl.Send("HMSET", attrKey, comm.CHAT_GID_ATTR_SWITCH, GROUP_STAT_OPEN)

	_, err := pl.Do("")
	return err
}

// GroupJoinOnline 成员加入群并登记在线路由（需已 ONLINE）。
func GroupJoinOnline(pool *redis.Pool, gid, uid, sid uint64, nid uint32) error {
	ok, err := GroupExists(pool, gid)
	if err != nil {
		return err
	}
	if !ok {
		return ErrGroupNotFound
	}

	blKey := fmt.Sprintf(comm.CHAT_KEY_GROUP_USR_BLACKLIST_SET, gid)
	rds := pool.Get()
	defer rds.Close()

	inBL, err := redis.Bool(rds.Do("SISMEMBER", blKey, uid))
	if err != nil {
		return err
	}
	if inBL {
		return ErrGroupBlacklisted
	}

	roleKey := fmt.Sprintf(comm.CHAT_KEY_GROUP_ROLE_TAB, gid)
	_, member, err := GroupGetRole(pool, gid, uid)
	if err != nil {
		return err
	}

	pl := pool.Get()
	defer func() {
		_, _ = pl.Do("")
		pl.Close()
	}()

	if !member {
		pl.Send("HSET", roleKey, uid, GROUP_ROLE_MEMBER)
		key := fmt.Sprintf(comm.CHAT_KEY_UID_TO_GID, uid)
		pl.Send("HSET", key, gid, 0)
	}

	ttl := time.Now().Unix() + comm.CHAT_SID_TTL
	memberUID := fmt.Sprintf(comm.CHAT_FMT_UID_SID_STR, uid, sid)

	key := fmt.Sprintf(comm.CHAT_KEY_GID_TO_UID_ZSET, gid)
	pl.Send("ZADD", key, ttl, uid)

	key = fmt.Sprintf(comm.CHAT_KEY_GID_TO_SID_ZSET, gid)
	pl.Send("ZADD", key, ttl, sid)

	key = fmt.Sprintf(comm.CHAT_KEY_GID_TO_NID_ZSET, gid)
	pl.Send("ZADD", key, ttl, nid)

	_ = memberUID
	_, err = pl.Do("")
	return err
}

// GroupAddMember 邀请/离线加人（仅成员表，不含在线路由）。
func GroupAddMember(pool *redis.Pool, gid, uid uint64) error {
	ok, err := GroupExists(pool, gid)
	if err != nil {
		return err
	}
	if !ok {
		return ErrGroupNotFound
	}

	blKey := fmt.Sprintf(comm.CHAT_KEY_GROUP_USR_BLACKLIST_SET, gid)
	rds := pool.Get()
	defer rds.Close()

	inBL, err := redis.Bool(rds.Do("SISMEMBER", blKey, uid))
	if err != nil {
		return err
	}
	if inBL {
		return ErrGroupBlacklisted
	}

	if _, ok, err := GroupGetRole(pool, gid, uid); err != nil {
		return err
	} else if ok {
		return ErrGroupExists
	}

	pl := pool.Get()
	defer func() {
		_, _ = pl.Do("")
		pl.Close()
	}()

	key := fmt.Sprintf(comm.CHAT_KEY_GROUP_ROLE_TAB, gid)
	pl.Send("HSET", key, uid, GROUP_ROLE_MEMBER)
	key = fmt.Sprintf(comm.CHAT_KEY_UID_TO_GID, uid)
	pl.Send("HSET", key, gid, 0)

	_, err = pl.Do("")
	return err
}

// GroupQuit 退群：清理在线路由；若无其他在线会话可保留成员身份（仅清 zset）。
func GroupQuit(pool *redis.Pool, gid, uid, sid uint64) error {
	pl := pool.Get()
	defer func() {
		_, _ = pl.Do("")
		pl.Close()
	}()

	key := fmt.Sprintf(comm.CHAT_KEY_GID_TO_SID_ZSET, gid)
	pl.Send("ZREM", key, sid)

	key = fmt.Sprintf(comm.CHAT_KEY_GID_TO_UID_ZSET, gid)
	pl.Send("ZREM", key, uid)

	key = fmt.Sprintf(comm.CHAT_KEY_GROUP_ROLE_TAB, gid)
	pl.Send("HDEL", key, uid)

	key = fmt.Sprintf(comm.CHAT_KEY_UID_TO_GID, uid)
	pl.Send("HDEL", key, gid)

	_, err := pl.Do("")
	return err
}

// GroupKick 踢人（需管理员权限，由调用方校验）。
func GroupKick(pool *redis.Pool, gid, uid uint64) error {
	pl := pool.Get()
	defer func() {
		_, _ = pl.Do("")
		pl.Close()
	}()

	key := fmt.Sprintf(comm.CHAT_KEY_GROUP_ROLE_TAB, gid)
	pl.Send("HDEL", key, uid)

	key = fmt.Sprintf(comm.CHAT_KEY_GID_TO_UID_ZSET, gid)
	pl.Send("ZREM", key, uid)

	key = fmt.Sprintf(comm.CHAT_KEY_UID_TO_GID, uid)
	pl.Send("HDEL", key, gid)

	gagKey := fmt.Sprintf(comm.CHAT_KEY_GROUP_USR_GAG_SET, gid)
	pl.Send("SREM", gagKey, uid)

	_, err := pl.Do("")
	return err
}

// GroupDismiss 解散群（仅 owner，由调用方校验）。
func GroupDismiss(pool *redis.Pool, gid uint64) error {
	rds := pool.Get()
	defer rds.Close()

	roleKey := fmt.Sprintf(comm.CHAT_KEY_GROUP_ROLE_TAB, gid)
	uids, err := groupRedisUIDs(rds.Do("HKEYS", roleKey))
	if err != nil && err != redis.ErrNil {
		return err
	}

	pl := pool.Get()
	defer func() {
		_, _ = pl.Do("")
		pl.Close()
	}()

	for _, uid := range uids {
		key := fmt.Sprintf(comm.CHAT_KEY_UID_TO_GID, uid)
		pl.Send("HDEL", key, gid)
	}

	keys := []string{
		fmt.Sprintf(comm.CHAT_KEY_GROUP_ROLE_TAB, gid),
		fmt.Sprintf(comm.CHAT_KEY_GROUP_INFO_TAB, gid),
		fmt.Sprintf(comm.CHAT_KEY_GID_TO_NID_ZSET, gid),
		fmt.Sprintf(comm.CHAT_KEY_GID_TO_UID_ZSET, gid),
		fmt.Sprintf(comm.CHAT_KEY_GID_TO_SID_ZSET, gid),
		fmt.Sprintf(comm.CHAT_KEY_GROUP_MESG_QUEUE, gid),
		fmt.Sprintf(comm.CHAT_KEY_GROUP_MSGID_INCR, gid),
		fmt.Sprintf(comm.CHAT_KEY_GROUP_USR_GAG_SET, gid),
		fmt.Sprintf(comm.CHAT_KEY_GROUP_USR_BLACKLIST_SET, gid),
		fmt.Sprintf(comm.CHAT_KEY_GID_ATTR, gid),
	}
	for _, k := range keys {
		pl.Send("DEL", k)
	}
	pl.Send("ZREM", comm.CHAT_KEY_GID_ZSET, gid)
	pl.Send("ZREM", comm.CHAT_KEY_GROUP_CAP_ZSET, gid)

	_, err = pl.Do("")
	return err
}

// GroupListMembers 返回成员 UID 列表。
func GroupListMembers(pool *redis.Pool, gid uint64) ([]uint64, error) {
	rds := pool.Get()
	defer rds.Close()

	key := fmt.Sprintf(comm.CHAT_KEY_GROUP_ROLE_TAB, gid)
	return groupRedisUIDs(rds.Do("HKEYS", key))
}

// GroupIsGagged 是否被禁言。
func GroupIsGagged(pool *redis.Pool, gid, uid uint64) (bool, error) {
	rds := pool.Get()
	defer rds.Close()

	key := fmt.Sprintf(comm.CHAT_KEY_GROUP_USR_GAG_SET, gid)
	return redis.Bool(rds.Do("SISMEMBER", key, uid))
}

// GroupSetManager 设置/取消管理员（不含 owner）。
func GroupSetManager(pool *redis.Pool, gid, uid uint64, manager bool) error {
	role, ok, err := GroupGetRole(pool, gid, uid)
	if err != nil {
		return err
	}
	if !ok {
		return ErrGroupNotMember
	}
	if role == GROUP_ROLE_OWNER {
		return ErrGroupNoPerm
	}

	rds := pool.Get()
	defer rds.Close()

	key := fmt.Sprintf(comm.CHAT_KEY_GROUP_ROLE_TAB, gid)
	if manager {
		_, err = rds.Do("HSET", key, uid, GROUP_ROLE_MANAGER)
	} else if role == GROUP_ROLE_MANAGER {
		_, err = rds.Do("HSET", key, uid, GROUP_ROLE_MEMBER)
	}
	return err
}

// GroupSetGag 禁言/解除禁言。
func GroupSetGag(pool *redis.Pool, gid, uid uint64, gag bool) error {
	rds := pool.Get()
	defer rds.Close()

	key := fmt.Sprintf(comm.CHAT_KEY_GROUP_USR_GAG_SET, gid)
	if gag {
		_, err := rds.Do("SADD", key, uid)
		return err
	}
	_, err := rds.Do("SREM", key, uid)
	return err
}

// GroupSetBlacklist 黑名单。
func GroupSetBlacklist(pool *redis.Pool, gid, uid uint64, add bool) error {
	rds := pool.Get()
	defer rds.Close()

	key := fmt.Sprintf(comm.CHAT_KEY_GROUP_USR_BLACKLIST_SET, gid)
	if add {
		if err := GroupKick(pool, gid, uid); err != nil {
			return err
		}
		_, err := rds.Do("SADD", key, uid)
		return err
	}
	_, err := rds.Do("SREM", key, uid)
	return err
}

// GroupParseMemberListJSON 简单 JSON 列表（面试/demo 用）。
func GroupParseMemberListJSON(uids []uint64) string {
	if len(uids) == 0 {
		return "[]"
	}
	s := "["
	for i, uid := range uids {
		if i > 0 {
			s += ","
		}
		s += strconv.FormatUint(uid, 10)
	}
	return s + "]"
}
