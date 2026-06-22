# 必嗨 IM（beehive-im）

面向高并发的即时通信系统，支持 **私聊、群聊、聊天室（弹幕）、推送**。  
架构：C 接入/网关（listend、frwder）+ Go 业务（usrsvr、chatroom、msgsvr 等）+ Redis / MySQL / Mongo。

---

## Quick Start（Docker Demo）

**首次使用**（macOS 需在 Linux 容器内编译 C 服务）：

```bash
docker compose --profile build run --rm builder /workspace/scripts/build-linux.sh
./scripts/up-demo.sh
./scripts/smoke-test.sh
./scripts/serve-demo.sh    # 浏览器 http://127.0.0.1:8088/
```

| 文档 | 说明 |
|------|------|
| **[面试 15 分钟演示脚本](doc/INTERVIEW_DEMO.md)** | 时间轴、话术、错误场景、命令速查 |
| [日常演示](doc/DEMO.md) | 端口、测试数据、验收命令 |
| [压测指南](doc/LOADTEST.md) | loadtest 工具、baseline 场景、调优 |
| [故障排查](doc/TROUBLESHOOT.md) | Top 10 + 日志地图 |
| [规模说明](doc/SCALE.md) | 单机 demo vs 百万在线（不夸大本机容量） |
| [K8s 演示路线图](doc/K8S_DEMO_ROADMAP.md) | 本机 64G VM、扩缩容、Prometheus |
| [Phase3 性能改造选型](doc/PHASE3_PERFORMANCE_EVOLUTION.md) | 异步 ACK vs Kafka、压测对比 |
| [百万演示容量规划](doc/MILLION_DEMO_PLAN.md) | 反向机器数、一日云成本、三种「百万」口径 |
| [低成本面试容量演示](doc/INTERVIEW_CAPACITY_DEMO.md) | 64G 标定 + 外推 + ¥100 内云上打点一次 |
| [弹幕核心接口手册](doc/弹幕核心接口.md) | 按接口熟悉弹幕/ROOM 链路代码入口 |
| [**弹幕系统名词解释 · 分层 Fan-out 寻址**](doc/弹幕系统的名词解释.md) | NID/RID/SID/CID/GID + 下行 RID→NID→CID 全过程 |
| [流程图与术语手册](doc/FLOWS_AND_GLOSSARY.md) | 全景流程、各子系统、术语表 |
| [上行→面板详细流程图](doc/assets/PUSH_TO_PANEL_FLOW.md) | ROOM-CHAT / ROOM-BC 端到端 Mermaid 图 |
| [7 月 IM 四周 OKR 检查表](doc/JULY_IM_OKR_CHECKLIST.md) | 每周 deliverable、可投简历线、45min Runbook |
| [两周面试冲刺计划](doc/TWO_WEEK_INTERVIEW_PLAN.md) | 64G K8s 分阶段任务、资源清单、14 天甘特 |
| [简历四条定稿（粘贴版）](doc/RESUME_BULLETS.md) | 乐视触达项目 bullet、自我介绍、关键词 |
| [架构图（一页）](doc/assets/ARCHITECTURE_DIAGRAM.md) | Mermaid 拓扑 |

测试用户：`uid=100001` / `100002`，种子房间 `rid=10001`。

---

## 版本与二进制

- 构建版本：`Makefile` 中 `VERSION=v.1.1` → 产物 `bin/*.v.1.1`
- 协议与命令：[doc/COMMAND.md](doc/COMMAND.md)、[doc/PROTOCOL.md](doc/PROTOCOL.md)

---

## 仓库结构（简）

```
src/clang/     # frwder、listend（C）
src/golang/    # usrsvr、chatroom、msgsvr、websocket 等
conf/templates/ + docker/gen-conf.sh   # 运行时配置渲染
demo/web/      # 浏览器双用户弹幕（推荐）
scripts/       # up-demo、smoke-*、loadtest-*、status
doc/           # 架构、演示、排障文档
```

---

## 更多

- 完整架构：[doc/ARCHITECTURE.md](doc/ARCHITECTURE.md)
- 实现范围：[doc/DEMO_SCOPE.md](doc/DEMO_SCOPE.md)
- 上游：[beehivestudio/beehive-im](https://github.com/beehivestudio/beehive-im)
