import fs from "node:fs";
import http from "node:http";
import path from "node:path";
import process from "node:process";
import { spawn } from "node:child_process";
import { createGateway } from "./gateway.mjs";

const ROOT = path.resolve(import.meta.dirname);
const SOURCE_DIR = path.join(ROOT, "suroi");
const BUN = path.join(ROOT, "bin", "bun");
const VERSION_FILE = path.join(ROOT, "VERSION.json");

function parseEnv(file) {
  if (!fs.existsSync(file)) return {};
  const out = {};
  for (const raw of fs.readFileSync(file, "utf8").split(/\r?\n/)) {
    const line = raw.trim();
    if (!line || line.startsWith("#")) continue;
    const i = line.indexOf("=");
    if (i < 1) continue;
    out[line.slice(0, i).trim()] = line.slice(i + 1).trim().replace(/^['"]|['"]$/g, "");
  }
  return out;
}

const env = { ...parseEnv(path.join(ROOT, ".env")), ...process.env };
const host = env.SERVER_HOST || "0.0.0.0";
const port = Number(env.SERVER_PORT || 31025);
const internalBasePort = Number(env.INTERNAL_BASE_PORT || 8000);
const maxGames = Math.max(1, Math.min(5, Number(env.MAX_GAMES || 1)));
const metadata = JSON.parse(fs.readFileSync(VERSION_FILE, "utf8"));

for (const required of [BUN, path.join(SOURCE_DIR, "package.json"), path.join(SOURCE_DIR, "client", "dist", "index.html"), path.join(SOURCE_DIR, "node_modules")]) {
  if (!fs.existsSync(required)) throw new Error(`Production package is incomplete: missing ${required}`);
}

let child;
let gateway;
let stopping = false;

function shutdown(signal = "SIGTERM") {
  if (stopping) return;
  stopping = true;
  console.log(`[start] Received ${signal}; shutting down`);
  try { gateway?.close(); } catch {}
  try { child?.kill("SIGTERM"); } catch {}
  setTimeout(() => process.exit(0), 3000).unref();
}

process.on("SIGINT", () => shutdown("SIGINT"));
process.on("SIGTERM", () => shutdown("SIGTERM"));

async function waitForMainServer(timeoutMs = 120000) {
  const started = Date.now();
  while (Date.now() - started < timeoutMs) {
    if (child?.exitCode !== null && child?.exitCode !== undefined) throw new Error(`Game server exited before becoming ready (code ${child.exitCode})`);
    const ready = await new Promise(resolve => {
      const req = http.get({ hostname: "127.0.0.1", port: internalBasePort, path: "/api/serverInfo", timeout: 2000 }, res => {
        res.resume();
        resolve(res.statusCode === 200);
      });
      req.on("timeout", () => { req.destroy(); resolve(false); });
      req.on("error", () => resolve(false));
    });
    if (ready) return;
    await new Promise(resolve => setTimeout(resolve, 1000));
  }
  throw new Error(`Internal game API did not become ready on 127.0.0.1:${internalBasePort}`);
}

try {
  child = spawn(BUN, ["run", "start"], { cwd: SOURCE_DIR, stdio: "inherit", env: { ...process.env, NODE_ENV: "production" } });
  child.once("error", error => { console.error("[start] Game process failed", error); process.exit(1); });
  child.once("exit", code => { if (!stopping) { console.error(`[start] Game process exited with code ${code}`); process.exit(code || 1); } });
  console.log("[start] Waiting for the internal game API...");
  await waitForMainServer();
  gateway = await createGateway({ host, port, internalBasePort, maxGames, staticRoot: path.join(SOURCE_DIR, "client", "dist"), version: metadata.version, upstreamCommit: metadata.upstreamCommit });
  console.log(`[start] Shring Outbreak ${metadata.version} is ready`);
  console.log("[start] Guest play is enabled; no account is required");
  console.log(`[start] Public URL: ${env.PUBLIC_URL || "https://shootup.shring.net"}`);
} catch (error) {
  console.error("[start] Fatal startup error:", error?.stack || error);
  try { child?.kill("SIGTERM"); } catch {}
  process.exit(1);
}
