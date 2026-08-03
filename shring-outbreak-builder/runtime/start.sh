#!/usr/bin/env bash
set -euo pipefail

cd /home/container 2>/dev/null || cd "$(dirname "$0")"

if [ ! -f .env ] && [ -f .env.example ]; then
  cp .env.example .env
fi

chmod +x ./bin/bun

if [ ! -x ./bin/bun ]; then
  echo "[start] ERROR: packaged Bun runtime is missing" >&2
  exit 1
fi

if [ ! -f ./suroi/client/dist/index.html ]; then
  echo "[start] ERROR: precompiled browser client is missing" >&2
  exit 1
fi

if [ ! -d ./suroi/node_modules ]; then
  echo "[start] ERROR: production dependencies are missing" >&2
  exit 1
fi

exec node ./start.mjs
