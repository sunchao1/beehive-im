# Demo Web 客户端

> 推荐演示流程见 [doc/INTERVIEW_DEMO.md](../doc/INTERVIEW_DEMO.md) §8～13 min。  
> **群聊源码精读（Go+C+前端，面试向）**：[doc/群聊Demo源码导读.md](../doc/群聊Demo源码导读.md)  
> **周末代码清单 + 验证计划**：[doc/群聊Demo代码清单与学习计划.md](../doc/群聊Demo代码清单与学习计划.md)

## 三步启动（docker compose）

**首次**（构建镜像并在容器内编译，改 Go/C 代码后重跑）：

```bash
docker compose --profile build run --rm --build builder
```

**启动全栈**（中间件 + 业务 + 演示页）：

```bash
docker compose --profile run --profile demo up -d
```

等价快捷方式：`./scripts/up-demo.sh`

**群聊演示页**：http://127.0.0.1:8088/group.html

**停止**：

```bash
docker compose --profile run --profile demo down
```

## 弹幕演示（index.html）

开 **两个** 浏览器窗口：

- 窗口 A：uid=`100001`，rid=`10001`，点「注册并连接」
- 窗口 B：uid=`100002`，rid=`10001`，点「注册并连接」
- 互发弹幕

## 群聊演示（group.html）

访问 http://127.0.0.1:8088/group.html ，开 **两个** 浏览器窗口：

| 步骤 | 窗口 A（uid=100001） | 窗口 B（uid=100002） |
|------|----------------------|----------------------|
| 1 | 注册并连接 | 注册并连接 |
| 2 | 创建群（默认名 demo-group） | — |
| 3 | 记下日志里的 `gid` | 填入同一 GID → 加入群 |
| 4 | 互发群消息 | 互发群消息 |

可选：A 用「邀请入群」拉 B（等价于 B 主动加群）；群主可用「解散群」结束演示。

终端验收：`./scripts/smoke-group.sh`

## 截图位

<!-- TODO: 双窗口同房弹幕截图 -->
<!-- TODO: 双窗口群聊截图 -->

## 协议

- HTTP：`/im/register`、`/im/iplist?type=2`
- WebSocket：`ws://127.0.0.1:8002/im`
- 帧格式：52 字节大端头 + protobuf 体（见 `doc/PROTOCOL.md`）
- 群聊：`GROUP-CREAT/JOIN/CHAT` 等 0x03xx（见 `doc/GROUP_DESIGN.md`）

## 依赖

无 CDN；`pb.js` 为内联最小 protobuf 编解码。

## 错误场景（演示用）

- **未进房发弹幕**：只连接不 join，发消息应失败（见 `smoke-fail.sh`）
- **未加群发消息**：群聊页未建群/加群时输入框禁用
- **重复 join / 错误 rid**：见 [INTERVIEW_DEMO.md](../doc/INTERVIEW_DEMO.md) 场景 C  
- 页面内暂未做「停 frwder」按钮；用终端 `docker compose --profile run stop runner` 模拟栈故障
