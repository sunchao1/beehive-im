# Phase 3：性能演进选型（异步 ACK vs Kafka · 面试项目）

> **目标**：在必嗨 demo 上做 **1 个主改造 + 可选 1 个辅改造**，产出 **before/after 压测报告**，简历可写「ingress QPS / P99 提升 X%」。  
> **原则**：改热路径、可演示、可回滚；不替换整个 RTMQ/frwder。

---

## 1. 当前瓶颈（压测已验证）

| 层级 | 现象 | 根因 |
|------|------|------|
| websocket | 50 人同房 P99 曾秒级 | sendq 小 + AsyncSend 1s 阻塞（**已部分修复**） |
| chatroom | 单连接 10 msg/s → ~3.4 ACK/s | **同步** roomChatHandler：fan-out 完才 ACK |
| chatroom | RTMQ worker 长时间占用 | 见 [ROOM_CHAT_SEQUENCE.md](assets/ROOM_CHAT_SEQUENCE.md) 🔴 链 |
| push | 全集群 nid 扫描 | `/room/push` 对每个 lsnd nid AsyncSend |

**下行 fan-out** 本身架构正确；要优化的是 **同步耦合** 与 **峰值削峰**。

---

## 2. 改造方案对比

### 方案 A：chatroom 异步 ACK + 广播 worker（首选）

| 项 | 说明 |
|----|------|
| **改什么** | `ChatRoomChatHandler`：校验通过后 **立即 `roomChatAck`**；`roomChatHandler` 中 fan-out 移到 **goroutine 或 `room_broadcast_chan`** |
| **语义变化** | ACK = 「已受理并进入广播队列」，非「已推到所有 nid」 |
| **改动面** | 主要 `mesg.go`；可选配置开关 `BEEHIVE_ASYNC_BROADCAST=1` |
| **预期** | 单连接 ingress **3 → 数十+ QPS**；50 人互刷 P99 再降 |
| **风险** | 客户端/产品需接受「发送成功 ≠ 全员已收」；需 metrics 统计广播失败 |
| **面试** | 「2017 同步语义；我拆 **受理** 与 **投递** 两阶段，和微信/直播常见做法一致」 |

**伪代码**：

```go
// 校验通过后
select {
case ctx.broadcast_chan <- &BroadcastItem{head, req, data}:
default: // 队列满则 roomChatFailed
}
return ctx.roomChatAck(head, req) // 立即 ACK
```

---

### 方案 B：Kafka（NATS）削峰 — chatroom 与接入解耦

| 项 | 说明 |
|----|------|
| **改什么** | chatroom 不再同步 `sendData` 循环；改为 **`Publish(topic, partition=rid)`**；consumer 按 **nid** 或 **websocket 实例** 订阅并下行 |
| **语义** | 与 A 类似；多一层 **持久化队列** |
| **改动面** | 新 `lib/broadcast/`、chatroom producer、websocket/独立 consumer Deployment |
| **预期** | chatroom **ingress 与 Kafka 写入** 绑定，峰值弹幕 **lag 可观测** 而非拖死 worker |
| **风险** | 运维复杂度；demo 单 broker；顺序/重复消费要设计 |
| **面试** | 「必嗨 RTMQ 适合 **命令路由**；Kafka 适合 **同房广播削峰**；各管一段」 |

**拓扑**：

```text
ROOM-CHAT → chatroom → Kafka topic(im.room, key=rid)
                              ↓
                    consumer(s) → frwder → websocket → TravRoomSession
```

---

### 方案 C：websocket 批量 fan-out

| 项 | 说明 |
|----|------|
| **改什么** | `TravRoomSession` 收集 cid 列表 → **batch** 入 sendq 或 **writev** |
| **预期** | **1 发 N 看** 场景 fan-out 耗时下降 |
| **改动面** | `upmesg.go`、`lws` send_routine |
| **与 A/B** | 可叠加；单独做也有效 |

---

### 方案 D：限流 + 优先级（产品向）

| 项 | 说明 |
|----|------|
| **改什么** | 用户/房间 token bucket；ROOM-BC 优先级 > ROOM-CHAT |
| **预期** | 稳定性 ↑，峰值 QPS 数字不一定 ↑ |
| **面试** | 作「生产必做」补充，不作唯一性能故事 |

---

## 3. 推荐组合（简历一条 bullet）

**最小闭环（4～6 周）**：

1. **方案 A**（必做）+ 压测报告  
2. **Prometheus**：`broadcast_queue_depth`、`ack_qps`、`fanout_lag`  
3. **可选**：方案 C 中 **1 发 1000 旁观** loadtest 场景  

**进阶（+4 周）**：

4. **方案 B** Kafka 单节点 + consumer lag 面板  
5. K8s 上 **chatroom 与 kafka-consumer 分开 Deployment**

---

## 4. 决策矩阵

| 维度 | A 异步 ACK | B Kafka | C batch fan-out |
|------|------------|---------|-----------------|
| 实现难度 | ★★☆ | ★★★★ | ★★★ |
| 压测对比明显度 | ★★★★ | ★★★★ | ★★★ |
| 与必嗨契合 | ★★★★★ | ★★★（外挂） | ★★★★ |
| 面试故事 | 同步→异步语义 | 削峰/日志/回放 | 接入层优化 |
| 运维成本 | 低 | 中 | 低 |

**结论**：先 **A**，有精力再加 **B** 或 **C**；不要三个同时开工。

> **方向二完整落地路径** → [方向二-RTMQ保留与Kafka削峰落地手册.md](方向二-RTMQ保留与Kafka削峰落地手册.md)（RTMQ 保留 Hub + Kafka 削峰 + K8s 路由分三期）

---

## 5. 方案 A 实施 checklist

- [ ] 新增 `room_broadcast_chan`（buffer 可配置，如 10000）
- [ ] `taskRoomBroadcastPop()` goroutine：从 chan 取 item，执行现有 nid 循环 `sendData`
- [ ] `ChatRoomChatHandler`：校验后入 chan + **立即 ACK**（chan 满则 failed）
- [ ] 保留 `room_mesg_chan` 异步落库 **不变**
- [ ] 配置项 `conf/chatroom.xml`：`ASYNC-BROADCAST enabled="1"`
- [ ] 指标：`im_broadcast_queue_depth`、`im_broadcast_drop_total`
- [ ] 压测：
  - `LOADTEST_CONNS=1 LOADTEST_RATE=10`（对比改前 ~3.4 QPS）
  - `LOADTEST_CONNS=50 LOADTEST_RATE=1`（对比 P99）
- [ ] 更新 [LOADTEST_REPORT.md](LOADTEST_REPORT.md) 新小节 `async-ack`
- [ ] 更新 [ROOM_CHAT_SEQUENCE.md](assets/ROOM_CHAT_SEQUENCE.md) ACK 步骤标注为 🟢

---

## 6. 方案 B 实施 checklist（可选）

- [ ] `docker-compose` 增加 `kafka`（或 Redpanda 单节点）
- [ ] topic：`beehive.room.broadcast`，partition ≥ 32，`key=rid`
- [ ] chatroom producer：`Publish` 替代 nid 循环（或 ACK 后 publish）
- [ ] consumer 进程（Go）：消费 → 按 Redis rid→nid **或** 固定 fan-out 到本地 websocket 负责的 nid
- [ ] 指标：`kafka_consumer_lag`
- [ ] 压测：burst 场景 `LOADTEST_RATE=50 -burst` 对比 lag vs 旧版失败率
- [ ] 文档：与 RTMQ 职责对比表（1 页）

---

## 7. 方案 C 实施 checklist（可选）

- [ ] loadtest 模式：`LOADTEST_MODE=watch`（只 JOIN，不发）+ `LOADTEST_SENDERS=1`
- [ ] `TravRoomSession` 优化：预分配 slice、批量 `AsyncSend` 或合并帧
- [ ] 压测：`LOADTEST_CONNS=1000 LOADTEST_SENDERS=1 LOADTEST_RATE=10`
- [ ] 指标：`im_fanout_duration_seconds` histogram

---

## 8. before / after 报告模板

```markdown
## 改造：异步 ACK（方案 A）

| 场景 | 改前 QPS | 改后 QPS | 改前 P99 | 改后 P99 | 备注 |
|------|----------|----------|----------|----------|------|
| 1 conn × 10/s | 3.4 | | 35ms | | |
| 50 conn × 1/s | 38.8 | | 119ms | | |

结论：ingress 与 chatroom worker 解耦；下行 fan-out 仍由 websocket 承担，扩容见 K8S_DEMO_ROADMAP。
```

---

## 9. C 语言关键节点（配合改造，不必改 C）

| 文件 | 面试要讲的 |
|------|------------|
| `src/clang/exec/frwder/frwd_mesg.c` | 上行/下行 RTMQ 路由 |
| `listend` 会话表 | SID↔CID |
| RTMQ 包头 | cmd/nid/length |

改造 A/B **主要动 Go**；C 层 **frwder 不变**  unless 多 frwder 分片（Phase 2+）。

---

## 10. 面试 Q&A 预制

**Q：为什么不用 Kafka 替换 RTMQ？**  
A：RTMQ 做 **会话级命令路由**（ONLINE、JOIN、点对点 nid）；Kafka 做 **同房广播日志流** 更合适，职责不同。

**Q：异步 ACK 丢消息怎么办？**  
A：广播队列满则 **ACK 失败**；成功 ACK 后投递失败靠 **metrics + 重试队列**；离线用户靠 **Mongo/历史**，不在 WS 热路径。

**Q：性能提升多少？**  
A：只报 **压测 JSON 数字**；强调 **ingress 与 fan-out 分离** 后，chatroom 可水平扩，websocket 按连接扩。

---

## 11. 相关文档

- [K8S_DEMO_ROADMAP.md](K8S_DEMO_ROADMAP.md)
- [LOADTEST.md](LOADTEST.md)
- [ARCHITECTURE.md](ARCHITECTURE.md) §11
- [SCALE.md](SCALE.md)

---

*建议执行顺序：方案 A → K8s Phase 2 压测 → 方案 B 或 C 二选一。*
