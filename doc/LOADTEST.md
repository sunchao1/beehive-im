# 本地压测指南（feature_spec_task_04）

> **目标**：产出可重复的单机压测 baseline（数百～数千 WS、同房 QPS），并明确 **单机 demo ≠ 百万在线**。  
> **关联**：[SCALE.md](SCALE.md) · [LOADTEST_REPORT.md](LOADTEST_REPORT.md) · [REDIS.md](REDIS.md)

---

## 1. 快速开始

```bash
# 1) 启动 demo 栈（若未启动）
./scripts/up-demo.sh

# 2) 可选：压测 profile（提高 nofile）
docker compose -f docker-compose.yml -f docker-compose.load.yml --profile run up -d runner

# 3) 门禁：status + smoke
./scripts/loadtest-gate.sh

# 4) 单次压测（默认 200 连接、60s、1 msg/s/conn）
./scripts/loadtest.sh

# 5) 三套 baseline 场景
./scripts/loadtest-baseline.sh
```

报告输出：`reports/loadtest/*.json`（已 gitignore，模板见 [LOADTEST_REPORT.md](LOADTEST_REPORT.md)）。

---

## 2. 压测工具 `tools/loadtest`

从 `src/golang` 模块运行（与 smoke 相同）：

```bash
cd src/golang
go run -mod=mod ../../tools/loadtest/main.go -h
```

| 参数 | 说明 | 默认 |
|------|------|------|
| `-uid-base` | 虚拟用户 uid 起始 | 200000 |
| `-conns` | 并发 WS 连接数 | 200 |
| `-rid` | 聊天室 id | 10001 |
| `-rate` | 每连接 msg/s（chat）或 ping/s（keepalive） | 1 |
| `-duration` | 全部连接建立后的压测时长 | 60s |
| `-mode` | `chat`（JOIN+ROOM-CHAT）或 `keepalive`（ONLINE+PING） | chat |
| `-ramp` | 连接建立分散时间 | 10s |
| `-burst` | chat 模式尽量打满 rate | false |
| `-usrsvr` | HTTP 注册/iplist | http://127.0.0.1:8000 |
| `-ws-addr` | 固定 WS（空则走 iplist） | |
| `-report` | JSON 报告路径 | reports/loadtest/latest.json |

**流程**：register → iplist → WS connect → ONLINE →（chat）ROOM-JOIN → 循环 ROOM-CHAT 或 PING。

**指标**：成功/失败计数、ROOM-CHAT ACK 延迟 P50/P95/P99、QPS。

---

## 3. 环境与调优（T04-05 / T04-06）

### 3.1 主机 / 容器

| 项 | 建议 | 检查 |
|----|------|------|
| 文件句柄 | `ulimit -n 65536` | `ulimit -n` |
| compose 压测 | `docker-compose.load.yml` 已为 runner 设 nofile | |
| 内核 | `net.core.somaxconn=4096`（Linux 宿主机） | `sysctl net.core.somaxconn` |

macOS 跑压测客户端时，若连接数 >1000，同样提高 `ulimit -n`。

### 3.2 必嗨可调参数（记录于压测报告）

| 组件 | 配置位置 | demo 默认 | 压测时可调 |
|------|----------|-----------|------------|
| websocket | `conf/templates/websocket.xml` `CONNECTIONS MAX` | 1024 | 4096+ |
| listend | `conf/templates/listend.xml` `CONNECTIONS MAX` | 1024 | 4096+ |
| frwder | `frwder.xml` RECVQ/SENDQ | 4096 | 加大队列 |
| websocket | `WORKER-NUM` / chan len | 10 / 20000 | 按 CPU 增 |
| Go Redis 池 | 各服务 xml | — | 连接池上限 |

改模板后：`./scripts/gen-conf.sh`（或 compose 重启 runner）再压。

### 3.3 压测前自检

```bash
./scripts/status.sh          # 端口 + 9 进程
./scripts/smoke-test.sh      # 功能路径
./scripts/loadtest-gate.sh   # 二者合并
```

---

## 4. Baseline 场景（T04-11）

由 `scripts/loadtest-baseline.sh` 执行：

| 场景 | 连接 | 模式 | 参数 | 目的 |
|------|------|------|------|------|
| S1 同房弹幕 | 200（heavy=500） | chat | 1 msg/s/conn, 60s | 同房 fan-out + ACK 延迟 |
| S2 保活 | 500（heavy=2000） | keepalive | ping 0.2/s, 120s | 连接数上限 |
| S3 burst | 100 | chat | 50 msg/s/conn, 30s, `-burst` | 短峰刺 |

环境变量：

- `LOADTEST_BASELINE_HEAVY=1` — S1/S2 用 spec 原值 500/2000 连接
- `LOADTEST_SKIP_GATE=1` — 跳过 gate（仅调试）

---

## 4.1 面试容量标定 L1 / L2 / L3（百万在线外推）

```bash
chmod +x scripts/loadtest-capacity.sh
LOADTEST_SKIP_GATE=1 ./scripts/loadtest-capacity.sh
```

| 场景 | 默认参数 | 目的 | 报告字段 |
|------|----------|------|----------|
| **L1** keepalive | 5000 conns, 120s | 标定 **C_pod**（连接/Pod） | `online_ok`, `extrapolation.pods_for_target_online` |
| **L2** 1 发 N 看 | 501 conns, **1 sender**, 5 msg/s | **fan-out 下行** | `est_downstream_qps`, `chat_qps`, `join_ok` |
| **L3** 多房 | 5000 conns, **100 rooms** | 多 rid 拓扑 | `online_ok`, `rooms` |

外推 flags（L1/L2 已内置 `-target-online 1000000 -conn-per-pod 5000`，按 L1 实测改 `LOADTEST_CONN_PER_POD`）：

```bash
LOADTEST_L1_CONNS=10000 LOADTEST_CONN_PER_POD=4000 ./scripts/loadtest-capacity.sh
```

手工单次：

```bash
LOADTEST_SKIP_GATE=1 LOADTEST_CONNS=501 LOADTEST_RATE=5 LOADTEST_EXTRA="-senders 1 -rooms 1 -target-online 1000000 -conn-per-pod 5000" ./scripts/loadtest.sh
```

填表：[assets/CAPACITY_EXTRAPOLATION.md](assets/CAPACITY_EXTRAPOLATION.md) · 话术：[RESUME_IM_NARRATIVE.md](RESUME_IM_NARRATIVE.md)

---

## 5. 指标采集（T04-09 / T04-10）

```bash
# 容器资源
docker stats beehive-runner --no-stream

# Redis 在线 sid 数量
docker exec beehive-redis redis-cli -a 111111 ZCARD im:sid:zset

# 房间相关（见 REDIS.md）
docker exec beehive-redis redis-cli -a 111111 ZCARD chat:rid:10001:nid:to:num:zset
```

Prometheus 接入为后续 K8s 阶段；本 task 以脚本 + JSON 报告为主。

---

## 6. Known Issues（T04-13）

| 现象 | 可能原因 | 处理 |
|------|----------|------|
| connect_fail 高 | ulimit、websocket MAX | compose.load + 调 CONNECTIONS |
| room_chat timeout | chatroom/frwder 队列满、Mongo 写慢 | 看 log/chatroom.log；降 rate |
| online_fail | iplist 空、token | 等 monitor 注册；status.sh |
| keepalive 大量 ping_fail | 连接被踢、超时 | 降 conns 或加长 PING 间隔 |
| 压测后 goroutine 涨 | 连接未 Close | 工具已 defer Close；查服务端泄漏 |

压测发现的 **P0 稳定性 bug** 应先修复再重跑 baseline（T04-14）。

---

## 7. 与百万在线的差距

单机 Docker demo（9 进程 + 单 Redis）适合证明 **架构可跑、协议可通**。  
百万在线需要：多 listend/websocket、frwder 分片、Redis Cluster、热路径异步化（参见 PDF 弹幕流水线 / [SCALE.md](SCALE.md) 演进表）。

**面试话术**：引用本目录 JSON 中的 **P95 延迟与 QPS**，并说明「这是单机容器 baseline，扩展靠水平加接入节点与分片」。

---

## 8. TCP 压测（可选 T04-04）

WebSocket 路径为默认验收路径。TCP listend :9002 可使用 `src/clang/exec/client` 扩展；当前 sprint **未纳入**自动化 baseline，需要时可单独加 `tools/loadtest-tcp/`。

---

*task_04 DoD：工具可重复跑、≥2 场景有数字、文档说明单机边界。*
