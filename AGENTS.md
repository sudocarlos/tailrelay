# Agent Development Guide

This guide provides coding agents with essential information for working with the tailrelay codebase, plus operational rules and recommended commands for development tasks.

## Project Overview

**tailrelay** is a Docker container that combines Tailscale, Caddy, socat, and a Go-based Web UI to expose local services (especially Start9 services) to a Tailscale network. The repo includes:

- Docker image building (multi-stage) with Tailscale, Caddy, socat, and Web UI
- Shell script orchestration for service startup
- Go Web UI (Caddy Admin API integration)
- Python and Bash integration tests
- Docker Compose for development and testing

## LLM Operational Rules (Read First)

1. **Prefer Make targets and documented scripts** before inventing new commands.
2. **Avoid long-running daemons** unless explicitly requested (e.g., `docker compose up -d`).
3. **Do not mutate host state** (system packages, global config) without explicit request.
4. **Use .env for tests** and never hardcode secrets or tokens.
5. **When running commands**, keep output small and relevant (pipe/grep if needed).
6. **If a change affects external behavior**, update README or release notes as required.

## Build, Test & Development Commands

### Make Targets (Preferred)

```bash
# Show available targets
make help

# Build Web UI binary locally (writes data/tailrelay-webui)
make dev-build

# Build dev Docker image using local binary
make dev-docker-build

# Remove build artifacts
make clean
```

### Build Docker Images

```bash
# Development image (multi-stage)
docker buildx build -t sudocarlos/tailrelay:dev --load .

# Production image
docker buildx build -t sudocarlos/tailrelay:latest .
```

### Development Environment (Compose)

```bash
# Start test environment
docker compose -f compose-test.yml up -d

# View logs
docker compose -f compose-test.yml logs tailrelay-test

# Stop test environment
docker compose -f compose-test.yml down

# Check listening ports
docker exec -it tailrelay-test netstat -tulnp | grep LISTEN
```

### Run Tests

```bash
# Full integration test suite (Python)
python docker-compose-test.py

# Full integration test suite (Bash)
./docker-compose-test.sh

# API-level test (Web UI / Caddy API)
./test_proxy_api.sh
```

### Running Single Health Checks

```bash
curl -sSL http://${TAILRELAY_HOST}:8080 && echo success || echo fail
curl -sSL http://${TAILRELAY_HOST}:9002/healthz && echo success || echo fail
```

## Web UI Development Workflow

Fast iteration without rebuilding the full image:

1. Build the Web UI binary: `make dev-build`
2. Mount `./data/tailrelay-webui` into the container (see compose-test.yml)
3. Restart the container: `docker compose -f compose-test.yml restart tailrelay`
4. Repeat as needed

## Architecture Notes

- **Container entrypoint**: [start.sh](start.sh) orchestrates Web UI, optional socat relays, Caddy startup, then hands off to `containerboot`.
- **Web UI**: Go application in [webui/](webui/) with embedded templates/static assets.
- **Caddy config**: Managed via Caddy Admin API; legacy Caddyfile remains for compatibility.
- **Relays**: `RELAY_LIST` is supported for migration but Web UI is preferred.

## Code Style Guidelines

### Shell Scripts (Bash/sh)

- Use `.sh` extension for shell scripts
- Shebangs: `#!/usr/bin/env bash` (Bash), `#!/bin/ash` (Alpine entrypoint)
- 4-space indentation, no tabs
- Env vars: `UPPER_SNAKE_CASE`, locals: `lower_snake_case`
- Quote variables: `"$VAR"` unless word splitting intended
- `set -e` for fail-fast, `set -x` for debugging

### Python Scripts

- Standard library imports first, third-party second
- Use type hints for function parameters/returns
- Prefer f-strings for messages
- Handle subprocess timeouts gracefully; return error codes

### Go (Web UI)

- Follow standard Go formatting (`gofmt`)
- Keep handlers in `internal/handlers/` and business logic in `internal/*`
- Prefer explicit error handling; avoid panics for runtime conditions
- Keep config types in `internal/config`

### Dockerfile

- Use `ARG` for build-time values, `ENV` for runtime
- Combine `RUN` steps to reduce layers
- Pin base image versions via `TAILSCALE_VERSION`

### Caddyfile Configuration

- Use tabs for indentation (Caddy convention)
- One site block per listening address
- Use full domain with port: `host.domain.ts.net:port`

## Environment Variables

**Required:**
- `TS_HOSTNAME` - Tailscale machine name (must match Caddy config and Web UI)
- `TS_STATE_DIR` - Tailscale state directory (default: `/var/lib/tailscale/`)

**Optional:**
- `RELAY_LIST` - Comma-separated `port:host:port` relay definitions (legacy)
- `TS_EXTRA_FLAGS` - Additional Tailscale flags
- `TS_AUTH_ONCE` - Authenticate once (default: `true`)
- `TS_ENABLE_METRICS` - Enable metrics endpoint (default: `true`)
- `TS_ENABLE_HEALTH_CHECK` - Enable health check endpoint (default: `true`)

**Test .env variables:**
- `TAILRELAY_HOST` - Test container hostname
- `TAILNET_DOMAIN` - Tailnet domain for testing
- `COMPOSE_FILE` - Path to Compose file

## File Map

```
.
├── Dockerfile               # Multi-stage container build (includes building the Web UI binary in a builder stage)
├── Dockerfile.dev           # Development image that copies the locally-built `data/tailrelay-webui` binary
├── start.sh                 # Container entrypoint: starts Web UI, optional socat relays, Caddy, then `containerboot`
├── webui/                   # Go Web UI source tree (see details below)
├── webui.yaml               # Default runtime config for the Web UI included in the image
├── data/                    # Local build outputs (e.g., `tailrelay-webui` produced by `make dev-build`)
├── compose-test.yml         # Docker Compose config used for development and integration testing
├── docker-compose-test.py   # Python-driven integration test harness (env-driven, curl checks)
├── docker-compose-test.sh   # Bash wrapper test script for quick runs
├── test_proxy_api.sh        # Example script that exercises Web UI/Caddy API endpoints (uses `curl`)
├── requirements.txt         # Python dependencies for the test harness (python-dotenv, etc.)
├── Caddyfile.example        # Example/legacy Caddyfile for manual Caddy configuration or troubleshooting
└── README.md                # Project overview and developer documentation
```

## Testing Strategy

- Build dev image
- Start containers via Compose
- Wait ~3 seconds for services to initialize
- Run curl health checks
- Validate ports and logs
- Clean up containers

**Health Check Endpoints:**
- HTTP proxy: `:8080`, `:8081`
- HTTPS proxy: `:8443`
- Tailscale health: `:9002/healthz`
- Tailscale metrics: `:9002/metrics`

## Common Pitfalls & Notes

1. **File Persistence**: Start9 removes files on reboot; back up `/home/start9/tailscale`.
2. **Hostname Matching**: `TS_HOSTNAME` must match the Tailscale hostname used in config.
3. **Tailnet Domain**: Use the exact Tailnet name from Tailscale Admin console.
4. **RELAY_LIST Format**: Strict `port:host:port` parsing; migrate to Web UI.
5. **Docker Network**: Use `--net start9` for Start9 deployments.
6. **TLS Certificates**: HTTPS must be enabled in Tailscale admin.
7. **Container Execution**: `start.sh` uses `exec` to hand off to `containerboot`.

## Version Information

- Container version: `v0.2.1` (see `start.sh` and release notes)
- Tailscale base: `v1.92.5` (Dockerfile ARG)
- Go version: `1.21` (Dockerfile ARG)

## Making Changes

When modifying the project:

1. Update version in `start.sh` (and release notes as needed)
2. Rebuild the image (dev or prod)
3. Run Python and Bash test scripts
4. Validate health endpoints
5. Update README.md for user-facing changes
