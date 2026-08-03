import fs from "node:fs/promises";
import path from "node:path";

const [sourceArg, publicUrlArg = "https://shootup.shring.net"] = process.argv.slice(2);
if (!sourceArg) throw new Error("Usage: node apply-patches.mjs <source-directory> [public-url]");

const sourceDir = path.resolve(sourceArg);
const publicUrl = publicUrlArg.replace(/\/$/, "");
const upstreamCommit = process.env.UPSTREAM_COMMIT || "85df5067b19a876cac4304232cc1e68ff1b07c7f";

async function read(relative) {
  return fs.readFile(path.join(sourceDir, relative), "utf8");
}

async function write(relative, content) {
  const file = path.join(sourceDir, relative);
  await fs.mkdir(path.dirname(file), { recursive: true });
  await fs.writeFile(file, content, "utf8");
}

async function replace(relative, transform) {
  const original = await read(relative);
  const updated = transform(original);
  await write(relative, updated);
}

function fullLogo() {
  return `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="720" height="190" viewBox="0 0 720 190" role="img" aria-label="Shring Outbreak">
  <defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1"><stop stop-color="#8df17d"/><stop offset="1" stop-color="#42b95c"/></linearGradient></defs>
  <g stroke="#101319" stroke-linejoin="round">
    <path d="M44 52 88 20l44 32v72l-44 32-44-32Z" fill="url(#g)" stroke-width="10"/>
    <path d="M65 72c12-22 37-22 48 0-7-4-15-6-24-6s-17 2-24 6Zm4 24c13 10 28 10 41 0v24c-11 13-30 13-41 0Z" fill="#f2f7ef" stroke-width="6"/>
    <circle cx="79" cy="88" r="4" fill="#101319" stroke="none"/><circle cx="101" cy="88" r="4" fill="#101319" stroke="none"/>
  </g>
  <text x="160" y="92" font-family="Arial Black,Arial,sans-serif" font-size="72" font-weight="900" fill="#f5f7fa" stroke="#101319" stroke-width="8" paint-order="stroke">SHRING</text>
  <text x="164" y="148" font-family="Arial Black,Arial,sans-serif" font-size="48" font-weight="900" letter-spacing="8" fill="#79e071" stroke="#101319" stroke-width="6" paint-order="stroke">OUTBREAK</text>
</svg>`;
}

function favicon() {
  return `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="128" height="128" viewBox="0 0 128 128" role="img" aria-label="Shring Outbreak">
  <rect width="128" height="128" rx="24" fill="#12151c"/>
  <circle cx="64" cy="64" r="48" fill="#6fdd65" stroke="#111" stroke-width="8"/>
  <path d="M38 48c9-20 45-21 53 0-8-5-16-7-26-7s-19 2-27 7Zm2 20c15 12 34 12 49 0v18c-13 17-37 17-49 0Z" fill="#eaf6e8" stroke="#111" stroke-width="5" stroke-linejoin="round"/>
  <circle cx="50" cy="61" r="5" fill="#111"/><circle cx="78" cy="61" r="5" fill="#111"/>
</svg>`;
}

const wsPublicUrl = publicUrl.replace(/^http/, "ws");
await write("client/src/scripts/config.ts", `import { type TeamMode } from "@common/constants";
import type { ModeName } from "@common/definitions/modes";

export const Config = {
    regions: {
        shring: {
            name: "Shring Network",
            mainAddress: "${publicUrl}",
            gameAddress: "${wsPublicUrl}/game/<gameID>",
            offset: 1
        }
    },
    defaultRegion: "shring"
} satisfies ConfigType as ConfigType;

export interface ConfigType {
    readonly regions: Record<string, Region>
    readonly defaultRegion: string
}

export interface Region {
    readonly name: string
    readonly flag?: string
    readonly mainAddress: string
    readonly gameAddress: string
    readonly offset: number
}

export interface ServerInfo {
    readonly protocolVersion: number
    readonly playerCount: number
    readonly teamMode: TeamMode
    readonly nextTeamMode?: TeamMode
    readonly teamModeSwitchTime?: number
    readonly mode: ModeName
    readonly nextMode?: ModeName
    readonly modeSwitchTime?: number
}
`);

await write("server/config.json", `${JSON.stringify({
  $schema: "config.schema.json",
  hostname: "127.0.0.1",
  port: 8000,
  map: "normal",
  teamMode: "solo",
  spawn: { mode: "default" },
  minTeamsToStart: 1,
  maxPlayersPerGame: 40,
  maxGames: 1,
  ipHeader: "X-Real-IP",
  maxSimultaneousConnections: 5,
  maxJoinAttempts: { count: 12, duration: 60000 },
  maxCustomTeams: 5,
  roles: {}
}, null, 2)}\n`);

await replace("package.json", text => {
  const pkg = JSON.parse(text);
  pkg.name = "shring-outbreak";
  pkg.description = "A Shring Network deployment of the open-source Suroi browser shooter.";
  return `${JSON.stringify(pkg, null, 2)}\n`;
});

await replace("client/index.html", text => {
  let output = text
    .replace(/<meta name="apple-itunes-app"[^>]*>\s*/g, "")
    .replaceAll("Suroi - 2D battle royale game", "Shring Outbreak - Browser Battle Royale")
    .replaceAll("Miss surviv.io and Surviv Reloaded? Suroi is an open-source 2D battle royale game inspired by surviv.io. Work in progress.", "A fast, guest-playable 2D browser battle royale for the Shring Network.")
    .replaceAll("suroi.io", "shootup.shring.net")
    .replaceAll("alt=\"Suroi logo\"", "alt=\"Shring Outbreak logo\"")
    .replace(/\s*<link\s+rel="icon"\s+type="image\/x-icon"[\s\S]*?\/>/g, "");
  if (!output.includes("id=\"shring-source-notice\"")) {
    output = output.replace("<!-- Main menu -->", `<!-- Main menu -->\n      <div id="shring-source-notice"><a href="./source.html" target="_blank" rel="noopener">GPL source, credits, and changes</a></div>`);
  }
  return output;
});

await fs.appendFile(path.join(sourceDir, "client/src/scss/pages/client/splash.scss"), `
/* Shring Outbreak deployment branding */
#splash-news { display: none !important; }
#splash-center { margin-inline: auto !important; }
#shring-source-notice { position: fixed; right: 14px; bottom: 10px; z-index: 20; font: 600 12px/1.2 Arial,sans-serif; opacity: .78; }
#shring-source-notice a { color: #e8f5e8; text-decoration: none; }
#shring-source-notice a:hover { text-decoration: underline; opacity: 1; }
`, "utf8");

await write("client/public/img/logos/suroi_beta.svg", fullLogo());
await write("client/public/img/logos/suroi_favicon.svg", favicon());

await replace("client/public/manifest.json", text => {
  const manifest = JSON.parse(text);
  manifest.name = "Shring Outbreak - 2D Battle Royale";
  manifest.short_name = "Shring Outbreak";
  manifest.description = "Guest-playable 2D browser battle royale for the Shring Network.";
  manifest.icons = [{ src: "./img/logos/suroi_favicon.svg", type: "image/svg+xml", sizes: "any" }];
  manifest.shortcuts = [];
  return `${JSON.stringify(manifest, null, 2)}\n`;
});

try {
  await replace("client/src/translations/en.hjson", text => text.replaceAll("Suroi", "Shring Outbreak").replaceAll("suroi.io", "shootup.shring.net"));
} catch (error) {
  console.warn("English translation branding patch skipped:", error.message);
}

await replace("server/src/server.ts", text => text.replace("Suroi Server v${version}", "Shring Outbreak Server v${version}"));

await write("client/public/source.html", `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Shring Outbreak source and credits</title>
<style>body{max-width:850px;margin:40px auto;padding:0 20px;background:#151922;color:#eef3ee;font:16px/1.6 system-ui,sans-serif}a{color:#79e071}code{background:#252b37;padding:2px 5px;border-radius:4px}</style></head>
<body><h1>Shring Outbreak source and credits</h1><p>This deployment is a modified build of <strong>Suroi</strong>, an open-source 2D battle royale game by the Suroi contributors.</p>
<p>Upstream project: <a href="https://github.com/HasangerGames/suroi">HasangerGames/suroi</a></p><p>Pinned upstream commit: <code>${upstreamCommit}</code></p>
<p>The upstream project and this modified deployment are provided under the GNU General Public License version 3. The complete modified source is included in the deployment under <code>/home/container/suroi</code>.</p>
<p>Shring modifications: branding, one Shring region, guest play, production configuration, precompiled browser client, health endpoint, and a single-port Pterodactyl gateway.</p></body></html>`);

await write("SHRING-MODIFICATIONS.md", `# Shring Outbreak modification notice

Modified from Suroi for the Shring Network on 2026-08-03.

- Upstream: https://github.com/HasangerGames/suroi
- Pinned commit: ${upstreamCommit}
- License: GNU GPL version 3
- Public URL: ${publicUrl}

Changes:

- Shring Outbreak branding and metadata
- guest-playable deployment with no required account
- one Shring server region
- production client compiled before deployment
- single public-port gateway for Pterodactyl
- health endpoint and deployment scripts
`);

console.log(`Applied Shring Outbreak patches to ${sourceDir}`);
