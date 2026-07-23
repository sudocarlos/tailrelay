---
name: tailscale-management
description: Tailscale VPN daemon management, CLI integration, authentication, and networking for the tailrelay container. Use when working with Tailscale configuration, login flows, device authentication, MagicDNS, HTTPS certificates, or network connectivity issues.
reviewed_at: 5100f3d
---

# Tailscale Management

## Overview

tailrelay runs Tailscale in **userspace networking mode** inside a Docker container. The daemon (`tailscaled`) is started by `start.sh` and managed throughout the container lifecycle. The Go Web UI wraps the Tailscale CLI for status/login/networking operations.

## Architecture

```
start.sh (entrypoint)
  └── tailscaled --tun=userspace-networking
        ├── SOCKS5 proxy on localhost:1055
        ├── Health check on :9002/healthz
        └── Metrics on :9002/metrics

webui/internal/tailscale/
  ├── client.go      # CLI wrapper (tailscale status, tailscale up, etc.)
  ├── status.go      # Status parsing structs
  ├── networking.go  # Prefs/NetworkingSummary — networking preferences via `tailscale set`
  └── cache.go       # StatusCache — background poller (15s interval)
```

## Daemon Startup

In `start.sh`, tailscaled starts with:

```bash
tailscaled --state="$TAILSCALED_STATE" \
  --socket="$TAILSCALED_SOCKET" \
  --tun=userspace-networking \
  --socks5-server=localhost:1055 \
  > /var/log/tailscaled.log 2>&1 &
```

Key flags:
- `--tun=userspace-networking` — No `NET_ADMIN` capability or `/dev/net/tun` required
- `--socks5-server=localhost:1055` — SOCKS5 proxy for outbound connections
- State persisted at `$TS_STATE_DIR/tailscaled.state`

## Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `TS_HOSTNAME` | *(required)* | Tailscale machine name |
| `TS_STATE_DIR` | `/var/lib/tailscale/` | State directory (persisted via Docker volume) |
| `TS_EXTRA_FLAGS` | *(empty)* | Additional flags passed to `tailscale up` |
| `TS_AUTH_ONCE` | `true` | Only authenticate once (don't re-auth on restart) |
| `TS_ENABLE_METRICS` | `true` | Expose metrics at `:9002/metrics` |
| `TS_ENABLE_HEALTH_CHECK` | `true` | Expose health at `:9002/healthz` |

## Web UI Integration

The Go package at `webui/internal/tailscale/` wraps CLI commands:

- **Status**: `tailscale status --json` → parsed into Go structs
- **Login**: LocalAPI `login-interactive`, or the CLI `tailscale login`/`up --authkey=` when a custom control server is configured → returns auth URL
- **Device list**: Extracted from status JSON
- **Network auth**: Requests from `100.x.y.z` IPs are auto-authenticated

### StatusCache

`cache.go` provides a `StatusCache` that polls `IsConnected` every 15 seconds in the background. This avoids hammering the Tailscale CLI on every request.

```go
cache := tailscale.NewStatusCache(client)
cache.Start(ctx)        // launches background goroutine
if cache.IsReady() {    // non-blocking, no I/O
    // Tailscale is connected
}
```

Use `StatusCache.IsReady()` wherever you need to gate on Tailscale connectivity (e.g. TLS cert provisioning, auth middleware).

### Authentication Flow

1. User visits Web UI → checks if request is from Tailscale IP
2. If not on Tailscale network → shows login page with Tailscale auth link
3. Login page polls `/api/tailscale/status` until device is connected
4. Once connected → session authenticated automatically

### Custom Control Server (Headscale) (`controlserver.go`)

`Config.Tailscale.ControlServer` (persisted in `webui.yaml`) lets the Web UI
authenticate against a self-hosted [Headscale](https://headscale.net)
instance instead of Tailscale's default control plane, set via the Control
Server field on the Tailscale page's connection status card.

- `tailscale.ValidateControlServerURL()` requires an `http`/`https` scheme
  and non-empty host; an empty string is always valid and means "use
  Tailscale's default control plane".
- `Client.Login(controlServer string)`: when `controlServer` is empty, this
  behaves as before (LocalAPI `login-interactive`). When set, `--login-server`
  isn't accepted by the LocalAPI endpoint or by `tailscale set` — only by the
  `login`/`up` CLI subcommands — so it instead runs
  `tailscale login --login-server=<url>` via the CLI in the background, then
  polls `/localapi/v0/status` for `AuthURL` exactly like the LocalAPI path.
- `Client.LoginWithAuthKey(key, controlServer string)`: appends
  `--login-server=<url>` to the existing `tailscale up --authkey=<key>` call.
- `Client.UpWithHostname(hostname, controlServer string)`: runs `tailscale up
  --hostname=<name> --reset`. `--reset` resets any *unspecified* flag
  (including `ControlURL`) to Tailscale's default control plane, so
  `--login-server=<url>` is appended whenever `controlServer` is set —
  otherwise every hostname change would silently detach the node from its
  Headscale server.
- `handlers.TailscaleHandler.Login`/`LoginWithKey`/`ChangeHostname` read the
  persisted control server (guarded by a `sync.Mutex` on the handler, since
  `*config.Config` has no locking of its own) and pass it through
  automatically — the frontend doesn't need to resend it with every request.
- `GET /api/tailscale/control-server` / `POST /api/tailscale/control-server/update`
  read and persist the setting via `config.Save`.
- Changing the control server has no effect on a device that's already
  registered until it's logged out and re-authenticated — Tailscale binds a
  node identity to whichever control server it first authenticated with.

### Networking Preferences (`networking.go`)

Unlike `tailscale up` (which requires re-specifying the complete set of
desired flags on every call, or `--reset`), `tailscale set` only changes the
flags explicitly passed — this is what the Web UI's Networking section on
the Tailscale page uses to toggle exit-node advertisement, subnet routes,
accept-routes, exit-node selection, and SSH without disturbing other
preferences (like the hostname set elsewhere via `UpWithHostname`).

- `Client.GetPrefs()` reads `/localapi/v0/prefs` (tailscaled's full `ipn.Prefs`,
  trimmed to the fields this app cares about: `RouteAll`, `ExitNodeIP`,
  `ExitNodeAllowLANAccess`, `RunSSH`, `AdvertiseRoutes`).
- `Client.GetNetworkingSummary()` derives a simplified `NetworkingSummary`.
  Tailscale has **no separate preference for exit-node advertisement** — it
  is implemented as the pair of default routes `0.0.0.0/0` and `::/0` inside
  `AdvertiseRoutes`. `summarizeNetworking()` detects that pair to set
  `AdvertiseExitNode` and excludes those two CIDRs from the `AdvertiseRoutes`
  field returned to the frontend (which only shows custom subnet routes).
- `Client.SetNetworking(opts NetworkingOptions)` builds one `tailscale set`
  invocation from whichever `NetworkingOptions` pointer fields are non-nil.
  The corresponding HTTP handler (`handlers/tailscale.go`'s
  `APINetworking`/`UpdateNetworking`) validates `advertise_routes` entries
  via `net/netip.ParsePrefix`, rejecting host-bit-set CIDRs and rejecting
  `0.0.0.0/0`/`::/0` directly (those must go through `advertise_exit_node`
  instead, keeping a single source of truth per UI control).

See the serve skill (`.agents/skills/serve/SKILL.md`) for `tailscale serve`/
`funnel` relay management, which is a distinct concern from these node-level
networking preferences.

### Machine Name

`Client.UpWithHostname` uses `tailscale set --hostname=<name>`. Unlike
`tailscale up --reset`, `set` changes only the machine name and preserves the
active control server and all other node preferences.

## HTTPS Certificates

Tailscale provides automatic TLS certificates for `*.ts.net` domains via `tailscale cert`:
- Must be enabled in [Tailscale Admin Console](https://login.tailscale.com/admin/dns) → HTTPS Certificates
- MagicDNS must be enabled (default for tailnets created after Oct 2022)
- Used by `tailscale serve` for HTTPS relay termination

## Troubleshooting

### Daemon won't start
```bash
docker logs tailrelay | grep tailscaled
cat /var/log/tailscaled.log
```

### Device not showing in Tailnet
```bash
docker exec tailrelay tailscale status
docker exec tailrelay tailscale up --hostname=$TS_HOSTNAME
```

### Health/metrics endpoints not responding
```bash
curl http://<host>:9002/healthz
curl http://<host>:9002/metrics
```

### State persistence
- Volume mount `/var/lib/tailscale` to persist login state across restarts
- Start9 removes files on reboot — back up `/home/start9/tailscale`
