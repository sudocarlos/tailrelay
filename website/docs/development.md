---
id: development
title: Development
---

# Development

## Local WebUI Development

For rapid iteration without rebuilding the full Docker image:

### 1. Build the WebUI Assets + Binary

```bash
make frontend-build   # Build Svelte SPA -> webui/cmd/webui/web/dist/
make dev-build        # Build Go binary with embedded SPA + build metadata
```

This compiles `./data/tailrelay-webui` with build metadata (version, commit,
date) and embeds the SPA assets from `web/dist/`.

**Frontend dev server:**

```bash
cd webui/frontend
npm run dev           # Starts Vite dev server with hot reload
```

**Dev asset override:** set `WEBUI_DEV_DIR` to a directory containing a
`dist/` subdirectory (e.g., `webui/cmd/webui/web`) to serve assets from disk
instead of the embedded files.

**Manual build:**

```bash
cd webui
CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
  -ldflags="-w -s" \
  -o ../data/tailrelay-webui ./cmd/webui
```

### 2. Development Options

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

## Building

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

## Testing

The test suite is split across three layers:

### 1. Go unit tests (no Docker required)

Covers `internal/auth`, `internal/serve`, `internal/handlers`,
`internal/backup`, and `internal/web`.

```bash
# From the repo root — uses the `make test` target
make test

# Or directly
cd webui && go test ./...

# Verbose output
cd webui && go test -v ./...
```

### 2. Integration tests (requires Docker)

Builds the full container image and smoke-tests container startup, process
presence, port availability, Tailscale health/metrics endpoints, and Web UI
API. The suite lives in `tests/integration/` and is driven by environment
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

### 3. CI pipeline

GitHub Actions runs all three layers automatically on push/PR to `main`:

- **frontend** job: `npm install` + `npm run build`
- **backend** job: `go vet ./...` + `go test ./...` + `go build ./...`
- **integration** job: full Docker build + `pytest tests/integration/ -v`

## Build Metadata

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

## Documentation Site

This documentation site lives in `website/` and is built with
[Docusaurus](https://docusaurus.io/). The [API Reference](/docs/api/) is
generated from `docs/openapi.yaml` via `docusaurus gen-api-docs all` and
rendered with [docusaurus-openapi-docs](https://github.com/PaloAltoNetworks/docusaurus-openapi-docs).
Run the generator before building:

```bash
cd website
npm run docusaurus gen-api-docs all
npm run build     # or npm start for local dev
```

### Screenshots

Screenshots for the docs site live in `website/static/img/screenshots/` and are
referenced by the [Screenshots](/docs/screenshots/) page. The sources are
captured under `docs/screenshots/` by a Playwright script that mocks every API
response, so no running container is needed — only the Vite dev server:

```bash
cd webui/frontend && npm run dev   # in one terminal
node docs/screenshots/take-screenshots.mjs
```

The script writes each capture to `docs/screenshots/` and mirrors it into
`website/static/img/screenshots/`. Commit both copies together.
