---
id: troubleshooting
title: Troubleshooting
---

# Troubleshooting

## Web UI Not Accessible

Check container status:

```bash
docker ps | grep tailrelay
```

Verify port mapping:

```bash
docker port tailrelay
```

Check logs:

```bash
docker logs tailrelay | grep -i webui
```

Verify listening port:

```bash
docker exec tailrelay netstat -tulnp | grep 8021
```

## Cannot Log In

Retrieve token:

```bash
docker exec tailrelay cat /var/lib/tailscale/.webui_token
```

Ensure you're accessing from the Tailscale network or clear your browser
cache.

## Relay Issues (`tailscale serve`)

Check current serve status:

```bash
docker exec tailrelay tailscale serve status
```

Force reconcile all enabled HTTPS, TCP, and Funnel relays from the saved UI
configuration:

```bash
curl -X POST http://localhost:8021/api/serve/reload
```

This is the only reconcile endpoint — it reconciles the live
`tailscale serve` configuration against every enabled relay in a single call.
See the [API Reference](/docs/api/) (`POST /api/serve/reload`, tag **Serve**) for
the full request/response shape.

Test target connectivity:

```bash
docker exec tailrelay nc -zv target-host target-port
```
