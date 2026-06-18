# 简历项目描述 · 直接粘贴版

> **项目名（简历）**：消息触达与在线推送平台 · 乐视  
> **勿写**：必嗨 / beehive / fork 开源名  
> **数字**：上线前把 `compare-sync-ack.json` vs `compare-async-ack.json` 里的 QPS/P99 填进第 3 条。

---

## 四条 bullet（推荐原样或微调）

**消息触达与在线推送平台** | 乐视 | *20XX.XX – 20XX.XX*

1. 负责面向 **千万级日活、亿级注册用户** 的 **消息触达**，按用户画像与 **在线/离线** 分流；离线通道采用 **UID 分表 inbox + 异步灌库**，保障全量用户 eventual 触达。

2. 主导触达链路 **2.0 演进**：在线用户由「写库拉取」升级为 **长连接接入层 fan-out**；引入 **业务分片、异步广播队列、接入水平扩展**，与离线 inbox **双通道并行**。

3. 完成架构重构与压测验证：同等灰度规模下，**在线段全量触达收尾由约 10 分钟级缩短至约 1 分钟内（约 50 秒级）**；同房 fan-out 场景下 chat 吞吐由改造前 ~24 QPS 提升至 **~39 QPS**，发送端 P99 由 **~1.9s 降至 ~119ms**（*本地 50 连接压测；异步 ACK 对比见 `compare-*.json`*）。

4. 架构支持 **百万级同时在线** 水平扩展：基于 K8s 标定 **每接入 Pod 长连接数与 fan-out 能力**，可推算生产所需接入规模；配套 Prometheus 监控连接数、下行延迟与队列背压。

---

## 更短版（空间紧时，合并为 2 条）

- 负责乐视 **千万级用户消息触达**，区分在线 push 与离线分表 inbox；主导在线段改造为 **长连接 fan-out + 异步广播**，全量触达由 **约 10 分钟级降至约 1 分钟内**，架构可 **线性扩展至百万同时在线**（压测标定 + K8s 演示）。

- 落地 **接入网关、业务分片、背压与监控**；本地/集群压测输出 **ingress、est_downstream、容量外推表**，支撑资源评估与扩容决策。

---

## 技能关键词（ATS / 面试官扫一眼）

`消息触达` `推送平台` `长连接` `WebSocket` `fan-out` `在线/离线` `分表` `水平扩展` `K8s` `Prometheus` `压测` `高并发` `IM`

---

## 自我介绍（45 秒，面试开场）

我在乐视做 **千万级用户的消息触达**，最早以 **分表灌库 + 用户拉取** 为主，全量活动要等 **十来分钟级**。后来我主导 **在线段改造**：用 **长连接接入和 fan-out** 把在线用户触达做到 **分钟级内（压测里约 50 秒）**，离线仍走 inbox。同时用 **K8s 压测标定每 Pod 能扛多少连接**，说明 **百万同时在线需要多少接入节点**——本地资源不够全真百万，但 **扩展路径和数字是算得出来的**。

---

## 配套材料（面试当天）

| 材料 | 路径 |
|------|------|
| 同步 vs 异步压测 | `reports/loadtest/compare-sync-ack.json` / `compare-async-ack.json` |
| 容量外推表 | `doc/assets/CAPACITY_EXTRAPOLATION.md` |
| 架构图 | `doc/assets/ARCHITECTURE_DIAGRAM.md` |
| 时序（同步/异步） | `doc/assets/ROOM_CHAT_SEQUENCE.md` |

**跑对比压测**：

```bash
# 容器内编译（网络不稳时加 GOPROXY）
docker compose --profile build run --rm -e GOPROXY=https://goproxy.cn,direct builder 'DIR=src/golang/exec/chatroom'

# 栈异常时先重启 demo，再跑对比
./scripts/up-demo.sh
chmod +x scripts/loadtest-sync-vs-async.sh
./scripts/loadtest-sync-vs-async.sh
```

**开启异步 ACK**（非压测脚本时）：

```bash
export BEEHIVE_CHATROOM_ASYNC_BROADCAST=1
# docker runner 内需重启 chatroom 进程并带上该环境变量
```

---

## 相关文档

- [RESUME_IM_NARRATIVE.md](RESUME_IM_NARRATIVE.md)
- [INTERVIEW_CAPACITY_DEMO.md](INTERVIEW_CAPACITY_DEMO.md)
- [PHASE3_PERFORMANCE_EVOLUTION.md](PHASE3_PERFORMANCE_EVOLUTION.md) — 方案 A 已实现：`BEEHIVE_CHATROOM_ASYNC_BROADCAST=1`
