---
name: caddy-proxy-management
description: Caddy reverse proxy management via the Admin API — CRUD operations, route configuration, TLS, and troubleshooting. Use when working with HTTP/HTTPS proxy configuration, the Caddy Admin API, reverse proxy handlers, or proxy-related Go code in internal/caddy/.
reviewed_at: b7ce114
---

# Caddy Proxy Management

## Overview

tailrelay uses **Caddy's Admin API** (not Caddyfile) for zero-downtime reverse proxy management. All proxy operations are atomic HTTP calls to `localhost:2019`.

## Architecture

```
Web UI Handler (internal/handlers/caddy.go)
  └── Manager (internal/caddy/manager.go)
        ├── ProxyManager (internal/caddy/proxy_manager.go)
        │     └── APIClient (internal/caddy/api_client.go)
        │           └── Caddy Admin API (localhost:2019)
        └── MetricsStore (internal/caddy/metrics_store.go)
              └── metrics_parser.go (Prometheus text parser)
```

### Key Files

| File | Purpose |
|------|---------|
| `internal/caddy/api_client.go` | Low-level HTTP client (GET, POST, PATCH, DELETE) |
| `internal/caddy/api_types.go` | Type-safe Caddy JSON config structs |
| `internal/caddy/proxy_manager.go` | High-level CRUD + @id tag management |
| `internal/caddy/manager.go` | Simplified interface for handlers + metrics poller wiring |
| `internal/caddy/metrics_store.go` | `MetricsStore`: timestamped snapshot ring buffer, disk persistence, `Query(window)` |
| `internal/caddy/metrics_parser.go` | Prometheus text-format parser; keys on Caddy `server` label |
| `internal/caddy/caddyfile.go` | Legacy Caddyfile support (compatibility only) |
| `internal/caddy/server_map.go` | Server mapping utilities |

## Proxy CRUD Operations

```go
manager := caddy.NewManager("http://localhost:2019", "tailrelay")

// Initialize server (one-time)
manager.InitializeServer([]string{":80", ":443"})

// Add proxy
proxy, err := manager.AddProxy(config.CaddyProxy{
    ID: "btcpay-proxy", Hostname: "myserver.tailnet.ts.net",
    Port: 21002, Target: "btcpayserver.embassy:80", Enabled: true,
})

// List / Get / Update / Delete / Toggle
proxies, _ := manager.ListProxies()
proxy, _   := manager.GetProxy("btcpay-proxy")
manager.UpdateProxy(proxy)
manager.DeleteProxy("btcpay-proxy")
manager.ToggleProxy("btcpay-proxy", false) // captures pause baseline automatically

// Status
running, _ := manager.GetStatus()
upstreams, _ := manager.GetUpstreams()
```

## Metrics Poller

`Manager` owns a `*MetricsStore` that is started during server startup and flushed on graceful shutdown.

```go
// In server.go Start():
manager.StartMetricsPoller(ctx, cfg.Paths.MetricsHistoryFile)

// In server.go shutdown:
manager.FlushMetrics()

// In handler:
data, err := manager.GetMetrics(window)   // window == 0 → all-time; > 0 → delta
manager.ResetMetrics()                    // clears all snapshots and baselines
```

### MetricsStore API

```go
store := caddy.NewMetricsStore("/var/lib/tailscale/metrics_history.json")
store.Start(ctx, fetchFn)          // background poller; flushes on ctx cancel
store.AddSnapshot(snap)            // append raw snapshot; prunes >31 days
store.RecordPause(label, hm)       // accumulate baseline before proxy is disabled
store.MarkPaused(label)            // mark proxy as explicitly disabled
store.Query(window)                // returns *MetricsData with baselines applied
store.Flush()                      // write to disk immediately
store.ResetMetrics()               // clear all state + overwrite file
```

### Counter-Reset Handling

Caddy resets **all** Prometheus counters whenever the HTTP app config changes (e.g. adding or removing any server). `snapshotMetrics` detects this in two cases:

1. **Pause** (server removed): a surviving server's raw counter drops below its previous snapshot value.
2. **Resume** (new server added): a new server key appears in the snapshot that was absent from the previous one.

When a reset is detected, baselines are saved for every server still present in the new snapshot. Servers that disappeared were already captured by `recordProxyPauseBaseline` (called from `ToggleProxy`) and are skipped to avoid double-counting.

### Query Delta Math

```
delta(window) = (raw_newest + baseline_newest) - (raw_oldest_in_window + baseline_oldest_in_window)
```

- Matching between newest and oldest snapshots is done by proxy **label** (`:port → target`), not by Caddy server name, so a proxy that changed server names after a pause/resume is still correctly matched.
- `nonNegative()` clamps any negative delta to 0 to handle Caddy restarts.
- Paused proxies (no active Caddy server) are included in results with `Paused: true` and counters synthesised from their baseline.

### Metrics Parser

`ParseMetrics` keys accumulators on the Caddy `server` Prometheus label (e.g. `srv0`) rather than the `host` label. This allows multiple proxies sharing the same Tailscale FQDN to be tracked as independent series. Falls back to `host` when the `server` label is absent.

## HTTPS Target Transport

`buildRoute` emits a TLS transport block based on `CaddyProxy` fields:

| `proxy.TLS` | `proxy.TLSCertFile` | Caddy transport |
|-------------|---------------------|-----------------|
| `false` | `""` | none (plain HTTP upstream) |
| `true` | `""` | `tls:{insecure_skip_verify:true}` |
| `false` | `"/path/ca.pem"` | `tls:{ca:{provider:"file",pem_files:[...]}}` |

When `proxy.HostHeader` is non-empty, `buildRoute` sets the upstream `Host` header to that value instead of the default `{http.reverse_proxy.upstream.hostport}` Caddy placeholder. This is exposed as the optional `host_header` JSON field on `CaddyProxy`.

`routeToProxyWithListen` (read path) restores these fields from the Caddy config:
- `insecure_skip_verify == true` → `proxy.TLS = true`
- `ca.pem_files` present → `proxy.TLSCertFile` set, `proxy.TLS` stays false
- `Host` header set to a static value → `proxy.HostHeader` set

## @id Tag Convention

Every proxy route gets an `@id` field for direct API access:

```json
{ "@id": "btcpay-proxy", "match": [...], "handle": [...] }
```

Access via: `GET/PATCH/DELETE /id/btcpay-proxy`

## Admin API Endpoints Used

| Method | Endpoint | Purpose |
|--------|----------|---------|
| `POST` | `/config/<path>` | Add/append config |
| `GET` | `/config/<path>` | Retrieve config |
| `PATCH` | `/config/<path>` | Replace config |
| `DELETE` | `/config/<path>` | Remove config |
| `GET` | `/id/<id>` | Get by @id tag |
| `PATCH` | `/id/<id>` | Update by @id tag |
| `DELETE` | `/id/<id>` | Remove by @id tag |
| `GET` | `/reverse_proxy/upstreams` | Upstream health status |
| `GET` | `/metrics` | Prometheus text-format metrics (parsed by `metrics_parser.go`) |

## Caddy Startup

In `start.sh`:
```bash
caddy start --config /etc/caddy/Caddyfile
```
- Admin API defaults to `localhost:2019`
- Caddy starts **before** the Web UI so the API is ready for proxy initialization
- A 1-second sleep ensures API readiness

## Legacy Compatibility

- `proxies.json` migration has been **removed**
- If a legacy `proxies.json` is detected, a one-time warning is logged
- Proxies must be recreated via the Web UI or API
- See this SKILL.md for current integration patterns

## Testing

```bash
# Unit tests
cd webui && go test ./internal/caddy/...

# Manual API checks
curl http://localhost:2019/config/ | jq
curl http://localhost:2019/config/apps/http/servers/tailrelay/routes | jq
curl http://localhost:2019/reverse_proxy/upstreams | jq
curl http://localhost:2019/metrics | head -50

# Metrics endpoint
curl http://localhost:8021/api/caddy/metrics | jq
curl "http://localhost:8021/api/caddy/metrics?window=1h" | jq
```

## Troubleshooting

### Caddy API not accessible
```bash
curl http://localhost:2019/config/
docker logs tailrelay | grep -i caddy
```

### Proxy added but not routing
```bash
curl "http://localhost:2019/id/<proxy-id>" | jq
curl "http://localhost:2019/reverse_proxy/upstreams" | jq
```

### Metrics show 0 after proxy was paused
This is expected during the poll interval gap. `MetricsStore` captures baselines at pause time via `recordProxyPauseBaseline`. If counters still show 0 after the next poll (15 s), check logs for `metrics_store: recorded pause baseline`.

### Performance reference
| Operation | Latency |
|-----------|---------|
| Add/Update/Delete proxy | 10–50ms |
| List proxies | 5–20ms |
| Metrics poll (background) | Every 15 s |
| Metrics flush to disk | Every 5 min + on shutdown |

## Best Practices

1. **Always use @id tags** for proxy identification
2. **Check status** before operations (`manager.GetStatus()`)
3. **Never edit Caddyfile manually** — let the API manage everything
4. **Use `compose-test.yml`** for testing configuration changes
5. **Admin API on localhost only** — never expose port 2019 externally
6. **Baselines are applied at query time** — snapshots store raw Caddy values only; never bake baselines into stored values or double-accumulation will occur

## Further Reading

- [Caddy Admin API docs](https://caddyserver.com/docs/api)
- [Caddy JSON structure](https://caddyserver.com/docs/json/)
