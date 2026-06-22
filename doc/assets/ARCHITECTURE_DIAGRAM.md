# beehive-im 演示架构图（一页）

> 面试/演示用单页视图。单机 Docker demo 拓扑；生产扩展见 [SCALE.md](../SCALE.md)。

```mermaid
flowchart TB
  subgraph clients [客户端]
    B1[浏览器 A uid=100001]
    B2[浏览器 B uid=100002]
    CLI[smoke / loadtest]
  end

  subgraph access [接入层 C/Go]
    WS1[websocket :8002 NID=20001]
    WS2[websocket-2 :8003 NID=20002]
    TCP[listend :9002 NID=10001]
  end

  subgraph gateway [网关层 C]
    FWD[frwder RTMQ<br/>28888 前端 / 28889 后端]
  end

  subgraph gobiz [业务层 Go]
    USR[usrsvr :8000<br/>注册/iplist/群生命周期/推送]
    ROOM[chatroom :8004<br/>聊天室/ROOM-BC HTTP]
    MSG[msgsvr<br/>私聊/群聊 fan-out]
    MON[monitor<br/>接入点注册]
    SEQ[seqsvr :50000<br/>rid/gid 分配]
    TSK[tasker]
  end

  subgraph store [存储]
    R[(Redis<br/>会话/房间/群拓扑)]
    M[(MySQL<br/>房间元数据)]
    MG[(Mongo<br/>消息落库)]
  end

  B1 --> WS1
  B2 --> WS2
  CLI --> WS1
  CLI --> TCP

  WS1 <-- RTMQ --> FWD
  WS2 <-- RTMQ --> FWD
  TCP <-- RTMQ --> FWD

  FWD --> USR
  FWD --> ROOM
  FWD --> MSG
  FWD --> MON

  USR --> R
  ROOM --> R
  ROOM --> M
  ROOM --> MG
  MSG --> R
  MSG --> MG
  USR --> SEQ
  ROOM --> SEQ

  WS1 -. LSND_INFO .-> MON
  WS2 -. LSND_INFO .-> MON
  MON --> R
  USR --> R
```

## 数据流（聊天室弹幕 · 分层 Fan-out 寻址）

1. 客户端 `ONLINE` → websocket → frwder → **usrsvr**（写 Redis 会话）
2. `ROOM-JOIN rid` → frwder → **chatroom**（写 rid↔sid↔nid 拓扑；websocket ChatTab 写 rid↔gid↔(sid,cid)）
3. `ROOM-CHAT` → **①** chatroom **RID→[NID]** → frwder → **②** 各 websocket `TravRoomSession(rid,gid)` → **③** `AsyncSend(cid)` → 客户端
4. 可选：`POST /room/push` → **ROOM-BC** 系统弹幕（下行 ②③ 同 ROOM-CHAT）

**详述**：[弹幕系统的名词解释.md](../弹幕系统的名词解释.md) §5

## 与群聊/推送的区别

| 能力 | 入口服务 | 广播依据 |
|------|----------|----------|
| 聊天室 | chatroom | Redis `room:rid:{rid}:to:nid:zset` |
| 群聊 | usrsvr + msgsvr | Redis 群成员 + gid→nid |
| 推送 BC/P2P | usrsvr HTTP | uid→sid→nid 或全站 lsnd |
