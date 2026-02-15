---
name: caddy-proxy-management
description: Caddy reverse proxy management via the Admin API — CRUD operations, route configuration, TLS, and troubleshooting. Use when working with HTTP/HTTPS proxy configuration, the Caddy Admin API, reverse proxy handlers, or proxy-related Go code in internal/caddy/.
---

# Caddy Proxy Management

## Overview

tailrelay uses **Caddy's Admin API** (not Caddyfile) for zero-downtime reverse proxy management. All proxy operations are atomic HTTP calls to `localhost:2019`.

## Architecture

```
Web UI Handler (internal/handlers/caddy.go)
  └── Manager (internal/caddy/manager.go)
        └── ProxyManager (internal/caddy/proxy_manager.go)
              └── APIClient (internal/caddy/api_client.go)
                    └── Caddy Admin API (localhost:2019)
```

### Key Files

| File | Purpose |
|------|---------|
| `internal/caddy/api_client.go` | Low-level HTTP client (GET, POST, PATCH, DELETE) |
| `internal/caddy/api_types.go` | Type-safe Caddy JSON config structs |
| `internal/caddy/proxy_manager.go` | High-level CRUD + @id tag management |
| `internal/caddy/manager.go` | Simplified interface for handlers |
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
manager.ToggleProxy("btcpay-proxy", false)

// Status
running, _ := manager.GetStatus()
upstreams, _ := manager.GetUpstreams()
```

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
- See `webui/MIGRATION_SUMMARY.md` for historical context

## Testing

```bash
# Unit tests
cd webui && go test ./internal/caddy/...

# Manual API checks
curl http://localhost:2019/config/ | jq
curl http://localhost:2019/config/apps/http/servers/tailrelay/routes | jq
curl http://localhost:2019/reverse_proxy/upstreams | jq

# Add test proxy via Web UI API
curl -X POST http://localhost:8021/api/caddy/proxies \
  -H "Content-Type: application/json" \
  -d '{"id":"test","hostname":"test.example.ts.net","port":8080,"target":"localhost:9000","enabled":true}'
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

### Performance reference
| Operation | Latency |
|-----------|---------|
| Add/Update/Delete proxy | 10–50ms |
| List proxies | 5–20ms |

## Best Practices

1. **Always use @id tags** for proxy identification
2. **Check status** before operations (`manager.GetStatus()`)
3. **Never edit Caddyfile manually** — let the API manage everything
4. **Use `compose-test.yml`** for testing configuration changes
5. **Admin API on localhost only** — never expose port 2019 externally

## Further Reading

- [webui/CADDY_API_GUIDE.md](../../webui/CADDY_API_GUIDE.md) — comprehensive API integration guide
- [Caddy Admin API docs](https://caddyserver.com/docs/api)
- [Caddy JSON structure](https://caddyserver.com/docs/json/)
