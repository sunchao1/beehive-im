# 群聊模块设计（T02-19）

## 服务选型

| 命令范围 | 服务 | 说明 |
|----------|------|------|
| 0x0301～0x031D | **usrsvr** | 建群/加群/管理/成员列表 |
| 0x0350～0x0367 | **usrsvr** | 群事件 NTF 广播 |
| 0x030B～0x030C | **msgsvr** | 群消息 fan-out + ACK |
| 存储 | Redis + Mongo | 成员/路由在 Redis；历史在 Mongo `group-mesg` |

## 数据流

```
Client WS
  → frwder → usrsvr (GROUP-JOIN 等，写 Redis gid→nid)
  → frwder → msgsvr (GROUP-CHAT，读 gid→nid map，按 nid 下发)
  → listend/websocket → 其他成员
```

## 关键 Redis 键

- `chat:gid:zset` — 活跃群索引
- `chat:gid:{gid}:role:tab` — 成员与角色（owner/manager/member）
- `chat:gid:{gid}:to:nid:zset` — 群消息 fan-out 接入点
- `chat:uid:{uid}:to:gid:htab` — 用户所属群

## 代码位置

- `src/golang/lib/chat/group_ops.go` — Redis 封装
- `src/golang/exec/usrsvr/controllers/gmesg.go` — 群生命周期
- `src/golang/exec/msgsvr/controllers/gmesg.go` — GROUP-CHAT

## 验收

```bash
./scripts/up-demo.sh
./scripts/smoke-group.sh
```
