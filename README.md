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
| [故障排查](doc/TROUBLESHOOT.md) | Top 10 + 日志地图 |
| [规模说明](doc/SCALE.md) | 单机 demo vs 百万在线（不夸大本机容量） |
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
scripts/       # up-demo、smoke-*、status
doc/           # 架构、演示、排障文档
```

---

## 更多

- 完整架构：[doc/ARCHITECTURE.md](doc/ARCHITECTURE.md)
- 实现范围：[doc/DEMO_SCOPE.md](doc/DEMO_SCOPE.md)
- 上游：[beehivestudio/beehive-im](https://github.com/beehivestudio/beehive-im)
