---
name: security-review
description: Security and privacy review for tailrelay — dependency CVE scanning, Go module auditing, auth review, and code-level vulnerability checks. Use when reviewing code for security issues, auditing Dockerfile dependencies, checking for secrets/injection risks, or assessing privacy of logged/persisted data.
reviewed_at: 7288840
---

# Security & Privacy Review

## Overview

tailrelay combines four networked services (Tailscale, Caddy, socat, Web UI) inside a single container. The attack surface includes: the Web UI HTTP server on port 8021, the Caddy Admin API on port 2019 (internal), socat TCP relays, and the Tailscale daemon. Review should cover all layers — pinned image/binary versions, Go module dependencies, authentication paths, input handling, log sanitization, and data persistence.

## Scope Map

| Area | Files / Packages |
|------|-----------------|
| Authentication | `webui/internal/auth/`, `webui/internal/handlers/auth_test.go` |
| HTTP handlers | `webui/internal/handlers/` |
| Caddy integration | `webui/internal/caddy/` |
| socat management | `webui/internal/socat/` |
| Tailscale integration | `webui/internal/tailscale/` |
| Backup & restore | `webui/internal/backup/` |
| Configuration | `webui/internal/config/`, `webui.yaml` |
| Web server | `webui/internal/web/server.go` |
| Container entrypoint | `start.sh` |
| Dockerfile deps | `Dockerfile` (ARGs at top of file) |
| Frontend | `webui/frontend/src/` |

---

## 1. Dockerfile Dependency Audit

### Pinned Versions (check these first)

All versions are pinned as `ARG` values at the top of `Dockerfile`:

```
ARG TAILSCALE_VERSION=v1.92.5
ARG CADDY_VERSION=2.11.2
ARG GO_VERSION=1.26.1
ARG NODE_VERSION=24
ARG ALPINE_VERSION=3.22
ARG MAILCAP_VERSION=2.1.54
ARG SOCAT_VERSION=1.8.0.3
```

### Tools — Use All That Are Available

Run the tools below and combine findings. Each covers different vulnerability sources.

#### trivy (container image + filesystem + Go modules)

```bash
# Scan the built image (broadest coverage)
trivy image sudocarlos/tailrelay:latest

# Scan repo filesystem for Go module vulns without building
trivy fs --scanners vuln .

# Output as table (default) or JSON for automation
trivy image --format json --output trivy-report.json sudocarlos/tailrelay:latest
```

#### grype (container image + SBOM)

```bash
# Scan the image
grype sudocarlos/tailrelay:latest

# Generate SBOM first, then scan (allows offline re-scanning)
syft sudocarlos/tailrelay:latest -o spdx-json > sbom.json
grype sbom:sbom.json
```

#### govulncheck (Go module CVEs, source-level)

```bash
# Run from the Go module root — finds vulnerabilities reachable in code
cd webui && govulncheck ./...

# Install if missing
go install golang.org/x/vuln/cmd/govulncheck@latest
```

govulncheck is the most precise for Go: it reports only vulnerabilities in code paths actually called, not just imported packages.

#### docker scout (Docker Hub integration)

```bash
# Requires Docker login
docker scout cves sudocarlos/tailrelay:latest

# Quick overview
docker scout quickview sudocarlos/tailrelay:latest
```

### Checking Version Advisories Manually

For each pinned component, check the official security advisory source:

| Component | Advisory Source |
|-----------|----------------|
| Tailscale | https://github.com/tailscale/tailscale/security/advisories |
| Caddy | https://github.com/caddyserver/caddy/security/advisories |
| Alpine Linux | https://security.alpinelinux.org/ |
| Go runtime | https://go.dev/doc/security/vuln/ |
| socat | https://www.exploit-db.com/search?q=socat (check CVE databases) |
| Node.js | https://nodejs.org/en/blog/vulnerability/ |

```bash
# Check Alpine package CVEs for mailcap and socat
docker run --rm alpine:3.22 sh -c "apk update && apk audit"
```

---

## 2. Go Module Audit

```bash
# Check for known vulnerabilities in go.sum
cd webui && govulncheck ./...

# Review all direct and indirect dependencies
go mod graph | head -50

# Check for outdated modules (informational)
go list -m -u all 2>/dev/null | grep '\['

# Verify go.sum integrity
go mod verify
```

Key modules to pay attention to in `webui/go.mod`:
- `golang.org/x/*` packages (crypto, net, text) — frequently have CVEs
- Any HTTP client/server libraries
- YAML parsers (`gopkg.in/yaml.v3`)

---

## 3. Authentication Review

### Auth Flow

tailrelay uses two auth mechanisms (can be used together):

1. **Session cookie auth** (`internal/auth/middleware.go`): Login via POST `/api/auth/login` with bcrypt-verified password. Session token is `crypto/rand`-generated (32 bytes), stored in a `HttpOnly; SameSite=Strict` cookie, expiry 24 h. Cookie gains `Secure` flag when served over TLS.

2. **Static API token** (`internal/auth/middleware.go`): Bearer token from `paths.token_file` (default `/var/lib/tailscale/.webui_token`). Written at container start with `0600` permissions.

### Checklist

- [x] Token file written with `0600` permissions (`os.WriteFile(path, data, 0600)`)
- [x] Session token uses `crypto/rand` (32 bytes → 64-char hex) — not `math/rand`
- [x] Auth middleware applied to all non-public routes (`server.go:setupRoutes`)
- [x] Password storage uses `bcrypt.DefaultCost` — verify `bcrypt.CompareHashAndPassword` usage
- [x] No credentials appear in logs (auth handlers use no logger calls containing passwords)
- [ ] Login handler rate-limits or locks out after repeated failures (not yet implemented)
- [ ] Auth token not returned in any API response body

---

## 4. Input Validation & Injection Risks

### Caddy Proxy Config (`internal/caddy/`, `internal/handlers/caddy.go`)

Caddy routes are constructed from user-supplied upstream URLs and hostnames. Check:

- [x] Upstream URL scheme validated in `validateProxyTarget()` — only `http`/`https` accepted (added v0.8.0)
- [x] Hostname normalised via `caddy.NormalizeHostname()` before use in Caddy JSON config
- [x] Caddy Admin API (`localhost:2019`) is not routed through the public mux
- [ ] Route `@id` tags are user-controlled — check for injection via crafted IDs (path traversal in IDs: `../../`)

```go
// Pattern to look for — unsanitized user input into Caddy config:
route := CaddyRoute{ID: userInput}  // dangerous if userInput contains ../
```

### socat Relay Config (`internal/socat/`)

socat relays involve port numbers and target addresses from user input:

- [x] Port numbers validated 1–65535 in `validateSocatRelay()` handler (`handlers/socat.go`) — added v0.8.0
- [x] Target host validated — shell metacharacters rejected in `validateSocatRelay()` — added v0.8.0
- [x] socat launched via `exec.Command(binary, arg1, arg2)` — **not** `sh -c`; no shell interpolation
- [ ] Relay list file (`relays.json`) written with `0644` — consider tightening to `0600`

```bash
# Check how socat is invoked in Go code
grep -n "exec\|Command\|socat" webui/internal/socat/*.go
```

### Backup & Restore (`internal/backup/`)

- [ ] Archive extraction (`tar`) does not allow path traversal (zip-slip: `../../etc/passwd` in archive entry names)
- [ ] Upload file size is bounded (no unbounded memory/disk exhaustion)
- [ ] Restore overwrites only expected paths (config dir), not arbitrary filesystem paths
- [ ] Backup download does not expose files outside `$TS_STATE_DIR`

### Configuration Loading (`internal/config/`)

- [ ] `webui.yaml` is read-only after startup (no hot-reload from untrusted source)
- [ ] YAML parsing uses `gopkg.in/yaml.v3` strict mode — no arbitrary Go type unmarshalling

---

## 5. Server-Side Request Forgery (SSRF)

The Web UI proxies requests to the Caddy Admin API and can be directed to arbitrary upstream URLs:

- [ ] Caddy proxy upstream URLs entered by users should be restricted to private/LAN ranges or validated against an allowlist
- [ ] The Caddy Admin API (`localhost:2019`) access is server-side only — the Web UI should not forward arbitrary user-controlled URLs to it
- [ ] Any "test connection" or "health check" features that make outbound requests must validate destination addresses

---

## 6. Privacy & Data Exposure

### Log Sanitization

- [ ] Auth tokens and passwords never appear in log output
- [ ] HTTP request bodies are truncated (check `MAX_LOG_BODY_SIZE` in `internal/logger/`)
- [ ] Tailscale auth keys (from `TS_AUTHKEY` env var) do not appear in logs
- [ ] `start.sh` does not log env vars that may contain secrets

### Persisted Data (`$TS_STATE_DIR` = `/var/lib/tailscale/`)

Files written by tailrelay:
- `.webui_token` — auth token (must be `0600`)
- `relays.json` — relay configuration (no secrets expected)
- `backups/` — tar.gz archives of full config + TLS certs (treat as sensitive)
- Tailscale state files — contain auth material (managed by Tailscale daemon)

- [ ] Backup archives do not include files outside the config dir (no `/etc/`, no env files)
- [ ] Backup download endpoint requires authentication (not publicly accessible)

### SSE Log Stream

The Web UI exposes a live log stream via Server-Sent Events:
- [x] SSE endpoint protected by `RequireAuth` middleware (`/api/logs/stream` in `server.go`)
- [x] `Access-Control-Allow-Origin: *` header removed from SSE response (v0.8.0)
- [ ] Log lines not yet stripped of ANSI escape sequences before streaming

---

## 7. Container Hardening

```bash
# Inspect the built image for common issues
docker inspect sudocarlos/tailrelay:latest | jq '.[0].Config'

# Check running process user (should not be root in production)
docker run --rm sudocarlos/tailrelay:latest id
```

- [ ] Container does not run as root (or justify why it must — Tailscale requires `NET_ADMIN`)
- [ ] Capabilities are minimal (`--cap-add NET_ADMIN` is required for Tailscale; others should be dropped)
- [ ] No secrets baked into image layers (`docker history` check)
- [ ] `ARG` values with sensitive data are not reachable at runtime (`ENV` vs `ARG` distinction)

```bash
# Check for secrets in image layers
docker history --no-trunc sudocarlos/tailrelay:latest | grep -i 'secret\|token\|key\|pass'
```

---

## 8. Frontend Security

```bash
# Audit npm dependencies for known vulnerabilities
cd webui/frontend && npm audit

# Fix automatically where safe
npm audit fix
```

- [x] No `npm audit` high/critical findings (0 vulnerabilities as of v0.8.0)
- [x] Auth uses `HttpOnly` session cookie — token not stored in `localStorage`
- [x] API requests use session cookie, not query-string tokens
- [x] `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Referrer-Policy` headers added (v0.8.0) in `securityHeaders` middleware (`web/server.go`)
- [ ] Content Security Policy (CSP) header not yet set
- [ ] Inline `<script>` usage not audited

---

## 9. Reporting Format

Structure findings as:

```
## Finding: <short title>

**Severity:** Critical | High | Medium | Low | Info
**Location:** `path/to/file.go:line_number`
**Category:** Auth | Injection | SSRF | Privacy | Dependency | Config

### Description
What the issue is and why it is a problem.

### Evidence
Code snippet or command output demonstrating the issue.

### Remediation
Specific fix recommendation with example code where applicable.
```

### Severity Definitions

| Severity | Criteria |
|----------|----------|
| Critical | Remote code execution, auth bypass, exposed secrets |
| High | Privilege escalation, SSRF, injection with impact |
| Medium | Information disclosure, missing validation, weak crypto |
| Low | Defense-in-depth gaps, minor misconfigurations |
| Info | Best practice recommendations, version upgrade suggestions |
