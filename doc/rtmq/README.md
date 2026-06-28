# RTMQ 源码阅读指南

> **设计文档（推荐先读）** → [RTMQ-技术设计文档.md](RTMQ-技术设计文档.md)（问题背景、架构、协议、API、线程模型）  
> **10 用户 × 5 接入 · 每步 RTMQ 标注（主文档）** → [群聊10人5接入-RTMQ逐步标注手册.md](群聊10人5接入-RTMQ逐步标注手册.md)  
> **单用户故事线 · 对照 17 篇（推荐读 RTMQ 时用）** → [群聊故事线-单用户对照17篇.md](群聊故事线-单用户对照17篇.md)  
> **10 用户全链路（2 接入简版）** → [群聊全链路演练-10用户.md](群聊全链路演练-10用户.md)  
> **先读总地图** → [00-总地图.md](00-总地图.md)（十七篇在整体中的位置 + 两条业务主线）  
> **群聊 Demo 不懂 RTMQ？** → [群聊Demo视角解读RTMQ.md](群聊Demo视角解读RTMQ.md)（17 篇在群聊里各干什么）  
> **源码位置**：`src/clang/lib/rtmq/`（约 5824 行 .c）+ `src/clang/incl/rtmq/`（约 673 行 .h）  
> **Go 对照**：`src/golang/lib/rtmq/rtmq_proxy.go`  
> **生成方式**：`python3 scripts/gen-rtmq-docs.py`（改源码后可重新生成）

---

## RTMQ 是什么

**RTMQ（Real-Time Message Queue）** 是必嗨自研的 **进程间 TCP 消息总线**：

- **Server**（`rtmq_recv` 等）：中心节点，维护连接、订阅表、按 **cmd/type** 和 **nid** 路由。
- **Proxy**（`rtmq_proxy` 等）：各业务/接入进程内的客户端库，连 Server 发收消息。
- **与 Kafka 区别**：低延迟、内存队列、**不持久化**；适合 IM 进程间转发，不是日志型 MQ。

必嗨 IM 里：**frwder / usrsvr / msgsvr / websocket** 各连一个 Proxy，Server 通常与 **frwder 同机** 或由独立 rtmq 进程承载（见 `conf/templates/frwder.xml` 28888/28889）。

---

## 两条核心 API（面试必背）

| API | 方向 | 语义 | frwder 对应 |
|-----|------|------|-------------|
| **publish(type, data)** | 广播 | 所有 SUB 了该 type 的连接都收到 | 上行：接入→业务 |
| **async_send(type, nid, data)** | 单播 | 只发给订阅了 type 且 nid 匹配的连接 | 下行：业务→指定接入 NID |

业务 IM 帧在 RTMQ 里包一层头：`flag=RTMQ_EXP_MESG`，`type=CMD_*`（与 `comm/mesg.go` 一致）。

---

## 线程模型（Server 端）

```text
[listen 线程]  accept → connq[idx]
       ↓ pipe RTMQ_CMD_ADD_SCK
[rsvr 线程 × N]  select 收发包 / 鉴权 / SUB / 拼帧
       ↓ 完整业务包 → recvq
[worker 线程 × M]  reg 回调 → 业务进程逻辑
       ↑
[dist 线程 × 1]  distq → 按 nid 投递到某个 rsvr 的发送队列
```

**Proxy 端**：`tsvr 线程`（连 Server、writev 发送）+ `worker 线程`（收下行、调 reg）。

---

## 阅读顺序（推荐）

1. **[00 总地图](00-总地图.md)** ← 建立脑海地图  
2. 按顺序 02 → 04 → 06 → 07 → 12 → 13 → 16 → 17  
3. 每读完一篇，回总地图 **§2 点亮对应编号**

| 顺序 | 文档 | 内容 |
|------|------|------|
| 0 | [00-总地图](00-总地图.md) | 整体流程图 + 两篇专属序列图 |
| 1 | [02-rtmq_mesg.md](02-rtmq_mesg.md) | 报头、系统 cmd、SUB/AUTH |
| 3 | [06-rtmq_recv_api.md](06-rtmq_recv_api.md) | init/publish/async_send |
| 4 | [07-rtmq_lsn.md](07-rtmq_lsn.md) | accept 流程 |
| 5 | [12-rtmq_worker.md](12-rtmq_worker.md) | 业务回调 |
| 6 | [13-rtmq_proxy.md](13-rtmq_proxy.md) | async_send 发出 |
| 7 | [16-rtmq_proxy_worker.md](16-rtmq_proxy_worker.md) | 下行进 frwder |
| 8 | [10/11 rsvr](10-rtmq_rsvr_part1.md) | 收包细节（可选深读） |
| 9 | [17-rtmq_proxy_go.md](17-rtmq_proxy_go.md) | 与 C 对照 |

---

## 文档目录

- **[群聊全链路演练（10 用户）](群聊全链路演练-10用户.md)**

- **[RTMQ 技术设计文档（设计视角）](RTMQ-技术设计文档.md)**

- **[00 总地图（必读）](00-总地图.md)**

- [01 架构与数据流](01-architecture.md)

- [02 协议层 rtmq_mesg](02-rtmq_mesg.md)
- [03 公共类型 rtmq_comm](03-rtmq_comm.md)
- [04 订阅模型 rtmq_sub 与 Server 上下文 rtmq_recv.h](04-rtmq_sub_and_recv_h.md)
- [05 Proxy 头文件 rtmq_proxy](05-rtmq_proxy_h.md)
- [06 Server 初始化与对外 API](06-rtmq_recv_api.md)
- [07 Server 监听与连接接入](07-rtmq_lsn.md)
- [08 Server 分发线程 rtmq_dist](08-rtmq_dist.md)
- [09 Server 订阅与 node 映射 rtmq_comm](09-rtmq_comm_server.md)
- [10 Server 接收线程 rtmq_rsvr（上）](10-rtmq_rsvr_part1.md)
- [11 Server 接收线程 rtmq_rsvr（下）](11-rtmq_rsvr_part2.md)
- [12 Server 工作线程 rtmq_worker](12-rtmq_worker.md)
- [13 Proxy 初始化 rtmq_proxy](13-rtmq_proxy.md)
- [14 Proxy 发送线程 rtmq_proxy_tsvr（上）](14-rtmq_proxy_tsvr_part1.md)
- [15 Proxy 发送线程 rtmq_proxy_tsvr（下）](15-rtmq_proxy_tsvr_part2.md)
- [16 Proxy 工作线程 rtmq_proxy_worker](16-rtmq_proxy_worker.md)
- [17 Go 封装对照 rtmq_proxy.go](17-rtmq_proxy_go.md)

---

## 与 core 库关系

RTMQ 依赖 `libcore`：`thread_pool`、`mref`、`queue`、`ring`、`avl_tree`、`pipe`、`wiov` 等。见 [两周投岗阅读清单](../两周投岗阅读清单.md) 附录 A。

---

## 重新生成

```bash
python3 scripts/gen-rtmq-docs.py
```
