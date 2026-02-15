# Agent Development Guide

Essential information for coding agents working with the tailrelay codebase.

## Project Overview

**tailrelay** is a Docker container combining Tailscale, Caddy, socat, and a Go Web UI to expose local services to a Tailscale network. For detailed component knowledge, see the [Skills Directory](#skills-directory) below.

## LLM Operational Rules (Read First)

1. **Prefer Make targets and documented scripts** before inventing new commands.
2. **Avoid long-running daemons** unless explicitly requested (e.g., `docker compose up -d`).
3. **Do not mutate host state** (system packages, global config) without explicit request.
4. **Use .env for tests** and never hardcode secrets or tokens.
5. **When running commands**, keep output small and relevant (pipe/grep if needed).
6. **If a change affects external behavior**, update README or release notes as required.

## Skills Directory

Detailed component knowledge is organized into Agent Skills at `.agents/skills/`:

| Skill | Path | When to Use |
|-------|------|-------------|
| **Tailscale** | `.agents/skills/tailscale/SKILL.md` | VPN daemon, CLI, authentication, MagicDNS, HTTPS certs |
| **Caddy** | `.agents/skills/caddy/SKILL.md` | Reverse proxy Admin API, CRUD ops, @id tags, TLS |
| **socat** | `.agents/skills/socat/SKILL.md` | TCP relays, RELAY_LIST, process management |
| **Web UI** | `.agents/skills/webui/SKILL.md` | Go app, handlers, auth, backup, frontend SPA, build |
| **Docker/CI** | `.agents/skills/docker-ci/SKILL.md` | Dockerfile, Compose, GitHub Actions, testing |

Read the relevant SKILL.md before making changes to that component.

## Quick Reference Commands

### Make Targets

```bash
make help              # Show all targets
make frontend-build    # Build SPA assets (Node.js/npm)
make dev-build         # Build Go binary with metadata (includes frontend-build)
make dev-docker-build  # Build dev Docker image (includes dev-build)
make clean             # Remove build artifacts
```

### Testing

```bash
cd webui && go test ./...       # Go unit tests
python docker-compose-test.py   # Python integration suite
./docker-compose-test.sh        # Bash integration suite
./test_proxy_api.sh             # API endpoint tests
```

### Docker

```bash
docker buildx build -t sudocarlos/tailrelay:latest --load .  # Production build
docker compose -f compose-test.yml up -d                      # Start test env
docker compose -f compose-test.yml down                       # Stop test env
```

### Health Checks

```bash
curl -sSL http://${TAILRELAY_HOST}:8080   # HTTP proxy
curl -sSL http://${TAILRELAY_HOST}:9002/healthz  # Tailscale health
curl -sSL http://localhost:8021            # Web UI
```

## Code Style Quick Reference

| Language | Key Rules |
|----------|-----------|
| **Go** | `gofmt`, handlers in `internal/handlers/`, explicit error handling, no panics |
| **Shell** | `#!/usr/bin/env bash` (or `#!/bin/ash` for Alpine), 4-space indent, quote `"$VARS"` |
| **Python** | Type hints, f-strings, handle subprocess timeouts, stdlib first |
| **Dockerfile** | `ARG` for build-time, `ENV` for runtime, combine `RUN` steps |

## Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `TS_HOSTNAME` | *(required)* | Tailscale machine name |
| `TS_STATE_DIR` | `/var/lib/tailscale/` | Tailscale state directory |
| `RELAY_LIST` | *(empty)* | Legacy comma-separated relay definitions |
| `TS_EXTRA_FLAGS` | *(empty)* | Additional Tailscale flags |
| `TS_AUTH_ONCE` | `true` | Authenticate once |
| `TS_ENABLE_METRICS` | `true` | Enable `:9002/metrics` |
| `TS_ENABLE_HEALTH_CHECK` | `true` | Enable `:9002/healthz` |

## File Map

```
├── AGENTS.md               # This file — agent entry point
├── Dockerfile / .dev        # Container images
├── Makefile                 # Build targets
├── start.sh                 # Container entrypoint
├── webui/                   # Go Web UI (see webui skill)
├── compose-test.yml         # Test Compose config
├── docker-compose-test.*    # Integration test scripts
├── .agents/skills/          # Agent Skills (see table above)
├── .agent/workflows/        # Dev workflows (dev-build, docker-test)
└── .github/workflows/       # CI pipeline
```

## Making Changes

1. Update version in `start.sh` (and release notes as needed)
2. Rebuild: `make dev-build` or `make dev-docker-build`
3. Run tests: `go test ./...` + integration scripts
4. Validate health endpoints
5. Update `README.md` for user-facing changes
