package controllers

import (
	"fmt"
	"os"

	"beehive-im/lib/comm"
	"beehive-im/lib/rdb"
)

// DrainRedisOnShutdown Pod 终止时清理本 NID 在 Redis 中的路由条目。
func (ctx *LsndCntx) DrainRedisOnShutdown() {
	if os.Getenv("BEEHIVE_DRAIN_NID_ON_EXIT") != "1" {
		return
	}
	addr, passwd := comm.RedisAddrFromEnv()
	pool := rdb.CreatePool(addr, passwd, 4)
	if pool == nil {
		ctx.log.Error("Drain redis: create pool failed addr:%s", addr)
		return
	}
	meta := comm.LsndDrainMeta{
		Nid:    ctx.conf.GetNid(),
		Type:   comm.LSND_TYPE_WS,
		Nation: ctx.conf.GetNation(),
		Opid:   ctx.conf.GetOpid(),
		Addr:   fmt.Sprintf(comm.IM_FMT_IP_PORT_STR, ctx.conf.GetIp(), ctx.conf.GetPort()),
	}
	if err := comm.DrainLsndNidFromRedis(pool, meta); err != nil {
		ctx.log.Error("Drain redis nid:%d failed! errmsg:%s", meta.Nid, err.Error())
		return
	}
	ctx.log.Info("Drain redis nid:%d done", meta.Nid)
}
