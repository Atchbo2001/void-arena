#!/usr/bin/env bash
set -euo pipefail

: "${UPSTREAM_COMMIT:=85df5067b19a876cac4304232cc1e68ff1b07c7f}"
: "${RELEASE_VERSION:=1.1.0}"
: "${PUBLIC_URL:=https://shootup.shring.net}"

ROOT="${GITHUB_WORKSPACE:-$(pwd)}"
WORK="$ROOT/work"
SOURCE="$WORK/suroi"
PACKAGE="$WORK/package"
ZIP="Shring-Outbreak-Suroi-v${RELEASE_VERSION}-PREBUILT-FLAT.zip"

rm -rf "$WORK" "$ROOT/$ZIP" "$ROOT/$ZIP.sha256"
mkdir -p "$SOURCE"

echo "[build] Downloading pinned Suroi source $UPSTREAM_COMMIT"
curl --fail --location --retry 4 --retry-delay 3 \
  --output "$WORK/suroi.tar.gz" \
  "https://github.com/HasangerGames/suroi/archive/${UPSTREAM_COMMIT}.tar.gz"
tar -xzf "$WORK/suroi.tar.gz" --strip-components=1 -C "$SOURCE"
test -f "$SOURCE/package.json"

echo "[build] Applying Shring changes"
node "$ROOT/shring-outbreak-builder/apply-patches.mjs" "$SOURCE" "$PUBLIC_URL"

echo "[build] Installing build dependencies"
cd "$SOURCE"
bun install --frozen-lockfile

echo "[build] Compiling production browser client"
NODE_ENV=production bun run build:client

test -s client/dist/index.html
test -d client/dist/scripts
test -d client/dist/img
grep -q "Shring Outbreak" client/dist/index.html
if grep -R -q "127.0.0.1:8000" client/dist/scripts; then
  echo "Compiled client still contains the development server address" >&2
  exit 1
fi

echo "[build] Replacing build dependencies with production dependencies"
rm -rf node_modules client/node_modules common/node_modules server/node_modules tests/node_modules
bun install --production --frozen-lockfile
test -d node_modules
du -sh node_modules client/dist

echo "[build] Assembling flat Pterodactyl package"
mkdir -p "$PACKAGE/bin" "$PACKAGE/suroi"
rsync -a --delete \
  --exclude '.git' \
  --exclude '.github' \
  --exclude 'tests' \
  "$SOURCE/" "$PACKAGE/suroi/"
cp "$(command -v bun)" "$PACKAGE/bin/bun"
cp "$ROOT/shring-outbreak-builder/runtime/gateway.mjs" "$PACKAGE/gateway.mjs"
cp "$ROOT/shring-outbreak-builder/runtime/start.mjs" "$PACKAGE/start.mjs"
cp "$ROOT/shring-outbreak-builder/runtime/start.sh" "$PACKAGE/start.sh"
cp "$ROOT/shring-outbreak-builder/runtime/.env.example" "$PACKAGE/.env.example"
cp "$ROOT/shring-outbreak-builder/runtime/README-FIRST.md" "$PACKAGE/README-FIRST.md"
cp "$ROOT/shring-outbreak-builder/runtime/THIRD_PARTY_NOTICES.md" "$PACKAGE/THIRD_PARTY_NOTICES.md"
cat > "$PACKAGE/VERSION.json" <<EOF
{
  "version": "${RELEASE_VERSION}",
  "upstreamCommit": "${UPSTREAM_COMMIT}",
  "builtAt": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "publicUrl": "${PUBLIC_URL}",
  "guestPlay": true
}
EOF
chmod +x "$PACKAGE/start.sh" "$PACKAGE/bin/bun"
test -x "$PACKAGE/bin/bun"
test -s "$PACKAGE/suroi/client/dist/index.html"
test -d "$PACKAGE/suroi/node_modules"
du -sh "$PACKAGE"

echo "[build] Smoke testing the packaged runtime without a global Bun"
cd "$PACKAGE"
NODE_DIR="$(dirname "$(command -v node)")"
SMOKE_PATH="$NODE_DIR:/usr/bin:/bin"
if PATH="$SMOKE_PATH" command -v bun >/dev/null 2>&1; then
  echo "Smoke-test PATH unexpectedly contains a global Bun executable" >&2
  exit 1
fi
PATH="$SMOKE_PATH" ./start.sh > smoke-test.log 2>&1 &
SERVER_PID=$!
cleanup() {
  kill -TERM "$SERVER_PID" 2>/dev/null || true
  wait "$SERVER_PID" 2>/dev/null || true
}
trap cleanup EXIT

healthy=0
for attempt in $(seq 1 120); do
  if curl --fail --silent http://127.0.0.1:31025/health > health.json; then
    healthy=1
    break
  fi
  if ! kill -0 "$SERVER_PID" 2>/dev/null; then
    cat smoke-test.log
    exit 1
  fi
  sleep 1
done

if [ "$healthy" -ne 1 ]; then
  cat smoke-test.log
  echo "Health endpoint did not become ready" >&2
  exit 1
fi

cat health.json
grep -q '"ok":true' health.json
grep -q '"guestPlay":true' health.json
curl --fail --silent http://127.0.0.1:31025/ | grep -q "Shring Outbreak"
curl --fail --silent http://127.0.0.1:31025/api/serverInfo > server-info.json
curl --fail --silent http://127.0.0.1:31025/api/getGame > game.json
cat server-info.json
cat game.json
cleanup
trap - EXIT

rm -f smoke-test.log health.json server-info.json game.json .env

echo "[build] Creating upload ZIP with workspace symlinks preserved"
cd "$PACKAGE"
zip -r -y -6 "$ROOT/$ZIP" .
cd "$ROOT"
sha256sum "$ZIP" > "$ZIP.sha256"
unzip -t "$ZIP"
SIZE=$(stat -c%s "$ZIP")
echo "ZIP size: $SIZE bytes"
if [ "$SIZE" -gt 524288000 ]; then
  echo "ZIP exceeds the 500 MB Pterodactyl upload target" >&2
  exit 1
fi

echo "[build] Completed $ZIP"
