# 01 · RTMQ 架构与数据流（摘要）

> **完整设计说明** → [RTMQ-技术设计文档.md](RTMQ-技术设计文档.md)（背景、目标、协议、线程模型、frwder 双端口）  
> **完整流程图与十七篇定位** → [00-总地图.md](00-总地图.md)

## 1. 在必嗨 IM 中的位置

```text
[Go/C 业务进程]                    [RTMQ Server 中心]
  rtmq_proxy (客户端库)  ←TCP→   rtmq_recv / rsvr / worker
       ↑                                ↑
   frwder / usrsvr / msgsvr / websocket  单进程或多线程
```

- **28889 BACKEND**：业务进程 Proxy 连 Server，`publish` 收上行。
- **28888 FORWARD**：接入进程 Proxy 连 Server，`async_send(nid)` 收下行。

frwder 本身 **不实现 RTMQ**，只是 **Proxy 里 reg 的回调** 做 `publish` / `async_send`。

---

## 2. 一包数据的两种走法

### 上行（publish）

```text
接入 Proxy.async_send → Server → 业务 Proxy.recv → worker → reg(type)
```

### 下行（async_send + nid）

```text
msgsvr Proxy.async_send(type, nid, frame) → dist → 目标 rsvr → 接入 Proxy → websocket
```

---

## 3. Server / Proxy 线程对照

| Server | 文件 | Proxy | 文件 |
|--------|------|-------|------|
| listen | rtmq_lsn.c | tsvr | rtmq_proxy_tsvr.c |
| rsvr | rtmq_rsvr.c | worker | rtmq_proxy_worker.c |
| worker | rtmq_worker.c | — | — |
| dist | rtmq_dist.c | — | — |

下一篇：[RTMQ-技术设计文档.md](RTMQ-技术设计文档.md) · [00-总地图.md](00-总地图.md) → [02-rtmq_mesg.md](02-rtmq_mesg.md)
