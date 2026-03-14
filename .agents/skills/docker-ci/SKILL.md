---
name: docker-ci-pipeline
description: Docker image building, Compose development environments, CI/CD pipeline, and testing infrastructure. Use when working with Dockerfiles, docker-compose, GitHub Actions CI, Make targets, integration tests, or deployment workflows.
reviewed_at: 7288840
---

# Docker & CI Pipeline

## Overview

tailrelay ships as a multi-arch Docker image built via a multi-stage Dockerfile. Development uses Docker Compose for local testing, and GitHub Actions for CI.

## Docker Images

### Production (Multi-Stage)

```bash
docker buildx build -t sudocarlos/tailrelay:latest --load .
```

`Dockerfile` stages:
1. **frontend-builder** (`node:{NODE_VERSION}-alpine`) — builds Vite/Svelte/Tailwind SPA assets; injects `VERSION` via `npm version`
2. **caddy-builder** (`golang:{GO_VERSION}-alpine`) — clones `caddyserver/caddy` at `v{CADDY_VERSION}` and compiles the `caddy` binary
3. **webui-builder** (`golang:{GO_VERSION}-alpine`) — builds the tailrelay-webui Go binary; copies frontend dist from `frontend-builder`
4. **tailscale-builder** (`golang:{GO_VERSION}-alpine`) — builds Tailscale binaries from source via `go install tailscale.com/cmd/...@{TAILSCALE_VERSION}`
5. **binary-dev** (`scratch`) — holds a pre-built local binary from `./data/tailrelay-webui` for dev builds
6. **binary-source** — selector stage; resolves to `webui-builder` (default) or `binary-dev` via `WEBUI_SOURCE` build arg
7. **main** (`alpine:{ALPINE_VERSION}`) — installs runtime deps (iptables, iproute2, socat, mailcap), copies all binaries from builder stages; restores legacy iptables symlinks for broad host compatibility

Key build args:
- `TAILSCALE_VERSION` (default: `v1.92.5`) — version tag passed to `go install tailscale.com/cmd/...@${TAILSCALE_VERSION}`
- `CADDY_VERSION` (default: `2.11.2`) — version tag used for `git clone --branch v${CADDY_VERSION} caddyserver/caddy`
- `GO_VERSION` (default: `1.26.1`)
- `NODE_VERSION` (default: `24`)
- `ALPINE_VERSION` (default: `3.22`)
- `WEBUI_SOURCE` (default: `webui-builder`; set to `binary-dev` for dev builds)
- `VERSION`, `COMMIT`, `DATE`, `BRANCH`, `BUILDER` — build metadata injected into Go binary and frontend via ldflags / `npm version`

### Development

```bash
make dev-build           # Build Go binary + frontend locally
make dev-docker-build    # Build Docker image using local binary
```

`dev-docker-build` passes `--build-arg WEBUI_SOURCE=binary-dev` so Docker copies `./data/tailrelay-webui` from the build context instead of compiling in-container. No separate `Dockerfile.dev` is needed.

## Make Targets

| Target | Description | Depends On |
|--------|-------------|------------|
| `make help` | Show available targets | — |
| `make test` | Run Go unit tests (`cd webui && go test ./...`) | — |
| `make integration-test` | Run integration tests via pytest | Docker, `.env` |
| `make frontend-build` | Build SPA assets (npm install + build) | Node.js |
| `make dev-build` | Build Go binary with metadata | `frontend-build` |
| `make dev-docker-build` | Build dev Docker image | `dev-build` |
| `make release` | Multi-platform push to Docker Hub + GHCR | Docker Buildx |
| `make clean` | Remove build artifacts | — |

## Docker Compose (Testing)

`compose-test.yml` provides a local test environment:

```bash
# Start
docker compose -f compose-test.yml up -d

# View logs
docker compose -f compose-test.yml logs tailrelay-test

# Stop
docker compose -f compose-test.yml down
```

### Test Environment Variables (`.env`)

```bash
TAILRELAY_HOST=tailrelay-test
TAILNET_DOMAIN=example.com
COMPOSE_FILE=compose-test.yml
```

Copy `.env.example` to `.env` and edit before running tests.

## CI Pipeline (GitHub Actions)

`.github/workflows/ci.yml` runs on push/PR to `main` and on published releases:

### Jobs

1. **frontend** — `npm install` + `npm run build` in `webui/frontend/` (Node.js 20 in CI)
2. **backend** — `go vet`, `go test -v`, `go build` in `webui/` (Go 1.24 in CI)
3. **integration** — Full Docker build + `pytest tests/integration/ -v`

### Integration Test Flow

```
npm install → npm run build → docker buildx build → pytest tests/integration/ -v
```

The pytest suite (`tests/integration/`) handles:
1. Build dev image
2. Start containers via Compose
3. Wait for services to initialize
4. Run HTTP health checks
5. Validate ports and logs
6. Clean up containers

## Testing Infrastructure

### Test Suite

Integration tests live in `tests/integration/` (Python/pytest). Shared helpers are in `tests/integration/helpers.py`.

Copy `.env.example` to `.env` and fill in values before running:

```bash
make integration-test   # pytest tests/integration/ -v
```

### Health Check Endpoints

| Endpoint | Port | Service |
|----------|------|---------|
| HTTP proxy | `:8080`, `:8081` | Caddy |
| HTTPS proxy | `:8443` | Caddy |
| Health check | `:9002/healthz` | Tailscale |
| Metrics | `:9002/metrics` | Tailscale |
| Web UI | `:8021` | Web UI |

### Running Tests

```bash
# Go unit tests
make test

# Integration tests (requires Docker + .env)
make integration-test
```

## Container Entrypoint

`start.sh` orchestrates all services:
1. Start `tailscaled` (userspace networking)
2. Start Caddy (with Caddyfile)
3. Wait 1 second for Caddy API
4. Start Web UI
5. Spawn socat relays (if `RELAY_LIST` set)
6. `wait` on tailscaled + webui PIDs

Handles `SIGTERM`/`SIGINT` for graceful shutdown.

## Common Pitfalls

1. **File persistence**: Start9 removes files on reboot — mount `/var/lib/tailscale` as a volume
2. **Hostname matching**: `TS_HOSTNAME` must match everywhere (Tailscale, Caddy, Web UI)
3. **Docker network**: Use `--net start9` for Start9 deployments to reach embassy services
4. **TLS certificates**: Must enable HTTPS in Tailscale Admin Console first
5. **Port conflicts**: Ensure host ports don't conflict with existing services

## Version Information

| Component | Version |
|-----------|---------|
| Container | `v0.8.0` (see `start.sh`) |
| Tailscale | `v1.92.5` (built from source via `go install tailscale.com/cmd/...`) |
| Caddy | `2.11.2` (built from source via `git clone caddyserver/caddy`) |
| Go | `1.26.1` (Dockerfile ARG) |
| Node.js (CI) | `20` (GitHub Actions) / `24` (Dockerfile ARG) |
