# 两周面试冲刺计划（64G K8s · 标定 · 投递）

> **终点**：面试三件套齐备 —— ① 本地 K8s Live 演示 ② 标定表 + 外推表 ③ 简历定稿可投。  
> **原则**：你补 **基础设施**；AI 补 **manifest / 脚本 / 配置 / 话术 / 压测报告解读**。  
> **诚实边界**：本机标定 **每 Pod 容量系数**，百万在线 = **同架构线性外推**，不声称本机真连 100 万。

---

## 0. 分工一览（你先看清要准备什么）

### 0.1 你必须准备（AI 替不了你）

| 资源 | 是否必须 | 说明 |
|------|----------|------|
| **64G Linux VM** | ✅ 必须 | 建议 **8C+**，给 K8s **48～56G**；macOS 宿主机上用 VMware/Parallels/UTM 均可 |
| **K8s 集群** | ✅ 必须 | 推荐 **k3d**（轻、带 Ingress、可内置 registry）；kind/minikube 也行 |
| **`kubectl` 可用** | ✅ 必须 | `kubectl get nodes` 正常 |
| **VM 内 Docker** | ✅ 必须 | 编译镜像、`k3d image import` 或本地 registry |
| **域名 / hosts** | ✅ 必须 | 如 `beehive.local` → Ingress NodePort IP（`/etc/hosts` 一行即可） |
| **远程镜像仓库** | ❌ 本地不必 | k3d 用 `k3d image import`；只有上云时才要 ACR/ECR 等 |
| **公网 LB** | ❌ 不必 | 面试 demo 走 NodePort + hosts |
| **云账号** | ⭕ 可选 | 两周内 **不做也行**；有余力第 2 周末 **4h 一次性打点**（≤¥100） |

### 0.2 AI 配合你完成（你说「开始 Phase X」即可）

| 交付 | 内容 |
|------|------|
| `deploy/k8s/` | namespace、ConfigMap、中间件、9 进程 Deployment、Ingress、资源 limit |
| `scripts/k8s-*.sh` | 一键 apply、等待 Ready、scale、压测封装 |
| 配置生成 | 从 `docker/gen-conf.sh` 衍生 K8s 版（`BEEHIVE_WS_IP` 指 Ingress） |
| Prometheus/Grafana | ServiceMonitor + 最小 dashboard（连接数、QPS、P99） |
| 压测报告 | 解读 JSON、填 [CAPACITY_EXTRAPOLATION.md](assets/CAPACITY_EXTRAPOLATION.md) |
| 简历/话术 | [RESUME_BULLETS.md](RESUME_BULLETS.md) 根据实测数微调 |

### 0.3 当前代码状态（计划前提）

| 项 | 状态 |
|----|------|
| Docker 9 进程 demo | ✅ 已通 |
| loadtest 工具 | ✅ 已通 |
| SENDQ + 非阻塞 AsyncSend | ✅ 已测（~39 QPS / P99 ~119ms） |
| 异步 ACK（`BEEHIVE_CHATROOM_ASYNC_BROADCAST=1`） | ✅ 代码已合入，**K8s 上需重跑对比** |
| `deploy/k8s/` | ❌ 待建（Phase 1 核心） |
| chatroom rid 分片 | ❌ 非两周 MVP，面试 **口述 + 架构图** 即可 |
| Kafka | ❌ 非两周 MVP，用异步队列故事替代 |

---

## 1. 两周甘特（按天）

```text
第 1 周                          第 2 周
D1 D2 | D3 D4 D5 | D6 D7        D8 D9 | D10 D11 | D12 D13 | D14
准备   Phase0    Phase1         L1/L2   观测+表   演示彩排   投递
K8s    Docker收尾 K8s起栈       压测    Grafana   15min脚本  简历上线
```

---

## Phase A — 基础设施（D1～D2，约 2 天）

**目标**：VM + K8s 空集群可 `kubectl apply`，不动业务。

### 你要做

1. 创建 **64G Linux VM**（Ubuntu 22.04 / Debian 12 推荐）。
2. 安装：**Docker**、**k3d**、**kubectl**、**helm**（可选，装 Prometheus 用）。
3. 创建集群（示例，按你环境改内存）：

```bash
# 示例：单节点 k3d，分配足够内存给 Pod
k3d cluster create beehive \
  --agents 0 \
  --servers 1 \
  --registry-create beehive-registry:0.0.0.0:5001 \
  -p "80:80@loadbalancer" \
  -p "8000:8000@loadbalancer" \
  -p "8002:8002@loadbalancer" \
  --k3s-arg "--kubelet-arg=eviction-hard=imagefs.available<1%,nodefs.available<1%@server:0"
```

4. 验证：`kubectl get nodes`；记录 **Ingress / LB IP**（k3d 一般是 `127.0.0.1` 或 VM IP）。
5. 在 **VM 和宿主机**（若压测从宿主机跑）各加一行 hosts：

```text
<VM_IP>  beehive.local
```

6. 把以下信息发给 AI（下阶段用）：

```text
K8S_TYPE=k3d|kind|minikube
VM_IP=
INGRESS_HOST=beehive.local
WS_PORT=8002
USRSVR_PORT=8000
可用内存约=__G
CPU核数=__
```

### 验收标准

- [ ] `kubectl get pods -A` 无异常
- [ ] 能从 VM 内 `curl -I http://beehive.local`（Ingress 占位也行）

### AI 阶段

- 等你确认集群就绪后，生成 `deploy/k8s/` 骨架与 `scripts/k8s-up.sh`。

---

## Phase B — Docker 基线收尾（D2～D3，约 1 天）

**目标**：Docker 栈数字齐全，作为 K8s 对比参照；异步 ACK 有 before/after。

### 你要做

1. VM 或宿主机：`./scripts/up-demo.sh` 确保 9 进程健康（**注意 frwder 不能挂**）。
2. 跑对比压测：

```bash
./scripts/loadtest-sync-vs-async.sh
# 或分场景
./scripts/loadtest-capacity.sh L1   # keepalive
./scripts/loadtest-capacity.sh L2   # 1发N看
```

3. 把 `reports/loadtest/*.json` 留着，截图终端 summary。

### AI 阶段

- 解读报告，更新 [LOADTEST_REPORT.md](LOADTEST_REPORT.md)、[RESUME_BULLETS.md](RESUME_BULLETS.md) 第 3 条数字。
- 若 `room_join` / `iplist` 失败，一起排查（多为 frwder 进程挂掉）。

### 验收标准

- [ ] `compare-sync-ack.json` / `compare-async-ack.json` 有有效 `chat_qps`、`latency_ms.p99`
- [ ] Docker baseline 三组数字可填外推表前两行

---

## Phase C — K8s 起栈（D3～D5，约 3 天）★ 核心

**目标**：`kubectl get pods -n beehive` 全绿；smoke 通过；**iplist 返回 Ingress 地址，不是 Pod IP**。

### 你要做

1. 在 VM 内编译 Linux 二进制并打镜像：

```bash
docker compose --profile build run --rm \
  -e GOPROXY=https://goproxy.cn,direct \
  builder 'DIR=src/golang/exec/chatroom'
# 全量：builder 'all'
```

2. 导入 k3d（无远程仓库时）：

```bash
docker build -f docker/Dockerfile.runner -t beehive-im:demo .
k3d image import beehive-im:demo -c beehive
```

3. 执行（AI 提供后）：

```bash
./scripts/k8s-up.sh
./scripts/smoke-test.sh   # WS 地址改为 ws://beehive.local:8002/im
```

4. 遇到问题贴：`kubectl describe pod`、`kubectl logs`、events。

### AI 阶段（你说「开始 Phase C」）

| 文件 | 说明 |
|------|------|
| `deploy/k8s/namespace.yaml` | `beehive` |
| `deploy/k8s/middleware/` | Redis / MySQL / Mongo + PVC |
| `deploy/k8s/apps/` | usrsvr、chatroom、websocket×1、frwder、monitor… |
| `deploy/k8s/ingress.yaml` | `beehive.local:8002` → websocket Service |
| `deploy/k8s/configmap.yaml` | `BEEHIVE_WS_IP=beehive.local` 等 |
| `scripts/k8s-up.sh` / `k8s-down.sh` | apply + wait |

**关键配置（面试必讲）**：

```text
客户端 WS 入口 = Ingress 主机名:端口
websocket Pod 扩缩 ≠ 改 iplist 逻辑（monitor 注册 NID → Redis rid→nid）
frwder 首版单副本即可
```

### 验收标准

- [ ] `register` + `iplist` 返回 `list: ["beehive.local:8002"]`（或你定的入口）
- [ ] 2 客户端 smoke：join + chat + 收消息
- [ ] `kubectl scale deployment websocket --replicas=3` 后 monitor 见 3 个 NID

---

## Phase D — 连接容量 L1（D6～D7，约 2 天）

**目标**：标定 **C_pod**（每 Pod 稳定长连接数），目标 **5k～2w** 阶梯。

### 场景

| 阶梯 | 命令思路 | 记录 |
|------|----------|------|
| 5k | `LOADTEST_MODE=keepalive LOADTEST_CONNS=5000` | connect_ok、失败率 |
| 10k | conns=10000 | 同上 + ws Pod 内存 |
| 15k～20k | 视 OOM 情况加 | **稳定上限** = C_pod |

### 你要做

1. **压测在集群外**（VM 本机或同网段），避免与 Pod 抢 CPU。
2. websocket 先 `replicas=4～6`，每 Pod `memory limit 6～8Gi`。
3. 每档跑完保存 JSON：`reports/loadtest/k8s-L1-5k.json` …
4. `kubectl top pods -n beehive` 截图。

### AI 阶段

- 调 `deploy/k8s` 资源 limit、副本数建议。
- 填 [CAPACITY_EXTRAPOLATION.md](assets/CAPACITY_EXTRAPOLATION.md) L1 行。

### 验收标准

- [ ] 至少一档 **≥5000** 连接、失败率 **<1%**、持续 **10min+**
- [ ] 得到 **C_pod** 实数（如 4000/Pod × 5 Pod = 2 万总连接）

---

## Phase E — fan-out 标定 L2（D8～D9，约 2 天）

**目标**：**1 发 N 看** 或 **异步 ACK 对比**；标定 **D_pod**（每 Pod 可持续 fan-out/s）。

### 场景（二选一主秀，建议 L2）

**L2 — 1 发 N 看（推荐主秀）**

```bash
# 1 发送者 + 500/1000/2000 旁观，5 msg/s
LOADTEST_EXTRA="-senders 1 -rooms 1" LOADTEST_CONNS=501 LOADTEST_RATE=5 ...
```

记录：`chat_qps`、`est_downstream_qps`、`latency_ms.p99`。

**ROOM-BC（备选/补充）**

```bash
curl -X POST http://beehive.local:8004/room/push -d '...'
# 或 scripts 封装多 rid 并行 push
```

### 你要做

1. 同步 vs 异步各跑一轮（`BEEHIVE_CHATROOM_ASYNC_BROADCAST=0/1`）。
2. websocket `replicas=1` 与 `replicas=3` 各跑同场景（扩缩容对比表）。

### AI 阶段

- 更新 [LOADTEST_REPORT.md](LOADTEST_REPORT.md) `k8s-scale` 小节。
- 生成 §5 扩缩容对比表实数。

### 验收标准

- [ ] 有 **ingress QPS** + **est_downstream** + **P99** 三套数字
- [ ] 能一句话：**「fan-out ≈ chat_qps × 同房人数」**

---

## Phase F — 多房公告 L3（D10，约 1 天，可压缩）

**目标**：**100 rid × 50～100 人** 并行 push，证明「公告触达」口径（面试口径 B）。

### 你要做

- 总连接 **5k～1w** 即可，不必真 100 万。
- 记录：**发完 100 房耗时**、总下行条数。

### 验收标准

- [ ] `k8s-L3-*.json` 或脚本日志
- [ ] 能口述：滚动 push 60s 内完成 vs 峰值并行差异

---

## Phase G — 可观测 + 外推一页纸（D11～D12，约 2 天）

**目标**：Grafana 能展示连接数；外推表填实数。

### 你要做

1. 安装 kube-prometheus-stack（helm）或轻量 Prometheus。
2. 打开 Grafana，截图 **连接数 / QPS**（业务 metrics 未埋完前可先用 loadtest 报告 + `kubectl top`）。
3. 打印/Canvas：[CAPACITY_EXTRAPOLATION.md](assets/CAPACITY_EXTRAPOLATION.md)。

### 外推公式（面试背）

```text
百万 ws Pod ≈ 1,000,000 / C_pod
公告 100 万人 ≈ 1,000,000 / (D_pod × ws_pod数)
```

### AI 阶段

- `deploy/prometheus/`、最小 Grafana dashboard JSON。
- websocket/chatroom 埋点（时间紧则 **Gauge 连接数 + Counter chat_ack** 两个就够）。

### 验收标准

- [ ] 外推表 3 行实数 + 公式
- [ ] Grafana 或等价截图 ≥2 张

---

## Phase H — 面试包 + 投递（D13～D14，约 2 天）

**目标**：15 分钟演示彩排 2 遍；简历上线。

### 交付物 checklist

- [ ] `reports/loadtest/k8s-L1-*.json`（L1）
- [ ] `reports/loadtest/k8s-L2-*.json`（L2）
- [ ] `compare-sync-ack.json` / `compare-async-ack.json`
- [ ] [CAPACITY_EXTRAPOLATION.md](assets/CAPACITY_EXTRAPOLATION.md) 填完
- [ ] [RESUME_BULLETS.md](RESUME_BULLETS.md) 四条定稿
- [ ] [INTERVIEW_CAPACITY_DEMO.md](INTERVIEW_CAPACITY_DEMO.md) §5 脚本练熟
- [ ] 架构图 + [ROOM_CHAT_SEQUENCE.md](assets/ROOM_CHAT_SEQUENCE.md) 异步 ACK 指图

### 15 分钟 Live 流程（背）

| 时间 | 内容 |
|------|------|
| 0～3 min | 架构图：接入 / frwder / chatroom / Redis 路由 |
| 3～5 min | `kubectl get pods` + Grafana 连接数 |
| 5～8 min | 重播 L2 或 ROOM-BC（**提前 backstage 起栈**） |
| 8～12 min | 外推表：「C_pod=___ → 百万约 ___ Pod」 |
| 12～15 min | 乐视 10min→50s + 同步/异步 ACK |

### 你要做

- 录屏 1 次完整演示（自用复习）。
-  Boss / 拉勾 / 脉脉 更新简历，**项目经历用 RESUME_BULLETS 四条**。

---

## 2. 风险与砍 scope（时间不够时）

| 优先级 | 必做 | 可砍 |
|--------|------|------|
| P0 | K8s 起栈 + smoke + L1 5k 连接 | L3 多房 BC |
| P0 | L2 1发N + 外推表 | HPA / Kafka |
| P0 | 简历四条 + 15min 脚本 | 云上复验 |
| P1 | 异步 ACK K8s 对比 | Prometheus 全量埋点 |
| P1 | ws scale 1→3 对比 | rid 分片代码 |

---

## 3. 你下一步只需回复 AI 四件事

复制填空后发我，即可从 **Phase C** 开始写 manifest：

```text
1. K8s 类型与版本：k3d x.x / kind / 其他
2. VM：内存__G CPU__核 系统__
3. Ingress 访问方式：beehive.local:8002 是否 OK
4. 压测跑在：VM 内 / 宿主机 / 另一台机器
```

---

## 4. 相关文档

- [INTERVIEW_CAPACITY_DEMO.md](INTERVIEW_CAPACITY_DEMO.md) — 三件套与 15min 脚本
- [K8S_DEMO_ROADMAP.md](K8S_DEMO_ROADMAP.md) — 技术细节
- [RESUME_BULLETS.md](RESUME_BULLETS.md) — 简历定稿
- [assets/CAPACITY_EXTRAPOLATION.md](assets/CAPACITY_EXTRAPOLATION.md) — 外推表

---

*两周目标不是「做完所有改造」，而是「能演示、能算数、能投简历」。*
