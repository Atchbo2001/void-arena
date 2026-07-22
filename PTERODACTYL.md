# Void Arena on Pterodactyl

This repository is the full Sour/Sauerbraten source fork. GitHub Actions compiles the game, web client, assets, and Linux backend into `void-arena-pterodactyl.zip`.

## Upload

1. Open the latest successful **Build Void Arena for Pterodactyl** workflow run.
2. Download the `void-arena-pterodactyl-...` artifact.
3. Extract the artifact once to obtain `void-arena-pterodactyl.zip`.
4. Upload that ZIP into `/home/container` in Pterodactyl and extract it there.
5. Do not place the files inside an extra nested folder. `/home/container/start.sh`, `/home/container/bin/sour`, and `/home/container/assets/dist` must exist.

## Node.js 22 egg settings

Use either startup command:

```bash
npm start
```

or:

```bash
bash ./start.sh
```

No npm packages are required. The Node.js egg is only being used as the Linux container environment; the actual backend is the compiled `bin/sour` server.

The startup script automatically uses Pterodactyl's `SERVER_PORT` allocation. The default configuration is `pterodactyl.yaml`. To use another file, define `SOUR_CONFIG` with its path.

## Reverse proxy

Proxy normal HTTP and WebSocket traffic to the Pterodactyl allocation. The same port serves the website, API, and `/ws/` WebSocket endpoint. WebSocket upgrade headers must be enabled in nginx.

## Server browser behavior

This fork does not contact `master.sauerbraten.org`, does not probe public Sauerbraten hosts, and does not broadcast third-party servers to browser clients. The in-game browser is populated only by integrated servers configured under `server.spaces` in `pterodactyl.yaml`.
