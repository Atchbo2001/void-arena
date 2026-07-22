#!/usr/bin/env bash

set -Eeuo pipefail

cd /home/container 2>/dev/null || cd "$(dirname "$0")"

PORT_VALUE="${SERVER_PORT:-${PORT:-1337}}"
CONFIG_FILE="${SOUR_CONFIG:-pterodactyl.yaml}"

if [[ ! "$PORT_VALUE" =~ ^[0-9]+$ ]] || (( PORT_VALUE < 1 || PORT_VALUE > 65535 )); then
  echo "Invalid SERVER_PORT/PORT value: $PORT_VALUE" >&2
  exit 64
fi

if [[ ! -x ./bin/sour ]]; then
  echo "Missing executable: ./bin/sour" >&2
  echo "Upload and extract the complete GitHub Actions Pterodactyl artifact." >&2
  exit 66
fi

if [[ ! -f "$CONFIG_FILE" ]]; then
  echo "Missing configuration file: $CONFIG_FILE" >&2
  exit 66
fi

if [[ ! -f ./assets/dist/.index.source ]]; then
  echo "Missing game assets: ./assets/dist/.index.source" >&2
  exit 66
fi

mkdir -p .cache/assets logs
chmod +x ./bin/sour

# Sour uses the Pterodactyl primary allocation for HTTP/WebSocket traffic.
# The integrated game servers stay inside this process, so no public master
# server or third-party UDP proxy is started.
echo "Starting Void Arena on 0.0.0.0:${PORT_VALUE}"
exec ./bin/sour serve --address 0.0.0.0 --port "$PORT_VALUE" "$CONFIG_FILE"
