# 群聊 Demo 接口全览

> **范围**：`demo/web/group.html` 双窗口群聊演示所经的全部接口（HTTP、WebSocket、Go 业务、C 转发、Redis/Mongo）。  
> **配套（面试向中文导读）**：[群聊Demo源码导读.md](群聊Demo源码导读.md) · [GROUP_DEMO_ARCHITECTURE.md](GROUP_DEMO_ARCHITECTURE.md) · [GROUP_DESIGN.md](GROUP_DESIGN.md) · [COMMAND.md](COMMAND.md)

---

## 1. 总览：Demo 走了哪些层

| 层级 | 技术 | Demo 是否经过 |
|------|------|---------------|
| 浏览器 | `group.html` + `group-app.js` + `pb.js` | ✅ |
| 演示 HTTP 代理 | `demo-web` 容器 / `scripts/demo-proxy.py` | ✅ |
| HTTP 业务 | usrsvr `:8000` | ✅ |
| WebSocket 接入 | Go `websocket` `:8002` / `:8003` | ✅ |
| TCP 接入 | C `listend` `:9002` / `:9003` | 🟡 栈内运行，demo 未用 |
| 消息转发 | C `frwder` RTMQ `:28888` / `:28889` | ✅ |
| 群生命周期 | Go `usrsvr` | ✅ |
| 群消息 fan-out | Go `msgsvr` | ✅ |
| 接入注册 | Go `monitor` + `LSND-INFO` | ✅（iplist 前置） |
| ID 分配 | Go `seqsvr` (Thrift) | ✅（register 分配 sid） |
| 存储 | Redis + Mongo + MySQL | ✅ Redis/Mongo；MySQL 仅种子用户 |
| 聊天室 | Go `chatroom` | 🟡 进程在跑，群 demo 不调用 |
| 推送 | usrsvr `/im/push` | ❌ 群 demo 未用 |

---

## 2. HTTP 接口（浏览器 → usrsvr）

Demo 通过 `8088` 同源代理访问（`BEEHIVE_USRSVR_URL` → `runner:8000`）。

| # | 方法 | 路径 | 用途 | Go 路由 | Go Handler |
|---|------|------|------|---------|------------|
| H-1 | GET | `/im/register?uid={uid}&nation=1&city=1&town=1` | 分配 uid/sid | `routers/router.go:15` | `UsrSvrRegisterCtrl.Register` → `register.go` |
| H-2 | GET | `/im/iplist?type=2&uid={uid}&sid={sid}&clientip=127.0.0.1` | 获取 WS 地址 + ONLINE token | `router.go:16` | `UsrSvrIplistCtrl.Iplist` → `iplist.go` |

**前端封装**：`demo/web/demo-common.js`（`registerUser` / `fetchIplist`）。

**iplist 依赖链**：websocket 周期上报 `LSND-INFO` → monitor 写 Redis `im:lsnd:*` → usrsvr 后台 task 刷新内存字典 → iplist 返回 `127.0.0.1:8002` 等。

**群 demo 未使用的 HTTP**（同 usrsvr，供运维/扩展）：`/im/group/config`、`/im/push`、`/im/query` 等，见 [HTTPSVR.md](HTTPSVR.md)。

---

## 3. WebSocket 协议（demo 实际收发的 cmd）

帧格式：**52 字节大端头** + **protobuf 体**（见 [PROTOCOL.md](PROTOCOL.md)）。

### 3.1 连接底座（0x01xx，群聊前置）

| cmd | 十六进制 | 关键字 | 方向 | Demo | Protobuf |
|-----|----------|--------|------|------|----------|
| ONLINE | 0x0101 | 上线 | C→S | ✅ | `MesgOnline` |
| ONLINE-ACK | 0x0102 | 上线应答 | S→C | ✅ | `MesgOnlineAck` |

### 3.2 群聊生命周期（0x0301–0x030A）

| cmd | 十六进制 | 关键字 | 方向 | Demo UI | 处理服务 |
|-----|----------|--------|------|---------|----------|
| GROUP-CREAT | 0x0301 | 建群 | C→S | ✅ | usrsvr |
| GROUP-CREAT-ACK | 0x0302 | 建群应答 | S→C | ✅ | usrsvr |
| GROUP-DISMISS | 0x0303 | 解散群 | C→S | ✅ 按钮 | usrsvr |
| GROUP-DISMISS-ACK | 0x0304 | 解散应答 | S→C | ✅ | usrsvr |
| GROUP-JOIN | 0x0305 | 加群 | C→S | ✅ | usrsvr |
| GROUP-JOIN-ACK | 0x0306 | 加群应答 | S→C | ✅ | usrsvr |
| GROUP-QUIT | 0x0307 | 退群 | C→S | ✅ 按钮 | usrsvr |
| GROUP-QUIT-ACK | 0x0308 | 退群应答 | S→C | ✅ | usrsvr |
| GROUP-INVITE | 0x0309 | 邀请入群 | C→S | ✅ 按钮 | usrsvr |
| GROUP-INVITE-ACK | 0x030A | 邀请应答 | S→C | ✅ | usrsvr |

### 3.3 群消息（0x030B–0x030C）

| cmd | 十六进制 | 关键字 | 方向 | Demo | 处理服务 |
|-----|----------|--------|------|------|----------|
| GROUP-CHAT | 0x030B | 群聊消息 | C→S / S→C | ✅ | msgsvr 收；websocket 下行 fan-out |
| GROUP-CHAT-ACK | 0x030C | 发送应答 | S→C | ✅（静默） | msgsvr |

### 3.4 群通知（0x035x，demo 可收到）

| cmd | 十六进制 | 关键字 | 方向 | Demo | 处理服务 |
|-----|----------|--------|------|------|----------|
| GROUP-JOIN-NTF | 0x0350 | 入群通知 | S→C | ✅ 系统行 | usrsvr 广播 |
| GROUP-QUIT-NTF | 0x0352 | 退群通知 | S→C | ✅ 系统行 | usrsvr 广播 |

### 3.5 服务端已实现、Demo UI 未覆盖的群命令

| 范围 | 示例 | 服务 |
|------|------|------|
| 0x030D–0x030E | GROUP-KICK | usrsvr |
| 0x0310–0x0313 | GROUP-GAG-ADD/DEL | usrsvr |
| 0x0314–0x0317 | GROUP-BL-ADD/DEL | usrsvr |
| 0x0318–0x031B | GROUP-MGR-ADD/DEL | usrsvr |
| 0x031C–0x031D | GROUP-USR-LIST | usrsvr |
| 0x0354–0x0367 | KICK/GAG/BL/MGR 各类 NTF | usrsvr |

完整表见 [COMMAND.md](COMMAND.md) §群聊。

### 3.6 接入运维（iplist 前置，非 demo 按钮触发）

| cmd | 十六进制 | 关键字 | 方向 | 服务 |
|-----|----------|--------|------|------|
| LSND-INFO | 0x0601 | 接入点上报 | websocket→monitor | monitor |
| LSND-INFO-ACK | 0x0602 | 上报应答 | monitor→websocket | monitor |

---

## 4. Go 服务与 Handler 映射

### 4.1 websocket（接入层，Demo 使用 WS 非 TCP）

**上行** — `exec/websocket/controllers/mesg.go`

| cmd | Handler | 行为 |
|-----|---------|------|
| 0x0101 ONLINE | `LsndMesgOnlineHandler` | 绑定 sid/cid/nid，转发 frwder |
| 0x0301–0x030B 等 | `LsndMesgCommHandler` | 要求已登录，补 cid/nid，`frwder.AsyncSend` |

**下行** — `exec/websocket/controllers/upmesg.go`

| cmd | Handler | 行为 |
|-----|---------|------|
| 0x0102 ONLINE-ACK | `LsndUpMesgOnlineAckHandler` | 更新登录态、seq、sid→cid |
| 0x0302 CREAT-ACK / 0x0306 JOIN-ACK | `LsndUpMesgGroupMemberAckHandler` | `chat_tab.ImGroupJoin(gid)` + 下发客户端 |
| 0x0308 QUIT-ACK | `LsndUpMesgGroupQuitAckHandler` | `ImGroupQuit(gid)` + 下发 |
| 0x030B GROUP-CHAT | `LsndUpMesgGroupChatHandler` | `TravImGroupSession` 本节点 fan-out |
| 0x030C CHAT-ACK | `LsndUpMesgCommHandler` | 单播给发送方 |
| 0x0350/0x0352 NTF | `LsndUpMesgGroupNtfHandler` | 群成员 fan-out |

**本节点群会话索引** — `lib/chat_tab/chat.go`（`ImGroupJoin` / `TravImGroupSession`）。

### 4.2 usrsvr（群生命周期 + ONLINE）

注册于 `exec/usrsvr/controllers/usrsvr.go:160–189`。

| cmd | Handler | 源文件 |
|-----|---------|--------|
| 0x0101 ONLINE | `UsrSvrOnlineHandler` | `mesg.go` |
| 0x0301 GROUP-CREAT | `UsrSvrGroupCreatHandler` | `gmesg.go` |
| 0x0303 GROUP-DISMISS | `UsrSvrGroupDismissHandler` | `gmesg.go` |
| 0x0305 GROUP-JOIN | `UsrSvrGroupJoinHandler` | `gmesg.go` |
| 0x0307 GROUP-QUIT | `UsrSvrGroupQuitHandler` | `gmesg.go` |
| 0x0309 GROUP-INVITE | `UsrSvrGroupInviteHandler` | `gmesg.go` |
| 0x030D GROUP-KICK | `UsrSvrGroupKickHandler` | `gmesg.go` |
| 0x0310/0x0312 GAG | `UsrSvrGroupGagAdd/DelHandler` | `gmesg.go` |
| 0x0314/0x0316 BL | `UsrSvrGroupBlacklistAdd/DelHandler` | `gmesg.go` |
| 0x0318/0x031A MGR | `UsrSvrGroupMgrAdd/DelHandler` | `gmesg.go` |
| 0x031C USR-LIST | `UsrSvrGroupUsrListHandler` | `gmesg.go` |

**发送辅助** — `gmesg_send.go`：`groupSendSimpleAck`、`groupBroadcast`（按 Redis `gid→nid` 广播 NTF）。

**Redis 封装** — `lib/chat/group_ops.go`：`GroupRegister`、`GroupJoinOnline`、`GroupQuit`、`GroupDismiss` 等。

### 4.3 msgsvr（群消息）

| cmd | Handler | 源文件 |
|-----|---------|--------|
| 0x030B GROUP-CHAT | `MsgSvrGroupChatHandler` | `gmesg.go` |
| 0x030C GROUP-CHAT-ACK | `MsgSvrGroupChatAckHandler` | `gmesg.go`（空实现） |

**下行**：`send_data` → `comm.go`（按 `head.nid` 经 frwder 推到各 websocket）。

**异步落库**：Redis `chat:gid:{gid}:mesg:queue` + Mongo `group-mesg` 集合。

### 4.4 monitor（接入点注册，iplist 前置）

| cmd | Handler | 源文件 |
|-----|---------|--------|
| 0x0601 LSND-INFO | `MonLsndInfoHandler` | `monitor/controllers/mesg.go` |

### 4.5 seqsvr（Thrift，无 WS cmd）

| 能力 | 调用方 | 用途 |
|------|--------|------|
| 分配 sid | usrsvr `register.go` | `/im/register` |
| 分配 gid | usrsvr `allocGid` | `INCR chat:gid:incr`（群 demo 同时用 Redis INCR） |

---

## 5. C 服务接口

### 5.1 frwder（RTMQ 路由中枢）

| 文件 | 函数 | 方向 | 行为 |
|------|------|------|------|
| `src/clang/exec/frwder/frwd_mesg.c:65` | `frwd_mesg_from_fw_def_hdl` | ↑ 接入→业务 | `rtmq_publish(BACKEND, cmd, data)` 广播给订阅该 cmd 的后端 |
| `src/clang/exec/frwder/frwd_mesg.c:96` | `frwd_mesg_from_bc_def_hdl` | ↓ 业务→接入 | `rtmq_async_send(FORWARD, cmd, head.nid, data)` 按 NID 推到指定 websocket/listend |

**端口**（`conf/templates/frwder.xml`）：

| 端口 | 角色 | 连接方 |
|------|------|--------|
| **28888** | FORWARD | websocket、listend（上行入口 / 下行出口） |
| **28889** | BACKEND | usrsvr、msgsvr、monitor、chatroom（业务订阅） |

frwder **不理解群语义**，只做 `cmd + nid` 路由。

### 5.2 listend（C TCP 接入，与 Go websocket 对称）

| 文件 | 函数 | 行为 |
|------|------|------|
| `src/clang/exec/listend/lsnd_mesg.c` | `lsnd_mesg_def_handler` | 客户端 TCP 帧 → 补 sid/cid/nid → 送 frwder FORWARD |
| 同上 | `lsnd_mesg_online_handler` | ONLINE 专用上行 |

Demo 使用 **Go websocket**（iplist `type=2`），listend 在栈内运行但浏览器不连 `:9002`。

**命令常量**（C 与 Go 对齐）：`src/clang/incl/cmd_list.h`（`CMD_GROUP_*` 0x0301–0x0367）。

---

## 6. Redis 键（群 Demo 读写）

定义：`lib/comm/key.go`。

### 6.1 上线 / 会话

| 键 | 操作 | 场景 |
|----|------|------|
| `im:sid:zset` | ZADD | ONLINE |
| `im:uid:zset` | ZADD | ONLINE |
| `im:sid:{sid}:attr` | HMSET CID/UID/NID | ONLINE |
| `im:uid:{uid}:to:sid:set` | SADD | ONLINE |

### 6.2 接入调度（iplist）

| 键 | 操作 | 场景 |
|----|------|------|
| `im:lsnd:type:zset` | ZADD | LSND-INFO |
| `im:lsnd:type:{typ}:nation:zset` | ZADD | LSND-INFO |
| `im:lsnd:type:{typ}:nation:{n}:op:zset` | ZADD | LSND-INFO |
| `im:lsnd:type:{typ}:nation:{n}:op:{op}:zset` | ZADD | 成员=`IP:PORT` |
| `im:lsnd:nid:zset` / `im:lsnd:nid:{nid}:attr` | ZADD/HSET | 接入点元数据 |

### 6.3 群生命周期

| 键 | 操作 | 场景 |
|----|------|------|
| `chat:gid:incr` | INCR | 建群分配 gid |
| `chat:gid:zset` | ZADD | 群存在索引 |
| `chat:group:cap:zset` | ZADD | 群容量 |
| `chat:gid:{gid}:role:tab` | HSET | 成员角色 owner/manager/member |
| `chat:gid:{gid}:info:tab` | HMSET | 群名/描述/owner |
| `chat:uid:{uid}:to:gid:htab` | HSET | 用户所属群 |
| `chat:gid:{gid}:attr` | HMSET | 群开关 |
| `chat:gid:{gid}:to:nid:zset` | ZADD | **群消息 fan-out 接入点** |
| `chat:gid:{gid}:to:uid:zset` | ZADD | 在线成员 uid |
| `chat:gid:{gid}:to:sid:zset` | ZADD | 在线成员 sid |
| `chat:gid:{gid}:usr:blacklist:set` | SISMEMBER/SADD | 邀请/加群校验 |
| `chat:gid:{gid}:usr:gag:set` | SISMEMBER | 禁言校验（GROUP-CHAT） |

### 6.4 群消息

| 键 | 操作 | 场景 |
|----|------|------|
| `chat:gid:{gid}:mesg:queue` | LPUSH / LTRIM | 群消息缓存队列 |
| Mongo `group-mesg` | Insert | 历史落库 |

---

## 7. Protobuf 消息体（Demo 涉及）

源：`doc/mesg/mesg.proto` → `lib/mesg/mesg.pb.go`；前端：`demo/web/pb.js`。

| 消息 | 主要字段 |
|------|----------|
| `MesgOnline` | uid, sid, token, app, version, terminal |
| `MesgOnlineAck` | uid, sid, seq, code, errmsg |
| `MesgGroupCreat` | uid, gid(占位0), name, desc |
| `MesgGroupJoin` / `MesgGroupQuit` / `MesgGroupDismiss` | uid, gid |
| `MesgGroupInvite` | uid, gid, to |
| `MesgGroupChat` | uid, gid, level, time, text |
| `MesgGroupCreatAck` 等 ACK | code, errmsg（成功时 `Ok:{gid}`） |
| `MesgGroupJoinNtf` / `MesgGroupQuitNtf` | uid, gid |

---

## 8. Demo 双窗口时序（接口调用顺序）

```
窗口 A/B  GET /im/register
窗口 A/B  GET /im/iplist
窗口 A/B  WS connect
窗口 A/B  0x0101 ONLINE → 0x0102 ONLINE-ACK
窗口 A    0x0301 GROUP-CREAT → 0x0302 ACK (Ok:gid)
窗口 B    0x0305 GROUP-JOIN → 0x0306 ACK + 0x0350 NTF → A
窗口 A/B  0x030B GROUP-CHAT → 0x030B fan-out + 0x030C ACK
可选      0x0309 INVITE / 0x0307 QUIT / 0x0303 DISMISS
```

**自动化验收**：`tools/smoke-group/main.go` / `./scripts/smoke-group.sh`。

---

## 9. 前端文件索引

| 文件 | 职责 |
|------|------|
| `demo/web/group.html` | 群聊演示 UI |
| `demo/web/group-app.js` | cmd 常量、WS 状态机、按钮逻辑 |
| `demo/web/demo-common.js` | register + iplist 重试 |
| `demo/web/pb.js` | 最小 protobuf 编解码 |

---

*与 `feature_spec_task_02` 群聊实现、`demo/web/group.html` 对齐。*
