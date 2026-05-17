---
name: serve-relay-management
description: tailscale serve relay management — HTTPS and TCP relay types, serve_relays.json format, reconciliation flow, API endpoints, migration from legacy caddy/socat configs, and ErrTailscaleNotReady handling. Use when working with internal/serve/, handlers/serve.go, or /api/serve/* endpoints.
reviewed_at: f7df402
---

# Serve Relay Management

## Overview

`internal/serve/` implements relay orchestration backed by `tailscale serve`.
It replaces the former Caddy (HTTPS) and socat (TCP) relay stacks.
Relay state is persisted to `serve_relays.json` and reconciled with
`tailscale serve` on startup and after every mutating operation.

## Relay Types

| Type    | Transport        | `tailscale serve` command         |
|---------|------------------|-----------------------------------|
| `https` | HTTPS termination | `tailscale serve https:<port> ...` |
| `tcp`   | Raw TCP forward   | `tailscale serve tcp:<port> tcp://...` |

## `serve_relays.json` Format

Default path: `/var/lib/tailscale/serve_relays.json` (configurable via
`paths.serve_relay_config` in `webui.yaml`).

```json
{
  "relays": [
    {
      "id": "https-443",
      "type": "https",
      "hostname": "myhost.example.ts.net",
      "listen_port": 443,
      "target_host": "192.168.1.10",
      "target_port": 8080,
      "target_https": false,
      "enabled": true,
      "autostart": true
    },
    {
      "id": "tcp-2222",
      "type": "tcp",
      "listen_port": 2222,
      "target_host": "192.168.1.20",
      "target_port": 22,
      "enabled": true,
      "autostart": true
    }
  ]
}
```

## Key Types (`webui/internal/config/types.go`)

```go
type ServeRelay struct {
    ID          string `json:"id"`
    Type        string `json:"type"`          // "https" or "tcp"
    Hostname    string `json:"hostname,omitempty"`
    ListenPort  int    `json:"listen_port"`
    TargetHost  string `json:"target_host"`
    TargetPort  int    `json:"target_port"`
    TargetHTTPS bool   `json:"target_https"`
    Enabled     bool   `json:"enabled"`
    Autostart   bool   `json:"autostart"`
}
```

## Manager (`webui/internal/serve/manager.go`)

### Public API

| Method | Description |
|--------|-------------|
| `NewManager(relayFile string) *Manager` | Create manager with relay config path |
| `ListRelays() ([]ServeRelay, error)` | Return all stored relays |
| `GetRelay(id string) (*ServeRelay, error)` | Get relay by ID |
| `UpsertRelay(relay ServeRelay) error` | Create/update relay and reconcile |
| `DeleteRelay(id string) error` | Remove relay and reconcile |
| `ToggleRelay(id string, enabled bool) error` | Enable/disable relay and reconcile |
| `Reconcile() error` | Reset serve config and reapply all enabled relays |
| `Status() (*ServeStatusJSON, error)` | Parse `tailscale serve status --json` |

### `ErrTailscaleNotReady`

```go
var ErrTailscaleNotReady = fmt.Errorf("tailscale not ready")
```

Returned by `Reconcile()` (and transitively by `UpsertRelay`, `DeleteRelay`,
`ToggleRelay`) when Tailscale is not yet authenticated or connected.

### `ErrRelayNotFound`

```go
var ErrRelayNotFound = fmt.Errorf("relay not found")
```

Returned by `DeleteRelay` and `ToggleRelay` when no relay with the given ID
exists in `serve_relays.json`. Use `errors.Is` to distinguish from
`ErrTailscaleNotReady` or internal errors.

**Callers use `errors.Is` via `writeServeResult`:**

```go
writeServeResult(w, manager.DeleteRelay(id), "Relay deleted successfully")
// nil → 200, ErrTailscaleNotReady → 202, ErrRelayNotFound → 404, other → 500
```

### Reconciliation Flow

1. Load `serve_relays.json`
2. `tailscale serve reset` — clears all serve rules
3. For each enabled relay, run:
   - HTTPS: `tailscale serve https:<port> http://<host>:<port>`
   - TCP:   `tailscale serve tcp:<port> tcp://<host>:<port>`
4. If step 2 returns a not-ready error → return `ErrTailscaleNotReady`

Startup reconcile is driven by `server.go` in a background goroutine that
polls `IsTailscaleReady()` every 2 s (up to 15 attempts).

## HTTP Handlers (`webui/internal/handlers/serve.go`)

### Endpoint Map

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET`  | `/api/serve/https/list`     | Yes | List HTTPS relays with `running` status |
| `GET`  | `/api/serve/https/get`      | Yes | Get single HTTPS relay (`?id=`) |
| `POST` | `/api/serve/https/create`   | Yes | Create HTTPS relay (JSON or multipart) |
| `POST` | `/api/serve/https/update`   | Yes | Update HTTPS relay (`id` required) |
| `POST` | `/api/serve/https/delete`   | Yes | Delete HTTPS relay (`?id=`) |
| `POST` | `/api/serve/https/toggle`   | Yes | Enable/disable HTTPS relay |
| `GET`  | `/api/serve/tcp/list`       | Yes | List TCP relays with `running` status |
| `GET`  | `/api/serve/tcp/get`        | Yes | Get single TCP relay (`?id=`) |
| `POST` | `/api/serve/tcp/create`     | Yes | Create TCP relay (JSON) |
| `POST` | `/api/serve/tcp/update`     | Yes | Update TCP relay (`id` required) |
| `POST` | `/api/serve/tcp/delete`     | Yes | Delete TCP relay (`?id=`) |
| `POST` | `/api/serve/tcp/toggle`     | Yes | Enable/disable TCP relay |
| `POST` | `/api/serve/reload`         | Yes | Trigger full reconcile |

### Response Status Codes

| Status | Meaning |
|--------|---------|
| `200 OK` | Operation applied to tailscale serve |
| `202 Accepted` | Config saved; Tailscale not yet ready — will apply on next reconcile |
| `400 Bad Request` | Missing/invalid fields |
| `404 Not Found` | Relay ID not found (delete/toggle) |
| `500 Internal Server Error` | Unexpected error |

### Legacy Shims

`/api/caddy/*` and `/api/socat/*` return **410 Gone** with a message
pointing to the new `/api/serve/https/*` and `/api/serve/tcp/*` routes.
These shims are registered before protected routes and do not require auth.

### `writeServeResult` Helper

All mutating handlers use the `writeServeResult(w, err, msg)` helper which
maps `nil` → 200, `ErrTailscaleNotReady` → 202, `ErrRelayNotFound` → 404,
and other errors → 500.

## Migration from Legacy Configs

On first startup with a legacy config, the following migrations are applied
automatically by `config.MigrateLegacyRelaysToServe`:

| Source file | Target |
|-------------|--------|
| `relays.json` (socat) | `serve_relays.json` with `type: tcp` |
| `proxies.json` (caddy) | `serve_relays.json` with `type: https` |
| `RELAY_LIST` env var | `serve_relays.json` with `type: tcp` |

## Running Status Detection

TCP/HTTPS relay running state is determined by matching `relay.ListenPort`
against the port keys in `tailscale serve status --json` → `.TCP`.
- TCP relay is `running` if `TCP[port].HTTPS == false`
- HTTPS relay is `running` if `TCP[port].HTTPS == true`

**Limitation:** this matching is port-based and can be unreliable if ports
are reused or serve config is edited outside the UI.

## Testing

Go unit tests: `webui/internal/serve/manager_test.go`

Integration tests covering serve relay behaviour:
- `TestServeRelays.test_tcp_relay_create_and_list`
- `TestServeRelays.test_tcp_relay_delete_removes_from_list`
- `TestLegacyEndpointShims.test_caddy_endpoint_returns_410`
- `TestLegacyEndpointShims.test_socat_endpoint_returns_410`
