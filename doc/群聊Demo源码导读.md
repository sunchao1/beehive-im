# 群聊 Demo 源码导读（面试向）

> **目标**：沿着 `demo/web/group.html` 双窗口群聊路线，把必嗨 IM **Go + C + 前端** 核心代码串起来，做到「看文档 → 点文件 → 读懂业务」。  
> **周末清单（文件列表 + 验证）**：[群聊Demo代码清单与学习计划.md](群聊Demo代码清单与学习计划.md)  
> **配套**：[GROUP_DEMO_ARCHITECTURE.md](GROUP_DEMO_ARCHITECTURE.md)（架构图）· [GROUP_DEMO_INTERFACES.md](GROUP_DEMO_INTERFACES.md)（接口清单）· [GROUP_DESIGN.md](GROUP_DESIGN.md)

---

## 目录

1. [怎么用这份文档](#1-怎么用这份文档)
2. [启动 Demo 栈](#2-启动-demo-栈)
3. [全局地图：一条群消息经过谁](#3-全局地图一条群消息经过谁)
4. [推荐阅读顺序（按天）](#4-推荐阅读顺序按天)
5. [协议与公共基础设施](#5-协议与公共基础设施)
6. [阶段一：HTTP 注册与接入调度](#6-阶段一http-注册与接入调度)
7. [阶段二：WebSocket 连接与上线](#7-阶段二websocket-连接与上线)
8. [阶段三：建群与加群](#8-阶段三建群与加群)
9. [阶段四：群消息与两段 Fan-out](#9-阶段四群消息与两段-fan-out)
10. [阶段五：邀请、退群、解散](#10-阶段五邀请退群解散)
11. [C 语言层：frwder 与 listend](#11-c-语言层frwder-与-listend)
12. [Redis 键与 Mongo 落库速查](#12-redis-键与-mongo-落库速查)
13. [全命令 → 代码索引表](#13-全命令--代码索引表)
14. [与聊天室 / 私聊的对比（面试高频）](#14-与聊天室--私聊的对比面试高频)
15. [故障排查](#15-故障排查)
16. [面试话术提纲](#16-面试话术提纲)

---

## 1. 怎么用这份文档

| 你想… | 怎么做 |
|--------|--------|
| **5 分钟建立全局观** | 读 [§3](#3-全局地图一条群消息经过谁) + 打开 [ARCHITECTURE_DIAGRAM.md](assets/ARCHITECTURE_DIAGRAM.md) |
| **跟一遍 Demo 操作** | 按 [§6→§10](#6-阶段一http-注册与接入调度) 顺序，每节有「前端发什么 → 后端谁处理」 |
| **查某个 cmd 在哪实现** | 直接看 [§13 索引表](#13-全命令--代码索引表) |
| **准备面试讲架构** | [§9 Fan-out](#9-阶段四群消息与两段-fan-out) + [§14 对比](#14-与聊天室--私聊的对比面试高频) + [§16 话术](#16-面试话术提纲) |
| **查 Redis 写了什么** | [§12](#12-redis-键与-mongo-落库速查) |

**代码根目录**：仓库 `beehive-im/`，下文路径均相对仓库根。

---

## 2. 启动 Demo 栈

```bash
# 首次或改 Go/C 代码后：容器内编译 Linux 二进制
docker compose --profile build run --rm --build builder

# 中间件 + 9 业务进程 + 演示页
docker compose --profile run --profile demo up -d
```

| 入口 | 地址 |
|------|------|
| 群聊页 | http://127.0.0.1:8088/group.html |
| usrsvr HTTP | http://127.0.0.1:8000 |
| WebSocket | ws://127.0.0.1:8002/im 或 :8003/im |
| 终端验收 | `./scripts/smoke-group.sh` |

**runner 内 9 进程**（`scripts/run-in-linux.sh`）：frwder、listend×2、websocket×2、usrsvr、msgsvr、monitor、chatroom、seqsvr、tasker。  
**群 Demo 主路径用到**：frwder、websocket、usrsvr、msgsvr、monitor、seqsvr。

---

## 3. 全局地图：一条群消息经过谁

```
浏览器 group.html
  │  HTTP  /im/register、/im/iplist        → usrsvr :8000
  │  WS    0x0101 ONLINE … 0x030B CHAT     → websocket :8002/:8003
  ▼
Go websocket（接入层）
  │  上行：补 cid/nid，AsyncSend → frwder FORWARD :28888
  │  下行：按 ImGroup 会话表 fan-out 到本节点各连接
  ▼
C frwder（RTMQ 路由，不懂群语义）
  │  上行：publish → BACKEND :28889 → 订阅该 cmd 的 Go 服务
  │  下行：async_send(FORWARD, head.nid) → 指定 websocket 节点
  ▼
Go 业务
  │  usrsvr：ONLINE、群生命周期 0x0301–0x031D、通知 0x035x
  │  msgsvr：GROUP-CHAT 0x030B，读 Redis gid→nid，再 fan-out
  ▼
Redis / Mongo
  │  成员、在线路由、消息队列、历史
```

**面试一句话**：群聊 = **usrsvr 管人**，**msgsvr 管消息**；下行 = **msgsvr 按 gid→nid** + **websocket 按 ImGroup→cid** 两段 fan-out，中间靠 **frwder 按 NID 路由**。

---

## 4. 推荐阅读顺序（按天）

| 天 | 主题 | 必读文件 |
|----|------|----------|
| **D1** | 前端 + HTTP + 协议头 | `demo/web/*`、`register.go`、`iplist.go`、`lib/comm/mesg.go` |
| **D2** | 接入 + frwder | `websocket/mesg.go`、`upmesg.go`、`frwd_mesg.c`、`lib/rtmq/rtmq_proxy.go` |
| **D3** | 上线 + iplist 链 | `usrsvr/mesg.go`（ONLINE）、`monitor/mesg.go`、`websocket/task.go`（LSND-INFO） |
| **D4** | 群生命周期 | `usrsvr/gmesg.go`、`lib/chat/group_ops.go`、`gmesg_send.go` |
| **D5** | 群消息 fan-out | `msgsvr/gmesg.go`、`chat_tab/chat.go`（ImGroup）、`upmesg.go`（GroupChatHandler） |
| **D6** | 对比 + 排查 | 本文 §14、§15，`tools/smoke-group/main.go` |

每天配合 **打开 group.html 操作一步、对照日志 `log/usrsvr.log` / `log/msgsvr.log`**。

---

## 5. 协议与公共基础设施

### 5.1 二进制帧（所有 WS 命令共用）

| 项 | 说明 | 代码 |
|----|------|------|
| 头长度 | 52 字节，大端 | `lib/comm/mesg.go` → `MESG_HEAD_SIZE` |
| 头字段 | cmd、len、sid、cid、nid、seq… | `lib/comm/mesg.go` → `MesgHeader`、`MesgHeadNtoh/Hton` |
| 体 | protobuf | `lib/mesg/mesg.pb.go`（由 `doc/mesg/mesg.proto` 生成） |

前端打包：`demo/web/app.js` / `group-app.js` 中 `packMsg()`；解码见 `demo/web/pb.js`。

### 5.2 命令字常量

| 文件 | 作用 |
|------|------|
| `src/golang/lib/comm/mesg.go` | Go 侧 `CMD_*`（群：58–103 行附近） |
| `src/clang/incl/cmd_list.h` | C 侧 `CMD_GROUP_*`，与 Go 对齐 |

### 5.3 Go 服务如何挂上 frwder

各 exec 进程启动时：

1. 读配置里 `FRWDER ADDR`（websocket 连 **28888**，usrsvr/msgsvr 连 **28889**）
2. `rtmq.NewProxy` → `lib/rtmq/rtmq_proxy.go`
3. `proxy.Register(cmd, handler, ctx)` 订阅 BACKEND 上某 cmd
4. 业务里 `proxy.AsyncSend(cmd, buf, len)` 发消息

**接入层** websocket 在 `listend.go` 初始化后调用 `MesgRegister()`（上行）和 `UpMesgRegister()`（下行）。

### 5.4 配置从哪来

Docker 启动时 `docker/gen-conf.sh` 把 `conf/templates/*.xml` 渲染到 `.run-conf/`，再被各进程 `-c` 加载。  
websocket NID、端口见 `conf/templates/websocket.xml` / `websocket-2.xml`。

---

## 6. 阶段一：HTTP 注册与接入调度

### 6.1 前端：注册

| 项 | 位置 |
|----|------|
| 页面 | `demo/web/group.html` |
| 逻辑 | `demo/web/group-app.js` → `goOnline()` → `DemoCommon.registerUser()` |
| HTTP 封装 | `demo/web/demo-common.js` |
| 请求 | `GET /im/register?uid=&nation=1&city=1&town=1` |

### 6.2 Go：register 处理

| 步骤 | 函数 | 文件 |
|------|------|------|
| 路由 | `beego.Router("/im/register", …)` | `exec/usrsvr/routers/router.go` |
| 入口 | `UsrSvrRegisterCtrl.Register` | `exec/usrsvr/controllers/register.go:23` |
| 解析参数 | `register_parse_param` | 同文件 `:56` |
| 分配 sid | `register_handler` → 调 seqsvr Thrift | 同文件 `:81+` |

**面试点**：sid 由 **seqsvr** 分配，不是 register 自己 INCR；群 gid 则是 usrsvr 里 **Redis INCR** `chat:gid:incr`（见 §8）。

### 6.3 前端：iplist

| 项 | 位置 |
|----|------|
| 调用 | `demo-common.js` → `fetchIplist()`，失败重试 15 次 |
| 请求 | `GET /im/iplist?type=2&uid=&sid=&clientip=127.0.0.1` |
| 结果 | `token`（ONLINE 鉴权）、`list[0]` → `ws://host/im` |

### 6.4 Go：iplist 处理

| 步骤 | 函数 | 文件 |
|------|------|------|
| 入口 | `UsrSvrIplistCtrl.Iplist` | `exec/usrsvr/controllers/iplist.go:17` |
| 读 Redis 字典 | `iplist_get` → `listend.dict.types[2]` | 同文件 `:209` |
| 生成 token | `iplist_token` | 同文件 `:183` |
| 后台刷新字典 | `listend_dict_update` | `exec/usrsvr/controllers/task.go:47` |

**iplist 数据从哪来** → 见 §6.5。

### 6.5 接入点注册链（iplist 前置，易挂）

```
websocket 定时任务
  → 发 CMD_LSND_INFO (0x0601)
  → frwder → monitor
  → 写 Redis im:lsnd:*
  → usrsvr task 读到内存
  → /im/iplist 才能返回 127.0.0.1:8002
```

| 步骤 | 代码 |
|------|------|
| websocket 上报 | `exec/websocket/controllers/task.go`（`CMD_LSND_INFO` 组包 + `AsyncSend`） |
| monitor 写 Redis | `exec/monitor/controllers/mesg.go` → `lsnd_info_handler` |
| Redis 键 | `im:lsnd:type:zset`、`im:lsnd:type:2:nation:CN:op:1:zset` 等，见 `lib/comm/key.go` |

**现象**：页面一直「iplist 等待接入点注册」→ runner 跑太久或 frwder 挂 → `docker compose --profile run restart runner`，等 20 秒。

### 6.6 demo-web 代理

| 文件 | 作用 |
|------|------|
| `docker-compose.yml` → `demo-web` 服务 | 8088 端口 |
| `scripts/demo-proxy.py` | 静态托管 `demo/web`，`/im/*` 代理到 `runner:8000` |

---

## 7. 阶段二：WebSocket 连接与上线

### 7.1 前端

| 项 | 代码 |
|----|------|
| 建连 | `group-app.js` → `connectWs()` |
| 发 ONLINE | `PB.encodeOnline({ uid, sid, token, … })` → `pb.js:81` |
| 收 ONLINE-ACK | `onMessage` case `0x0102`，更新 `state.seq`，启用建群/加群按钮 |

### 7.2 Go websocket 上行

| cmd | Handler | 文件 | 要点 |
|-----|---------|------|------|
| 0x0101 | `LsndMesgOnlineHandler` | `websocket/controllers/mesg.go:121` | 绑定 sid/cid，**设置 head.nid = 本节点 NID** |
| 0x030x | `LsndMesgCommHandler` | `mesg.go:68` | 要求 `CONN_STATUS_LOGIN`，转发 frwder |

注册表：`MesgRegister()` — `mesg.go:20`（含 GROUP 上行显式注册 `:51-57`）。

### 7.3 C frwder 上行

`frwd_mesg_from_fw_def_hdl` → `rtmq_publish(backend, …)`  
文件：`src/clang/exec/frwder/frwd_mesg.c:65-78`

### 7.4 Go usrsvr：ONLINE 业务

| 步骤 | 函数 | 文件 |
|------|------|------|
| 注册 handler | `UsrSvrOnlineHandler` | `usrsvr/controllers/usrsvr.go` |
| 解析/校验 token | `online_parse` / `online_check` | `usrsvr/controllers/mesg.go` |
| 写 Redis 会话 | `online_handler` | `mesg.go:304` |
| 回 ACK | `online_ack` | 同文件 |

Redis：`im:sid:zset`、`im:sid:{sid}:attr`（CID/UID/NID）等。

### 7.5 Go websocket 下行 ONLINE-ACK

| Handler | 文件 | 要点 |
|---------|------|------|
| `LsndUpMesgOnlineAckHandler` | `websocket/controllers/upmesg.go:166` | 置 `CONN_STATUS_LOGIN`，`SessionSetCid` |

---

## 8. 阶段三：建群与加群

### 8.1 设计分工（必背）

见 `doc/GROUP_DESIGN.md`：

| 职责 | 服务 | cmd 范围 |
|------|------|----------|
| 群生命周期 | **usrsvr** | 0x0301–0x031D |
| 群通知 | **usrsvr** | 0x0350–0x0367 |
| 群消息 | **msgsvr** | 0x030B–0x030C |

### 8.2 前端：建群

| 项 | 代码 |
|----|------|
| 按钮 | `group-app.js` → `createGroup()` |
| 编码 | `pb.js` → `encodeGroupCreat`（**gid 必填占位 0**） |
| 收 ACK | 解析 `errmsg` 中 `Ok:{gid}`，写入 GID 输入框 |

### 8.3 Go usrsvr：GROUP-CREAT

| 步骤 | 函数 | 文件 |
|------|------|------|
| Handler | `UsrSvrGroupCreatHandler` | `usrsvr/controllers/gmesg.go:82` |
| 解析 PB | `groupCreatParse` | 同文件 `:57` |
| 分配 gid | `allocGid` → Redis `INCR chat:gid:incr` | 同文件 `:72` |
| 写群元数据 | `chat.GroupRegister` | `lib/chat/group_ops.go:74` |
| 创建者在线路由 | `chat.GroupJoinOnline` | `group_ops.go:106` |
| ACK | `groupSendSimpleAck`，`errmsg=Ok:{gid}` | `gmesg.go:109` |

**GroupJoinOnline 写的关键 Redis**：

- `chat:gid:{gid}:to:nid:zset` ← **后面 msgsvr fan-out 用**
- `chat:gid:{gid}:to:sid:zset`、`to:uid:zset`
- `chat:gid:{gid}:role:tab`（owner）

### 8.4 Go websocket：CREAT-ACK 后本地建索引

| Handler | 文件 | 作用 |
|---------|------|------|
| `LsndUpMesgGroupMemberAckHandler` | `upmesg.go` | 解析 `Ok:gid`，调 `chat_tab.ImGroupJoin` |

**ImGroup 会话表**（第二段 fan-out 用）：

- `lib/chat_tab/chat.go` → `ImGroupJoin`、`TravImGroupSession`
- `lib/chat_tab/comm.go` → `session_join_im_group`

### 8.5 前端 + Go：加群

| 角色 | 操作 | 代码路径 |
|------|------|----------|
| B 窗口 | `GROUP-JOIN` | `group-app.js` → `joinGroup()` |
| usrsvr | `UsrSvrGroupJoinHandler` | `gmesg.go:152` |
| 广播 | `GROUP-JOIN-NTF` | `gmesg.go:180` → `groupBroadcast` |
| websocket | JOIN-ACK + NTF 下行 | `upmesg.go` |

`groupBroadcast` 实现：`gmesg_send.go:40` — 读 Redis `gid→nid`，对每个 nid 经 frwder 下发。

---

## 9. 阶段四：群消息与两段 Fan-out

这是 **面试最核心的 10 分钟**。

### 9.1 前端

| 项 | 代码 |
|----|------|
| 发送 | `sendChat()` → `CMD_GROUP_CHAT` → `encodeGroupChat` |
| 接收 | `onMessage` case `0x030B` → `decodeGroupChat` → 日志 `[uid] text` |

### 9.2 上行路径（到 msgsvr）

```
group-app.js sendChat
  → websocket LsndMesgCommHandler (mesg.go)
  → frwder frwd_mesg_from_fw_def_hdl (frwd_mesg.c)
  → msgsvr MsgSvrGroupChatHandler (gmesg.go:200)
```

### 9.3 msgsvr 业务逻辑

| 步骤 | 函数 | 文件 |
|------|------|------|
| 解析 | `group_chat_parse` | `msgsvr/controllers/gmesg.go:37` |
| 校验成员 | `chat.GroupGetRole` | `lib/chat/group_ops.go:58` |
| 校验禁言 | `chat.GroupIsGagged` | `group_ops.go:307` |
| 异步落库 | `group_mesg_chan` → `storage` | `gmesg.go:293-351` |
| **第一段 fan-out** | `GroupGetGidToNidSet` + 循环 `send_data` | `gmesg.go:162-173` |
| 给发送方 ACK | `group_chat_ack` | `gmesg.go:117` |

`send_data`：`msgsvr/controllers/comm.go:25` — 设置 `head.Nid = 目标接入点`。

**第一段 fan-out**：按 Redis `chat:gid:{gid}:to:nid:zset` 决定推哪些 **websocket 节点**（8002、8003 各一个 NID）。

### 9.4 第二段 fan-out（接入层内）

msgsvr 把 GROUP-CHAT 推到某 NID 的 websocket 后：

| Handler | 文件 | 逻辑 |
|---------|------|------|
| `LsndUpMesgGroupChatHandler` | `upmesg.go` | 解析 `MesgGroupChat.gid` |
| | | `chat.TravImGroupSession(gid, LsndRoomSendDataCb, …)` |
| `LsndRoomSendDataCb` | `upmesg.go:393` | 对每个 (sid,cid) `lws.AsyncSend` |

**为何需要 ImGroup 表**：msgsvr 下行包头里的 sid 往往是**发送方** sid；若用默认 `CommHandler` 只按 sid 找连接，**其他成员收不到**。聊天室用 `TravRoomSession`，群聊用 `TravImGroupSession`，模式相同。

### 9.5 C frwder 下行

`frwd_mesg_from_bc_def_hdl` → `rtmq_async_send(forward, type, **hhead.nid**, data, len)`  
文件：`frwd_mesg.c:96-108`

### 9.6 双窗口跨节点（A:8002，B:8003）

1. A、B 加群后，Redis `gid→nid` 里有两个 NID  
2. A 发消息 → msgsvr 向 **两个 NID** 各推一份 GROUP-CHAT  
3. 各 websocket 节点在自己的 ImGroup 表里找成员连接下发  

**Demo 价值**：一条消息走满 **gid→nid→cid** 三段寻址。

---

## 10. 阶段五：邀请、退群、解散

| 操作 | 前端 | usrsvr Handler | 备注 |
|------|------|----------------|------|
| 邀请 | `inviteUser()` | `UsrSvrGroupInviteHandler` `gmesg.go:218` | 只写成员表，不写 nid；被邀请人仍需 JOIN |
| 退群 | `quitGroup()` | `UsrSvrGroupQuitHandler` `gmesg.go:187` | `GroupQuit` 清 zset；websocket `ImGroupQuit` |
| 解散 | `dismissGroup()` | `UsrSvrGroupDismissHandler` `gmesg.go:117` | 仅 owner；`GroupDismiss` 删群相关键 |

通知：`GROUP-QUIT-NTF` 等走 `groupBroadcast`，下行 `LsndUpMesgGroupNtfHandler`（`upmesg.go`）。

**Demo 未点但已实现**（扩展阅读）：踢人 0x030D、禁言 0x0310、管理员 0x0318、成员列表 0x031C — 均在 `gmesg.go` 同文件后续 handler。

---

## 11. C 语言层：frwder 与 listend

### 11.1 frwder（必嗨 IM 的「枢纽」）

| 文件 | 内容 |
|------|------|
| `src/clang/exec/frwder/frwd_mesg.c` | 上下行默认 handler |
| `src/clang/exec/frwder/frwder.h` | 上下文 `forward` / `backend` 两个 RTMQ |
| `conf/templates/frwder.xml` | 28888 / 28889 端口 |

**记忆口诀**：

- **28888 FORWARD** = 接入层（websocket/listend）连这里，上行进、下行出  
- **28889 BACKEND** = 业务层（usrsvr/msgsvr/monitor）连这里  

frwder **不解析 protobuf**，只看 `MesgHeader.cmd` 和 `MesgHeader.nid`。

### 11.2 listend（TCP 接入，Demo 未用但架构对称）

| 文件 | 作用 |
|------|------|
| `src/clang/exec/listend/lsnd_mesg.c` | TCP 客户端消息 → 补 nid → 送 frwder |
| `conf/templates/listend.xml` | `:9002`，连 FORWARD |

浏览器 Demo 走 **Go websocket**（`iplist type=2`）。压测/CLI 可走 listend（`type=1`）。  
**面试**：「接入层 C/Go 双实现，业务只认 cmd，经 frwder 解耦。」

### 11.3 Go RTMQ 封装（读 frwder 的 Go 视角）

| API | 文件 |
|-----|------|
| `Register(cmd, cb, ctx)` | `lib/rtmq/rtmq_proxy.go:321` |
| `AsyncSend(cmd, data, len)` | 同文件 `:365` |

---

## 12. Redis 键与 Mongo 落库速查

定义文件：`src/golang/lib/comm/key.go`（47 行起为群相关）。

| 键模式 | 谁写 | 群 Demo 何时 |
|--------|------|--------------|
| `chat:gid:incr` | INCR | 建群 |
| `chat:gid:zset` | ZADD | 群存在索引 |
| `chat:gid:{gid}:role:tab` | HSET | 建群/加群/退群 |
| `chat:gid:{gid}:info:tab` | HMSET | 建群（群名） |
| `chat:gid:{gid}:to:nid:zset` | ZADD | 加群/建群在线路由 **← fan-out 核心** |
| `chat:gid:{gid}:to:sid:zset` | ZADD | 在线 sid |
| `chat:uid:{uid}:to:gid:htab` | HSET | 用户所属群 |
| `chat:gid:{gid}:mesg:queue` | LPUSH | 发群消息（异步） |
| `im:lsnd:*` | monitor | iplist |

Mongo：集合 `group-mesg`，结构 `GroupChatRow` — `msgsvr/gmesg.go:302`。

---

## 13. 全命令 → 代码索引表

### 13.1 HTTP

| 接口 | Go Handler | 文件 |
|------|------------|------|
| GET /im/register | `Register` | `usrsvr/controllers/register.go` |
| GET /im/iplist | `Iplist` | `usrsvr/controllers/iplist.go` |

### 13.2 WebSocket — Demo 涉及

| cmd | 关键字 | 上行 Go | 下行 Go | 业务 Go |
|-----|--------|---------|---------|---------|
| 0x0101 | ONLINE | `LsndMesgOnlineHandler` `websocket/mesg.go` | — | `UsrSvrOnlineHandler` `usrsvr/mesg.go` |
| 0x0102 | ONLINE-ACK | — | `LsndUpMesgOnlineAckHandler` `upmesg.go` | （usrsvr 发） |
| 0x0301 | GROUP-CREAT | `LsndMesgCommHandler` | — | `UsrSvrGroupCreatHandler` `gmesg.go` |
| 0x0302 | CREAT-ACK | — | `LsndUpMesgGroupMemberAckHandler` | `gmesg.go` |
| 0x0305 | GROUP-JOIN | 同上 | — | `UsrSvrGroupJoinHandler` |
| 0x0306 | JOIN-ACK | — | `LsndUpMesgGroupMemberAckHandler` | `gmesg.go` |
| 0x0309 | GROUP-INVITE | 同上 | — | `UsrSvrGroupInviteHandler` |
| 0x030B | GROUP-CHAT | 同上 | `LsndUpMesgGroupChatHandler` | `MsgSvrGroupChatHandler` `msgsvr/gmesg.go` |
| 0x030C | CHAT-ACK | — | `LsndUpMesgCommHandler` | `msgsvr/gmesg.go` |
| 0x0350 | JOIN-NTF | — | `LsndUpMesgGroupNtfHandler` | `groupBroadcast` `gmesg_send.go` |
| 0x0601 | LSND-INFO | `websocket/task.go` | — | `MonLsndInfoHandler` `monitor/mesg.go` |

### 13.3 C

| 组件 | 关键函数 | 文件 |
|------|----------|------|
| frwder 上行 | `frwd_mesg_from_fw_def_hdl` | `clang/exec/frwder/frwd_mesg.c:65` |
| frwder 下行 | `frwd_mesg_from_bc_def_hdl` | 同文件 `:96` |
| listend 默认上行 | `lsnd_mesg_def_handler` | `clang/exec/listend/lsnd_mesg.c` |

### 13.4 前端 Demo

| 文件 | 职责 |
|------|------|
| `demo/web/group.html` | UI |
| `demo/web/group-app.js` | WS 状态机、按钮、日志 |
| `demo/web/demo-common.js` | register + iplist 重试 |
| `demo/web/pb.js` | protobuf 编解码 |
| `scripts/demo-proxy.py` | 8088 代理 |
| `tools/smoke-group/main.go` | 终端 E2E（对照学习协议） |

### 13.5 Handler 注册入口（找「谁订阅了 cmd」）

| 服务 | 注册函数 | 文件 |
|------|----------|------|
| websocket 上行 | `MesgRegister` | `websocket/controllers/mesg.go:20` |
| websocket 下行 | `UpMesgRegister` | `websocket/controllers/upmesg.go:20` |
| usrsvr | `UsrSvrInit` 内 `ctx.frwder.Register` | `usrsvr/controllers/usrsvr.go:160` |
| msgsvr | 同模式 | `msgsvr/controllers/msgsvr.go:135` |

---

## 14. 与聊天室 / 私聊的对比（面试高频）

| 维度 | 群聊 0x03xx | 聊天室 0x04xx | 私聊 0x02xx |
|------|-------------|---------------|-------------|
| 生命周期服务 | usrsvr | chatroom | usrsvr（关系） |
| 消息服务 | msgsvr | chatroom | msgsvr |
| 拓扑 Redis 键 | `chat:gid:{gid}:to:nid:zset` | `room:rid:*` | uid/sid 点对点 |
| 接入层索引 | `ImGroupJoin` / `TravImGroupSession` | `RoomJoin` / `TravRoomSession` | 按 sid 单播 |
| Demo 页面 | `group.html` | `index.html` | 暂无 |

**同一条架构套路**：业务算 fan-out 目标 NID → frwder 按 nid 投递 → 接入层在本节点展开到 cid。

**群 Demo 没覆盖但代码可类推**：`index.html` 弹幕（chatroom）、`smoke-push`（0x05xx 推送）、`CMD_CHAT` 私聊（msgsvr `pmesg.go`）。

---

## 15. 故障排查

| 现象 | 原因 | 处理 |
|------|------|------|
| iplist 一直等待 | Redis `im:lsnd:*` TTL 过期 / monitor 未注册 | `docker compose --profile run restart runner`，等 20s |
| CREAT-ACK gid 未 set | 前端未传 proto required gid | `pb.js encodeGroupCreat` 需 `gid=0` |
| 互看不到消息 | websocket 未做 ImGroup fan-out（旧版）或 ImGroup 未 JOIN | 确认 `upmesg.go` 有 `LsndUpMesgGroupChatHandler` |
| Get ip list failed | 同上 iplist | 重启 runner |
| frwder assertion | C 层异常，后续 RTMQ 断 | 重启 runner，查 `log/frwder.log` |

详表：[TROUBLESHOOT.md](TROUBLESHOOT.md)

---

## 16. 面试话术提纲

### 16.1 30 秒项目介绍

「必嗨 IM 是 C+Go 分层架构：C 做 frwder 高 IO 路由和 TCP 接入，Go 做业务。群聊 Demo 走 HTTP 注册、WS 长连接、RTMQ 中转；**usrsvr 管群成员**，**msgsvr 管群消息**；下行是 **gid→nid→cid 两段 fan-out**，和当年大规模弹幕是同一套寻址思路。」

### 16.2 3 分钟讲清 GROUP-CHAT

1. 客户端 `GROUP-CHAT` → websocket 补 nid → frwder FORWARD  
2. msgsvr 校验 Redis 成员 → 读 `gid→nid` 列表 → 对每个 nid `AsyncSend`  
3. frwder BACKEND 收到 → `async_send(FORWARD, nid)` → 目标 websocket  
4. websocket `TravImGroupSession` → 本节点所有群成员 WS 连接  
5. 异步 LPUSH Redis + Mongo 落库  

### 16.3 常问追问

| 问题 | 答要点 |
|------|--------|
| 为何分 usrsvr 和 msgsvr？ | 生命周期与消息 fan-out 解耦；msgsvr 可水平扩展读 gid→nid |
| frwder 挂会怎样？ | 全栈 RTMQ 断；status 看 9 进程，重启 runner |
| 如何保证多接入？ | 每 websocket 唯一 NID；Redis 记录 gid 在哪些 nid 有在线成员 |
| 和微信/WhatsApp 差异？ | Demo 单机栈；架构上预留 NID 多接入，未做全球多 Region |
| 群 Demo 占全系统多少？ | **架构主链路 ~70%**；**产品功能 ~30%**（还有弹幕、私聊、推送） |

---

## 附录 A：进程启动顺序（读 run-in-linux.sh）

1. frwder（等 28889）  
2. seqsvr  
3. msgsvr、tasker、usrsvr、monitor、chatroom  
4. websocket（及 multinode 的 websocket-2、listend-2）  
5. listend 前台保活  

理解顺序：**先 frwder，再 BACKEND 业务，再 FORWARD 接入**。

---

## 附录 B：建议携带的代码阅读清单（打印勾选）

- [ ] `demo/web/group-app.js` — 客户端状态机  
- [ ] `lib/comm/mesg.go` — CMD 常量与包头  
- [ ] `clang/exec/frwder/frwd_mesg.c` — 上下行各 15 行  
- [ ] `websocket/mesg.go` + `upmesg.go` — 接入层  
- [ ] `usrsvr/gmesg.go` + `lib/chat/group_ops.go` — 群生命周期  
- [ ] `msgsvr/gmesg.go` — GROUP-CHAT  
- [ ] `lib/chat_tab/chat.go` — ImGroup 第二段 fan-out  
- [ ] `tools/smoke-group/main.go` — 无 UI 协议对照  

---

*文档版本：与 docker-compose demo-web、websocket ImGroup fan-out、group.html 对齐。维护时请同步更新 [GROUP_DEMO_INTERFACES.md](GROUP_DEMO_INTERFACES.md)。*
