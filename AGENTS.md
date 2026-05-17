# Agent Development Guide

Essential information for coding agents working with the tailrelay codebase.

## Project Overview

**tailrelay** is a Docker container combining Tailscale and a Go Web UI to expose local services to a Tailscale network via `tailscale serve`. For detailed component knowledge, see the [Skills Directory](#skills-directory) below.

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
| **Serve Relays** | `.agents/skills/serve/SKILL.md` | tailscale serve relay management, ErrTailscaleNotReady, legacy shims |
| **Web UI** | `.agents/skills/webui/SKILL.md` | Go app, handlers, auth, backup, frontend SPA, build |
| **Docker/CI** | `.agents/skills/docker-ci/SKILL.md` | Dockerfile, Compose, GitHub Actions, testing |
| **Security Review** | `.agents/skills/security-review/SKILL.md` | CVE scanning, auth review, injection risks, privacy audit |
| **Testing & CI/CD** | `.agents/skills/testing-cicd/SKILL.md` | Writing Go tests, integration tests, extending ci.yml |
| **Documentation** | `.agents/skills/documentation/SKILL.md` | README, CHANGELOG, release notes, AGENTS.md, SKILL.md files |

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
├── .agents/skills/         # Agent Skills (see table above)
├── .agents/workflows/      # Dev workflows (dev-build, docker-test)
└── .github/workflows/      # CI pipeline
```

## Documentation Review Status

> When updating a doc, check what changed since its `reviewed_at` commit:
> `git log --oneline <reviewed_at>..HEAD -- <paths>`
> Update `reviewed_at` to the current HEAD commit after completing a full review.

| Document | `reviewed_at` | Paths Covered |
|----------|---------------|---------------|
| `AGENTS.md` | `e413322` | `AGENTS.md`, `Makefile`, `start.sh` |
| `.agents/skills/webui/SKILL.md` | `e413322` | `webui/`, `Makefile` |
| `.agents/skills/docker-ci/SKILL.md` | `e413322` | `Dockerfile`, `.github/workflows/`, `compose-test.yml` |
| `.agents/skills/tailscale/SKILL.md` | `e413322` | `webui/internal/tailscale/`, `start.sh` |
| `.agents/skills/serve/SKILL.md` | `e413322` | `webui/internal/serve/`, `webui/internal/handlers/serve.go`, `webui/internal/handlers/legacy.go` |
| `webui/README.md` | `b7ce114` | `webui/` |
| `README.md` | `b7ce114` | `README.md`, `webui/internal/web/server.go` |
| `.agents/skills/security-review/SKILL.md` | `b7ce114` | `webui/internal/auth/`, `webui/internal/handlers/`, `webui/internal/backup/`, `Dockerfile`, `start.sh` |
| `.agents/skills/testing-cicd/SKILL.md` | `e413322` | `tests/`, `webui/internal/*/\*_test.go`, `.github/workflows/ci.yml` |
| `.agents/skills/documentation/SKILL.md` | `e413322` | `README.md`, `CHANGELOG.md`, `webui/README.md`, `AGENTS.md`, `.agents/skills/` |

## Making Changes

1. Update version in `start.sh` (and release notes as needed)
2. Rebuild: `make dev-build` or `make dev-docker-build`
3. Run tests: `go test ./...` + `pytest tests/integration/ -v`
4. Validate health endpoints
5. Update `README.md` for user-facing changes
