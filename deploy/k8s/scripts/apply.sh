#!/bin/bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
cd "${ROOT}/deploy/k8s"

chmod +x "${ROOT}/deploy/k8s/scripts/"*.sh

"${ROOT}/deploy/k8s/scripts/render-config.sh"

kubectl apply -f namespace.yaml

for f in middleware/*.yaml; do
  kubectl apply -f "${f}"
done

echo "[apply] waiting for redis..."
kubectl wait --for=condition=ready pod -l app=redis -n beehive --timeout=180s 2>/dev/null || true

kubectl create configmap beehive-conf \
  --from-file="${ROOT}/deploy/k8s/.rendered/" \
  -n beehive \
  --dry-run=client -o yaml | kubectl apply -f -

for f in apps/frwder.yaml apps/seqsvr.yaml apps/msgsvr.yaml apps/tasker.yaml \
         apps/usrsvr.yaml apps/monitor.yaml apps/chatroom.yaml \
         apps/websocket.yaml apps/websocket-2.yaml; do
  kubectl apply -f "${f}"
done

kubectl apply -f ingress.yaml
kubectl apply -f hpa-websocket.yaml

echo "[apply] done. kubectl get pods -n beehive"
