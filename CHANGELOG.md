# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
