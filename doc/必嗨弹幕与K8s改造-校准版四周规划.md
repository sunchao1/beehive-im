# 必嗨 IM / 弹幕 · 四周规划校准版

> **说明**：对照你提供的「2 阶段落地完整规划」，按 **beehive-im 仓库真实代码** 校准。  
> **原则**：保留正确方向（iplist 破局、双段 fan-out、K8s Ingress）；剔除 **仓库里不存在的 RocketMQ/Kafka 概念**；区分 **群聊 0x03xx** 与 **弹幕/聊天室 0x04xx**。

---

## 0. 必须先纠正的 5 个概念混同

| 你文档里的说法 | 必嗨实际 |
|----------------|----------|
| frwder = WebSocket 网关、百万 WS 连接 | **错**。WS 在 **Go websocket**；**frwder** 是 **RTMQ 双 Server 桥**（28888/28889），不终结浏览器连接 |
| RTMQ = Broker + Dledger + NameServer + PVC 持久化 + 死信队列 | **错**。RTMQ 是 **内存 TCP 总线**，设计文档 §11：**无 HA、无持久化、断线丢包** |
| MsgSvrGroupChatHandler + room_id | **半错**。该 handler 是 **群聊 gid**；弹幕是 **chatroom + ROOM-CHAT + rid→nid** |
| RTMQ 内置 iplist 做 Fanout 寻址 | **半错**。**iplist** 给 **客户端选接入 URL**；Fanout 寻址靠 **Redis gid→nid / rid→nid**，下行靠 **async_send(nid)** |
| frwder Deployment 无限 HPA + Redis 注册 nid | **半错**。**NID 来自配置文件**，不是 Pod 启动自注册；扩 ws Pod = **新 NID 新 xml**，不是同一 Deployment 克隆 |

**记忆**：客户端 → **websocket(NID)** → RTMQ FORWARD → **frwder publish** → **msgsvr/chatroom** → RTMQ BACKEND **async_send(nid)** → websocket → **ImGroup/ChatTab 第二段**。

---

## 1. 四周目标（校准后，可闭环）

| 周 | 目标 | 验收 |
|----|------|------|
| **1～2** | RTMQ 17 篇 + **群聊 10 人 5 接入**全链路 + **弹幕 rid→nid 对照读** | 70 RTMQ 包、双段 fan-out 白板、口述 30s |
| **3** | K8s 骨架跑通（**已有** `deploy/k8s/`）+ **iplist 方案 A** + Ingress | smoke 注册/上线/群聊 |
| **4** | 扩缩容/监控/故障演练（**诚实边界**）+ 架构说明文档 | Grafana 占位指标、delete pod 重连、**不宣称 Dledger** |

---

## 2. 前 2 周：源码与场景（按真实模块拆）

### 2.1 RTMQ 阅读顺序（7 天，对齐仓库）

| 天 | 读什么 | 对应你文档里哪块 | 备注 |
|----|--------|------------------|------|
| D1 | [00 总地图](rtmq/00-总地图.md) + [故事线](rtmq/群聊故事线-单用户对照17篇.md) | Fanout 总览 | 先建立地图 |
| D2 | 02/05/06 publish·async_send | 生产消费 | **不是** Kafka partition |
| D3 | 07/09/11 AUTH·SUB | 节点感知 | **SUB=进程级**，非用户 |
| D4 | 08 dist + frwd_mesg.c | 一级下行 | async_send(nid) |
| D5 | 13～17 Go Proxy | writev/sendq | 对应 AsyncSend |
| D6 | msgsvr gmesg + websocket upmesg | **二级** ImGroup | 群聊第二段 |
| D7 | chatroom + TravRoomSession | **二级** ChatTab | **弹幕第二段** |

**不要读（仓库无实现）**：Dledger、NameServer、死信队列、RTMQ 消息持久化 PVC、room QPS 分布式限流（群聊/弹幕 Demo 级无完整实现）。

### 2.2 验证场景（7 天，分两条业务线）

#### A. 群聊线（你已在做，10 人 × 5 NID）

| 场景 | 工具 | 看什么 |
|------|------|--------|
| 10 人建群各发 1 句 | smoke-group / group.html | **70 RTMQ 包**、10×10 WS 收包 |
| 跨 ws-1/ws-2 | 多节点 compose | Redis **gid→nid** 两个 NID |
| 停 1 个 ws Pod | compose restart | **async_send 失败**、该 NID 用户收不到 |

#### B. 弹幕线（chatroom 0x04xx，与群聊并行读）

| 场景 | 代码锚点 | 说明 |
|------|----------|------|
| 进房 ROOM-JOIN | chatroom + Redis **rid→nid** | GID 是大房分片，≠ 群 gid |
| 发 ROOM-CHAT | chatroom fan-out | 一级 **rid→nid**，二级 **TravRoomSession(rid,gid)** |
| 多 NID 同房 | [弹幕名词解释 §5](弹幕系统的名词解释.md) | 与群聊同 **RTMQ 模式**，业务 handler 不同 |

**你列的 10 场景（5000 人、敏感词、跨机房）**：属于 **产品化/压测二期**，当前 Demo **不具备** 完整限流/敏感词/多机房；2 周内改为 **「3 个必做 + 3 个选做」**，避免落空。

**2 周必做验收**：

1. 群聊 10 人脚本通过 + RTMQ 包计数  
2. 白板画清 **群聊 vs 弹幕** 两条 fan-out（handler 名不同，RTMQ 相同）  
3. 改造清单（见 §4）  

---

## 3. 后 2 周：六大问题 · 校准答案

### 问题 1：iplist + K8s 公网 IP

**你文档方案 A（Ingress 统一入口）→ ✅ 正确，且已在代码落地第一步**

- 实现：`BEEHIVE_WS_IPLIST=wss://域名/im`（[deploy/k8s/apps/usrsvr.yaml](../deploy/k8s/apps/usrsvr.yaml)）
- Compose **默认不变**：未设 env → 仍走 LSND_INFO → Redis

**需修正**：

- 废弃的是 **「iplist 返回 Pod IP」**，不是废弃 **NID**  
- Fanout **不读 iplist**；读 **Redis chat:gid:{gid}:to:nid** 或 **room:rid:*:to:nid**

### 问题 2：frwder 与 RTMQ 同进程

**✅ 瓶颈判断正确；❌ 拆分形态需校准**

| 现状 | 事实 |
|------|------|
| 同进程 | frwder **一个进程内** 2× RTMQ Server + frwd 桥 |
| 拆分目标 | **frwder 进程** 与 **websocket/usrsvr/msgsvr** 已是不同 Pod；要进一步拆的是 **RTMQ Server 独立 Pod**，不是把 frwder 当 WS 网关拆 |

**务实路线（4 周内）**：

1. K8s 上 **frwder 单独 Deployment**（已做）  
2. 调 `frwder.xml` 线程/队列 + rtmq-bench 数字  
3. **文档化** 多 Hub 分区（NID 段 / 地域），**不实现 Dledger**

### 问题 3：frwder / 网关横向扩展

**✅ websocket 可扩；⚠️ 每副本需唯一 NID**

- **不能** 同一 `websocket.xml` HPA 到 10 副本（10 个进程同 NID=20001 → 路由混乱）  
- **可以**：StatefulSet + ordinal 分配 20001～20010，或 10 个 Deployment 各 1 副本（compose multinode 模式）

**frwder 横向扩**：多实例 = 多 RTMQ Hub，各业务 Proxy **改连不同 28888/28889** → **大改**，非 4 周必做。

### 问题 4：RTMQ 高可用

**你文档 Dledger/PVC/死信 → ❌ 不适用于本仓库 RTMQ**

| 设计文档 §11 真实行为 | 生产补偿 |
|------------------------|----------|
| 单点 Hub | 监控 + 快速重启 |
| 无持久化 | **离线 inbox / 拉取**（乐视叙事） |
| 断线丢包 | 业务幂等；在线信令尽力送达 |
| Proxy iplist 多地址 | failover **连下一个 Hub**，非热备 |

**4 周内可演示**：delete frwder pod → Proxy 2s 重连 → 短暂丢信令窗口（诚实讲清）。

### 问题 5：其他服务能否水平扩

| 组件 | 能否多副本 | 条件 |
|------|------------|------|
| websocket | ✅ | **每 Pod 唯一 NID** |
| usrsvr / msgsvr / chatroom | ⚠️ | **同一 cmd 仅一个 SUB 者**，否则 publish 重复消费 |
| frwder(RTMQ Server) | ⚠️ | 多 Hub 分区，非简单 replicas |
| seqsvr | ❌ 单点 | Thrift 单实例 |
| redis/mysql/mongo | ✅ | 托管或集群 |

### 问题 6：节点感知剥离给 K8s

**✅ 方向对；⚠️ 需拆两层**

| 层次 | 现状 | K8s 改造 |
|------|------|----------|
| **客户端接入发现** | LSND_INFO → Redis → iplist | **方案 A 固定 Ingress URL**（已做）或 Endpoints watch 写 Redis |
| **下行 fan-out 路由** | Redis **gid/rid → nid** + RTMQ async_send | **不靠 K8s DNS**；靠 ONLINE/JOIN 写 NID；Pod 下线需 **preStop 删 Redis NID** 或 TTL |

**Headless DNS 给 frwder**：对 **业务进程连 Hub** 有用（Proxy 连 28888）；**不能替代** Redis 里「哪个用户在哪个 NID」。

---

## 4. 改造需求清单（2 周结束应产出）

| 优先级 | 项 | 状态 |
|--------|-----|------|
| P0 | iplist 方案 A env | ✅ 已实现 |
| P0 | deploy/k8s 骨架 | ✅ 已实现 |
| P0 | 群聊 10 人文档+脚本 | ✅ 已有 |
| P1 | ws 多副本 **NID 分配**（StatefulSet） | 待做 |
| P1 | Pod preStop 清理 Redis nid zset | 待做 |
| P1 | Prometheus **/metrics** 暴露（usrsvr/ws） | 待做 |
| P2 | frwder 与 RTMQ Server 拆成两容器/两 Pod | 可选 |
| P2 | 多 RTMQ Hub 分区 | 远期 |
| **不做** | Dledger、NameServer、RTMQ PVC | 与源码不符 |

---

## 5. 后 2 周 K8s 执行顺序（校准）

### 第 3 周

| 天 | 任务 |
|----|------|
| D15-D17 | `apply.sh` 跑通 minikube/kind + Ingress host |
| D18 | 改 `BEEHIVE_WS_IPLIST` 与 group.html 走 Ingress |
| D19 | websocket **StatefulSet** 原型（2 NID） |
| D20 | preStop hook 删 `chat:gid:*:to:nid` / room 等价键 |
| D21 | 文档：Compose vs K8s 差异一页 |

### 第 4 周

| 天 | 任务 |
|----|------|
| D22 | delete ws pod → 重连 → 群聊/弹幕复测 |
| D23 | frwder restart → 观察 Proxy 重连窗口 |
| D24 | 暴露基础 metrics（连接数、AsyncSend 计数） |
| D25 | Grafana 大盘 v1 |
| D26-D28 | 弹幕 ROOM-CHAT 在 K8s smoke + 面试 demo 录屏 |

**不做（除非你 fork 新中间件）**：Helm 全栈、KEDA 缩到 0、Chaos Mesh、Consul/Nacos。

---

## 6. Prometheus 指标（校准：先能采再求全）

### 可先落地（Go 侧加 counter/gauge）

| 指标 | 来源 |
|------|------|
| `beehive_ws_connections` | websocket 进程 |
| `beehive_rtmq_async_send_total` | Proxy AsyncSend |
| `beehive_group_chat_handler_total` | msgsvr |
| `beehive_iplist_static_hit` | usrsvr（静态 iplist 命中） |
| `beehive_iplist_redis_miss` | usrsvr（字典空） |

### 你文档中的 RTMQ 指标（需 C Server 埋点或 rtmq-bench 外推）

`mq_queue_backlog`、`dead_msg_total` 等 **当前无现成 exporter**。

---

## 7. 最终拓扑（校准版 ASCII）

```text
                    客户端 WSS
                         │
              Ingress（唯一公网入口）  ← iplist 方案 A
                         │
         ┌───────────────┴───────────────┐
         │  websocket Pod (NID=20001…)   │  ← WS + ImGroup/ChatTab 第二段
         │  非 frwder                     │
         └───────────────┬───────────────┘
                         │ RTMQ Proxy → :28888
                         ▼
              ┌─────────────────────┐
              │ frwder Pod           │
              │  FORWARD 28888       │
              │  BACKEND  28889      │  ← RTMQ Server + 桥（同进程）
              └──────────┬──────────┘
                         │ publish / async_send
         ┌───────────────┼───────────────┐
         ▼               ▼               ▼
    usrsvr(30000)   msgsvr(31000)   chatroom(33xxx)
         │               │               │
         └───────────────┴───────────────┘
                         │
                    Redis Cluster
              gid→nid / rid→nid / sid→attr
```

**没有**：独立 RTMQ StatefulSet Broker 集群、Consul、Dledger。

---

## 8. 与你原文档的「保留 / 删除」对照

| 保留 | 删除或改写 |
|------|------------|
| Ingress 统一公网、取消 Pod EIP | Dledger、NameServer、死信队列 |
| frwder/RTMQ 资源争抢 → 拆 Pod | frwder=WebSocket 网关 |
| 双段 Fan-out | RTMQ partition / room_id 分片（改用 gid/rid） |
| K8s 替代 iplist **客户端发现** | RTMQ 用 Headless 完全替代 Redis nid 路由 |
| HPA websocket（配 NID 策略） | msgsvr 无限制 HPA |
| 诚实 HA 边界 + 离线补偿 | 「消息不丢不重」绝对承诺 |

---

## 9. 相关文档索引

| 文档 | 用途 |
|------|------|
| [四周计划-RTMQ群聊与K8s架构演进.md](四周计划-RTMQ群聊与K8s架构演进.md) | 原四周日历 |
| [deploy/k8s/README.md](../deploy/k8s/README.md) | K8s 已落地步骤 |
| [群聊10人5接入-RTMQ逐步标注手册.md](rtmq/群聊10人5接入-RTMQ逐步标注手册.md) | 群聊 RTMQ |
| [弹幕系统的名词解释.md](弹幕系统的名词解释.md) | 弹幕 fan-out |
| [RTMQ-技术设计文档.md](rtmq/RTMQ-技术设计文档.md) §11 | HA 真实边界 |

*校准版 2026-06 · 基于 beehive-im 源码，非 RocketMQ 架构迁移。*
