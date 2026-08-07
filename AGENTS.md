# Agent Development Guide

Essential information for coding agents working with the tailrelay codebase.

## Project Overview

**tailrelay** is a Docker container combining Tailscale and a Go Web UI to expose local services to a Tailscale network via `tailscale serve`. For detailed component knowledge, see the [Skills Directory](#skills-directory) below.

## LLM Operational Rules (Read First)

1. **Prefer Make targets and documented scripts** before inventing new commands.
2. **Avoid long-running daemons** unless explicitly requested (e.g., `docker compose up -d`).
3. **Never stop a process with `pkill -f <pattern>`.** `-f` matches full command
   lines, including the shell running your own command, so `pkill -f vite` kills
   the session mid-command — any uncommitted work in that command is lost. Use
   `pgrep -af <pattern> | grep -v pgrep | awk '{print $1}' | xargs -r kill`, or
   better, spawn and tear down the process inside a single script. Never chain a
   kill onto a `git commit`/`git push`.
4. **Do not mutate host state** (system packages, global config) without explicit request.
5. **Use .env for tests** and never hardcode secrets or tokens.
6. **When running commands**, keep output small and relevant (pipe/grep if needed).
7. **If a change affects external behavior**, update README or release notes as required.

## Skills Directory

Detailed component knowledge is organized into Agent Skills at `.agents/skills/`:

| Skill | Path | When to Use |
|-------|------|-------------|
| **Tailscale** | `.agents/skills/tailscale/SKILL.md` | VPN daemon, CLI, authentication, MagicDNS, HTTPS certs |
| **Serve & Funnel** | `.agents/skills/serve/SKILL.md` | `tailscale serve`/`tailscale funnel` relay management, serve_relays.json, reconciliation |
| **Web UI** | `.agents/skills/webui/SKILL.md` | Go app, handlers, auth, backup, frontend SPA, build |
| **Docker/CI** | `.agents/skills/docker-ci/SKILL.md` | Dockerfile, Compose, GitHub Actions, testing |
| **Security Review** | `.agents/skills/security-review/SKILL.md` | CVE scanning, auth review, injection risks, privacy audit |
| **Testing & CI/CD** | `.agents/skills/testing-cicd/SKILL.md` | Writing Go tests, integration tests, extending ci.yml |
| **Documentation** | `.agents/skills/documentation/SKILL.md` | README, CHANGELOG, release notes, AGENTS.md, SKILL.md files, screenshots |

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
pytest tests/integration/ -v    # Python integration suite
```

### Docker

```bash
docker buildx build -t sudocarlos/tailrelay:latest --load .  # Production build
docker compose -f compose-test.yml up -d                      # Start test env
docker compose -f compose-test.yml down                       # Stop test env
```

### Health Checks

```bash
curl -sSL http://${TAILRELAY_HOST}:9002/healthz  # Tailscale health
curl -sSL http://localhost:8021                   # Web UI
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
| `RELAY_LIST` | *(empty)* | Legacy: comma-separated `listen:host:port` (migrated to serve_relays.json on first start) |
| `TS_EXTRA_FLAGS` | *(empty)* | Additional Tailscale flags |
| `TS_AUTH_ONCE` | `true` | Authenticate once |
| `TS_ENABLE_METRICS` | `true` | Enable `:9002/metrics` |
| `TS_ENABLE_HEALTH_CHECK` | `true` | Enable `:9002/healthz` |

## File Map

```
├── AGENTS.md               # This file — agent entry point
├── CHANGELOG.md            # Release history (Keep a Changelog format)
├── Dockerfile              # Container image (multi-stage)
├── Makefile                # Build targets
├── start.sh                # Container entrypoint
├── webui/                  # Go Web UI (see webui skill)
├── tests/                  # Integration test suite (pytest)
├── compose-test.yml        # Test Compose config
├── docs/openapi.yaml       # OpenAPI 3.1 spec — source of truth for the API
├── docs/screenshots/       # Screenshot source files (captured via take-screenshots.mjs)
├── website/                # Docusaurus docs site (renders docs/openapi.yaml)
├── .agents/skills/         # Agent Skills (see table above)
├── .agents/workflows/      # Dev workflows (dev-build, docker-test)
└── .github/workflows/      # CI pipeline + docs.yml (GitHub Pages deploy)
```

## Making Changes

1. Update version in `start.sh` (and release notes as needed)
2. Rebuild: `make dev-build` or `make dev-docker-build`
3. Run tests: `go test ./...` + `pytest tests/integration/ -v`
4. Validate health endpoints
5. Update `README.md` for user-facing changes
