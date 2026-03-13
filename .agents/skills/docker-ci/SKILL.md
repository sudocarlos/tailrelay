---
name: docker-ci-pipeline
description: Docker image building, Compose development environments, CI/CD pipeline, and testing infrastructure. Use when working with Dockerfiles, docker-compose, GitHub Actions CI, Make targets, integration tests, or deployment workflows.
reviewed_at: feb333e
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
1. **frontend-builder** (`node:{NODE_VERSION}-alpine`) — builds Vite/Svelte/Tailwind SPA assets
2. **webui-builder** (`golang:{GO_VERSION}-alpine`) — builds Caddy from source (`git clone caddyserver/caddy`) and the tailrelay-webui Go binary
3. **tailscale-builder** (`golang:{GO_VERSION}-alpine`) — builds Tailscale binaries from source via `go install tailscale.com/cmd/...@{TAILSCALE_VERSION}`
4. **binary-dev** (`scratch`) — holds a pre-built local binary from `./data/tailrelay-webui` for dev builds
5. **binary-source** — selector stage; resolves to `webui-builder` (default) or `binary-dev` via `WEBUI_SOURCE` build arg
6. **main** (`alpine:3.22`) — installs runtime deps (iptables, iproute2, socat, mailcap), copies all binaries from builder stages

Key build args:
- `TAILSCALE_VERSION` (default: `v1.94.2`) — version tag passed to `go install tailscale.com/cmd/...@${TAILSCALE_VERSION}`
- `CADDY_VERSION` (default: `2.11.2`) — version tag used for `git clone --branch v${CADDY_VERSION} caddyserver/caddy`
- `GO_VERSION` (default: `1.26.1`)
- `NODE_VERSION` (default: `24`)
- `WEBUI_SOURCE` (default: `webui-builder`; set to `binary-dev` for dev builds)
- `VERSION`, `COMMIT`, `DATE`, `BRANCH`, `BUILDER` — build metadata

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
| `make frontend-build` | Build SPA assets (npm install + build) | Node.js |
| `make dev-build` | Build Go binary with metadata | `frontend-build` |
| `make dev-docker-build` | Build dev Docker image | `dev-build` |
| `make clean` | Remove `data/tailrelay-webui` | — |

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

`.github/workflows/ci.yml` runs on push/PR to `main`:

### Jobs

1. **frontend** — `npm install` + `npm run build` in `webui/frontend/`
2. **backend** — `go vet`, `go test -v`, `go build` in `webui/`
3. **integration** — Full Docker build + `docker-compose-test.sh`

### Integration Test Flow

```
npm install → npm run build → docker buildx build → docker-compose-test.sh
```

`docker-compose-test.sh` runs:
1. Build dev image
2. Start containers via Compose
3. Wait for services to initialize
4. Run curl health checks
5. Validate ports and logs
6. Clean up containers

## Testing Infrastructure

### Test Scripts

| Script | Language | Purpose |
|--------|----------|---------|
| `docker-compose-test.py` | Python | Full integration test suite (env-driven, curl checks) |
| `docker-compose-test.sh` | Bash | Quick integration test wrapper |
| `test_proxy_api.sh` | Bash | Web UI / Caddy API endpoint tests |

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
cd webui && go test ./...

# Python integration suite
python docker-compose-test.py

# Bash integration suite
./docker-compose-test.sh

# API endpoint tests
./test_proxy_api.sh
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
| Container | `v0.4.1` (see `start.sh`) |
| Tailscale | `v1.92.5` (built from source via `go install tailscale.com/cmd/...`) |
| Caddy | `2.11.2` (built from source via `git clone caddyserver/caddy`) |
| Go | `1.26.1` (Dockerfile ARG) |
| Node.js (CI) | `24` |
