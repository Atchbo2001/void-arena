# Shring Outbreak — precompiled Pterodactyl package

This package is built on GitHub Actions from a pinned Suroi commit. The memory-intensive Vite and sprite compilation has already been completed before deployment.

## Target

- Public URL: `https://shootup.shring.net`
- Pterodactyl allocation: `192.168.1.65:31025`
- Egg: Node.js 22
- Account required: no
- Existing nginx, DNS, TLS certificate, and Certbot configuration: unchanged

## Install

1. Stop the Pterodactyl server.
2. Remove the previous bootstrap package and its partially built `suroi` folder.
3. Extract this ZIP directly into `/home/container`.
4. Set the startup command to:

```bash
bash ./start.sh
```

5. Start the server.

The server should start the already-compiled game directly. It must not run Vite, spritesheetc, `bun install`, or any build step.

## Verify

```bash
curl -fsS http://192.168.1.65:31025/health
curl -fsS https://shootup.shring.net/health
```

Healthy output includes:

```json
{"ok":true,"service":"shring-outbreak","guestPlay":true}
```

## Package layout

- `bin/bun` — portable Bun runtime built into the package
- `suroi/` — complete modified GPL source, compiled client, and production dependencies
- `start.mjs` — process supervisor
- `gateway.mjs` — static server and single-port HTTP/WebSocket gateway
- `VERSION.json` — build provenance
- `THIRD_PARTY_NOTICES.md` — upstream and licensing notice

## Current gameplay scope

This is the stable Suroi multiplayer shooter base under Shring branding. It is guest-playable and does not require Shring Auth. Zombie waves, vehicles, construction, and persistent progression are not claimed as implemented in this release.
