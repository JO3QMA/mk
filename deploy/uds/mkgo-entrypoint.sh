#!/bin/sh
# Run schema migrations then exec the mk-go server.
# compose.uds.yaml から /app/.config/default.yml がマウントされる想定。
set -e

cd /app

echo "[mkgo-entrypoint] running migrations..."
/app/migrate -config /app/.config/default.yml -direction up

echo "[mkgo-entrypoint] starting misskey server..."
exec /app/misskey -config /app/.config/default.yml
