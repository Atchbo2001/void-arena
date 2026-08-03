import fs from "node:fs";
import fsp from "node:fs/promises";
import http from "node:http";
import net from "node:net";
import path from "node:path";
import { pipeline } from "node:stream/promises";

const MIME = new Map([
  [".html", "text/html; charset=utf-8"], [".js", "text/javascript; charset=utf-8"], [".mjs", "text/javascript; charset=utf-8"],
  [".css", "text/css; charset=utf-8"], [".json", "application/json; charset=utf-8"], [".svg", "image/svg+xml"],
  [".png", "image/png"], [".jpg", "image/jpeg"], [".jpeg", "image/jpeg"], [".webp", "image/webp"], [".gif", "image/gif"],
  [".ico", "image/x-icon"], [".woff", "font/woff"], [".woff2", "font/woff2"], [".mp3", "audio/mpeg"], [".ogg", "audio/ogg"], [".wasm", "application/wasm"]
]);

function proxyHttp(req, res, port, rewrite = value => value) {
  const headers = { ...req.headers, host: `127.0.0.1:${port}` };
  const upstream = http.request({ hostname: "127.0.0.1", port, path: rewrite(req.url || "/"), method: req.method, headers }, upstreamRes => {
    res.writeHead(upstreamRes.statusCode || 502, upstreamRes.headers);
    upstreamRes.pipe(res);
  });
  upstream.on("error", error => {
    if (!res.headersSent) res.writeHead(502, { "content-type": "application/json; charset=utf-8" });
    res.end(JSON.stringify({ ok: false, error: "upstream_unavailable", message: error.message }));
  });
  req.pipe(upstream);
}

function proxyUpgrade(req, socket, head, port, rewrite = value => value) {
  const upstream = net.connect(port, "127.0.0.1");
  upstream.setNoDelay(true);
  socket.setNoDelay(true);
  upstream.once("connect", () => {
    const url = rewrite(req.url || "/");
    const headers = { ...req.headers, host: `127.0.0.1:${port}`, connection: "Upgrade", upgrade: req.headers.upgrade || "websocket" };
    let request = `${req.method || "GET"} ${url} HTTP/${req.httpVersion}\r\n`;
    for (const [key, value] of Object.entries(headers)) {
      if (value === undefined) continue;
      if (Array.isArray(value)) for (const item of value) request += `${key}: ${item}\r\n`;
      else request += `${key}: ${value}\r\n`;
    }
    request += "\r\n";
    upstream.write(request);
    if (head?.length) upstream.write(head);
    socket.pipe(upstream).pipe(socket);
  });
  const fail = error => {
    try { socket.write("HTTP/1.1 502 Bad Gateway\r\nConnection: close\r\n\r\n"); } catch {}
    socket.destroy(error);
    upstream.destroy();
  };
  upstream.once("error", fail);
  socket.once("error", () => upstream.destroy());
}

export function createGateway({ host, port, internalBasePort, maxGames, staticRoot, version, upstreamCommit }) {
  const server = http.createServer(async(req, res) => {
    try {
      const requestUrl = new URL(req.url || "/", `http://${req.headers.host || "localhost"}`);
      const pathname = decodeURIComponent(requestUrl.pathname);
      res.setHeader("x-content-type-options", "nosniff");
      res.setHeader("referrer-policy", "same-origin");
      res.setHeader("permissions-policy", "camera=(), microphone=(), geolocation=()");

      if (pathname === "/health") {
        const check = await new Promise(resolve => {
          const probe = http.get({ hostname: "127.0.0.1", port: internalBasePort, path: "/api/serverInfo", timeout: 2500 }, response => {
            response.resume(); resolve(response.statusCode === 200);
          });
          probe.on("timeout", () => { probe.destroy(); resolve(false); });
          probe.on("error", () => resolve(false));
        });
        res.writeHead(check ? 200 : 503, { "content-type": "application/json; charset=utf-8", "cache-control": "no-store" });
        res.end(JSON.stringify({ ok: check, service: "shring-outbreak", version, upstream: "suroi", upstreamCommit, guestPlay: true }));
        return;
      }

      if (pathname.startsWith("/api/") || pathname === "/team") {
        proxyHttp(req, res, internalBasePort);
        return;
      }

      const gameMatch = pathname.match(/^\/game\/(\d+)(\/.*)?$/);
      if (gameMatch) {
        const gamePortNumber = Number(gameMatch[1]);
        if (!Number.isInteger(gamePortNumber) || gamePortNumber < 1 || gamePortNumber > maxGames) {
          res.writeHead(404); res.end("Unknown game server"); return;
        }
        proxyHttp(req, res, internalBasePort + gamePortNumber, url => url.replace(/^\/game\/\d+/, "") || "/play");
        return;
      }

      let relative = pathname.replace(/^\/+/, "");
      if (!relative || relative.endsWith("/")) relative += "index.html";
      let file = path.resolve(staticRoot, relative);
      const rootResolved = path.resolve(staticRoot) + path.sep;
      if (!file.startsWith(rootResolved)) { res.writeHead(403); res.end("Forbidden"); return; }
      let stat;
      try { stat = await fsp.stat(file); } catch { stat = undefined; }
      if (!stat?.isFile()) file = path.join(staticRoot, "index.html");
      const ext = path.extname(file).toLowerCase();
      const headers = {
        "content-type": MIME.get(ext) || "application/octet-stream",
        "cache-control": ext === ".html" ? "no-cache, no-store, must-revalidate" : "public, max-age=31536000, immutable"
      };
      const fileStat = await fsp.stat(file);
      headers["content-length"] = fileStat.size;
      res.writeHead(200, headers);
      await pipeline(fs.createReadStream(file), res);
    } catch (error) {
      console.error("[gateway] HTTP error", error);
      if (!res.headersSent) res.writeHead(500, { "content-type": "text/plain; charset=utf-8" });
      res.end("Internal server error");
    }
  });

  server.on("upgrade", (req, socket, head) => {
    const pathname = new URL(req.url || "/", "http://localhost").pathname;
    if (pathname === "/team") {
      proxyUpgrade(req, socket, head, internalBasePort);
      return;
    }
    const match = pathname.match(/^\/game\/(\d+)(\/.*)?$/);
    if (!match) { socket.destroy(); return; }
    const gameNumber = Number(match[1]);
    if (!Number.isInteger(gameNumber) || gameNumber < 1 || gameNumber > maxGames) { socket.destroy(); return; }
    proxyUpgrade(req, socket, head, internalBasePort + gameNumber, url => url.replace(/^\/game\/\d+/, "") || "/play");
  });

  return new Promise((resolve, reject) => {
    server.once("error", reject);
    server.listen(port, host, () => {
      console.log(`[gateway] Listening on http://${host}:${port}`);
      resolve(server);
    });
  });
}
