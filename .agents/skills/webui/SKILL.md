---
name: webui-development
description: Go Web UI application development — handlers, authentication, backup, frontend SPA, build workflow, and testing. Use when working with the webui/ directory, Go code, frontend assets, HTML templates, the SPA build system, or any Web UI feature development.
---

# Web UI Development

## Overview

The Web UI is a Go application serving a single-page application (SPA) on port 8021. It manages Tailscale, Caddy proxies, socat relays, and backups through a browser interface.

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
│   ├── caddy/              # Caddy API integration (see caddy skill)
│   ├── config/             # YAML config parsing
│   ├── handlers/           # HTTP request handlers
│   ├── logger/             # Structured logging
│   ├── socat/              # Socat process management (see socat skill)
│   ├── tailscale/          # Tailscale CLI wrapper (see tailscale skill)
│   └── web/                # HTTP server, routing, middleware
├── frontend/               # SPA build system (Node.js/npm/esbuild)
│   ├── src/
│   │   ├── index.js        # Main SPA JavaScript
│   │   └── index.css       # Main SPA stylesheet
│   └── package.json        # Build config (esbuild)
├── config/                 # Example webui.yaml
├── examples/               # Usage examples (caddy_api_example.go)
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

Outputs bundled JS/CSS to `cmd/webui/web/static/` where they are embedded into the Go binary via `//go:embed`.

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
- `caddy.go` — Proxy CRUD
- `socat.go` — Relay management
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

Built with **esbuild** via npm:
- Source: `frontend/src/index.js` + `frontend/src/index.css`
- Output: bundled into `cmd/webui/web/static/`
- Icons: Bootstrap Icons SVG sprite at `web/static/vendor/bootstrap-icons/bootstrap-icons.svg`

### Update Bootstrap Icons

```bash
./update-bootstrap-icons.sh          # Latest version
./update-bootstrap-icons.sh 1.11.3   # Specific version
```

## Configuration Reference

| Setting | Default | Purpose |
|---------|---------|---------|
| `server.port` | `8021` | Web UI listen port |
| `auth.enable_tailscale_auth` | `true` | Auto-auth from Tailscale IPs |
| `auth.enable_token_auth` | `true` | Token-based authentication |
| `paths.config_dir` | `/var/lib/tailscale` | Config directory |
| `paths.token_file` | `.webui_token` | Auth token file |
| `paths.relays_file` | `relays.json` | Socat relay config |
| `paths.backup_dir` | `backups/` | Backup storage |

## Testing

```bash
# Run all Go tests
cd webui && go test ./...

# Run specific package tests
go test ./internal/backup/...
go test ./internal/web/...

# API integration test
./test_proxy_api.sh
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
- Dependencies: Go 1.21+, `gopkg.in/yaml.v3` (everything else is stdlib)
