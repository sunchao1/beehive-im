#!/bin/bash
# 等待 docker-compose 中间件就绪（MySQL 首次启动会自动执行 docker/mysql/init.sql）
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "${ROOT}"

if ! command -v docker >/dev/null 2>&1; then
    echo "docker 未安装"
    exit 1
fi

echo "[init-db] starting middleware..."
docker compose up -d redis mysql mongo

echo "[init-db] waiting for health checks..."
docker compose ps

echo "[init-db] MySQL schema/seed: docker/mysql/init.sql (first boot only)"
echo "[init-db] Mongo indexes/user: docker/mongo/init.js (first boot only)"
echo "[init-db] Redis password: 111111 (see docker/redis/redis.conf)"
echo ""
echo "Connect from host (conf/*.xml defaults):"
echo "  Redis: 127.0.0.1:6379"
echo "  MySQL: 127.0.0.1:3306  user=root pass=111111 db=testdb"
echo "  Mongo: 127.0.0.1:27017 user=beehive pass=111111 db=chat"
