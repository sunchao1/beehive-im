# 压测报告（baseline 模板）

> 将 `reports/loadtest/baseline-*.json` 中的数字填入下表。  
> **注意**：以下为 **单机 Docker demo** 数据，不可外推为百万在线容量。扩展路径见 [SCALE.md](SCALE.md)。

---

## 环境

| 项 | 值 |
|----|-----|
| 日期 | 2026-06-16 |
| 机器 | macOS，Docker Desktop（runner 单容器 9 进程） |
| 部署 | `./scripts/up-demo.sh` + 可选 `docker-compose.load.yml` |
| 版本 | `Makefile` VERSION=v.1.1 |
| 房间 rid | 10001 |

---

## 场景 S1：同房弹幕

**命令**：`LOADTEST_CONNS=200 LOADTEST_RATE=1 LOADTEST_DURATION=60s ./scripts/loadtest.sh`  
**报告**：`reports/loadtest/baseline-s1-chat.json`

| 指标 | 值 |
|------|-----|
| 连接成功 | |
| ONLINE 成功 | |
| JOIN 成功 | |
| CHAT 成功 / 失败 | |
| CHAT QPS | |
| 延迟 P50 (ms) | |
| 延迟 P95 (ms) | |
| 延迟 P99 (ms) | |
| runner CPU/内存峰值 | |

**简要结论**（1～2 句）：_例：200 连接 1msg/s 下 P95 &lt; XX ms，瓶颈在 chatroom fan-out / frwder。_

---

## 场景 S2：连接保活

**命令**：`LOADTEST_MODE=keepalive LOADTEST_CONNS=500 ./scripts/loadtest.sh`  
**报告**：`reports/loadtest/baseline-s2-keepalive.json`

| 指标 | 值 |
|------|-----|
| 连接成功 | |
| PING 成功 / 失败 | |
| im:sid:zset 数量（压测后） | |
| 是否出现连接拒绝 | |

**简要结论**：_例：500 连接稳定保活；距 listend MAX=1024 仍有 headroom。_

---

## 场景 S3：burst 高 QPS

**命令**：`LOADTEST_CONNS=100 LOADTEST_RATE=50 LOADTEST_DURATION=30s LOADTEST_EXTRA="-burst" ./scripts/loadtest.sh`  
**报告**：`reports/loadtest/baseline-s3-burst.json`

| 指标 | 值 |
|------|-----|
| CHAT QPS（实测） | |
| 失败率 | |
| P99 延迟 (ms) | |

**简要结论**：_例：短峰刺下失败率上升，需 Kafka 式削峰才适合生产弹幕峰值。_

---

## 示例数据（2026-06-16，Docker demo 单机实测）

> 环境：runner 单容器 9 进程 + Redis/MySQL/Mongo；rid=10001。  
> 完整 JSON 见本地 `reports/loadtest/`（未进 git）。

| 场景 | conns | 成功指标 | QPS | P50 ms | P95 ms | P99 ms | 备注 |
|------|-------|----------|-----|--------|--------|--------|------|
| S1 同房 chat（改前） | 50 | chat 906 / fail 40 | 24.0 | 6.4 | 121 | 1923 | SENDQ=128 + AsyncSend 1s 阻塞 |
| S1 同房 chat（改后） | 50 | chat 2793 / fail 13 | 38.8 | 4.9 | 31 | 119 | SENDQ=8192 + 队列满即 drop |
| S1 轻量 | 20 | chat 300 / fail 0 | 12.0 | 6.4 | 93 | 401 | 无失败 baseline |
| S2 keepalive | 50 | ping 300 / fail 0 | 7.4 | — | — | — | 0.2 ping/s/conn，30s |
| 单连接高发送 | 1 | chat 101 / fail 180 | 3.4 | 5.9 | 12 | 35 | 目标 10 msg/s，测 chatroom ACK 上限 |

**结论（面试用）**：调大 SENDQ 并去掉 AsyncSend 1 秒阻塞后，50 人同房 P99 从 ~1.9s 降到 ~120ms，QPS 24→39；但 **chatroom 单线程 RTMQ** 仍是硬顶（单连接压 10 msg/s 时成功 QPS 仅 ~3.4，大量 room_chat 超时）。扩展需多接入节点 + chatroom 异步化（见 [SCALE.md](SCALE.md)）。

---

## SENDQ / AsyncSend 优化实验（2026-06-16）

**改动**：

- `conf/templates/websocket.xml`：`SENDQ MAX` 128 → **8192**
- `src/golang/lib/lws/websocket.go`：`AsyncSend` 队列满时 **立即失败**（去掉 `time.After(1s)`）

**命令**：

```bash
# 50 人同房互刷（与改前同场景）
LOADTEST_SKIP_GATE=1 LOADTEST_CONNS=50 LOADTEST_RATE=1 LOADTEST_DURATION=60s \
  LOADTEST_REPORT=reports/loadtest/sendq8192-50chat.json ./scripts/loadtest.sh

# 单连接高发送（测 chatroom 处理上限，非 fan-out）
LOADTEST_SKIP_GATE=1 LOADTEST_CONNS=1 LOADTEST_RATE=10 LOADTEST_DURATION=30s \
  LOADTEST_REPORT=reports/loadtest/sendq8192-1sender-10qps.json ./scripts/loadtest.sh
```

**报告 JSON**（相对 `src/golang/` 运行目录）：`src/golang/reports/loadtest/sendq8192-*.json`

| 对比项 | 改前 | 改后 | 变化 |
|--------|------|------|------|
| CHAT 成功 / 失败（50 conn） | 906 / 40 | 2793 / 13 | 失败率显著下降 |
| QPS | 24.0 | 38.8 | +61% |
| P99 (ms) | 1923 | 119 | −94% |
| Max (ms) | 9036 | 406 | −96% |

**单发送者说明**：1 连接、10 msg/s 时房内无其他接收者，**不测 fan-out**，只测 **ROOM-CHAT 请求→ACK** 路径；成功 QPS ~3.4 说明 chatroom 串行处理约 **~300ms/条** 量级（含 RTMQ 多跳）。要测 fan-out 上限需「1 发送 + N 旁观连接同房间」，当前 loadtest 工具尚未支持，可后续扩展。

**runner 快照**（压测后）：CPU ~18%，MEM ~467 MiB / 7.75 GiB。

---

## 瓶颈判断 checklist

- [x] connect_fail → ulimit / websocket CONNECTIONS MAX  
- [x] chat timeout 高 → **SENDQ 过小 + AsyncSend 1s 阻塞**（已验证调优有效）；剩余瓶颈在 chatroom RTMQ 串行  
- [ ] CPU 打满在 runner → 水平加 websocket、优化 fan-out  
- [ ] Redis 延迟 → 单节点瓶颈，需 Cluster  

---

## 与架构演进

| 本报告证明 | 不能证明 |
|------------|----------|
| 协议与 ROOM-CHAT 热路径在单机可测 | 100 万同时在线 |
| rid→nid fan-out 可工作 | 5 万 QPS 单房 |
| 多连接保活可行 | 生产 SLA |

下一步：K8s 部署 + HPA 指标（连接数/QPS）以本 baseline 为 requests/limits 参考。  
百万演示反向容量与一日云成本见 [MILLION_DEMO_PLAN.md](MILLION_DEMO_PLAN.md)。

---

*更新：跑完 baseline 后把 JSON 路径 commit 到团队 wiki 或面试笔记即可，JSON 本身可不进 git。*
