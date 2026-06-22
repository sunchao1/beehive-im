# 上行 → 下行：消息推到用户面板全流程

> **「用户消息面板」** = 客户端 UI（如 `demo/web` 的 `#log` 区域）：收到 `ROOM-CHAT` / `ROOM-BC` 后解析 Protobuf，渲染一行文字。  
> **本文两条主路径**：① 用户发弹幕（ROOM-CHAT） ② 运营/系统 push（HTTP `/room/push` → ROOM-BC）。  
> **图例**：**↑ 上行** Client→服务端　**↓ 下行** 服务端→Client　**服务** usrsvr / websocket / frwder / chatroom　**分层 Fan-out 寻址** ① RID→NID ② (RID,GID)→(SID,CID) ③ CID→fd（详 [弹幕系统的名词解释.md §5](../弹幕系统的名词解释.md#5-分层-fan-out-寻址从-rid-到-cid)）　🔴 同步阻塞　🟢 异步后台　⚪ 非阻塞入队

---

## 目录

1. [总览：两条 push 路径](#1-总览两条-push-路径)
2. [路径 A：用户发弹幕 → 别人面板显示](#2-路径-a用户发弹幕--别人面板显示)
3. [路径 B：运营 HTTP push → 在线用户面板](#3-路径-b运营-http-push--在线用户面板)
4. [接入层：从包到 fd 的最后一百米](#4-接入层从包到-fd-的最后一百米)
5. [客户端：面板如何收消息](#5-客户端面板如何收消息)
6. [与乐视「触达 push」对照](#6-与乐视触达-push对照)

---

## 1. 总览：两条 push 路径

```mermaid
flowchart TB
  subgraph pathA [路径 A — 用户弹幕 ROOM-CHAT]
    A1[用户 A 输入文字点发送] --> A2[WS 上行 0x040B]
    A2 --> A3[chatroom 校验 + fan-out]
    A3 --> A4[各 websocket 同房推送]
    A4 --> A5[用户 B/C… 面板显示一行]
    A3 --> A6[ROOM-CHAT-ACK → A 发送框反馈]
  end

  subgraph pathB [路径 B — 系统公告 ROOM-BC]
    B1[运营 curl POST /room/push] --> B2[chatroom HTTP]
    B2 --> B3[写 Redis BC 集合 + 组 ROOM-BC 包]
    B3 --> B4[按 lsnd nid 列表 fan-out]
    B4 --> B5[websocket 同房 TravRoomSession]
    B5 --> B6[在线用户面板显示公告]
  end

  subgraph shared [共用基础设施]
    WS[websocket / listend]
    FWD[frwder RTMQ]
    R[(Redis 拓扑)]
  end

  A2 --> WS --> FWD
  B4 --> FWD --> WS
  A3 --> R
  B3 --> R
```

| 对比 | 路径 A ROOM-CHAT | 路径 B ROOM-BC |
|------|------------------|----------------|
| 触发 | 客户端 WS 上行 | HTTP `POST :8004/room/push?dim=room&rid=` |
| 业务入口 | chatroom RTMQ handler | chatroom HTTP `push.go` |
| 下行 cmd | `0x040B` ROOM-CHAT | `0x040D` ROOM-BC |
| fan-out 依据 | **rid → nid**（在房用户所在接入节点） | 当前实现：**全集群 lsnd nid**（已知可优化为仅本 rid 相关 nid） |
| 面板渲染 | `decodeRoomChat` → `[uid] text` | `decodeBc` → 内嵌 chat → 同上 |
| 面试说法 | 互动弹幕 / 同房 fan-out | **系统消息触达 / 公告 push** |

---

## 2. 路径 A：用户发弹幕 → 别人面板显示

### 2.1 端到端泳道图（详细）

```mermaid
sequenceDiagram
    autonumber
    box 客户端 A（发送方）
        participant UI_A as 消息面板/输入框
        participant JS_A as demo/web app.js
    end
    box 接入层 Go
        participant WS_A as websocket<br/>cid=100 sid=A
    end
    box 网关 C
        participant FWD as frwder
    end
    box 业务 Go
        participant CR as chatroom
        participant BG as 落库协程
    end
    box 接入层 Go
        participant WS_B as websocket<br/>cid=200 sid=B
    end
    box 客户端 B（接收方）
        participant JS_B as app.js
        participant UI_B as 消息面板 #log
    end

    Note over UI_A,UI_B: 前置：双方均已 register → iplist → WS 连接 → ONLINE → ROOM-JOIN 同房

    UI_A->>JS_A: 输入「你好」点发送
    JS_A->>JS_A: PB.encodeRoomChat<br/>{uid, rid, gid, text}
    JS_A->>WS_A: WS 帧：48B头 + ROOM-CHAT 0x040B

    rect rgb(255,235,235)
        Note over WS_A,FWD: 🔴 上行
        WS_A->>WS_A: recv_routine ReadMessage 🔴
        WS_A->>WS_A: LsndMesgCommHandler 🔴
        WS_A->>FWD: frwder.AsyncSend ⚪
        FWD->>CR: RTMQ → worker 收包 🔴
    end

    rect rgb(235,255,235)
        CR->>CR: parse + validate 🔴
        CR->>BG: room_mesg_chan 🟢（Mongo/Redis 历史）
    end

    alt 同步 ACK（默认）
        rect rgb(255,235,235)
            CR->>CR: 查 rid→nid 列表 🔴
            loop 每个 nid
                CR->>FWD: sendData(ROOM-CHAT) ⚪
            end
            CR->>FWD: roomChatAck → A
        end
    else 异步 ACK BEEHIVE_CHATROOM_ASYNC_BROADCAST=1
        CR->>CR: room_broadcast_chan 🟢
        CR->>FWD: roomChatAck → A（立即）
        CR->>FWD: 后台 worker fan-out
    end

    rect rgb(255,235,235)
        Note over FWD,UI_B: 🔴 下行 fan-out（每个接入节点）
        FWD->>WS_A: ROOM-CHAT 到 NID=20001
        FWD->>WS_B: ROOM-CHAT 到 NID=20001（或另一 NID）
        WS_B->>WS_B: LsndUpMesgRoomChatHandler 🔴
        WS_B->>WS_B: TravRoomSession(rid)<br/>遍历同房 cid 🔴
        loop 每个旁观连接
            WS_B->>WS_B: AsyncSend(cid) → sendq ⚪
            WS_B->>WS_B: send_routine WriteMessage 🔴
        end
    end

    WS_B->>JS_B: WS onmessage 二进制帧
    JS_B->>JS_B: parseHeader + decodeRoomChat
    JS_B->>UI_B: log `[uid] 你好` → 面板多一行

    FWD->>WS_A: ROOM-CHAT-ACK
    WS_A->>JS_A: ACK（发送方可选展示「已发送」）
```

### 2.2 分层数据流（同一条弹幕）

```mermaid
flowchart LR
  subgraph L0 [L0 用户面板]
    IN[输入框 Send]
    OUT[面板 #log 新增一行]
  end

  subgraph L1 [L1 客户端协议]
    PB1[Protobuf<br/>MesgRoomChat]
    HDR[48 字节 MesgHeader<br/>cmd=0x040B sid cid seq]
  end

  subgraph L2 [L2 WebSocket/TCP]
    WSFR[WS 帧 或 TCP 字节流]
    FD[(fd 内核写)]
  end

  subgraph L3 [L3 接入进程 websocket]
    POOL[(ConnPool<br/>cid→Client)]
    TAB[(ChatTab<br/>rid→sid,cid)]
    SQ[sendq chan]
  end

  subgraph L4 [L4 frwder]
    RT[RTMQ 按 nid 路由]
  end

  subgraph L5 [L5 chatroom]
    RN[rid→nid 列表]
    ACK[ROOM-CHAT-ACK]
  end

  IN --> PB1 --> HDR --> WSFR
  WSFR -->|上行| RT --> RN
  RN -->|每 nid 一包| RT
  RT -->|下行 ROOM-CHAT| TAB
  TAB -->|TravRoomSession| SQ --> WSFR --> FD --> OUT
  RN --> ACK
```

### 2.3 关键步骤对照表

| 步骤 | 位置 | 输入 | 输出 | 类型 |
|------|------|------|------|------|
| 组包 | app.js | text | WS BinaryMessage | 🔴 客户端 |
| 上行转发 | websocket | ROOM-CHAT | frwder 队列 | ⚪ |
| 业务处理 | chatroom | rid, text | nid 列表 + ACK | 🔴/🟢 |
| 跨节点投递 | frwder | nid=20001 | websocket 进程 | 🔴 路由 |
| 节点内广播 | websocket | rid | 每个 cid 的 sendq | 🔴+⚪ |
| 写 socket | send_routine | sendq | fd → 内核 | 🔴 |
| 渲染面板 | app.js | ROOM-CHAT body | DOM 一行 | 🔴 客户端 |

---

## 3. 路径 B：运营 HTTP push → 在线用户面板

### 3.1 端到端时序图（触达 / 公告）

```mermaid
sequenceDiagram
    autonumber
    participant OP as 运营/脚本 curl
    participant CR as chatroom HTTP :8004
    participant RDS as Redis
    participant FWD as frwder
    participant WS as websocket
    participant JS as 客户端 app.js
    participant UI as 消息面板

    Note over OP,UI: 用户已 ONLINE + ROOM-JOIN；面板空或已有历史

    OP->>CR: POST /room/push?dim=room&rid=10001&expire=3600<br/>Body: PB(MesgRoomChat 或透传 bytes)

    CR->>RDS: INCR room msgid
    CR->>RDS: HSET/ZADD room:rid:10001:broadcast:*
    CR->>RDS: ZRANGEBYSCORE im:lsn:nid 得 nid 列表

    loop 每个 lsnd NID（demo 扫全集群）
        CR->>FWD: AsyncSend ROOM-BC 0x040D ⚪
    end

    CR-->>OP: HTTP JSON {code:0}

    FWD->>WS: 下行 ROOM-BC（head.nid=本节点）

    WS->>WS: LsndUpMesgRoomBcHandler
    WS->>WS: TravRoomSession(rid, gid=0)<br/>只推 **本节点且在房** 的连接 🔴

    loop 每个 cid 在房
        WS->>WS: sendq → send_routine → fd
    end

    WS->>JS: WS BinaryMessage cmd=0x040D
    JS->>JS: decodeBc → decodeRoomChat
    JS->>UI: log `[uid] 公告内容` → 面板显示
```

### 3.2 HTTP push 与 ROOM-CHAT 下行差异

```mermaid
flowchart TB
  subgraph trigger [触发方式]
    U[用户 WS 上行]
    H[HTTP POST /room/push]
  end

  subgraph chatroom [chatroom 内]
    H1[ChatRoomChatHandler]
    H2[RoomPush pushHandler]
  end

  subgraph down [下行包]
    P1[CMD_ROOM_CHAT 0x040B]
    P2[CMD_ROOM_BC 0x040D]
  end

  subgraph acc [websocket 下行 handler]
    D1[LsndUpMesgRoomChatHandler]
    D2[LsndUpMesgRoomBcHandler]
  end

  subgraph panel [用户面板 app.js]
    R1[case ROOM_CHAT]
    R2[case ROOM_BC]
  end

  U --> H1 --> P1 --> D1 --> R1
  H --> H2 --> P2 --> D2 --> R2
  R1 --> LOG[#log 面板]
  R2 --> LOG
```

**代码位置**：

- HTTP push：`src/golang/exec/chatroom/controllers/push.go` → `pushHandler`
- 客户端渲染：`demo/web/app.js` → `case CMD.ROOM_CHAT` / `ROOM_BC`

---

## 4. 接入层：从包到 fd 的最后一百米

```mermaid
flowchart TB
  subgraph frwder_in [frwder 送来一包 ROOM-CHAT/BC]
    PKT[MesgHeader.nid = 本 websocket NID<br/>+ Protobuf body]
  end

  subgraph ws_proc [websocket 进程内]
    H[RTMQ 下行 handler]
    T[ChatTab.TravRoomSession<br/>rid → 多个 sid,cid]
    AS[AsyncSend cid, bytes]
    Q[Client.sendq chan]
    SR[send_routine goroutine]
    WM[conn.WriteMessage WS帧]
  end

  subgraph os [操作系统]
    TCP[TCP 连接 同一 fd]
  end

  PKT --> H --> T
  T -->|每个在房 cid| AS --> Q --> SR --> WM --> TCP
```

| 对象 | 谁维护 | 作用 |
|------|--------|------|
| **fd** | OS；Go runtime 封装 | 一条 TCP 连接的内核句柄 |
| **websocket.Conn** | `lib/lws` Client | 封装 fd + WS 协议 |
| **cid** | websocket 进程内自增 | 找 `ConnPool[cid]` |
| **ChatTab (sid,cid)** | websocket | 知道该连接 join 了哪些 **rid** |
| **sendq** | 每 Client 一个 chan | 背压；满则 drop（SENDQ=8192） |

**listend（TCP）路径**：结构相同，只是没有 WS 帧，直接 `writev` 到 fd；handler 为 `lsnd_upmesg_room_chat_handler`。

---

## 5. 客户端：面板如何收消息

### 5.1 demo/web 消息流

```mermaid
flowchart LR
  WS[WebSocket onmessage] --> PARSE[parseHeader 48B]
  PARSE --> CMD{cmd?}
  CMD -->|0x040B| CHAT[decodeRoomChat]
  CMD -->|0x040D| BC[decodeBc → decodeRoomChat]
  CMD -->|0x0406| JOIN[JOIN-ACK 启用输入框]
  CHAT --> LOG[log 函数]
  BC --> LOG
  LOG --> DOM[#log div 追加 div.msg]
```

### 5.2 用户视角时间线

```text
时间 ──────────────────────────────────────────────────────────────►

[连接阶段]
  注册 → iplist → WS.connect → ONLINE-ACK → ROOM-JOIN-ACK → 输入框可用

[发弹幕 — 路径 A]
  A 输入「你好」→ Send
  … 100～500ms（视 fan-out 负载）…
  B 面板出现：[100001] 你好
  A 可选收到 ROOM-CHAT-ACK（demo 未单独展示）

[收公告 — 路径 B]
  运营 curl push
  … 秒级内 …
  所有 **在线且已 join 该 rid** 的用户面板出现公告行
```

### 5.3 面板收不到消息的常见断点

| 断点 | 现象 | 查什么 |
|------|------|--------|
| 未 JOIN | 连接正常但无 ROOM-CHAT | JOIN-ACK code |
| rid 不在 ChatTab | 下行被 TravRoomSession 跳过 | 是否同房 |
| room status=0 | JOIN 失败 Room closed | MySQL CHAT_ROOM_INFO_TAB |
| frwder 挂 | iplist 失败 | runner 进程 |
| sendq 满 | 部分用户收不到 | SENDQ drop 日志 |
| 未解析 ROOM-BC | 公告乱码 | app.js decodeBc |

---

## 6. 与乐视「触达 push」对照

```mermaid
flowchart LR
  subgraph le [乐视 触达 2.0 概念]
    L1[运营配置公告/活动]
    L2[在线用户]
    L3[离线用户 inbox]
  end

  subgraph demo [本 demo 对应]
    D1[POST /room/push]
    D2[路径 B ROOM-BC fan-out]
    D3[未完整 demo 灌库拉取]
  end

  L1 -.-> D1
  L2 -.-> D2
  L3 -.-> D3
```

| 乐视说法 | 本系统步骤 |
|----------|------------|
| 在线 push 秒级触达 | 路径 B：HTTP → ROOM-BC → websocket → 面板 |
| 用户互动弹幕 | 路径 A：ROOM-CHAT |
| 离线 eventual | Mongo/Redis 历史 + 未来 SYNC（demo 弱） |
| 接入水平扩展 | 多 NID websocket + frwder 路由 |

---

## 7. 相关文档

| 文档 | 内容 |
|------|------|
| [ROOM_CHAT_SEQUENCE.md](ROOM_CHAT_SEQUENCE.md) | ROOM-CHAT 逐步 🔴🟢⚪ 标注 |
| [FLOWS_AND_GLOSSARY.md](../FLOWS_AND_GLOSSARY.md) | 全景 + 术语 |
| [ARCHITECTURE_DIAGRAM.md](ARCHITECTURE_DIAGRAM.md) | 一页拓扑 |
| [demo/web/app.js](../../demo/web/app.js) | 面板渲染代码 |

---

*建议：面试演示时 **路径 B curl 一条** + **路径 A 双浏览器互发** 各走一遍，指 `#log` 面板变化对应上表步骤。*
