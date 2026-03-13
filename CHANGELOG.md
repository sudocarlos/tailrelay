# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.8.0] - 2026-03-13

### Added

#### Testing
- **Go unit test suite** — 5 new test files covering `internal/auth` (10 tests), `internal/handlers` (auth, caddy, socat, backup; 69 tests total), `internal/caddy` (extended manager and proxy_manager tests)
- **Python integration test suite** — `tests/integration/` pytest suite with 16 named tests across 5 classes (startup, Tailscale endpoints, Web UI API, Caddy Admin API, socat relay)
- `make test` and `make integration-test` targets for running unit and integration tests

#### Build
- **Tailscale built from source** — replaced `tailscale/tailscale` base image with Alpine + `go install tailscale.com/cmd/...@${TAILSCALE_VERSION}`; adds `tailscale-builder` stage
- **Caddy built from source** — split into dedicated `caddy-builder` stage (`git clone caddyserver/caddy` at pinned tag)
- **Frontend version injection** — `VERSION` ARG passed to `npm version` during Docker build and `make frontend-build` so the SPA reports the correct version
- **`ALPINE_VERSION` ARG** pinned at top of Dockerfile alongside all other dependency versions
- `make release` target for multi-platform (amd64/arm64) push to Docker Hub and GHCR

#### Infrastructure
- Tailscale status cache (`internal/tailscale/cache.go`) to reduce repeated CLI calls
- Legacy iptables symlinks restored in container for hosts without nftables (e.g. Synology)
- `MAILCAP_VERSION` and `SOCAT_VERSION` pinned via apk version constraints

### Changed
- `Dockerfile.dev` removed — dev builds now use `--build-arg WEBUI_SOURCE=binary-dev` on the main Dockerfile
- Integration tests consolidated under `tests/integration/` with `helpers.py` as the single source of truth for shared utilities; removed root-level `docker-compose-test.py`, `test_proxy_api.sh`
- CI integration job updated to use `pytest tests/integration/ -v` instead of shell scripts
- Go version updated to `1.26.1`, Node to `24` in Dockerfile build args
- Tailscale peers table shows DNS name (first label only) and "just now" for recently-seen peers
- `caddy start` no longer requires `--config` flag (auto-discovers `/etc/caddy/Caddyfile`)

### Fixed
- Integration test failures: Caddy port checks now target Admin API (`:2019`); Tailscale health/metrics tests skip gracefully when `TS_AUTHKEY` is absent; Web UI API tests use correct endpoint paths and Bearer token auth

### Docker

```
docker pull sudocarlos/tailrelay:v0.8.0
```

## [0.7.0-rc1] - 2026-01-01

This release candidate introduces a complete frontend rewrite and significant backend additions.

### Added

#### Frontend
- **Svelte 5 + Tailwind CSS SPA** — replaced vanilla JS/Bootstrap with a modern, reactive single-page application
- Vite-based build pipeline; assets embedded directly into the Go binary at build time
- Improved UI responsiveness and component structure

#### Authentication
- **bcrypt password hashing** — replaces plain-text password storage

#### Tailscale Management
- New Web UI section for Tailscale node information and status
- Tailscale hostname change now migrates existing Caddy proxy routes automatically

#### Caddy / Proxy
- **TLS certificate startup check** — validates certs on boot for `*.ts.net` proxies
- Replaced per-server metrics with global server metrics
- Caddy proxy hostnames migrate automatically when Tailscale hostname changes

#### Backup & Restore
- Full configuration backup and restore support via the Web UI

### Changed
- Improved logging throughout the backend

### Fixed
- Various bug fixes and stability improvements

### Upgrade Notes
- The frontend assets are now compiled via `make frontend-build` (Node 22 + Vite required for building from source)
- Docker image users: no action required — assets are pre-built in the image
- Password hashes are automatically upgraded to bcrypt on first login after upgrade

### Docker
```
docker pull sudocarlos/tailrelay:v0.7.0-rc1
```
