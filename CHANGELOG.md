# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.10.1] - 2026-08-11

### Fixed
- **Peer exit-node selection under userspace networking** (#93) — tailrelay runs tailscaled with `--tun=userspace-networking`, which has no kernel TUN device and cannot install host routes. `tailscale set --exit-node=<peer>` previously stored `ExitNodeIP` but silently redirected nothing. The networking API now rejects a non-empty `exit_node` in `UpdateNetworking` with HTTP 409 (clearing it and `advertise_exit_node` stay allowed), and the dashboard dropdown hides peer exit-node options under userspace networking, rendering only "None" and "Run as exit node". A new `userspace_networking` flag is surfaced in `GetNetworkingSummary` and `docs/openapi.yaml`.
- **Address dropdown dismissal in macOS Safari** (#99) — replaced the native `<details>` address popover with a state-driven `AddressMenu.svelte` using the shared portal pattern (document-level outside-click + `Escape` listeners that run only while open). The portaled menu is fixed-positioned beneath the trigger, measured after mount to escape table-overflow clipping, and repositioned on scroll/resize. `CopyButton` uses `stopImmediatePropagation` so copying keeps the menu open with the inline "Copied" feedback visible, and mouse-out dismissal bridges the trigger/menu gap with a shared 120ms leave timer.
- **AddressMenu a11y warning** — the portaled `<div role="menu">` carries a no-op `onkeydown`; Svelte flagged `a11y_interactive_supports_focus`. Added `tabindex="-1"` so the menu is focusable programmatically while staying out of tab order.

### Changed
- **Tailscale** bumped from `v1.98.9` to `v1.102.2` in Dockerfile (via `v1.98.10`)
- **Node.js** bumped from `24.18.0` to `24.19.0` in Dockerfile and CI (via `24.18.1`)
- **Frontend tooling** — bumped `vite` in `webui/frontend/package.json` from `8.1.5` to `8.2.1` (via `8.2.0`)
- **Frontend deps** — bumped `undici` from `7.28.0` to `7.29.0` in `/webui/frontend` (#90)
- **Website deps** — bumped `fast-uri` from `3.1.4` to `3.1.5` in `/website` (#91)

### Security
- **Website dependency overrides** (#101) — bumped `js-yaml` `4.3.0 -> 4.3.1` (alert #63) and `brace-expansion` `5.0.8 -> 5.0.9` (alert #56) overrides in `website/package.json`; dismissed image-size alerts #64/#65 (no upstream fix, build-time-only).

### Docker
```
docker pull sudocarlos/tailrelay:v0.10.1
docker pull ghcr.io/sudocarlos/tailrelay:v0.10.1
```

## [0.10.0] - 2026-07-27

### Added
- **Control Server field on the Tailscale page** — connect to a self-hosted [Headscale](https://headscale.net) instance instead of Tailscale's default control plane
  - New "Control Server" input inside the "Authentication Required" section, shown after the Login URL / Auth Key tabs while logged out, persisted via `GET /api/tailscale/control-server` and `POST /api/tailscale/control-server/update`
  - Automatically applied as `tailscale login --login-server=<url>` / `tailscale up --authkey=<key> --login-server=<url>` on subsequent logins
  - URL validation (must be a valid `http://`/`https://` URL) on both the client and server
- **Headscale auth keys** — the "Auth Key" login flow now also accepts `hskey-` prefixed keys (Headscale) in addition to `tskey-` (Tailscale)

### Changed
- **Tailscale** bumped from `v1.98.8` to `v1.98.9` in Dockerfile
- **Funnel section on the dashboard** — hidden while connected to a custom control server, since Funnel is a Tailscale-cloud-only feature not supported by self-hosted Headscale
- **Frontend tooling** — bumped `vite` in `webui/frontend/package.json` from `8.1.4` to `8.1.5`
- **Go** bumped from `1.26.4` to `1.26.5` in Dockerfile
- **Docusaurus** bumped from `3.10.1` to `3.10.2` in `website/package.json`, with `brace-expansion` overridden to `5.0.8` to resolve `GHSA-mh99-v99m-4gvg` (transitive via `serve-handler` → `minimatch`)

### Fixed
- **`make dev-docker-build`/`make release`** — auto-detect the container engine (`docker` or `podman`) instead of hardcoding `docker`, since a shell `alias docker=podman` isn't visible to Make's non-interactive recipe shell. `dev-build`/`dev-docker-build` also now default `GOARCH`/`--platform` to the host's native architecture (`go env GOARCH`) instead of relying on implicit defaults, so builds on Apple Silicon are native `linux/arm64` with no QEMU emulation.
- **Changing the hostname while connected to a custom control server** — `POST /api/tailscale/hostname` now uses `tailscale set --hostname=<name>`, which changes only the machine name and preserves the active control server and all other node preferences.
- **Web relays with a custom control server** — relays stored as `https` now use Tailscale Serve's HTTP listener under Headscale, avoiding its unsupported HTTPS certificate-enablement endpoint. Relay list responses report the effective `listener_scheme`, and dashboard links correctly use `http://` in that mode.
- **Stale relay hostname after re-authentication** — HTTPS/Funnel relay URLs no longer freeze on the machine name captured at relay-creation time. The `hostname` field is no longer persisted in `serve_relays.json`; list responses now always compute it live from the current Tailscale/Headscale MagicDNS name, so relay links update correctly after a `tailscale logout` + re-`tailscale up` assigns a new auto-generated machine name.
- **Silent logout after a machine is removed from the tailnet** — when tailscaled reports "authkey already used" (the stale identity from the invalidated auth key), the connection card now surfaces the daemon's error as a toast and no longer offers a "Connect" reconnect button for this state, since retrying `tailscale up` with the same stale identity fails the same way. The existing re-authentication form (Auth Key / browser login) is shown instead so the user can log back in.
- **Hostname reverting to the container ID after Logout + re-authenticating** — `tailscale login`/`tailscale up --authkey=<key>` triggered from the Web UI now reapply the hostname last set via the "Change Hostname" field (`--hostname=<name>`), instead of leaving it to tailscaled's own default. Previously, a Logout followed by re-authenticating through the Web UI silently dropped back to tailscaled's OS-default hostname (e.g. the container ID), and the control server could take a while to propagate the resulting rename to `DNSName`, leaving Serve/Funnel relay links pointing at the wrong location in the meantime. The hostname preference is now persisted to `webui.yaml` and reused on every future login.
- **Funnel/HTTP-scheme detection drifting from the actual control server** — Funnel visibility and the `--https`/`--http` choice for web relays previously relied solely on the persisted Control Server setting, which could be empty/stale if a node was authenticated against a custom control server outside the Web UI (CLI login, restored state). Both are now derived live from tailscaled's `ControlURL` preference (`tailscale debug prefs`), so they stay correct regardless of how the node was authenticated. The persisted setting is still used to build `--login-server=<url>` for the next login/connect.

### Docker
```
docker pull sudocarlos/tailrelay:v0.10.0
docker pull ghcr.io/sudocarlos/tailrelay:v0.10.0
```

## [0.9.5] - 2026-07-14

### Added
- **Networking section on the Tailscale page** — manage `tailscale set` networking preferences from the dashboard
  - Exit node dropdown combines a "Run as exit node" option with selecting a peer already advertising as one, with **Allow LAN access** shown as a sub-option while using a peer's exit node
  - Advertise one or more subnet routes, with CIDR validation (rejects malformed/host-bit-set CIDRs and the reserved exit-node CIDRs `0.0.0.0/0`/`::/0`)
  - Accept routes advertised by other tailnet nodes
  - Run Tailscale SSH
  - New `GET /api/tailscale/networking` and `POST /api/tailscale/networking/update` API endpoints

### Changed
- **Relay card UI** — replaced per-card Play/Pause buttons, inline "Auto" checkbox, and separate Edit/Delete buttons with an iOS-style toggle switch and a portal-based "..." overflow menu containing Edit, Start on boot, and Delete actions
  - Status indicator now lives inside the type icon badge (colored background + white icon when running, amber pulse when toggling, neutral when stopped)
  - New `CopyButton` component for one-click address copying on relay/proxy/funnel cards
  - Shared `portal` Svelte action extracted from `Tooltip.svelte` into `actions/portal.js`
- **Frontend tooling** — bumped `vite` in `webui/frontend/package.json` from `8.1.3` to `8.1.4`

### Docker
```
docker pull sudocarlos/tailrelay:v0.9.5
docker pull ghcr.io/sudocarlos/tailrelay:v0.9.5
```

## [0.9.4] - 2026-07-07

### Fixed
- **CI provenance/SBOM attestations re-enabled** — reverted the rollback; attestation changes were not the cause of GHCR publish failures on the v0.9.3 release pipeline

### Security
- **`golang.org/x/crypto`** bumped from `v0.48.0` to `v0.52.0` (addresses vulnerabilities in SSH and crypto packages)

### Docker
```
docker pull sudocarlos/tailrelay:v0.9.4
docker pull ghcr.io/sudocarlos/tailrelay:v0.9.4
```

## [0.9.3] - 2026-07-05

### Added
- **Tailscale Funnel support** — expose local services to the public internet on ports `443`, `8443`, or `10000` directly from the dashboard
  - New **Funnel** dashboard section shows a card for each of the three funnel-eligible ports
  - Free ports show a placeholder card that opens the configure dialog with the port pre-filled
  - Ports already used by an existing serve relay show a disabled "in use" card
  - Configured funnel ports support the same lifecycle actions as serve relays: edit, start/stop, autostart, delete
  - Supports both HTTPS reverse-proxy and raw TCP funnel transports
  - New `/api/serve/funnel/{list,get,create,update,delete,toggle}` API endpoints
- **GHCR release** — CI now also publishes tagged releases to `ghcr.io/sudocarlos/tailrelay` alongside Docker Hub

### Changed
- **Tailscale** bumped from `v1.98.4` to `v1.98.8` in Dockerfile
- **Node.js** bumped from `24.17.0` to `24.18.0` in Dockerfile and CI workflows

### Fixed
- **GHCR publish failure** — disabled provenance/SBOM attestations on the release Docker build, which GHCR rejected with a `403` when pushing to the brand-new `ghcr.io/sudocarlos/tailrelay` package

### Security
- **Docusaurus dependency upgrades** — resolved 9 Dependabot alerts in `website/` by pinning transitive dependencies to patched versions:
  - `js-yaml` (DoS via merge key aliases, prototype pollution in merge)
  - `serialize-javascript` (RCE via RegExp flags, CPU exhaustion DoS)
  - `uuid` (missing buffer bounds check)
  - `lodash` (code injection via template, prototype pollution)
  - `yaml` (stack overflow via deeply nested collections)

### Docker
```
docker pull sudocarlos/tailrelay:v0.9.3
docker pull ghcr.io/sudocarlos/tailrelay:v0.9.3
```

## [0.9.2] - 2026-06-20

### Changed
- **Node.js** bumped from `24.16.0` to `24.17.0` in Dockerfile
- **Tailscale** bumped from `v1.98.3` to `v1.98.4` in Dockerfile
- **Go** bumped from `1.26.3` to `1.26.4` in Dockerfile

### Docker
```
docker pull sudocarlos/tailrelay:v0.9.2
```
 
## [0.9.1] - 2026-05-22

### Changed
- **Node.js** bumped from `24.15.0` to `24.16.0` in Dockerfile
- **Tailscale** bumped from `v1.96.5` to `v1.98.3` in Dockerfile

### Docker
```
docker pull sudocarlos/tailrelay:v0.9.1
```

## [0.9.0] - 2026-05-18

This release replaces the Caddy and socat relay stack with native `tailscale serve`. All relay types (HTTPS and TCP) are now managed directly through the Tailscale daemon — no third-party proxy processes required.

### Added
- **Tailscale Serve relay manager** — unified relay backend (`internal/serve`) for HTTPS and TCP relays with persisted desired state in `serve_relays.json`.
- **Autostart reconciliation** — relays marked as autostart are automatically applied via `tailscale serve` on every container boot (`ReconcileAutostart`).
- **Search box on dashboard** — the relay list type filter buttons have been replaced with a single search box for faster relay lookup.

### Changed
- **Runtime architecture** — all proxy and TCP relay functionality now runs via `tailscale serve`; Caddy and socat have been removed entirely.
- **API surface** — relay endpoints moved from `/api/caddy/*` and `/api/socat/*` to `/api/serve/https/*` and `/api/serve/tcp/*`.
- **Container image** — removed Caddy and socat runtime dependencies from `Dockerfile` and `start.sh`.
- **Delete confirmation** — the relay delete dialog now shows a styled relay card for clearer confirmation of what will be removed.

### Fixed
- **Serve reconciliation correctness** — addressed edge cases in relay state reconciliation identified in review (PR #23).

### Removed
- **Caddy** — removed `internal/caddy/`, `internal/handlers/caddy.go`, and all Caddy Admin API integration.
- **socat** — removed `internal/socat/`, `internal/handlers/socat.go`, and socat process management.
- **Caddy metrics endpoints** — dropped `/api/caddy/metrics` and `/api/caddy/metrics/reset`.
- **Legacy config paths** — `caddy_config`, `socat_relay_config`, `caddy_proxy_config`, `caddy_server_map` removed from `webui.yaml`; `serve_relay_config` is the only relay config path.

### Upgrade Notes

> **Breaking changes** — this release removes Caddy and socat entirely. Review the points below before upgrading.

- **Caddy and socat are gone.** All relay traffic (HTTPS and TCP) now flows through `tailscale serve`. There are no Caddy or socat processes in the container.
- **API endpoints have changed.** If you call the relay API directly (e.g. from scripts or external tooling), update your paths:
  - HTTPS relays: `/api/caddy/proxies/*` → `/api/serve/https/*`
  - TCP relays: `/api/socat/relays/*` → `/api/serve/tcp/*`
  - Caddy metrics: `/api/caddy/metrics` and `/api/caddy/metrics/reset` have been removed with no replacement.
- **Migration is automatic.** On first start after upgrading, existing `proxies.json` (Caddy) and `relays.json` (socat) are automatically merged into `serve_relays.json` and all relays are reconciled via `tailscale serve`. No manual steps are required.
- **`RELAY_LIST` env var** is also automatically migrated to `serve_relays.json` as TCP relays on first start.
- **`webui.yaml` config paths** `caddy_config`, `socat_relay_config`, `caddy_proxy_config`, and `caddy_server_map` are no longer read. The only relay config path is `serve_relay_config`.

### Docker

```
docker pull sudocarlos/tailrelay:v0.9.0
```

## [0.8.8] - 2026-05-14

### Changed
- **Tailscale v1.96.5** — updated from v1.96.4

### Docker

```
docker pull sudocarlos/tailrelay:v0.8.8
```

## [0.8.7] - 2026-05-14

### Changed
- **Caddy 2.11.3** — updated from 2.11.2

### Docker

```
docker pull sudocarlos/tailrelay:v0.8.7
```

## [0.8.6] - 2026-03-31


### Added
- **Custom Host header override** — `CaddyProxy` gains an optional `host_header` field; when set, the specified value replaces the default upstream `{http.reverse_proxy.upstream.hostport}` placeholder as the `Host` header sent to the backend; useful for HTTPS backends that require a specific SNI/hostname. Exposed in the Add/Edit modal under a new "Advanced" collapsible section alongside Trusted Proxies.

### Changed
- **Metrics relay labels truncated** — relay target hostnames longer than 30 characters are now shortened to `<first 12 chars>…<last 12 chars>:<port>` in charts, table rows, and the relay filter select; full target is still visible via the native `title` tooltip on table cells.

### Fixed
- **Log `component` field fallback** — the log console now resolves the component badge from the `source` or `Source` field with a `'main'` fallback, preventing blank component labels for log entries emitted by the Go server.
- **Tooltip tap-target and gap** — tooltip trigger zones now have a `p-2 -m-2` padding/negative-margin expansion for easier touch interaction; an 8 px transparent gap bridges the tooltip popup so the mouse cursor can reach it without the popup disappearing; outside-click dismiss is now handled at document level.
- **picomatch CVE-2026-33672** — bumped `picomatch` to `4.0.4` to address the reported vulnerability.

### Removed
- **`webui/cmd/webui/web/dist/` removed from version control** — compiled frontend assets are now gitignored; the CI `backend` job installs Node and runs `npm run build` before `go build` so the `//go:embed all:web/dist` directive always finds the required directory.

### Docker

```
docker pull sudocarlos/tailrelay:v0.8.6
```

## [0.8.5] - 2026-03-28

### Changed
- **Automated releases** — pushing a `v*.*.*` tag now triggers the CI pipeline; after `frontend`, `backend`, and `integration` jobs pass, a `release` job builds multi-platform images (`linux/amd64`, `linux/arm64`), pushes `vX.Y.Z` and `latest` tags to both Docker Hub and GHCR, and creates a GitHub Release with the matching CHANGELOG section as release notes

### Docker

```
docker pull sudocarlos/tailrelay:v0.8.5
```

## [0.8.4] - 2026-03-28

### Changed
- **Tailscale v1.96.4** — updated from v1.92.5

### Docker

```
docker pull sudocarlos/tailrelay:v0.8.4
```

## [0.8.3] - 2026-03-23

### Added
- **Metrics history** — `MetricsStore` samples Caddy's cumulative counters every 15 s and persists timestamped snapshots to `$STATE_DIR/metrics_history.json` (31-day rolling window); history survives proxy pause/resume cycles and container restarts
- **Metrics time-window filter** — `GET /api/caddy/metrics?window=1h|1d|1w|1m` returns traffic deltas within the requested period; the Metrics page adds an All / 1h / 1d / 1w / 1m button group that restarts the 15 s poll on selection
- **Metrics relay filter** — dropdown above the charts lets users narrow the view to a single relay; resets automatically if the selected relay disappears from the data
- **Metrics reset** — `POST /api/caddy/metrics/reset` clears all stored history and baselines; UI exposes a "Reset counters" button with a confirmation prompt
- **HTTPS target transport** — three-mode control when adding or editing a proxy: HTTP target (plain), HTTPS target (insecure, skips certificate verification), or HTTPS target (custom CA cert); insecure mode emits `tls:{insecure_skip_verify:true}` to Caddy; switching modes auto-removes any queued cert
- **Portal-based `Tooltip` component** — `Tooltip.svelte` mounts directly on `document.body` via a Svelte action and positions with `position:fixed`, preventing clipping by overflow containers
- **Preset target auto-apply** — selecting a preset in the Add/Edit modal now automatically sets the relay type and TLS mode from the target's `type`/`protocol` metadata

### Changed
- Metrics chart labels and table rows now always show `:port → target` (compact form) instead of the raw Tailscale FQDN; section headings updated from "per Host" to "per Relay"
- Prometheus metrics parser now keys accumulators on the Caddy `server` label (e.g. `srv0`) rather than the `host` label, so multiple proxies sharing the same FQDN are tracked as separate series
- `GET /api/caddy/metrics` now accepts an optional `?window=` query parameter; callers that omit the parameter continue to receive all-time cumulative totals

### Fixed
- Metrics counters preserved across proxy pause/resume cycles — when a proxy is disabled its Caddy server is deleted and counters reset; the store captures the last-known totals as a baseline so they reappear correctly after re-enable
- Caddy-wide counter reset detection — adding or removing any server resets all Prometheus counters; the poller detects this and saves baselines for all surviving relays to prevent counter regression

### Docker

```
docker pull sudocarlos/tailrelay:v0.8.3
```

## [0.8.2] - 2026-03-14

### Fixed
- **HTTPS relay target accepts bare `host:port`** — the proxy target field no longer requires an `http://` or `https://` scheme prefix; any accidentally supplied scheme is stripped before storage and before the address is passed to Caddy's `reverse_proxy` dial field (passing `http://host:port` to Caddy caused the upstream to be unreachable); the Add/Edit modal placeholder and client-side validation have been updated to reflect the bare `host:port` format (fixes #4, PR #5)
- **Path traversal in backup upload handler** — multipart filename is now sanitised with `filepath.Base` before being joined to the backup directory, preventing a crafted filename such as `../../etc/cron.d/evil.tar.gz` from writing outside the backup directory
- **Zip-slip in backup restore (certificates)** — tar entries under `certificates/` are now checked to remain within the certificates directory after path resolution; entries that would escape are silently skipped
- **Path guard missing separator in backup delete** — `strings.HasPrefix` check now appends `filepath.Separator` to the base directory, preventing a sibling directory whose name starts with the backup dir name from bypassing the guard
- **Unquoted `Content-Disposition` filename in backup download** — filename is now quoted per RFC 6266, preventing header parameter injection via semicolons in filenames

### Security
- Resolved four backup-related security findings identified during internal audit (path traversal write, zip-slip restore, path guard bypass, header injection)

### Docker

```
docker pull sudocarlos/tailrelay:v0.8.2
```

## [0.8.1] - 2026-03-14

### Fixed
- **HTTPS relay target requires URL scheme** — selecting a preset in the Add Relay form now automatically prepends `http://` when the HTTPS relay toggle is on; the target input placeholder updates to show the expected `http://host:port` format; `saveProxy()` validates the scheme client-side before submitting, surfacing a clear error instead of a raw 400 from Caddy

### Docker

```
docker pull sudocarlos/tailrelay:v0.8.1
```

## [0.8.0] - 2026-03-14

### Security
- **Go standard library CVEs mitigated** — govulncheck identified GO-2026-4601 (`net/url` IPv6 parsing), GO-2026-4602 (`os` FileInfo escape), and GO-2026-4603 (`html/template` meta content); these are fixed in go1.25.8 / go1.24.10 but the project already uses go1.26.1 which is the current latest stable and does not have patched releases available; no action required on the go version
- **Input validation for socat relay ports and target host** — listen and target ports are now validated to be within 1–65535; target host is rejected if it contains shell metacharacters, preventing potential command injection
- **Caddy proxy target URL scheme validation** — upstream target must use `http://` or `https://`; `file://`, `unix:`, and other schemes are now rejected at the API handler level
- **SSE log stream CORS header removed** — `Access-Control-Allow-Origin: *` on the `/api/logs/stream` endpoint was removed; the endpoint is already protected by session auth
- **Path traversal fix in backup download handler** — replaced a fragile string-length comparison with `strings.HasPrefix(path, dir+separator)` matching the same pattern used in `backup.Manager.Delete`
- **Security response headers** — all responses now include `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, and `Referrer-Policy: strict-origin-when-cross-origin`

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
- Go version remains `1.26.1` (current latest stable), Node `24` in Dockerfile build args
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
