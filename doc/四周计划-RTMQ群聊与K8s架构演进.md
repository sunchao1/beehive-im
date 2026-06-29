# 四周计划：RTMQ 啃透 + 10 人群聊验证 + K8s 架构演进

> **目标**：前 2 周把 RTMQ 17 篇 + 10 用户群聊场景吃透；后 2 周 K8s 可部署、可扩缩、可观测，并回答 **iplist / frwder / RTMQ HA / 水平扩展** 的破局思路。  
> **配套**：[群聊故事线-单用户对照17篇.md](rtmq/群聊故事线-单用户对照17篇.md) · [群聊10人5接入-RTMQ逐步标注手册.md](rtmq/群聊10人5接入-RTMQ逐步标注手册.md) · [im消息架构师路径.md](im消息架构师路径.md)

---

## 一、你的核心问题 · 先给结论

| 问题 | 结论（必嗨现状） | K8s 下建议 |
|------|------------------|------------|
| **iplist 能不能改？** | **能**，且 **应该改**（至少改「返回什么地址」） | 不要返回 Pod IP；返回 **Ingress/LB 的固定域名或 VIP** |
| **一 Pod 一公网 IP 成本高吗？** | **高，且没必要** | 多 ws Pod 共 **一个** `LoadBalancer`/`Ingress`；NID 仍是进程内路由 ID，≠ 公网 IP |
| **frwder+RTMQ 同 Pod 是瓶颈吗？** | **是单点 + 单进程吞吐上限** | 短期 **垂直扩容 + 调队列/线程**；中期 **frwder 多副本需架构分区**；长期 **RTMQ Server 集群化**（源码无内置 HA） |
| **frwder 能否横向扩展？** | **当前拓扑： practically 单实例**（所有 Proxy 连同一 28888/28889） | 可跑多个 frwder **仅当** Proxy 分区连不同 Hub，或改 RTMQ 为共享路由层（大改） |
| **RTMQ 高可用？** | **无内置 HA**（设计文档 §11：单点、断线丢消息、无持久化） | 监控重启 + 多 Hub failover（Proxy `iplist` 多地址）≠ 热备；生产靠 **业务幂等 + 离线补偿** |
| **除 websocket 外谁可水平扩展？** | 见 §五 | usrsvr/msgsvr **可多副本**但需注意 **SUB 单播语义**；seqsvr/monitor 通常单实例或主从 |
| **节点感知能否交给 K8s？** | **能，且推荐** | 用 **Service Endpoints / readiness** 替代「LSND_INFO→Redis→iplist 选 Pod IP」 |

---

## 二、前 2 周：RTMQ + 10 人群聊（可执行日历）

### 第 1 周：地图 + 上行 + 启动 SUB

| 天 | 任务 | 验收 |
|----|------|------|
| **D1** | [00-总地图](rtmq/00-总地图.md) + [群聊故事线-单用户对照17篇](rtmq/群聊故事线-单用户对照17篇.md) 通读 | 能画 frwder⊃2×Server；说出 publish vs async_send |
| **D2** | **02→05→06** + [逐步入参出参 §0](rtmq/群聊故事线-逐步入参出参.md) | 能写出 RTMQ 头 + MesgHeader 两层壳 |
| **D3** | **07→09→11**（AUTH/SUB/sub hash）+ 对照故事线 **阶段 0** | 能背 BACKEND/FORWARD sub 表示例 |
| **D4** | **13→17** + 跟 **100001 ONLINE** 读 `mesg.go` / `usrsvr/mesg.go` | 断点打在 `AsyncSend` 与 `UsrSvrOnlineHandler` |
| **D5** | **10→12** + `frwd_mesg.c` 全文 | 能口述 FORWARD publish / BACKEND async_send(nid) |
| **D6** | **10 人×5 接入**：[群聊10人5接入手册](rtmq/群聊10人5接入-RTMQ逐步标注手册.md) §2～§4 | `./scripts/smoke-group.sh` 或手动 10 uid |
| **D7** | 复盘：每步 **有/无 RTMQ** 自检表（手册 §10） | 15 分钟脱稿讲 ONLINE + CREAT |

### 第 2 周：下行 fan-out + 压测 + 闭卷

| 天 | 任务 | 验收 |
|----|------|------|
| **D8** | **08 dist** 精读 + 故事线 **GROUP-CHAT D9-D15** + `msgsvr/comm.go` `send_data` | 能写 `send_data` 五个参数含义 |
| **D9** | **16 proxy worker** + `upmesg.go` ImGroup 第二段 | 解释「7 RTMQ 包 vs 10 浏览器收包」 |
| **D10** | 10 人各发 1 句：数 RTMQ 包 = **70**；Redis 键对照 `key.go` | 表格填 gid→nid、ImGroup |
| **D11** | `./scripts/rtmq-bench.sh` + [RTMQ-技术设计文档 §11](rtmq/RTMQ-技术设计文档.md) 可靠性 | 能答「RTMQ 挂了怎么办」 |
| **D12** | 17 篇 **闭卷对照表**：每篇在 100001 哪一幕 | 填完 [故事线 §11 锚点表](rtmq/群聊故事线-单用户对照17篇.md) |
| **D13** | 模拟面试：30s/60s 口述（见简历附录）+ 白板画双段 fan-out | 录音自听一遍 |
| **D14** | 整理 **1 页 cheat sheet**（自写，非抄文档） | 只保留：两层壳、sub 表、70 包、08 何时出现 |

**第 2 周末标准（「啃透」定义）**：

- 任意 cmd 能判断 **有无 RTMQ**  
- ONLINE / GROUP-CHAT 能写 **入参出参**  
- 能解释 **SUB 是进程级、加群改 Redis 不改 sub**  
- **不要求**背完 5800 行 C 每一行  

---

## 三、后 2 周：K8s + 观测 + 架构破局

### 第 3 周：最小 K8s 跑通 + Prometheus

> **已落地骨架**：`deploy/k8s/` · iplist 方案 A 代码（`BEEHIVE_WS_IPLIST`）· 见 [deploy/k8s/README.md](../deploy/k8s/README.md)

| 天 | 任务 | 产出 |
|----|------|------|
| **D15** | `deploy/k8s/` 骨架：namespace、ConfigMap（xml）、Secret | 目录结构 |
| **D16** | **有状态依赖**：Redis/MySQL/Mongo（Helm 或单 Pod 先行） | 集群内 Service 可达 |
| **D17** | **frwder Deployment**（1 副本）+ ClusterIP 28888/28889 | ws/usrsvr/msgsvr 连 `frwder:28888` |
| **D18** | **usrsvr + msgsvr + monitor + seqsvr** Deployment | smoke 注册/上线 |
| **D19** | **websocket Deployment**（先 2 副本，不同 NID ConfigMap）+ **Ingress**（WS） | 浏览器连 `wss://域名/im` |
| **D20** | **HPA**：websocket 按 CPU 或自定义 metrics 扩缩（先 CPU 版） | `kubectl scale` 可见 |
| **D21** | **Prometheus**：Pod annotations 或 ServiceMonitor；Grafana 大盘 | 至少：连接数、RTMQ 队列 drop、HTTP 5xx |

**关键指标（必盯）**

| 组件 | 指标 | 说明 |
|------|------|------|
| websocket | 活跃 WS 连接数、goroutine | 扩缩容依据 |
| frwder/RTMQ | recvq/distq 满、drop_total | 瓶颈预警 |
| msgsvr | GROUP_CHAT 处理延迟 | fan-out 段 |
| usrsvr | `/im/iplist` 延迟、空列表次数 | iplist 链健康 |
| Redis | 连接数、延迟 | gid→nid 读 |

### 第 4 周：iplist 破局 + 容灾演练 + 架构文档

| 天 | 任务 | 产出 |
|----|------|------|
| **D22** | 实现 **iplist 快改方案 A**（见 §四） | `/im/iplist` 返回 Ingress 域名 |
| **D23** | **LSND_INFO 改报 LB 地址** 或 **旁路 K8s Endpoints**（方案 B/C 选一） | 去掉「Pod IP 进 iplist」 |
| **D24** | 演练：`kubectl delete pod` ws → 重连 → 群消息仍通 | 录屏 5 分钟 |
| **D25** | 演练：缩 frwder 副本（若仍单副本则测重启）→ 观察 Proxy 重连 | 文档化丢消息窗口 |
| **D26** | 写 **《必嗨 K8s 架构说明》** 1 篇：谁可扩、谁单点、iplist 演进 | 面试用 |
| **D27** | 与简历对齐：乐视触达 + Beehive K8s 演示脚本 | 45 分钟 demo 包 |
| **D28** | 缓冲 / 补压测 / 修 bug | — |

---

## 四、iplist 破局（三档，由易到难）

### 现状链（传统运维思维）

```text
websocket 定时 CMD_LSND_INFO(0x0601)
  → frwder → monitor → Redis im:lsnd:*
  → usrsvr task listend_dict_update
  → GET /im/iplist 返回「外网 IP:端口」列表
```

问题在 K8s：**Pod IP 会变**；**每个 Pod 一个公网 IP** 贵且没必要。

### 方案 A · 快改（推荐第 3 周做）

**iplist 仍走 HTTP，但返回固定入口，不返回 Pod IP。**

| 项 | 改法 |
|----|------|
| usrsvr 配置 | `WS_PUBLIC_URL=wss://im.example.com/im` 或 `ws://LB:port/im` |
| iplist 逻辑 | `iplist_get` 优先读配置 **固定 URL 列表**；Redis 字典作备用 |
| websocket | 仍报 LSND_INFO（兼容 monitor 统计），**客户端不再依赖报文中的 Pod IP** |
| 扩缩容 | Ingress 后 **HPA 任意扩 ws**；客户端始终连同一域名 |

**成本**：低。**NID 路由不变**（进程 xml 里仍 20001…）；**fan-out 仍用 Redis gid→nid**。

### 方案 B · 中改（LSND_INFO 报「逻辑地址」）

websocket 上报 LSND_INFO 时：

- `ip` = Ingress 对外域名或 LB VIP（环境变量注入）  
- `port` = 443 / 80  
- `nid` = 本 Pod 仍唯一（ConfigMap 按 Pod ordinal 或启动脚本分配）

monitor 写 Redis 的仍是「用户可连的地址」，但 **多个 NID 可映射同一 LB**（iplist 返回多条相同 URL 或轮询）。

### 方案 C · 重构（把「节点存在感知」交给 K8s）

| 原必嗨 | K8s 替代 |
|--------|----------|
| Redis `im:lsnd:*` 存活表 | **Service Endpoints** + readinessProbe |
| usrsvr 内存 dict | usrsvr **watch Endpoints** 或调 K8s API |
| iplist 选 IP | 返回 **Service DNS** 或固定 Ingress；NID 与 Pod 映射用 **Downward API** 注入 |

**fan-out 第二段仍靠 NID**：msgsvr 的 `async_send(nid)` 不变；变的是 **NID 生命周期**：

- Pod **Ready** → 注册 NID 到 Redis（或 Sidecar 写）  
- Pod **Terminating** → preStop 从 Redis 删 NID / 踢连接  

这才是你说的「K8s 天生支持的感知」——**不必再靠 TTL 猜 Pod 还在不在**。

---

## 五、各组件水平扩展能力

| 组件 | 能否水平扩展 | 条件 / 注意 |
|------|--------------|-------------|
| **websocket** | ✅ | 每副本 **唯一 NID**；前接 LB/Ingress；连接有状态 |
| **listend** | ✅ | 同 websocket |
| **usrsvr** | ⚠️ 可多副本 | HTTP 无状态；**多个 BACKEND Proxy 都 SUB 同一 cmd 时 publish 会投多个**——生产通常 **1 或 active-passive** |
| **msgsvr** | ⚠️ 同上 | GROUP_CHAT 一般 **单 msgsvr SUB**，否则一条上行被多实例各处理一次 |
| **chatroom** | ⚠️ 同上 | 聊天室/弹幕同理 |
| **frwder+RTMQ** | ⚠️ **单 Hub 为主** | 多实例 = 多套独立 sub 表，**Proxy 必须分区连不同 Hub**（按 NID 段 / 地域） |
| **seqsvr** | ❌ 单点 | 可主从或换分布式 ID |
| **monitor** | ⚠️ | 一般 1 实例够用 |
| **tasker** | ⚠️ | 定时任务需 leader 选举 |
| **Redis/MySQL/Mongo** | ✅ | 用云托管或 Operator HA |

**记忆**：**RTMQ publish 是「所有 SUB 了 cmd 的连接都收到」**——业务服务 **不能随便水平扩多副本** 除非做 **消费分区** 或 **只让一个实例 SUB**。

---

## 六、frwder 瓶颈与 RTMQ HA（面试级实话）

### frwder 是不是大瓶颈？

**是架构上的单点与吞吐汇聚点**，不一定是你的第一个瓶颈：

1. **连接数**：所有 ws/usrsvr/msgsvr Proxy 各 1 TCP → frwder  
2. **包量**：群聊 fan-out = msgsvr 对每个 NID 一包，经 BACKEND→FORWARD  
3. **线程/队列**：`frwder.xml` RECVQ/DISTQ 满则 **丢包**（设计如此）

**短期**：调 `RECV_THD_NUM`、`DISTQ MAX`、机器核数；你已有 **~14 万/s** publish 压测数字。  
**中期**：**多个 frwder 分区**（如北方 Hub / 南方 Hub，NID 段划分）。  
**长期**：RTMQ Server **无内置集群**；要么接受 Hub 单点 + 快速重启，要么 **换/NATS/自研集群版**（超出当前 fork 范围）。

### RTMQ 高可用「任何保证」？

设计文档明确：

- 无持久化、无副本、断线 **丢消息**  
- Proxy 有 **iplist 多 Server 地址 failover**（连下一个 Hub），**不是热备同步**  
- 生产语义：**在线信令尽力送达 + 业务幂等 + 离线/inbox 补偿**（与乐视双通道一致）

---

## 七、10 人群聊在 K8s 验证清单

部署完成后必须跑通：

```text
[ ] 10 uid register + iplist（固定 Ingress URL）
[ ] 10 uid ONLINE（5 ws Pod，每 Pod 2 人）
[ ] 100001 CREAT + 9× JOIN
[ ] 每人 1 条 GROUP-CHAT → 10 浏览器都收到
[ ] RTMQ 包计数 ~70（日志或 tcpdump 抽样）
[ ] scale websocket 3→5→3，新用户仍可 JOIN（方案 A/B 下）
[ ] delete 1 ws pod → 该 Pod 用户重连 → 群仍可用
[ ] Grafana 能看到连接数变化
```

---

## 八、四周后你多出来的「35k+ 叙事」

```text
不仅读懂 RTMQ 17 篇 + 10 人群聊全链路，
还能讲清：
  · iplist 在 K8s 下为何必须改成 Ingress/LB 模式
  · 双段 fan-out 与 NID 的关系（NID ≠ 公网 IP）
  · frwder 单点边界与扩 Hub 分区思路
  · RTMQ 无 HA 时的业务补偿语义
  · 亲手 kubectl 扩 ws + Prometheus 看指标 + delete pod 演练
```

这比「只读过文档」强一个量级——**但四周填不满「RTMQ 集群化实现者」**，填的是 **「IM 架构 + 云原生落地 + 诚实边界」**。

---

## 九、风险与优先级（时间不够时砍什么）

| 必做 | 可砍 |
|------|------|
| RTMQ 故事线 + 08 dist + 70 包 | C 12 worker 逐行 |
| K8s 跑通 + Ingress iplist 快改 | K8s Endpoints watch 重构（方案 C） |
| Prometheus 3～5 个核心指标 | KEDA 自定义 metrics |
| delete pod 重连演练 | Chaos Mesh |
| 1 篇架构演进说明 | 多 frwder 分区实装 |

---

*文档版本：2026-06 · 与 beehive-im feature 分支现状一致（无 deploy/k8s 时需从 D15 新建）*
