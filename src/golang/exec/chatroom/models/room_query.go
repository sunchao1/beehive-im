package models

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/garyburd/redigo/redis"
)

// RoomInfo 聊天室元数据（MySQL）。
type RoomInfo struct {
	Rid    uint64
	Name   string
	Status int
	Owner  uint64
}

// RoomGet 读取聊天室信息；不存在返回 sql.ErrNoRows。
func (db *RoomDbObj) RoomGet(rid uint64) (*RoomInfo, error) {
	row := db.mysql.QueryRow(`
		SELECT rid, name, status, owner FROM CHAT_ROOM_INFO_TAB WHERE rid=?`, rid)
	info := &RoomInfo{Rid: rid}
	err := row.Scan(&info.Rid, &info.Name, &info.Status, &info.Owner)
	if err != nil {
		return nil, err
	}
	return info, nil
}

// RoomDismiss 将聊天室标记为关闭。
func (db *RoomDbObj) RoomDismiss(rid uint64) error {
	res, err := db.mysql.Exec(`
		UPDATE CHAT_ROOM_INFO_TAB SET status=?, update_time=UNIX_TIMESTAMP() WHERE rid=?`,
		ROOM_STAT_CLOSE, rid)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// RoomIsOpen 判断聊天室是否处于开启状态。
func (db *RoomDbObj) RoomIsOpen(rid uint64) (bool, error) {
	info, err := db.RoomGet(rid)
	if err != nil {
		return false, err
	}
	return info.Status == ROOM_STAT_OPEN, nil
}

// RoomIsOwner 判断 uid 是否为房主。
func (db *RoomDbObj) RoomIsOwner(rid, uid uint64) (bool, error) {
	info, err := db.RoomGet(rid)
	if err != nil {
		return false, err
	}
	return info.Owner == uid, nil
}

// RoomOnlineCount 返回 rid 在线 SID 数。
func (c *RoomCacheObj) RoomOnlineCount(rid uint64) (int, error) {
	rds := c.redis.Get()
	defer rds.Close()
	key := fmt.Sprintf(ROOM_KEY_RID_TO_SID_ZSET, rid)
	return redis.Int(rds.Do("ZCARD", key))
}

// RoomSidInRoom 判断 sid 是否在 rid 中（TTL 未过期）。
func (c *RoomCacheObj) RoomSidInRoom(rid, sid uint64) (bool, error) {
	rds := c.redis.Get()
	defer rds.Close()
	key := fmt.Sprintf(ROOM_KEY_RID_TO_SID_ZSET, rid)
	score, err := redis.Int64(rds.Do("ZSCORE", key, sid))
	if err == redis.ErrNil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return score > time.Now().Unix(), nil
}

// RoomDismissRedis 清理聊天室 Redis 拓扑。
func (c *RoomCacheObj) RoomDismissRedis(rid uint64) error {
	rds := c.redis.Get()
	defer rds.Close()

	keys := []string{
		fmt.Sprintf(ROOM_KEY_RID_ATTR, rid),
		fmt.Sprintf(ROOM_KEY_ROOM_INFO_TAB, rid),
		fmt.Sprintf(ROOM_KEY_ROOM_ROLE_TAB, rid),
		fmt.Sprintf(ROOM_KEY_RID_TO_SID_ZSET, rid),
		fmt.Sprintf(ROOM_KEY_RID_TO_UID_SID_ZSET, rid),
		fmt.Sprintf(ROOM_KEY_RID_TO_NID_ZSET, rid),
		fmt.Sprintf(ROOM_KEY_RID_GID_TO_NUM_ZSET, rid),
		fmt.Sprintf(ROOM_KEY_ROOM_USR_GAG_SET, rid),
		fmt.Sprintf(ROOM_KEY_ROOM_USR_BLACKLIST_SET, rid),
	}
	for _, k := range keys {
		_, _ = rds.Do("DEL", k)
	}
	_, _ = rds.Do("ZREM", ROOM_KEY_RID_ZSET, rid)
	_, _ = rds.Do("ZREM", ROOM_KEY_ROOM_GROUP_CAP_ZSET, rid)
	return nil
}
