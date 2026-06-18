# 7 月 IM 四周 OKR 检查表

> **唯一目标（7 月）**：云 K8s 上可演示的 IM/触达 demo + 45 分钟面试 Runbook + **可开始投 IM 岗**。  
> **原则**：每周有 **可勾选交付物**；未达「可投简历线」不进入 C++/RTC（8 月）。  
> **口径**：本机/云上 **标定系数 + 外推**；不说「本机真连 100 万」。

---

## 总览甘特

```text
W1          W2              W3                W4
云 K8s 起栈   压测 L1/L2      容灾 + 观测        Runbook + 投递
smoke 通     演进指标链       重连演示           简历 IM 版
Ingress/LB   外推表填数       Pod kill/drain     边投边补 KEDA
```

| 周 | 主题 | 周末必须达成 |
|----|------|--------------|
| **W1** | 基础设施 + K8s 起栈 | `kubectl get pods -n beehive` 全绿，smoke 通 |
| **W2** | 压测标定 + 版本演进 | L1/L2 JSON + 外推表有实数 + v0→v2 对比表 |
| **W3** | 容灾 + 可观测 | Pod 故障演示 + Grafana 截图 + 重连脚本 |
| **W4** | 面试包 + 投递 | 45min Runbook 练 2 遍 + 简历定稿 + 投 ≥5 家 |

---

## 你需要提前准备的资源（W1 前）

| 资源 | 必须 | 说明 |
|------|------|------|
| 64G Linux VM 或云主机 | ✅ | 8C+，装 Docker + k3d/kubeadm + kubectl |
| 域名或 hosts | ✅ | 如 `beehive.yourdomain.com` 或 `/etc/hosts` |
| 镜像方案 | ✅ | k3d `image import` **或** 阿里云 ACR / 腾讯云 TCR |
| 云 LB（可选） | ⭕ | SLB/CLB 指 Ingress；NodePort 也可先跑通 |
| 压测机 | ✅ | 与集群同 VPC/同网段，避免与 Pod 抢 CPU |

**W1 Day1 发给 AI 的信息模板**：

```text
K8S: k3d|kubeadm|ACK/TKE/EKS
VM/云: __核 __G 系统__
入口: http(s)://____:8002  ws://____:8002/im
镜像: 本地 import | 仓库地址 ____
压测跑在: 集群节点内 | 同 VPC 另一台
```

---

## W1 — 云 K8s 起栈（Day 1～7）

### OKR

| O（目标） | KR（关键结果） |
|-----------|----------------|
| 业务栈在 K8s 跑通 | smoke 双客户端 join + chat 互发 |
| 客户端只认统一入口 | iplist 返回 **LB/Ingress 地址**，非 Pod IP |

### 每日建议

| Day | 任务 | 交付 |
|-----|------|------|
| 1 | VM/云主机 + k3d 集群 + kubectl | `kubectl get nodes` OK |
| 2 | 编译 Linux 二进制 + 打镜像 + 导入/推送 | `beehive-im:demo` 可用 |
| 3 | apply `deploy/k8s/` 中间件（Redis/MySQL/Mongo） | 3 Pod Running |
| 4 | apply 业务 Deployment（9 进程拆 Pod） | usrsvr/chatroom/ws/frwder… |
| 5 | Ingress + Service + ConfigMap（`BEEHIVE_WS_IP`） | curl register/iplist OK |
| 6 | `./scripts/smoke-test.sh` 改 WS 地址 | join + chat 成功 |
| 7 | **缓冲/修 bug** + `websocket replicas=2` 试 scale | monitor Redis 见 2 NID |

### W1 检查清单（周末逐项打勾）

- [ ] `kubectl get pods -n beehive` 无 CrashLoopBackOff
- [ ] `register` + `iplist` 返回统一 WS 入口（非 10.x Pod IP）
- [ ] 2 客户端（uid 100001/100002）同房 rid=10001 互发弹幕
- [ ] MySQL `CHAT_ROOM_INFO_TAB` rid=10001 **status=1**
- [ ] `scripts/k8s-up.sh` / `k8s-down.sh` 可重复执行
- [ ] 架构图指一遍：Ingress → ws → frwder → chatroom → Redis rid→nid

### W1 未通过则不进入 W2

常见卡点：frwder 单点挂、iplist 失败、room closed、Pod IP 进配置。

---

## W2 — 压测标定 + 版本演进（Day 8～14）

### OKR

| O | KR |
|---|-----|
| 标定容量系数 | L1 至少一档 ≥5000 稳定连接；L2 有 ingress + est_downstream |
| 演进故事有数字 | v0/v1/v2 对比表（SENDQ / 异步 ACK） |

### 压测场景（必跑）

| ID | 命令思路 | 报告文件 | 记录 |
|----|----------|----------|------|
| **L1** | keepalive, conns=5000→10000 阶梯 | `k8s-L1-5k.json` … | **C_pod** |
| **L2** | 1 发 + 100 看, rate=5, 60s | `k8s-L2-1send100.json` | ingress, est_downstream, P99 |
| **演进** | Docker 已有 + K8s 重跑 async | `compare-sync/async-ack.json` | 0.7→3.4 QPS 等 |

环境变量（chatroom Pod）：

```yaml
BEEHIVE_CHATROOM_ASYNC_BROADCAST: "1"   # L2 异步场景
```

### 版本演进对比表（W2 填完）

| 版本 | 改动 | 场景 | chat_qps | P99 ms | 备注 |
|------|------|------|----------|--------|------|
| v0 | SENDQ=128 | 50 人互刷 | ~24 | ~1923 | 已有 |
| v1 | SENDQ=8192 | 50 人互刷 | ~39 | ~119 | 已有 |
| v2 | 异步 ACK | 1发100看 | 0.7→**3.4** | sync 53 / async 126 | 已有 |
| v3 | K8s + 多 ws 副本 | L1/L2 | _填_ | _填_ | W2 目标 |

### W2 检查清单

- [ ] `reports/loadtest/k8s-L1-*.json` 至少 1 个
- [ ] `reports/loadtest/k8s-L2-*.json`
- [ ] [CAPACITY_EXTRAPOLATION.md](assets/CAPACITY_EXTRAPOLATION.md) **C_pod、D_pod 已填实数**
- [ ] 能口算：`百万 ws Pod ≈ 1,000,000 / C_pod`
- [ ] 能一句话：`fan-out ≈ chat_qps × 同房人数`
- [ ] （可选）多 broadcast worker 重跑 L2，看 QPS 是否 ≥5

### W2 未通过则不进入 W3

没有 JSON 数字 = 面试外推表是空的，先补压测再演容灾。

---

## W3 — 容灾 + 可观测（Day 15～21）

### OKR

| O | KR |
|---|-----|
| 故障可演示 | delete pod / drain 后系统恢复 |
| 客户端可恢复 | 断线后自动重连 + 可继续收消息 |
| 指标可见 | Grafana ≥2 张截图 |

### 容灾演示脚本（最小集）

| 步骤 | 操作 | 期望 | 面试说什么 |
|------|------|------|------------|
| 1 | 压测 keepalive 2000 连接 | Grafana 连接数上升 | 正常负载 |
| 2 | `kubectl delete pod -l app=websocket --force` | 部分连接断 | 单 Pod 故障 |
| 3 | 观察 Pod 重建 + 客户端重连 | 连接数恢复 | K8s 自愈 + 客户端重连 |
| 4 | `kubectl drain <node> --ignore-daemonsets` | 业务 Pod 迁移 | 节点维护 |
| 5 | ROOM-BC curl push | 在线用户仍能收到 | 触达路径独立于单 Pod |

### 消息「不丢失」口径（演示前背熟）

| 类型 | 演示 | 说法 |
|------|------|------|
| 系统公告 / ROOM-BC | push 后在线收到 | 持久化 + push + 离线补偿 |
| 同房弹幕 | 异步 ACK | **受理 ≠ 全员已收**；峰值靠队列+扩缩 |

### 可观测（最小）

| 指标 | 来源 | 用途 |
|------|------|------|
| Pod CPU/内存 | `kubectl top` / Grafana | L1 OOM 边界 |
| 连接数（可选埋点） | websocket `/metrics` | 扩缩容故事 |
| loadtest 报告 | JSON | QPS/P99 演进 |

Prometheus 装不完：**kubectl top + 压测 JSON + 截图** 也可过 W3。

### W3 检查清单

- [ ] 录屏或笔记：`delete websocket pod` 全过程 ≤5 min
- [ ] demo/web 或 loadtest **断线重连**脚本可跑
- [ ] Grafana 或等价截图 ≥2（连接/资源/QPS 任一）
- [ ] PDB 或 `replicas≥2` 已配置（explain 为什么）
- [ ] （可选）HPA：`replicas 2→4` 手动或 CPU 触发
- [ ] （可选）KEDA：连接数/custom metric 触发 scale

### W3 未通过则不进入 W4 投递

容灾讲不清 = IM 架构岗会被认为「只会 happy path」。

---

## W4 — 面试包 + 投递（Day 22～28）

### OKR

| O | KR |
|---|-----|
| 45 min 演示闭环 | 自己练 **2 遍** 计时 |
| 简历可投 | IM 版定稿 + 投 ≥5 家 |
| 材料齐 | 三件套打包 |

### 45 分钟 Runbook（背顺序）

| 时间 | 内容 | 材料 |
|------|------|------|
| 0～5 | 乐视触达 1.0→2.0；在线/离线双通道 | [RESUME_BULLETS.md](RESUME_BULLETS.md) |
| 5～10 | 架构图：Ingress/ws/frwder/chatroom/Redis | ARCHITECTURE_DIAGRAM |
| 10～15 | `kubectl get pods` + Grafana | Live |
| 15～25 | L2 压测结果 + v0→v2 演进表 | JSON + 表 |
| 25～35 | 外推：「C_pod=___ → 百万 ___ Pod」 | CAPACITY_EXTRAPOLATION |
| 35～42 | 容灾：delete pod + 重连 | 录屏备份 |
| 42～45 | Q&A：1500 上行口径、异步 ACK 语义 | 本文 W2/W3 口径 |

### 面试三件套（W4 打包）

```
interview-pack/
├── reports/loadtest/k8s-L1-*.json
├── reports/loadtest/k8s-L2-*.json
├── reports/loadtest/compare-sync-ack.json
├── reports/loadtest/compare-async-ack.json
├── doc/assets/CAPACITY_EXTRAPOLATION.md   # 填实数 PDF/截图
├── screenshots/grafana-*.png
├── screenshots/kubectl-pods.png
├── demo-fault-injection.mp4               # W3 录屏
└── RESUME_IM.pdf                          # 乐视四条版
```

### 简历 IM 版结构（W4 定稿）

1. **乐视 · 消息触达与在线推送** — 四条 bullet（[RESUME_BULLETS.md](RESUME_BULLETS.md)）
2. **架构验证实验室（可选第二项目）** — K8s 部署、压测标定、异步 ACK 演进、容灾演练
3. **技能**：长连接、fan-out、K8s、Prometheus、压测、触达/inbox

**勿写**：必嗨 / beehive / 「本机已验证百万真连接」

### 投递策略（W4 起）

| 档位 | 岗位关键词 | 期望 | 本周投递数 |
|------|------------|------|------------|
| 冲刺 | IM 架构师 / 推送平台 / 消息中间件 | 38～55K | 2～3 |
| 主力 | IM 高级 / 长连接网关 / 触达开发 | 30～42K | 3～5 |
| 备选 | Go 服务端 / 基础架构（偏消息） | 28～38K | 2 |

### W4 检查清单

- [ ] 45 min Runbook **完整练 2 遍**（含 Q&A）
- [ ] 三件套文件夹齐
- [ ] 简历 IM 版 PDF +  Boss/拉勾/脉脉 更新
- [ ] **投递 ≥5 家**（记录公司/岗位/日期）
- [ ] 准备 3 个乐视生产故障/优化故事（非 demo）
- [ ] （可选）云上 4h 复验录屏补进三件套

---

## 可投简历线（7 月末必须全部满足）

> **满足以下全部 = 7 月 OKR 完成，可主攻 IM 岗；8 月再开 RTC。**

- [ ] K8s smoke：双客户端同房 chat 成功
- [ ] L1 + L2 压测 JSON 各 ≥1
- [ ] 外推表 C_pod 已填
- [ ] v0→v2 演进对比表有实数
- [ ] 容灾演示做过 1 次（录屏或 Live）
- [ ] 45 min Runbook 练过 2 遍
- [ ] 简历 IM 版定稿
- [ ] 已投递 ≥5 家

**未满足项**：继续补，**不启动 8 月 XRTC 计划**。

---

## 每周日晚自检（5 分钟）

```text
本周 OKR 达成？  Y / N
阻塞项：________（frwder / iplist / OOM / 时间）
下周只攻 1 件事：________
是否虚荣进度（只写代码没压测）： Y / N
```

---

## 8 月预告（7 月完成后才启动）

| 周 | 内容 |
|----|------|
| 8 月 W1-2 | XRTC 1.0 一条主链路 + 架构图 |
| 8 月 W3 | FFmpeg 最小闭环 |
| 8 月 W4 | RTC 面试题 + 第二份简历 variant |

IM demo **只维护**（修 bug、补面试反馈），不新开大功能。

---

## 相关文档

- [TWO_WEEK_INTERVIEW_PLAN.md](TWO_WEEK_INTERVIEW_PLAN.md) — 技术阶段细节
- [INTERVIEW_CAPACITY_DEMO.md](INTERVIEW_CAPACITY_DEMO.md) — 三件套与口径
- [RESUME_BULLETS.md](RESUME_BULLETS.md) — 简历四条
- [K8S_DEMO_ROADMAP.md](K8S_DEMO_ROADMAP.md) — manifest 清单

---

*7 月只打一张牌：让面试官相信「这套百万在线 IM 架构，我能讲清楚、测得出、部署得上、坏得 recover」。*
