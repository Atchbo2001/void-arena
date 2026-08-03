# Shring Outbreak build pipeline

This directory builds a guest-playable Shring-branded deployment from the GPL-3.0 Suroi source at a pinned upstream commit.

It exists only on the `shring-outbreak-build` branch and does not modify the Void Arena `main` branch.

The GitHub Actions workflow performs all memory-intensive client and sprite compilation on GitHub-hosted runners, smoke-tests the completed runtime, and uploads a flat Pterodactyl ZIP. The Pterodactyl server does not compile the client during startup.

Deployment defaults:

- Public URL: `https://shootup.shring.net`
- Public allocation: `31025`
- Internal Suroi main port: `8000`
- Internal game worker port: `8001`
- Guest play: enabled
- Shring Auth: not required

The resulting package contains the modified source, compiled client, production dependencies, a portable Bun runtime, GPL license, modification notice, single-port gateway, and health endpoint.
