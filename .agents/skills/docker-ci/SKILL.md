---
name: docker-ci-pipeline
description: Docker image building, Compose development environments, CI/CD pipeline, and testing infrastructure. Use when working with Dockerfiles, docker-compose, GitHub Actions CI, Make targets, integration tests, or deployment workflows.
reviewed_at: 39911f4
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
2. **webui-builder** (`golang:{GO_VERSION}-alpine`) — builds the tailrelay-webui Go binary; copies frontend dist from `frontend-builder`
3. **binary-dev** (`scratch`) — holds a pre-built local binary from `./data/tailrelay-webui` for dev builds
4. **binary-source** — selector stage; resolves to `webui-builder` (default) or `binary-dev` via `WEBUI_SOURCE` build arg
5. **main** (`ghcr.io/tailscale/tailscale:{TAILSCALE_VERSION}`) — based on the official Tailscale image; installs runtime deps (iptables, iproute2, mailcap), copies webui binary from builder stage; restores legacy iptables symlinks for broad host compatibility

Key build args:
- `TAILSCALE_VERSION` (default: `v1.102.2`) — image tag for `ghcr.io/tailscale/tailscale`
- `GO_VERSION` (default: `1.26.5`)
- `NODE_VERSION` (default: `24.19.0`)
- `ALPINE_VERSION` (default: `3.22`)
- `WEBUI_SOURCE` (default: `webui-builder`; set to `binary-dev` for dev builds)
- `VERSION`, `COMMIT`, `DATE`, `BRANCH`, `BUILDER` — build metadata injected into Go binary and frontend via ldflags / `npm version`

### Development

```bash
make dev-build           # Build Go binary + frontend locally
make dev-docker-build    # Build Docker image using local binary
```

`dev-docker-build` passes `--build-arg WEBUI_SOURCE=binary-dev` so Docker copies `./data/tailrelay-webui` from the build context instead of compiling in-container. No separate `Dockerfile.dev` is needed.

Both `dev-build` and `dev-docker-build` default `GOARCH` to the host's native architecture (via `go env GOARCH`), and `dev-docker-build` passes it through as `--platform linux/$(GOARCH)`. On Apple Silicon this builds a native `linux/arm64` binary/image with no QEMU emulation. Override with `make dev-docker-build GOARCH=amd64` to cross-build for a different arch.

`dev-docker-build` and `release` also auto-detect the container engine via `CONTAINER_ENGINE ?= $(shell command -v docker ... || echo podman)`, so podman-only setups (no `docker` binary — a shell `alias docker=podman` isn't visible to Make's non-interactive recipe shell) work without edits. `BUILDX_LOAD` adds `--load` only for docker, since podman's `buildx build` shim always loads locally and rejects that flag. Override with `make dev-docker-build CONTAINER_ENGINE=podman` if detection picks the wrong engine. Podman's `buildx build` compatibility shim ships with recent Podman Desktop/CLI releases (verified against podman 6.0.1) but isn't guaranteed on every install — a "command not found" here means the podman install lacks the shim, not a bug in the detection logic. `make release`'s multi-platform `--push` hasn't been exercised against podman; if it doesn't push cleanly, run it with `CONTAINER_ENGINE=docker` instead.

## Make Targets

| Target | Description | Depends On |
|--------|-------------|------------|
| `make help` | Show available targets | — |
| `make test` | Run Go unit tests (`cd webui && go test ./...`) | — |
| `make integration-test` | Run integration tests via pytest | Docker, `.env` |
| `make frontend-build` | Build SPA assets (npm install + build) | Node.js |
| `make dev-build` | Build Go binary with metadata (native `GOARCH` by default) | `frontend-build` |
| `make dev-docker-build` | Build dev Docker image for native host platform (`linux/$(GOARCH)`) | `dev-build` |
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

`.github/workflows/ci.yml` runs on push/PR to `main`, on `v*.*.*` tag pushes, and on published releases:

### Jobs

1. **frontend** — `npm install` + `npm run build` in `webui/frontend/` (Node.js 24.18.0 in CI)
2. **backend** — installs Node.js, runs `npm install` + `npm run build` (required for `//go:embed all:web/dist`), then `go vet`, `go test -v`, `go build` in `webui/` (Go 1.24 in CI)
3. **integration** — Full Docker build + `pytest tests/integration/ -v`
4. **release** — Runs only on `v*.*.*` tag pushes, after all three above pass; extracts changelog notes, logs in to Docker Hub + GHCR, builds multi-platform image (`linux/amd64`, `linux/arm64`) and pushes `vX.Y.Z` + `latest` tags to both registries, and creates a GitHub Release

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
2. Start Web UI
3. `wait` on tailscaled + webui PIDs

Handles `SIGTERM`/`SIGINT` for graceful shutdown.

## Common Pitfalls

1. **File persistence**: Start9 removes files on reboot — mount `/var/lib/tailscale` as a volume
2. **Hostname matching**: `TS_HOSTNAME` must match everywhere (Tailscale, Web UI)
3. **Docker network**: Use `--net start9` for Start9 deployments to reach embassy services
4. **TLS certificates**: Must enable HTTPS in Tailscale Admin Console first
5. **Port conflicts**: Ensure host ports don't conflict with existing services
6. **Repo `default_workflow_permissions`**: a job's `permissions:` block can only narrow `GITHUB_TOKEN`, never grant more than the repo's Settings → Actions → General → Workflow permissions default allows. If GHCR pushes or GitHub Release creation start failing with `403`, check `gh api repos/<owner>/<repo>/actions/permissions/workflow` is `write`, not `read`.

## Bumping Dependencies

When updating a pinned version in the Dockerfile, touch every location in the table below.

| Dependency | Dockerfile ARG | `docker-ci/SKILL.md` | `security-review/SKILL.md` | `CHANGELOG.md` |
|------------|---------------|----------------------|---------------------------|----------------|
| Tailscale | `TAILSCALE_VERSION` | Version Information table | Pinned Versions block | `[Unreleased]` → `### Changed` |
| Go | `GO_VERSION` | Version Information table | Pinned Versions block | `[Unreleased]` → `### Changed` |
| Node.js | `NODE_VERSION` (Dockerfile) + `node-version` (ci.yml, 4 occurrences) | Version Information table | Pinned Versions block | `[Unreleased]` → `### Changed` |

### Step-by-step

1. Edit the `ARG` value at the top of `Dockerfile`.
2. Update the **Version Information** table at the bottom of this file.
3. Update the **Pinned Versions** block in `.agents/skills/security-review/SKILL.md`.
4. Add a `### Changed` bullet to the `[Unreleased]` section in `CHANGELOG.md`.
5. Advance `reviewed_at` in this file to the new HEAD SHA after committing.

### Example CHANGELOG entry

```markdown
## [Unreleased]

### Changed
- **Tailscale** bumped from `vX.Y.Z` to `vA.B.C` in Dockerfile
- **Node.js** bumped from `X.Y.Z` to `A.B.C` in Dockerfile
```

## Version Information

| Component | Version |
|-----------|---------|
| Container | `v0.9.0` (see `start.sh`) |
| Tailscale | `v1.102.2` (`ghcr.io/tailscale/tailscale` base image) |
| Go | `1.26.5` (Dockerfile ARG) |
| Node.js (CI) | `24.19.0` (GitHub Actions + Dockerfile ARG) |
