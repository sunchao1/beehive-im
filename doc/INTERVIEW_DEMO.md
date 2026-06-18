# 面试现场演示脚本（15 分钟）

> **目标**：让面试官在 15 分钟内看到「真系统」——能起、能测、能双用户弹幕、能讲清楚架构。  
> **前置**：已完成 [feature_spec_task_01](../todo_task1_list.md) + [feature_spec_task_02](DEMO_SCOPE.md)（Docker 全栈 + 业务 smoke）。  
> **首次排练**：预留 **30 分钟**（含编译、起栈、走一遍 smoke）。

---

## 0. 演示前检查（ backstage，不对观众念）

```bash
# macOS：C 服务需在 Linux 容器内编译
docker compose --profile build run --rm builder /workspace/scripts/build-linux.sh

./scripts/up-demo.sh          # 起中间件 + runner（9 进程）
./scripts/status.sh           # 期望 pass 全绿
./scripts/smoke-test.sh       # 成功路径 + 3 条失败路径
```

| 项 | 值 |
|----|-----|
| 测试用户 | uid=`100001`、`100002` |
| 种子房间 | rid=`10001`（demo-room） |
| 二进制版本 | `*.v.1.1`（`Makefile` 中 `VERSION=v.1.1`） |
| 架构图 | [assets/ARCHITECTURE_DIAGRAM.md](assets/ARCHITECTURE_DIAGRAM.md) |
| ROOM-CHAT 时序（同步/异步） | [assets/ROOM_CHAT_SEQUENCE.md](assets/ROOM_CHAT_SEQUENCE.md) |

---

## 1. 时间轴（15 分钟）

### 0～3 min — 架构（对着架构图讲）

**打开**：[doc/assets/ARCHITECTURE_DIAGRAM.md](assets/ARCHITECTURE_DIAGRAM.md)（Mermaid 图）

**话术要点**（按顺序）：

1. **分层**：客户端 → 接入（websocket/listend）→ **frwder RTMQ** → Go 业务 → Redis/MySQL/Mongo。
2. **为什么 C + Go**：C 做 epoll 接入和高 I/O 转发；Go 做业务迭代（usrsvr/chatroom/msgsvr）。
3. **聊天室 vs 群聊**：同房弹幕走 **chatroom** + rid→nid 路由；群聊走 **usrsvr** 管成员 + **msgsvr** fan-out。
4. **水平扩展预留**：每个接入点唯一 **NID**；monitor 注册进 Redis；iplist 按运营商返回 WS 地址（demo 默认 127.0.0.1:8002/8003）。
5. **刻意未做**：敏感词（见 [DEMO_SCOPE.md](DEMO_SCOPE.md)）；百万在线单机不声称（见 [SCALE.md](SCALE.md)）。

---

### 3～8 min — 终端：一键起栈 + 健康检查

**操作**（若已 backstage 起好，可只演示 status + curl）：

```bash
./scripts/up-demo.sh
./scripts/status.sh
```

**指着输出讲**：

- 中间件：Redis / MySQL / Mongo healthcheck
- 应用端口：8000 usrsvr、8002/8003 websocket、8004 chatroom、9002 listend
- runner 内 **9 个进程**（frwder + listend + websocket + 7×Go）

**可选一条命令证明 HTTP 通**：

```bash
curl -s 'http://127.0.0.1:8000/im/register?uid=100001&nation=1&city=1&town=1' | head -c 200
```

期望 JSON 含 `"sid"` 且 `"code":0`。

---

### 8～13 min — 浏览器双用户弹幕 + 可选扩展

**主路径（必做）**：

```bash
./scripts/serve-demo.sh
# 浏览器开两个窗口 → http://127.0.0.1:8088/
# A: uid=100001  B: uid=100002  同 rid=10001 → 互发弹幕
```

**边演示边讲**：

- 注册/iplist 拿 token → WS `ONLINE` → `ROOM-JOIN` → `ROOM-CHAT`
- 消息经 frwder 到 chatroom，再按 **nid** 推到 B 所在 websocket

**可选 30 秒扩展（时间够再做）**：

```bash
# 群聊 smoke（终端另开）
./scripts/smoke-group.sh

# 系统推送 P2P
./scripts/smoke-push.sh

# 当前房间在线人数（真实 Redis）
curl -s 'http://127.0.0.1:8004/room/query?option=room-num&rid=10001'
```

---

### 13～15 min — Q&A 备用

| 可能问题 | 回答要点 |
|----------|----------|
| seqsvr 单点？ | demo 单实例；生产可主从或 Snowflake 替代 rid/gid |
| 怎么扩接入？ | 多 listend/websocket、共享 frwder；iplist 返回多地址；已有多节点 smoke |
| 敏感词？ | 架构预留 TODO，当前 sprint 不做，上线前必补 |
| 性能多少？ | 单机 demo：20 连接 P50≈6ms、50 连接 QPS≈24（见 [LOADTEST_REPORT.md](LOADTEST_REPORT.md)）；非百万在线 |
| 和乐视弹幕关系？ | 同源架构思路；本仓库是开源复刻/演进，demo 为可复现单机栈 |

---

## 2. 命令 Cheat Sheet（复制即用）

```bash
# ── 构建与启动 ──
docker compose --profile build run --rm builder /workspace/scripts/build-linux.sh
./scripts/up-demo.sh
./scripts/status.sh
./scripts/stop-linux.sh          # 停 runner，保留中间件
docker compose down              # 停全部

# ── 自动化验收 ──
./scripts/smoke-test.sh          # 弹幕 + 失败路径
./scripts/smoke-group.sh
./scripts/smoke-room.sh
./scripts/smoke-push.sh
./scripts/smoke-multinode.sh

# ── Demo Web ──
./scripts/serve-demo.sh          # http://127.0.0.1:8088/

# ── 手工探测 ──
curl -s 'http://127.0.0.1:8000/im/register?uid=100001&nation=1&city=1&town=1'
curl -s 'http://127.0.0.1:8000/im/iplist?type=2&uid=100001&sid=<SID>&clientip=127.0.0.1'
curl -s 'http://127.0.0.1:8004/room/query?option=room-num&rid=10001'

# ── 日志 ──
docker logs beehive-runner --tail 80
tail -30 log/chatroom.log log/usrsvr.log log/monitor.log
```

**端口速查**：见 [DEMO.md](DEMO.md#端口)

---

## 3. 错误场景演示（体现真系统）

> 三个场景均可 **稳定复现**，无需改 conf。详细排障见 [TROUBLESHOOT.md](TROUBLESHOOT.md)。

### 场景 A — frwder 未就绪 → iplist/上线失败（T03-04）

**现象**：register 成功，iplist 空或 WS 连上后 ONLINE 超时。

**复现**：

```bash
docker compose --profile run stop runner
# 或容器内：kill frwder 进程（演示用 stop runner 即可）
curl -s 'http://127.0.0.1:8000/im/iplist?type=2&uid=100001&sid=1&clientip=127.0.0.1'
# list 为空或 smoke 失败
```

**30 秒内定位**：

```bash
./scripts/status.sh                    # 8002/8000 可能仍通，但进程数 < 9
docker logs beehive-runner 2>&1 | tail -20
grep -i frwder log/monitor.log 2>/dev/null || true
```

**恢复**：`./scripts/up-demo.sh` 或 `docker compose --profile run up -d runner`

**讲解**：接入层依赖 frwder RTMQ；monitor 靠 websocket 上报 LSND_INFO 填 iplist。

---

### 场景 B — 未 JOIN 发 ROOM-CHAT（T03-05）

**复现**：

```bash
./scripts/smoke-fail.sh
# 或 demo/web：连接后不填 rid/不 join，直接发消息 → 应失败
```

**期望**：`ROOM-CHAT-ACK` 的 `code != 0`（如 `ERR_SVR_CHECK_FAIL` 20012，未进房）。

**讲解**：业务层校验 sid 是否在 `room:rid:{rid}:to:sid:zset`，不是静默丢包。

---

### 场景 C — 重复 JOIN / 错误 rid（T03-06）

**复现**（smoke-fail 已含非法 rid；重复 join 可手工）：

```bash
# 非法 rid — 已包含在 smoke-fail.sh
./scripts/smoke-fail.sh

# 重复 join：demo/web 同一用户连两次 join 同一 rid
# 期望 ROOM-JOIN-ACK code=20009 ERR_SVR_DATA_COLLISION
```

**讲解**：错误码对齐 [ERRNO.md](ERRNO.md)；失败路径有自动化 smoke。

---

### 场景 D（可选）— demo/web 故障说明（T03-07）

当前 **未做** 页面内「停 frwder」按钮；演示时用 **场景 A** 的终端操作即可。  
若面试官问：说明生产会用 K8s readiness / 健康检查，demo 用脚本模拟。

---

## 4. 相关文档

| 文档 | 用途 |
|------|------|
| [DEMO.md](DEMO.md) | 日常 Docker 演示 |
| [DEMO_SCOPE.md](DEMO_SCOPE.md) | 实现边界 |
| [TROUBLESHOOT.md](TROUBLESHOOT.md) | Top 10 故障 + 日志地图 |
| [SCALE.md](SCALE.md) | 单机 vs 百万在线 |
| [K8S_DEMO_ROADMAP.md](K8S_DEMO_ROADMAP.md) | K8s 扩缩容 + Prometheus 演示 |
| [PHASE3_PERFORMANCE_EVOLUTION.md](PHASE3_PERFORMANCE_EVOLUTION.md) | 异步 ACK / Kafka 改造与压测 |
| [INTERVIEW_CAPACITY_DEMO.md](INTERVIEW_CAPACITY_DEMO.md) | 64G 标定 + 百万外推 + ¥100 内云上打点 |
| [assets/CAPACITY_EXTRAPOLATION.md](assets/CAPACITY_EXTRAPOLATION.md) | 外推一页纸（填数） |
| [assets/ROOM_CHAT_SEQUENCE.md](assets/ROOM_CHAT_SEQUENCE.md) | 同步/异步时序 |
| [ARCHITECTURE.md](ARCHITECTURE.md) | 完整架构说明 |
| [COMMAND.md](COMMAND.md) | 协议命令矩阵 |

---

## 5. 完成标准自检（DoD）

- [ ] 新人按本文可在 30 min 内完成首次排练
- [ ] 架构图 + TROUBLESHOOT + SCALE 齐全
- [ ] 场景 A/B/C 各演示一次且无手工改 conf
