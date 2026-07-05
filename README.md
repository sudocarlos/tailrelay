# tailrelay

A Docker container that exposes local services to your Tailscale network. Combines **Tailscale VPN**, **Tailscale Serve** (HTTPS + TCP relays), and a **Web UI** for browser-based management.

[![Docker Pulls](https://img.shields.io/docker/pulls/sudocarlos/tailrelay)](https://hub.docker.com/r/sudocarlos/tailrelay)
[![GitHub Release](https://img.shields.io/github/v/release/sudocarlos/tailrelay)](https://github.com/sudocarlos/tailrelay/releases)
[![License](https://img.shields.io/github/license/sudocarlos/tailrelay)](https://github.com/sudocarlos/tailrelay/blob/main/LICENSE)

📖 **[Full documentation and API reference](https://sudocarlos.github.io/tailrelay/)**

## Table of Contents

- [Features](#features)
- [Getting Started](#getting-started)
- [Troubleshooting](#troubleshooting)
- [Screenshots](#screenshots)
- [Development](#development)
- [Documentation](#documentation)
- [Contributing](#contributing)

## Features

tailrelay provides secure remote access to self-hosted services without exposing ports:

- **Browser-Based Management**: A responsive Web UI running on port `8021` to manage your relays.
- **Automatic TLS**: Tailscale Serve terminates TLS for HTTPS relays with automatic MagicDNS hostnames.
- **HTTPS Relays**: Easily configure HTTPS reverse proxies/relays through the UI.
- **TCP Relays**: Forward non-HTTP protocols through Tailscale Serve.
- **Funnel**: Expose a service to the public internet on port `443`, `8443`, or `10000` via Tailscale Funnel.
- **Backup & Restore**: Save, download, upload, and restore configurations.
- **Dual Authentication**: Authenticate via Tailscale network identity (peer IP) or secure token.
- **Multi-Platform**: Multi-arch Docker images built for `amd64` and `arm64`.

This makes tailrelay useful for exposing local or Start9 services (like BTCPayServer, LND, electrs, and Mempool) to your Tailnet securely without Tor.

### Web UI details

The frontend is a single-page application built with **Svelte 5** (runes mode), **Tailwind CSS v4**, and **Vite**. All assets (JS, CSS, icons) are bundled locally for zero external CDN requests at runtime.

- **Dashboard**: Real-time Tailscale status, search filtering, and system health info.
- **Tailscale Management**: Easily connect, disconnect, or deauthorize the node.
- **Relay Configuration**: Add, edit, delete, toggle, and auto-reconcile HTTPS and TCP relays.
- **Funnel Configuration**: Dedicated dashboard section for the three funnel-eligible ports (443, 8443, 10000), showing each as configured, in-use-by-a-relay, or available to configure.
- **Live Log Viewer**: Streaming container logs (SSE) with live log-level control.
- **UX Conveniences**: Keyboard shortcuts (`n` for new relay, `r` to refresh, `b` for backups, `l` for logs, `t` for theme), local-storage-persisted dark mode.

### Web UI Authentication & Access

#### Authentication
1. **Tailscale Network Authentication**: Devices on your Tailscale network are automatically authenticated. If the container is not connected, the Web UI shows a Tailscale login link and polls until the device is connected.
2. **Token Authentication**: A token is generated on first startup at `/var/lib/tailscale/.webui_token` for scripted access or CLI integration.

#### Access
The Web UI is accessible on port `8021`:
- **Secure/Remote**: `https://your-hostname.your-tailnet.ts.net:8021` (once connected and HTTPS is enabled)
- **Local**: `http://localhost:8021`

## Getting Started

### Quick Start

```bash
# Pull the image
docker pull sudocarlos/tailrelay:latest

# Run the container
docker run -d --name tailrelay \
  -v /path/to/data:/var/lib/tailscale \
  -e TS_HOSTNAME=myserver \
  -p 8021:8021 \
  --net bridge \
  sudocarlos/tailrelay:latest

# Access the Web UI and follow the Tailscale login link
open http://localhost:8021
```

### Prerequisites

1. A Tailscale account with an active Tailnet ([tailscale.com](https://tailscale.com))
2. [HTTPS certificates enabled](https://tailscale.com/kb/1153/enabling-https) in Tailscale Admin console
3. Docker or Podman installed

### Tailscale Setup

1. Log into Tailscale Admin console and click [DNS](https://login.tailscale.com/admin/dns) to enable MagicDNS.
   - Tailnets created on or after October 20, 2022 have MagicDNS enabled by default.
2. Review [MagicDNS](https://tailscale.com/kb/1081/magicdns) to understand how it works.
3. Verify or set your [Tailnet name](https://tailscale.com/kb/1217/tailnet-name)
4. Scroll down and enable HTTPS under HTTPS Certificates

### StartOS Deployment

tailrelay is available as a StartOS package via [sudocarlos/tailrelay-startos](https://github.com/sudocarlos/tailrelay-startos).

**Sideloading:**

1. Download the latest `tailrelay.s9pk` from the [tailrelay-startos releases page](https://github.com/sudocarlos/tailrelay-startos/releases), or clone the repo and run `make` to build it yourself.
2. In the StartOS web UI menu, navigate to **System → Sideload Service**.
3. Drag and drop or select the `tailrelay.s9pk` file to install.
4. Once installed, navigate to **Services → Tailrelay** and click **Start**.

## Troubleshooting

### Web UI Not Accessible

Check container status:
```bash
docker ps | grep tailrelay
```

Verify port mapping:
```bash
docker port tailrelay
```

Check logs:
```bash
docker logs tailrelay | grep -i webui
```

Verify listening port:
```bash
docker exec tailrelay netstat -tulnp | grep 8021
```

### Cannot Log In

Retrieve token:
```bash
docker exec tailrelay cat /var/lib/tailscale/.webui_token
```

Ensure you're accessing from Tailscale network or clear browser cache.

### Relay Issues (`tailscale serve`)

Check current serve status:
```bash
docker exec tailrelay tailscale serve status
```

Force reconcile all enabled relays from the saved UI configuration:
```bash
curl -X POST http://localhost:8021/api/serve/reload
```

Test target connectivity:
```bash
docker exec tailrelay nc -zv target-host target-port
```

## Screenshots

The Web UI supports light and dark themes and is fully responsive on mobile.

### Login

<table>
<tr>
  <th>Light — Desktop</th>
  <th>Dark — Desktop</th>
</tr>
<tr>
  <td><img src="docs/screenshots/login-light-desktop.png" alt="Login light desktop" width="480"/></td>
  <td><img src="docs/screenshots/login-dark-desktop.png" alt="Login dark desktop" width="480"/></td>
</tr>
<tr>
  <th>Light — Mobile</th>
  <th>Dark — Mobile</th>
</tr>
<tr>
  <td><img src="docs/screenshots/login-light-mobile.png" alt="Login light mobile" width="240"/></td>
  <td><img src="docs/screenshots/login-dark-mobile.png" alt="Login dark mobile" width="240"/></td>
</tr>
</table>

### Dashboard

<table>
<tr>
  <th>Light — Desktop</th>
  <th>Dark — Desktop</th>
</tr>
<tr>
  <td><img src="docs/screenshots/dashboard-light-desktop.png" alt="Dashboard light desktop" width="480"/></td>
  <td><img src="docs/screenshots/dashboard-dark-desktop.png" alt="Dashboard dark desktop" width="480"/></td>
</tr>
<tr>
  <th>Light — Mobile</th>
  <th>Dark — Mobile</th>
</tr>
<tr>
  <td><img src="docs/screenshots/dashboard-light-mobile.png" alt="Dashboard light mobile" width="240"/></td>
  <td><img src="docs/screenshots/dashboard-dark-mobile.png" alt="Dashboard dark mobile" width="240"/></td>
</tr>
</table>

**Log console expanded (dark):**

![Dashboard with log console expanded](docs/screenshots/dashboard-logs-dark-desktop.png)

**Mobile navigation menu open:**

<table>
<tr>
  <td><img src="docs/screenshots/dashboard-mobile-menu-light.png" alt="Mobile menu light" width="240"/></td>
  <td><img src="docs/screenshots/dashboard-mobile-menu-dark.png" alt="Mobile menu dark" width="240"/></td>
</tr>
</table>

### Tailscale

<table>
<tr>
  <th>Light — Desktop</th>
  <th>Dark — Desktop</th>
</tr>
<tr>
  <td><img src="docs/screenshots/tailscale-light-desktop.png" alt="Tailscale light desktop" width="480"/></td>
  <td><img src="docs/screenshots/tailscale-dark-desktop.png" alt="Tailscale dark desktop" width="480"/></td>
</tr>
<tr>
  <th>Light — Mobile</th>
  <th>Dark — Mobile</th>
</tr>
<tr>
  <td><img src="docs/screenshots/tailscale-light-mobile.png" alt="Tailscale light mobile" width="240"/></td>
  <td><img src="docs/screenshots/tailscale-dark-mobile.png" alt="Tailscale dark mobile" width="240"/></td>
</tr>
</table>

### Backups

<table>
<tr>
  <th>Light — Desktop</th>
  <th>Dark — Desktop</th>
</tr>
<tr>
  <td><img src="docs/screenshots/backups-light-desktop.png" alt="Backups light desktop" width="480"/></td>
  <td><img src="docs/screenshots/backups-dark-desktop.png" alt="Backups dark desktop" width="480"/></td>
</tr>
</table>

**Mobile (dark):**

<img src="docs/screenshots/backups-dark-mobile.png" alt="Backups dark mobile" width="240"/>

## Development

### Local WebUI Development

For rapid iteration without rebuilding the full Docker image:

#### 1. Build the WebUI Assets + Binary

```bash
make frontend-build   # Build Svelte SPA -> webui/cmd/webui/web/dist/
make dev-build        # Build Go binary with embedded SPA + build metadata
```

This compiles `./data/tailrelay-webui` with build metadata (version, commit, date) and embeds the SPA assets from `web/dist/`.

**Frontend dev server:**
```bash
cd webui/frontend
npm run dev           # Starts Vite dev server with hot reload
```

**Dev asset override:**
Set `WEBUI_DEV_DIR` to a directory containing a `dist/` subdirectory (e.g., `webui/cmd/webui/web`) to serve assets from disk instead of the embedded files.

**Manual build:**
```bash
cd webui
CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
  -ldflags="-w -s" \
  -o ../data/tailrelay-webui ./cmd/webui
```

#### 2. Development Options

**Option A: Mount Binary (Recommended)**

Mount the local binary for instant updates:

```yaml
# compose-test.yml
services:
  tailrelay:
    volumes:
      - ./data/tailrelay-webui:/usr/bin/tailrelay-webui:ro
      - ./tailscale/:/var/lib/tailscale
```

Then restart:
```bash
docker compose -f compose-test.yml restart tailrelay
```

**Iteration workflow:**
1. Edit code in `webui/` or `webui/frontend/`
2. Run `make frontend-build` (if frontend changed)
3. Run `make dev-build`
4. Restart container
5. Test changes

**Option B: Build Development Image**

```bash
make dev-docker-build
```

This builds a Docker image using the local binary.

### Building

```bash
# Development build with local binary
make frontend-build
make dev-build
make dev-docker-build

# Production build (multi-stage)
docker buildx build -t sudocarlos/tailrelay:latest .

# Show available targets
make help
```

### Testing

The test suite is split across three layers:

#### 1. Go unit tests (no Docker required)

Covers `internal/auth`, `internal/serve`, `internal/handlers`, `internal/backup`,
and `internal/web`.

```bash
# From the repo root — uses the `make test` target
make test

# Or directly
cd webui && go test ./...

# Verbose output
cd webui && go test -v ./...
```

#### 2. Integration tests (requires Docker)

Builds the full container image and smoke-tests container startup, process presence,
port availability, Tailscale health/metrics endpoints, and Web UI API. The suite lives in `tests/integration/` and is driven by environment
variables.

```bash
# Setup (one-time)
cp .env.example .env
# Edit TAILRELAY_HOST and TAILNET_DOMAIN in .env

pip install pytest   # one-time

# Run via Make
make integration-test

# Or directly
pytest tests/integration/ -v
```

**Environment variables** (all have defaults; override in `.env` or shell):

| Variable | Default | Purpose |
|----------|---------|---------|
| `TAILRELAY_HOST` | `tailrelay-test` | Container hostname / Docker service name |
| `TAILNET_DOMAIN` | `example.com` | Tailnet domain (used for HTTPS cert checks) |
| `COMPOSE_FILE` | `compose-test.yml` | Compose file to spin up the stack |
| `BUILD_IMAGE` | `1` | Set to `0` to skip `docker compose build` |
| `IMAGE_TAG` | `sudocarlos/tailrelay:dev` | Image to build/run |
| `STARTUP_WAIT` | `8` | Seconds to wait after container start |

#### 3. CI pipeline

GitHub Actions runs all three layers automatically on push/PR to `main`:
- **frontend** job: `npm install` + `npm run build`
- **backend** job: `go vet ./...` + `go test ./...` + `go build ./...`
- **integration** job: full Docker build + `pytest tests/integration/ -v`

### Build Metadata

The `dev-build` target injects build information:

```go
var (
  version = "dev"      // Git describe output
  commit  = "none"     // Short commit hash
  date    = "unknown"  // Build timestamp (UTC)
  branch  = "unknown"  // Git branch
  builtBy = "local"    // System username
)
```

Access these in `webui/cmd/webui/main.go`.

## Documentation

The full documentation site — Getting Started, Authentication, Development,
Troubleshooting, and the complete **API Reference** generated from
[`docs/openapi.yaml`](docs/openapi.yaml) — is published at
**[sudocarlos.github.io/tailrelay](https://sudocarlos.github.io/tailrelay/)**.

The Web UI backend exposes a JSON API on port `8021`. Every `/api/*` route
requires authentication (Tailscale network identity or session cookie/bearer
token) except the public setup/login/status/info endpoints — see the
[API Reference](https://sudocarlos.github.io/tailrelay/api/) for the full
authentication scheme, every endpoint, and request/response shapes.

## Contributing

Contributions welcome:

- **Issues**: [GitHub Issues](https://github.com/sudocarlos/tailrelay/issues)
- **Pull Requests**: [GitHub PRs](https://github.com/sudocarlos/tailrelay/pulls)
- **Documentation**: The docs site lives in `website/` (Docusaurus) and
  renders `docs/openapi.yaml` as the API reference — see
  [website/README.md](website/README.md) for the local dev workflow.

### Development Setup

```bash
# Clone repository
git clone https://github.com/sudocarlos/tailrelay.git
cd tailrelay

# Build the development image (compiles binary, Svelte UI, and loads Docker image)
make dev-docker-build

# Start the test environment (includes tailrelay container and a target container)
docker compose -f compose-test.yml up -d
```

See [Development](#development) section for WebUI development workflow.

### License

Open source project. Licensed under the BSD 3-Clause License. See [LICENSE](LICENSE) for details.

### Acknowledgments

- [Tailscale](https://tailscale.com) - VPN platform and `tailscale serve`
- [Start9](https://start9.com) - Inspiration for this project
- Original project by [@hollie](https://github.com/hollie/tailscale-caddy-proxy)

### Release Notes

See [CHANGELOG.md](CHANGELOG.md) for the full release history.

## Review Status

<!-- reviewed_at: ec9e4ac | paths: README.md webui/internal/web/server.go -->
Last full review completed at commit `ec9e4ac`. To check what has changed since:
```bash
git log --oneline ec9e4ac..HEAD -- README.md webui/internal/web/server.go
```
