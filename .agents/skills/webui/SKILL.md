---
name: webui-development
description: Go Web UI application development — handlers, authentication, backup, frontend SPA, build workflow, and testing. Use when working with the webui/ directory, Go code, frontend assets, HTML templates, the SPA build system, or any Web UI feature development.
reviewed_at: e413322
---

# Web UI Development

## Overview

The Web UI is a Go application serving a single-page application (SPA) on port 8021. It manages Tailscale serve relays and backups through a browser interface.

## Project Structure

```
webui/
├── cmd/webui/              # Main entry point (main.go)
│   └── web/                # Embedded static assets & templates
│       ├── static/         # CSS, JS, vendor assets
│       └── templates/      # HTML templates
├── internal/
│   ├── auth/               # Authentication middleware
│   ├── backup/             # Backup & restore (tar.gz)
│   ├── config/             # YAML config parsing
│   ├── handlers/           # HTTP request handlers
│   ├── logger/             # Structured logging
│   ├── serve/              # tailscale serve relay management (see serve skill)
│   │   └── manager.go          # ServeManager — Reconcile, ErrTailscaleNotReady
│   ├── tailscale/          # Tailscale CLI wrapper (see tailscale skill)
│   │   ├── client.go       # CLI wrapper (status, login, etc.)
│   │   ├── status.go       # Status parsing structs
│   │   └── cache.go        # StatusCache — background poller (15s interval)
│   └── web/                # HTTP server, routing, middleware
├── frontend/               # SPA build system (Vite + Svelte 5 + Tailwind CSS 4)
│   ├── src/
│   │   ├── App.svelte      # Root Svelte component
│   │   ├── main.js         # SPA entry point
│   │   └── lib/
│   │       ├── components/
│   │       │   ├── AddModal.svelte     # Add relay modal
│   │       │   └── Tooltip.svelte      # Portal-based tooltip
│   │       └── stores/
├── config/                 # Example webui.yaml
├── go.mod / go.sum
└── README.md
```

## Build Workflow

### Full build (frontend + backend)

```bash
make dev-build   # Runs frontend-build first, then Go build
```

### Frontend only

```bash
make frontend-build   # cd webui/frontend && npm install && npm run build
```

Outputs bundled JS/CSS to `cmd/webui/web/dist/` where they are embedded into the Go binary via `//go:embed`.

### Backend only

```bash
cd webui
CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
  -ldflags="-w -s" -o ../data/tailrelay-webui ./cmd/webui
```

### Dev asset override

Set `WEBUI_DEV_DIR` to serve templates/static from disk instead of embedded:
```bash
WEBUI_DEV_DIR=webui/cmd/webui/web ./data/tailrelay-webui
```

## Build Metadata

Injected via ldflags at build time:

```go
var (
  version = "dev"      // git describe --tags
  commit  = "none"     // git rev-parse --short HEAD
  date    = "unknown"  // UTC timestamp
  branch  = "unknown"  // git branch
  builtBy = "local"    // whoami
)
```

Access: `./tailrelay-webui --version`

## Internal Packages

### `tailscale/` — Tailscale CLI Wrapper

- **`client.go`**: Wraps CLI commands (`tailscale status --json`, `tailscale up`, etc.)
- **`status.go`**: Go structs for parsing status JSON
- **`cache.go`**: `StatusCache` — background goroutine polls `IsConnected` every 15 seconds; provides a non-blocking `IsReady()` check used by TLS cert probes and auth middleware. Starts via `StatusCache.Start(ctx)`.

### `serve/` — Tailscale Serve Relay Manager

- **`manager.go`**: `ServeManager` — loads config, calls `tailscale serve` CLI to apply relay rules, exposes `Reconcile()` which returns `ErrTailscaleNotReady` (sentinel) when Tailscale is not yet connected. Callers use `errors.Is()` to handle not-ready gracefully (config is persisted; apply is deferred).

### `auth/` — Authentication Middleware

- **Tailscale auth**: Auto-authenticate requests from `100.x.y.z` IPs
- **Token auth**: File-based token at configured path (generated first run)
- Middleware checks both methods; either grants access

### `backup/` — Backup & Restore

- Creates compressed tar.gz archives of configuration + certificates
- Stored in `$TS_STATE_DIR/backups/`
- Tests: `internal/backup/backup_test.go`

### `config/` — Configuration

Parses `webui.yaml`:
```yaml
server:
  port: 8021
auth:
  enable_tailscale_auth: true
  enable_token_auth: true
paths:
  config_dir: /var/lib/tailscale
  token_file: /var/lib/tailscale/.webui_token
  relays_file: /var/lib/tailscale/relays.json
  backup_dir: /var/lib/tailscale/backups
```

### `handlers/` — HTTP Handlers

Route handlers for all API endpoints:
- `serve.go` — Serve relay CRUD (TCP + HTTPS relays via `tailscale serve`)
- `legacy.go` — 410 Gone shims for `/api/caddy/*` and `/api/socat/*` (migration hints)
- `tailscale.go` — Status, login
- `backup.go` — Backup operations
- `dashboard.go` — System overview

### `web/` — HTTP Server

- Router setup and middleware chain
- Static file serving
- Tests: `internal/web/server_test.go`

### `logger/` — Logging

Structured logging with configurable verbosity and body size limits (`MAX_LOG_BODY_SIZE`).

## Frontend SPA

Built with **Vite + Svelte 5 + Tailwind CSS 4** via npm:
- Source: `frontend/src/App.svelte` + `frontend/src/lib/` components
- Entry: `frontend/src/main.js`
- Output: `cmd/webui/web/dist/` (embedded via `//go:embed`)
- Icons: Lucide Svelte (`@lucide/svelte`) — imported directly as Svelte components

> **Note:** Bootstrap Icons and the SVG sprite are no longer used. Icons are now Lucide Svelte components imported directly.

## Configuration Reference

| Setting | Default | Purpose |
|---------|---------|---------|
| `server.port` | `8021` | Web UI listen port |
| `auth.enable_tailscale_auth` | `true` | Auto-auth from Tailscale IPs |
| `auth.enable_token_auth` | `true` | Token-based authentication |
| `paths.config_dir` | `/var/lib/tailscale` | Config directory |
| `paths.token_file` | `.webui_token` | Auth token file |
| `paths.relays_file` | `serve_relays.json` | Serve relay config |
| `paths.backup_dir` | `backups/` | Backup storage |
| `paths.metrics_history_file` | *(removed)* | No longer used |

## Testing

```bash
# Run all Go tests
make test

# Run specific package tests
cd webui && go test ./internal/backup/...
cd webui && go test ./internal/web/...

# Integration tests (requires Docker + .env)
make integration-test
```

## Development Iteration

1. Edit code in `webui/` or `webui/frontend/`
2. Run `make frontend-build` (if frontend changed)
3. Run `make dev-build`
4. Restart container: `docker compose -f compose-test.yml restart tailrelay`
5. Test at `http://localhost:8021`

## Code Style

- Follow `gofmt` formatting
- Handlers in `internal/handlers/`, business logic in `internal/*`
- Explicit error handling; avoid panics for runtime conditions
- Config types in `internal/config`
- Dependencies: Go 1.26.1+, `gopkg.in/yaml.v3` (everything else is stdlib)
