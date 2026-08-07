---
name: serve-relay-management
description: tailscale serve and funnel relay management — HTTPS, TCP, and Funnel relay types, serve_relays.json format, reconciliation flow, API endpoints, migration from legacy caddy/socat configs, ErrTailscaleNotReady and ErrFunnelNotAllowed handling. Use when working with internal/serve/, handlers/serve.go, or /api/serve/* endpoints.
---

# Serve Relay Management

## Overview

`internal/serve/` implements relay orchestration backed by `tailscale serve`
and `tailscale funnel`. It replaces the former Caddy (HTTPS) and socat (TCP)
relay stacks. Relay state is persisted to `serve_relays.json` and reconciled
with `tailscale serve`/`tailscale funnel` on startup and after every mutating
operation.

## Relay Types

| Type     | Transport                        | Command                                    |
|----------|-----------------------------------|---------------------------------------------|
| `https`  | Web reverse proxy (tailnet-only)  | `tailscale serve --https <port> ...` (Tailscale) or `--http <port> ...` (custom control server) |
| `tcp`    | Raw TCP forward (tailnet-only)    | `tailscale serve tcp:<port> tcp://...`       |
| `funnel` | Public internet exposure          | `tailscale funnel --https=<port> ...` or `--tcp=<port> ...` |

`funnel` relays additionally set `FunnelTransport` (`"https"` or `"tcp"`,
defaults to `"https"`) to select which `tailscale funnel` flag is used.
Funnel is only permitted on ports `443`, `8443`, and `10000`
(`serve.FunnelPorts`, checked via `serve.IsFunnelPort`) — this is a hard
limitation of Tailscale Funnel, not a tailrelay choice.

Web relays retain the persisted `https` type for compatibility. When tailscaled
is actually authenticated against a custom control server, the manager uses
`tailscale serve --http` because self-hosted controllers (e.g. Headscale)
cannot provide Tailscale's HTTPS certificate provisioning and reject `--https`
serve requests. HTTPS list responses include `listener_scheme` so the UI can
render the correct access URL and running state.

## `serve_relays.json` Format

Default path: `/var/lib/tailscale/serve_relays.json` (configurable via
`paths.serve_relay_config` in `webui.yaml`).

```json
{
  "relays": [
    {
      "id": "https-443",
      "type": "https",
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
    },
    {
      "id": "funnel-10000",
      "type": "funnel",
      "funnel_transport": "tcp",
      "listen_port": 10000,
      "target_host": "192.168.1.30",
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
    Type        string `json:"type"`          // "https", "tcp", or "funnel"
    ListenPort  int    `json:"listen_port"`
    TargetHost  string `json:"target_host"`
    TargetPort  int    `json:"target_port"`
    TargetHTTPS bool   `json:"target_https"`
    Enabled     bool   `json:"enabled"`
    Autostart   bool   `json:"autostart"`
    // FunnelTransport selects "https" or "tcp" when Type is "funnel".
    FunnelTransport string `json:"funnel_transport,omitempty"`
}
```

## Manager (`webui/internal/serve/manager.go`)

### Public API

| Method | Description |
|--------|-------------|
| `NewManager(relayFile string) *Manager` | Create manager with relay config path |
| `NewManagerWithControlServerDetection(relayFile string, checker controlServerChecker, fallback bool) *Manager` | Production constructor: `WebListenerScheme` prefers a live check via `checker` (satisfied by `*tailscale.Client`), falling back to `fallback` if `checker` is nil or the live check errors |
| `ListRelays() ([]ServeRelay, error)` | Return all stored relays |
| `GetRelay(id string) (*ServeRelay, error)` | Get relay by ID |
| `UpsertRelay(relay ServeRelay) error` | Create/update relay and reconcile |
| `DeleteRelay(id string) error` | Remove relay and reconcile |
| `ToggleRelay(id string, enabled bool) error` | Enable/disable relay and reconcile |
| `Reconcile() error` | Reset serve config and reapply all enabled relays |
| `Status() (*ServeStatusJSON, error)` | Parse `tailscale serve status --json` |
| `IsFunnelPort(port int) bool` | Whether port is one of `FunnelPorts` (443/8443/10000) |
| `WebListenerScheme() string` | `"http"` or `"https"` for the next `https`-type relay reconcile — see below |

### `WebListenerScheme` and Custom Control Server Detection

`WebListenerScheme()` decides `--https` vs `--http` for `https`-type relays.
It is **not** driven solely by the persisted `Config.Tailscale.ControlServer`
setting (which can drift — e.g. a node authenticated against a self-hosted
Headscale instance outside the Web UI, via CLI login or restored state,
while the setting stays unsaved/empty). Instead:

1. If a `controlServerChecker` was supplied (production: `*tailscale.Client`,
   via `IsCustomControlServer()` → live `ControlURL` from
   `/localapi/v0/prefs`), that live result wins.
2. If no checker is configured, or the live check errors (e.g. tailscaled
   unreachable during startup), the manager falls back to the
   `customControlServer` flag seeded from the persisted config value at
   construction time (`SetCustomControlServer` updates this fallback after
   `Login`/`LoginWithKey`/`Connect`; see the tailscale skill's live-detection
   section for the full picture, including the matching frontend
   `hideFunnel` derived store).

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

### `ErrFunnelNotAllowed`

```go
var ErrFunnelNotAllowed = fmt.Errorf("funnel not allowed")
```

Returned when `tailscale funnel` refuses to apply a relay because the
tailnet policy file is missing the `funnel` node attribute for this device
(see [Funnel node attribute](https://tailscale.com/kb/1223/tailscale-funnel#node-attribute-required)).
Detected by pattern-matching the CLI error text in `isFunnelNotAllowed`.

**Callers use `errors.Is` via `writeServeResult`:**

```go
writeServeResult(w, manager.DeleteRelay(id), "Relay deleted successfully")
// nil → 200, ErrTailscaleNotReady → 202, ErrRelayNotFound → 404,
// ErrFunnelNotAllowed → 409, other → 500
```

### Reconciliation Flow

1. Load `serve_relays.json`
2. `tailscale serve reset` — clears all serve **and** funnel rules
3. For each enabled relay, run:
   - Web relay: `tailscale serve --bg --https <port> http://<host>:<port>`
     with Tailscale, or `--http` with a custom control server
   - TCP:   `tailscale serve --bg --tcp <port> tcp://<host>:<port>`
   - Funnel (https transport): `tailscale funnel --bg --https <port> http://<host>:<port>`
   - Funnel (tcp transport): `tailscale funnel --bg --tcp <port> tcp://<host>:<port>`
4. If step 2 returns a not-ready error → return `ErrTailscaleNotReady`
5. If a funnel relay fails to apply because Funnel isn't allowed for this
   device → return `ErrFunnelNotAllowed`

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
| `GET`  | `/api/serve/funnel/list`    | Yes | List funnel relays with `running` status |
| `GET`  | `/api/serve/funnel/get`     | Yes | Get single funnel relay (`?id=`) |
| `POST` | `/api/serve/funnel/create`  | Yes | Create funnel relay (JSON); port must be 443/8443/10000 |
| `POST` | `/api/serve/funnel/update`  | Yes | Update funnel relay (`id` required) |
| `POST` | `/api/serve/funnel/delete`  | Yes | Delete funnel relay (`?id=`) |
| `POST` | `/api/serve/funnel/toggle`  | Yes | Enable/disable funnel relay |
| `POST` | `/api/serve/reload`         | Yes | Trigger full reconcile |

### Response Status Codes

| Status | Meaning |
|--------|---------|
| `200 OK` | Operation applied to tailscale serve/funnel |
| `202 Accepted` | Config saved; Tailscale not yet ready — will apply on next reconcile |
| `400 Bad Request` | Missing/invalid fields, or funnel port outside 443/8443/10000 |
| `404 Not Found` | Relay ID not found (delete/toggle) |
| `409 Conflict` | Funnel not allowed — tailnet policy file lacks the `funnel` node attribute |
| `500 Internal Server Error` | Unexpected error |

### Legacy Shims

`/api/caddy/*` and `/api/socat/*` return **410 Gone** with a message
pointing to the new `/api/serve/https/*` and `/api/serve/tcp/*` routes.
These shims are registered before protected routes and do not require auth.

### `writeServeResult` Helper

All mutating handlers use the `writeServeResult(w, err, msg)` helper which
maps `nil` → 200, `ErrTailscaleNotReady` → 202, `ErrRelayNotFound` → 404,
`ErrFunnelNotAllowed` → 409, and other errors → 500.

## Migration from Legacy Configs

On first startup with a legacy config, the following migrations are applied
automatically by `config.MigrateLegacyRelaysToServe`:

| Source file | Target |
|-------------|--------|
| `relays.json` (socat) | `serve_relays.json` with `type: tcp` |
| `proxies.json` (caddy) | `serve_relays.json` with `type: https` |
| `RELAY_LIST` env var | `serve_relays.json` with `type: tcp` |

There is no legacy source for `funnel` relays — they are a new relay type.

## Running Status Detection

- TCP/HTTPS relay running state is determined by matching `relay.ListenPort`
  against the port keys in `tailscale serve status --json` → `.TCP`.
  - TCP relay is `running` if `TCP[port].HTTPS == false`
  - HTTPS relay is `running` if `TCP[port].HTTPS == true`
- Funnel relay running state is determined by matching `relay.ListenPort`
  against the port suffix of keys in `tailscale serve status --json` →
  `.AllowFunnel` (keyed `"host:port"`); see `funnelIsRunning` in
  `handlers/serve.go`.

**Limitation:** this matching is port-based and can be unreliable if ports
are reused or serve/funnel config is edited outside the UI.

## Hostname Display

`ServeRelay` has no persisted `Hostname`/`hostname` field. HTTPS and Funnel
list responses (`APIListHTTPS`, `APIListFunnel`) add a `hostname` field to
the JSON response computed live from `TSClient.GetStatusSummary().MagicDNSName`
on every request — it is never stored in `serve_relays.json`. This avoids a
stale hostname being displayed for relays created before a `tailscale logout`
+ re-`tailscale up`, which assigns a new auto-generated machine name.

## Testing

Go unit tests: `webui/internal/serve/manager_test.go`
- Funnel-specific: `TestManagerUpsertFunnelRelay`,
  `TestManagerUpsertFunnelRejectsInvalidPort`,
  `TestManagerUpsertFunnelRejectsInvalidTransport`,
  `TestFunnelRelaysExcludedFromTypedLists`

Integration tests covering serve/funnel relay behaviour:
- `TestTCPRelay.test_tcp_relay_create_and_list`
- `TestTCPRelay.test_tcp_relay_delete_removes_from_list`
- `TestFunnelRelay.test_funnel_relay_create_and_list`
- `TestFunnelRelay.test_funnel_relay_delete_removes_from_list`
- `TestFunnelRelay.test_funnel_relay_rejects_disallowed_port`
- `TestLegacyEndpointShims.test_caddy_endpoint_returns_410`
- `TestLegacyEndpointShims.test_socat_endpoint_returns_410`
