#!/bin/bash
set -euo pipefail

echo "=== Go dependencies ==="
cd /workspace
go mod download

echo "=== Submodule init ==="
git submodule update --init --recursive third_party/misskey

echo "=== Wait for PostgreSQL ==="
for i in $(seq 1 30); do
    pg_isready -h localhost -p 5432 -U misskey && break
    echo "Waiting for PostgreSQL... ($i/30)"
    sleep 1
done

echo "=== Database migration ==="
make migrate-up || echo "Migration failed (may already be applied)"

echo "=== Frontend build ==="
cd /workspace/third_party/misskey
# Corepackがバージョン不一致時にダウンロード確認を求めないようにする
export COREPACK_ENABLE_DOWNLOAD_PROMPT=0
pnpm install --frozen-lockfile
pnpm build

echo "=== Done! Run 'make dev' to start the server ==="
