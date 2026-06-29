# Beehive IM · Kubernetes 部署

> **前置**：在 Linux/amd64 构建镜像与二进制（与 Docker Compose 相同）  
> **iplist 方案 A**：usrsvr 通过 `BEEHIVE_WS_IPLIST` 返回固定 WS 入口，不依赖 Pod IP。

---

## 1. 构建

```bash
# 编译 Linux 二进制
docker compose --profile build run --rm --build builder

# 运行时镜像（含 bin + C 动态库）
docker build -f docker/Dockerfile.runtime -t beehive-im-runtime:latest .
```

本地集群（minikube / kind）需加载镜像：

```bash
minikube image load beehive-im-runtime:latest
# 或: kind load docker-image beehive-im-runtime:latest
```

---

## 2. 渲染配置并部署

```bash
# 渲染 conf/templates → deploy/k8s/.rendered/
./deploy/k8s/scripts/render-config.sh

# 创建 ConfigMap + 按序 apply
./deploy/k8s/scripts/apply.sh
```

**修改 WS 入口**（Ingress 域名）：

```bash
# 编辑 deploy/k8s/apps/usrsvr.yaml 中 BEEHIVE_WS_IPLIST
# 示例：wss://beehive.local/im
# 或 NodePort：$(minikube ip):30002
kubectl apply -f deploy/k8s/apps/usrsvr.yaml
```

`/etc/hosts` 增加：`127.0.0.1 beehive.local`（minikube tunnel / ingress-nginx）

---

## 3. 目录结构

```text
deploy/k8s/
  namespace.yaml
  kustomization.yaml
  middleware/          # redis mysql mongo
  apps/                  # frwder usrsvr websocket …
  ingress.yaml           # HTTP + WebSocket 入口
  hpa-websocket.yaml
  scripts/render-config.sh
  scripts/apply.sh
  scripts/run-service.sh # 容器内单进程启动
```

---

## 4. iplist 方案 A（已实现）

| 方式 | 配置 |
|------|------|
| **环境变量（推荐 K8s）** | usrsvr Pod：`BEEHIVE_WS_IPLIST=wss://beehive.local/im` |
| **XML** | `usrsvr.xml` 内 `<IPLIST><STATIC TYPE="2" ADDR="…"/></IPLIST>` |
| **legacy** | 不设上述项 → 仍走 LSND_INFO → Redis 字典 |

客户端 / smoke 已支持 **完整 `ws://` / `wss://` URL**（`lib/comm/iplist.go`）。

---

## 5. 扩缩容

```bash
# websocket HPA（CPU）
kubectl get hpa -n beehive

# 手动扩 websocket
kubectl scale deployment websocket -n beehive --replicas=3
```

**注意**：每副本需 **不同 NID**（当前骨架为 2 套 websocket/websocket-2；多副本需 StatefulSet + 按 ordinal 分配 NID，见 [四周计划](../四周计划-RTMQ群聊与K8s架构演进.md)）。

---

## 6. Prometheus（占位）

Pod 模板已预留 annotation：

```yaml
prometheus.io/scrape: "true"
prometheus.io/port: "8000"
```

usrsvr 尚未暴露 `/metrics`；接入 Prometheus 时需在各服务加 exporter 或 Beego metrics。

---

## 7. 验证

```bash
kubectl get pods -n beehive
curl "http://beehive.local/im/register?uid=100001&nation=1&city=1&town=1"
curl "http://beehive.local/im/iplist?type=2&uid=100001&sid=...&clientip=127.0.0.1"
# list[0] 应为 BEEHIVE_WS_IPLIST 配置值
```

群聊：`http://beehive.local/` 需自行部署 demo-web 或 port-forward usrsvr:8000。

---

## 8. 下线

```bash
kubectl delete namespace beehive
```
