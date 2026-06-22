# beehive-im 技术架构文档

> 版本：v.1.1  
> 分支：feature_sunchao  
> 最后更新：2026-06-12

---

## 1. 项目概述

**beehive-im（必嗨 IM）** 是一套面向高并发的即时通信系统，设计目标为千万级并发在线，功能涵盖私聊、群聊、聊天室与推送。

| 维度 | 说明 |
|------|------|
| 语言 | C（接入/网关/RTMQ）+ Go（业务服务） |
| 协议 | 48 字节自定义二进制头 + Protobuf 报体 |
| 依赖管理 | C 第三方库手动下载；Go 使用 legacy GOPATH + govendor |
| 容器化 | 无 Docker / K8s / CI |
| 当前状态 | 开发中，部分协议与 HTTP 接口未完成 |

---

## 2. 整体架构

### 2.1 分层架构图

```
┌─────────────────────────────────────────────────────────────┐
│                        客户端层                              │
│   TCP 长连接          WebSocket           HTTP REST          │
└──────────┬─────────────────┬──────────────────┬─────────────┘
           │                 │                  │
┌──────────▼─────────┐ ┌─────▼──────────┐ ┌────▼────────────┐
│  C listend :9002   │ │ Go websocket   │ │ usrsvr  :8000   │
│  (TCP 接入层)       │ │ :8002 (WS接入) │ │ chatroom:8004   │
└──────────┬─────────┘ └─────┬──────────┘ └────┬────────────┘
           │                 │                  │
           └────────────┬────┘                  │
                        │                       │
           ┌────────────▼───────────────────────▼──────────┐
           │           C frwder (消息网关)                   │
           │   FORWARD :28888  ←→  BACKEND :28889           │
           │              自研 RTMQ 消息总线                   │
           └────────────┬───────────────────────────────────┘
                        │
     ┌──────────────────┼──────────────────┐
     │                  │                  │
┌────▼─────┐    ┌───────▼──────┐   ┌──────▼──────┐
│  usrsvr  │    │   msgsvr     │   │  chatroom   │
│ 用户中心  │    │  消息中心     │   │  聊天室     │
└────┬─────┘    └───────┬──────┘   └──────┬──────┘
     │                  │                  │
┌────▼──────────────────▼──────────────────▼──────┐
│  seqsvr :50000  │  tasker  │  monitor           │
│  (Thrift RPC)   │  (定时)  │  (监控)            │
└────────────────────┬─────────────────────────────┘
                     │
     ┌───────────────┼───────────────┐
     │               │               │
┌────▼────┐   ┌──────▼─────┐  ┌─────▼─────┐
│  Redis  │   │   MySQL    │  │  MongoDB  │
│  状态层  │   │  元数据     │  │  消息历史  │
└─────────┘   └────────────┘  └───────────┘
```

### 2.2 核心组件

| 组件 | 实现 | 默认端口 | 职责 |
|------|------|----------|------|
| listend | C | 9002 | TCP 长连接接入，维护 SID↔CID 会话表 |
| websocket | Go | 8002 | WebSocket 接入（与 listend 功能重叠） |
| frwder | C | 28888/28889 | 消息转发中枢，连接接入层与业务层 |
| RTMQ | C + Go Proxy | — | 自研 TCP 消息中间件（鉴权、保活、路由） |
| usrsvr | Go | 8000 | 注册、IP 列表、上线鉴权 |
| msgsvr | Go | — | 私聊、SYNC 等消息处理 |
| chatroom | Go | 8004 | 聊天室加入/退出/弹幕/广播 |
| seqsvr | Go + Thrift | 50000 | SID / RID / SEQ 全局 ID 分配 |
| tasker | Go | — | Redis TTL 清理、在线人数统计 |
| monitor | Go | — | 侦听层/转发层状态上报 |

---

## 3. 消息协议

### 3.1 报头格式（48 字节）

| 字段 | 类型 | 长度 | 含义 |
|------|------|------|------|
| type | uint32 | 4 | 命令 ID |
| length | uint32 | 4 | 报体长度（不含报头） |
| sid | uint64 | 8 | 会话 ID（每设备一个） |
| cid | uint64 | 8 | 连接 ID（节点内唯一） |
| nid | uint32 | 4 | 节点 ID（内部路由用） |
| seq | uint64 | 8 | 流水号 |
| dsid | uint64 | 8 | 目标会话 ID |
| dseq | uint64 | 8 | 目标流水号 |

详见 `doc/PROTOCOL.md`。

### 3.2 RTMQ 路由

**上行（客户端 → 业务）**：
```
Client → listend/websocket → frwder FORWARD → rtmq_publish → BACKEND → Go 服务
```

**下行（业务 → 客户端）**：
```
Go 服务 → frwder BACKEND → 按 nid 路由 → listend/websocket → Client
```

核心代码：`src/clang/exec/frwder/frwd_mesg.c`

### 3.3 分层 Fan-out 寻址（聊天室 / 弹幕核心设计）

聊天室下行（`ROOM-CHAT`、`ROOM-BC`）不采用「业务层按 SID 逐连接推送」，而是 **分层 Fan-out 寻址**（*Hierarchical Fan-out Addressing*）：

```text
(RID, GID) ──① 拓扑路由──► [NID…] ──② 会话展开──► [(SID,CID)…] ──③ 连接投递──► Client
```

| 段 | 名称 | 执行方 | 输入 → 输出 | 依据 |
|----|------|--------|-------------|------|
| **①** | 拓扑路由 | **chatroom** + frwder | **RID → [NID…]** | Redis `room:rid:{rid}:to:nid:zset` |
| **②** | 会话展开 | **websocket** ChatTab | **(RID,GID) → [(SID,CID)…]** | 内存 `TravRoomSession`（JOIN 时写入） |
| **③** | 连接投递 | **websocket** lws | **CID → fd** | `LwsCntx.pool[cid]` |

**要点**：

- chatroom 向每个 **NID** 发 **一条** RTMQ（`sendData` 时 `cid=0`），**不是**向整节点所有连接广播。
- **RID 过滤在接入层 ②** 完成；同一 NID 上多间房的连接互不干扰。
- 与 **NID 路由**、**接入/业务分离**、**Redis 拓扑注册** 共同构成必嗨 IM 水平扩展的基础。

**详述**：[弹幕系统的名词解释.md](弹幕系统的名词解释.md) §5 · [FLOWS §1.2](FLOWS_AND_GLOSSARY.md#12-一条消息的通用路径)

---

## 4. 核心业务流程

### 4.1 用户连接

```
1. GET /im/register?uid=...        → usrsvr → seqsvr 分配 SID
2. GET /im/iplist?type=2&uid=...    → usrsvr → ipdict 选接入点 + 生成 token
3. 连接 listend:9002 或 ws://host:8002/im
4. 发送 CMD_ONLINE (0x0101)        → 侦听层 → RTMQ → usrsvr
5. usrsvr 校验 token、写 Redis、分配 seq → CMD_ONLINE_ACK
6. 侦听层更新 SID↔CID 映射，状态变为 LOGIN
```

### 4.2 私聊消息（CMD_CHAT 0x0201）

```
Client → 侦听层 → RTMQ → msgsvr
  → 异步写 Mongo
  → 查 Redis im:uid:{uid}:to:sid:set → 发给双方所有在线终端
  → CMD_CHAT_ACK 回复发送方
```

### 4.3 聊天室 / 弹幕消息

项目中无独立「弹幕」模块，直播弹幕由聊天室子系统承载：

| 命令 | ID | 用途 |
|------|-----|------|
| ROOM-CHAT | 0x040B | 用户发送聊天室文本（含 level/text/data） |
| ROOM-BC | 0x040D | 服务端广播（含 expire，适合系统弹幕） |
| HTTP POST /room/push | — | 运营侧推送 |

**ROOM-CHAT 广播流程**（`broadcast_async.go` / `mesg.go`）— 即 **§3.3 分层 Fan-out 寻址** 的实现：

```
1. 异步写入 Mongo 历史（room_mesg_chan）
2. ① 拓扑路由：查 rid → nid 列表（room.node / Redis room:rid:{rid}:to:nid:zset）
3. 对每个 nid：sendData(ROOM-CHAT, cid=0, targetNid) → frwder BACKEND
4. 各 websocket：② TravRoomSession(rid,gid) → ③ AsyncSend(cid) 写 WS
5. 回复 ROOM-CHAT-ACK 给发送方（单播，带发送方 SID/CID/NID）
```

**大房间分片**：每组最多 10000 人（`CHAT_ROOM_GROUP_MAX_NUM`），通过 Redis 键管理分组与侦听层分布。详见 `doc/REDIS.md`、[弹幕系统的名词解释.md](弹幕系统的名词解释.md)。

---

## 5. 存储设计

### 5.1 Redis（核心状态层）

| 键模式 | 用途 |
|--------|------|
| `im:sid:zset` | 在线会话 SID 集合 |
| `im:sid:{sid}:attr` | 会话属性（UID/NID/CID） |
| `im:uid:{uid}:to:sid:set` | 用户多终端 SID 集合 |
| `chat:rid:*` | 聊天室分组、人数、侦听层分布 |
| `chat:lsn:nid:*` | 侦听层拓扑注册 |
| `chat:fwd:nid:*` | 转发层拓扑注册 |

### 5.2 MySQL

| 表 | 用途 |
|----|------|
| IM_SID_GEN_TAB | SID 段分配 |
| IM_SEQ_GEN_TAB | SEQ 段分配 |
| CHAT_ROOM_INFO_TAB | 聊天室元信息 |
| IM_RID_GEN_TAB | RID 段分配 |

建表脚本：`doc/database/mysql.sh`

### 5.3 MongoDB

- 私聊/聊天室消息历史
- 聊天室黑名单
- 集合索引：`RoomMesg`, `RoomBlacklist`（`doc/database/mongo.eval`）

---

## 6. 水平扩展设计

- **分层 Fan-out 寻址**（§3.3）：业务层 **RID→NID** 拓扑路由 + 接入层 **(RID,GID)→(SID,CID)** 会话展开；扩接入 = 扩 **NID**，非 chatroom 逐连接推送
- **NID / GID**：每个进程配置唯一节点 ID；聊天室 **GID** 为大房分片（每组 ≤1 万人）
- **拓扑注册**：侦听层/转发层在 Redis 注册地址映射（`im:lsnd:nid:*`、`room:rid:*:to:nid:*`）
- **智能接入**：`conf/ipdict.txt` 按 IP 地理/运营商选择最优接入点
- **聊天室分片**：按 GID 分组，多侦听层分担同一房间

---

## 7. 目录结构

```
beehive-im/
├── Makefile              # 统一编译入口
├── make/                 # 编译规则
├── 3rd/                  # C 第三方库下载脚本
├── bin/                  # 编译产物 + start.sh / stop.sh
├── conf/                 # 全部服务 XML 配置
├── doc/                  # 协议、API、Redis、数据库文档
├── src/
│   ├── clang/
│   │   ├── lib/          # C 核心库（core, access, rtmq, sdk, chat, mesg）
│   │   ├── exec/         # C 可执行程序（listend, frwder, client）
│   │   └── demo/         # C WebSocket demo
│   └── golang/
│       ├── lib/          # Go 公共库（comm, rtmq, rdb, mongo, lws, mesg...）
│       ├── exec/         # Go 业务服务
│       ├── demo/         # Go 示例
│       └── vendor/       # Go 依赖（govendor 管理）
└── tools/                # 运维脚本
```

---

## 8. 构建与部署

### 8.1 编译

```bash
export GOPATH=/path/to/parent   # 需含 beehive-im 目录
export GO111MODULE=off
make all                        # 全量编译 → bin/*.v.1.1
make DIR=src/golang/exec/usrsvr # 单模块编译
```

### 8.2 启动顺序（bin/start.sh）

```
1. frwder          （消息网关，必须先启动）
2. seqsvr          （ID 分配服务）
3. msgsvr          （消息中心）
4. tasker          （定时任务）
5. usrsvr          （用户中心）
6. monitor         （监控）
7. chatroom        （聊天室）
8. websocket       （WS 接入）
9. listend         （TCP 接入）
```

**前置依赖**（需手动启动）：
- Redis：`redis-server ../conf/redis.conf`
- MySQL：执行 `doc/database/mysql.sh`
- MongoDB：执行 `doc/database/mongo.eval`

### 8.3 端口一览

| 组件 | 端口 |
|------|------|
| usrsvr HTTP | 8000 |
| websocket WS | 8002 |
| chatroom HTTP | 8004 |
| listend TCP | 9002 |
| seqsvr Thrift | 50000 |
| frwder FORWARD | 28888 |
| frwder BACKEND | 28889 |
| Redis | 6379 |

---

## 9. 功能完成度

| 模块 | 状态 |
|------|------|
| 通用 ONLINE/OFFLINE/PING/SYNC/KICK | ✅ 已实现 |
| 私聊 CHAT / 黑名单 / 禁言 | ✅ 已实现 |
| 聊天室 JOIN/QUIT/CHAT/BC/KICK/统计 | ✅ 已实现 |
| 群聊全部 (0x03xx) | ❌ 未实现 |
| 推送 BC/P2P (0x05xx) | ❌ 未实现 |
| 聊天室 CREAT/DISMISS | ❌ 未实现 |
| HTTP 大量 query/config 接口 | ❌ 未完成 |
| 敏感词过滤 | ❌ TODO |

详见 `doc/COMMAND.md` 与 `doc/HTTPSVR.md`。

---

## 10. 可运行性评估

### 10.1 当前阻塞项

| 问题 | 说明 |
|------|------|
| 无预编译二进制 | bin/ 目录无 .v.1.1 可执行文件 |
| C 第三方库缺失 | 3rd/ 需执行 download.sh 并编译 |
| Makefile macOS 兼容 | func_cpu_cores 依赖 /proc/cpuinfo；第 70 行注释导致 shell 续行失效 |
| Go vendor 不完整 | vendor/ 仅有 vendor.txt，需 govendor fetch |
| GOPATH 布局 | import 路径为 beehive-im/src/golang/...，需 $GOPATH/src/beehive-im |
| 基础设施 | Redis / MySQL / MongoDB 需手动安装配置 |

### 10.2 最小可运行步骤

```bash
# 1. GOPATH 布局
ln -s /path/to/beehive-im $GOPATH/src/beehive-im

# 2. 下载并编译 C 第三方库
cd 3rd && ./download.sh

# 3. 拉 Go 依赖
cd src/golang && govendor fetch ...

# 4. 编译（Linux 环境，或修复 Makefile macOS 兼容）
export GOPATH=... GO111MODULE=off
make all

# 5. 启动基础设施
redis-server conf/redis.conf
mysql < doc/database/mysql.sh

# 6. 启动服务
cd bin && ./start.sh
```

---

## 11. 百万并发弹幕差距分析

### 11.1 架构层

| 缺口 | 现状 | 百万级需求 |
|------|------|------------|
| 弹幕广播 | 逐节点 sendData | fan-out 优化、批量聚合、专用弹幕通道 |
| 热路径持久化 | 每条消息同步入 channel 写 Mongo | 先广播后异步落库 |
| 限流与背压 | 无 | 用户/房间 QPS 限制、优先级队列 |
| 消息采样 | 无 | 超大规模房间客户端合并 + 服务端抽样 |
| 敏感词/风控 | TODO | 实时过滤、黑名单、反垃圾 |
| 网关集群 | start.sh 单 frwder | 多实例 + 一致性哈希 |
| Redis | 单节点 | Redis Cluster 分片 |

### 11.2 工程层

- 无容器化 / K8s / 弹性伸缩
- 无自动化测试与压测基准
- 无可观测性（Metrics / Tracing / 告警）
- 无 CI/CD
- 技术栈老化（beego、mgo、GOPATH）

### 11.3 弹幕特有能力

- 弹幕样式/轨道/透明度：协议有扩展字段，无专用渲染协议
- 礼物/付费弹幕优先通道：未实现
- 历史弹幕回放：无专用 API
- 弹幕密度自适应：未实现

---

## 12. 项目优点

1. **清晰的分层解耦**：接入 → RTMQ 网关 → 业务 → 存储，职责边界明确
2. **自研 RTMQ 消息总线**：内部通信完全可控，不依赖外部 MQ
3. **C + Go 混合架构**：C 处理高 I/O，Go 处理业务，兼顾性能与效率
4. **高效二进制协议**：固定头 + Protobuf，省带宽、解析快
5. **完善的协议文档**：PROTOCOL / COMMAND / HTTPSVR / REDIS 等文档体系
6. **水平扩展预留**：NID/GID、聊天室分组、Redis 拓扑注册
7. **智能接入调度**：ipdict 按 IP 选接入点
8. **会话管理成熟**：SID/CID 双键、重复登录踢下线、心跳保活
9. **聊天室核心链路可用**：JOIN/QUIT/CHAT/BC/KICK/统计
10. **C 核心库丰富**：共享内存、内存池、红黑树等长期 IM 沉淀

---

## 13. 相关文档索引

| 文档 | 路径 | 内容 |
|------|------|------|
| 协议定义 | doc/PROTOCOL.md | 报头 + 全部 PB 消息 |
| 命令状态 | doc/COMMAND.md | WS/TCP 实现矩阵 |
| HTTP API | doc/HTTPSVR.md | REST 接口 |
| 错误码 | doc/ERRNO.md | 系统/服务级错误码 |
| Redis 键 | doc/REDIS.md | 聊天室键设计 |
| 侦听层设计 | doc/design/listend.md | SID/CID 表 |
| RTMQ API | doc/ctrl/rtmq.md | Go RTMQ Proxy |
| 数据库 | doc/database/ | MySQL / MongoDB 脚本 |
| 协议源码 | doc/mesg/mesg.proto | Protobuf 源定义 |
