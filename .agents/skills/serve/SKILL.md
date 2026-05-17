---
name: serve-relay-management
description: tailscale serve relay management — HTTPS and TCP relay types, serve_relays.json format, reconciliation flow, API endpoints, migration from legacy caddy/socat configs, and ErrTailscaleNotReady handling. Use when working with internal/serve/, handlers/serve.go, or /api/serve/* endpoints.
reviewed_at: e413322
---

# Serve Relay Management

## Overview

tailrelay exposes local services to the Tailscale network via `tailscale serve`. The `internal/serve/` package owns relay state persistence and reconciliation. HTTP handlers in `internal/handlers/serve.go` provide the REST API consumed by the SPA.

## Architecture

```
webui/internal/
├── config/
│   ├── types.go          # ServeRelay, ServeRelayList structs
│   ├── config.go         # LoadServeRelays / SaveServeRelays helpers
│   └── migration.go      # MigrateFromEnvVar, MigrateLegacyRelaysToServe
├── serve/
│   ├── manager.go        # Manager — CRUD + Reconcile + ErrTailscaleNotReady
│   └── manager_test.go
└── handlers/
    ├── serve.go          # ServeHandler — /api/serve/https/* and /api/serve/tcp/*
    └── legacy.go         # 410 Gone shims for /api/caddy/* and /api/socat/*
```

## Relay Types

| Type | Description | tailscale serve command |
|------|-------------|------------------------|
| `https` | HTTP(S) reverse proxy over Tailscale HTTPS | `tailscale serve --https <port> http[s+insecure]://host:port` |
| `tcp` | Raw TCP forwarding | `tailscale serve --tcp <port> tcp://host:port` |

## `serve_relays.json` Schema

```json
{
  "relays": [
    {
      "id":           "https-443",
      "type":         "https",
      "hostname":     "myhost.example.ts.net",
      "listen_port":  443,
      "target_host":  "localhost",
      "target_port":  8080,
      "target_https": false,
      "enabled":      true,
      "autostart":    true
    },
    {
      "id":           "tcp-2222",
      "type":         "tcp",
      "listen_port":  2222,
      "target_host":  "192.168.1.10",
      "target_port":  22,
      "enabled":      true,
      "autostart":    true
    }
  ]
}
```

**Field reference:**

| Field | Type | Notes |
|-------|------|-------|
| `id` | string | Auto-generated as `"<type>-<port>"` if empty on create |
| `type` | string | `"https"` or `"tcp"` |
| `hostname` | string | HTTPS only; filled from Tailscale MagicDNS name if empty |
| `listen_port` | int | 1–65535; port Tailscale listens on |
| `target_host` | string | Upstream host (required) |
| `target_port` | int | Upstream port (required) |
| `target_https` | bool | HTTPS only; if true uses `https+insecure://` for upstream |
| `enabled` | bool | Whether the relay is currently active |
| `autostart` | bool | Reconciled at startup (used during migration; currently same as `enabled`) |

Default location: `/var/lib/tailscale/serve_relays.json` (configurable via `paths.serve_relay_config` in `webui.yaml`).

## Manager (`internal/serve/manager.go`)

### Key Methods

```go
manager := serve.NewManager(cfg.Paths.ServeRelayConfig)

manager.ListRelays()              // []ServeRelay, error
manager.GetRelay(id)             // *ServeRelay, error
manager.UpsertRelay(relay)       // error — creates or updates; calls Reconcile()
manager.DeleteRelay(id)          // error — removes relay; calls Reconcile()
manager.ToggleRelay(id, enabled) // error — flip enabled; calls Reconcile()
manager.Reconcile()              // error — reset + reapply all enabled relays
manager.Status()                 // *ServeStatusJSON, error — parse serve status --json
```

### Reconciliation Flow

1. Load `serve_relays.json`
2. Run `tailscale serve reset` — clears all active serve rules
3. For each relay where `enabled == true`, sorted by `listen_port` then `id`:
   - HTTPS: `tailscale serve --bg --https <port> http[s+insecure]://<host>:<port>`
   - TCP: `tailscale serve --bg --tcp <port> tcp://<host>:<port>`

Any mutation (`UpsertRelay`, `DeleteRelay`, `ToggleRelay`) persists state first, then calls `Reconcile()`.

### `ErrTailscaleNotReady`

```go
var ErrTailscaleNotReady = errors.New("tailscale not ready: reconcile deferred")
```

`Reconcile()` returns this sentinel when `tailscale serve reset` fails with a not-ready signal (e.g. `netMap is nil`, `not logged in`, `NeedsLogin`). Relay config is already persisted on disk.

**Call-site contract:**

- HTTP mutation handlers (`CreateTCP`, `DeleteHTTPS`, etc.) return **202 Accepted** with `{"status":"pending"}` instead of 500.
- `ReloadServe` returns 202 Accepted.
- `reconcileRelaysAsync` (post-login) logs and continues.
- `backup.Restore` logs and continues.
- `server.go` startup goroutine retries up to 15 times × 2s; distinguishes this sentinel from real errors in the log.

Use `errors.Is(err, serve.ErrTailscaleNotReady)` to check.

### Running Status Detection

`Status()` parses `tailscale serve status --json` and returns a `ServeStatusJSON`:

```go
type ServeStatusJSON struct {
    TCP map[string]struct {
        HTTPS      bool   `json:"HTTPS,omitempty"`
        TCPForward string `json:"TCPForward,omitempty"`
    } `json:"TCP"`
    Web map[string]struct {
        Handlers map[string]struct {
            Proxy string `json:"Proxy,omitempty"`
        } `json:"Handlers"`
    } `json:"Web"`
}
```

- TCP relays: match `relay.ListenPort` (as string) against `TCP` map keys; `HTTPS == false`
- HTTPS relays: match `relay.ListenPort` against `TCP` map keys; `HTTPS == true`

> **Known limitation:** Status matching is port-based. If two relays share the same port via out-of-band config edits, status detection may be incorrect. If a future `tailscale serve status` release exposes relay IDs, prefer matching on ID.

## REST API Endpoints

All routes require Bearer token or Tailscale IP authentication.

### TCP Relays

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/api/serve/tcp/list` | `APIListTCP` | List all TCP relays with running status |
| GET | `/api/serve/tcp/get?id=<id>` | `APIGetTCP` | Get single TCP relay |
| POST | `/api/serve/tcp/create` | `CreateTCP` | Create TCP relay (JSON body) |
| POST | `/api/serve/tcp/update` | `UpdateTCP` | Update TCP relay (JSON body with `id`) |
| POST/DELETE | `/api/serve/tcp/delete?id=<id>` | `DeleteTCP` | Delete TCP relay |
| POST | `/api/serve/tcp/toggle` | `ToggleTCP` | Enable/disable TCP relay (`{"id":"...","enabled":true}`) |

### HTTPS Relays

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/api/serve/https/list` | `APIListHTTPS` | List all HTTPS relays with running status |
| GET | `/api/serve/https/get?id=<id>` | `APIGetHTTPS` | Get single HTTPS relay |
| POST | `/api/serve/https/create` | `CreateHTTPS` | Create HTTPS relay (JSON or multipart form) |
| POST | `/api/serve/https/update` | `UpdateHTTPS` | Update HTTPS relay |
| POST/DELETE | `/api/serve/https/delete?id=<id>` | `DeleteHTTPS` | Delete HTTPS relay |
| POST | `/api/serve/https/toggle` | `ToggleHTTPS` | Enable/disable HTTPS relay |

### Other

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/serve/reload` | Force reconcile; returns 202 if Tailscale not ready |

### Legacy Shims (410 Gone)

`/api/caddy/*` and `/api/socat/*` are permanently removed. Any request returns:

```http
HTTP/1.1 410 Gone
Content-Type: application/json

{"error":"gone","message":"This endpoint has been removed. Use /api/serve/https/... instead.","migrate":"/api/serve/https/..."}
```

## Migration

Three migration paths run at startup (all idempotent — skipped if `serve_relays.json` already exists):

| Source | Function | Description |
|--------|----------|-------------|
| `RELAY_LIST` env var | `config.MigrateFromEnvVar` | Parses `listen:host:port` entries into TCP relays |
| `relays.json` (socat) | `config.MigrateLegacyRelaysToServe` | Converts socat relay records to `type: tcp` |
| `proxies.json` (caddy) | `config.MigrateLegacyRelaysToServe` | Converts Caddy proxy records to `type: https` |

## Testing

```bash
# Go unit tests
cd webui && go test ./internal/serve/...
cd webui && go test ./internal/handlers/...

# Integration tests
pytest tests/integration/test_integration.py::TestTCPRelay -v
pytest tests/integration/test_integration.py::TestLegacyEndpoints -v
```

## Common Pitfalls

1. **Tailscale must be connected** to apply serve rules. Config is always persisted; reconcile is deferred if Tailscale is not ready. Check logs for `"deferring reconcile"` messages.
2. **`tailscale serve reset` clears ALL rules** including any manually set via the CLI. Always manage serve rules through the Web UI or the `serve_relays.json` file.
3. **HTTPS requires MagicDNS + HTTPS certs** enabled in the Tailscale admin console. Without them, `tailscale serve --https` will fail.
4. **Port conflicts** — two relays cannot share the same `listen_port`. The second `UpsertRelay` call will be rejected by `tailscale serve` during reconcile.
