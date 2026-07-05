---
id: authentication
title: Authentication & Access
---

# Web UI Authentication & Access

## Authentication

1. **Tailscale Network Authentication** — devices on your Tailscale network
   are automatically authenticated. If the container is not connected, the
   Web UI shows a Tailscale login link and polls until the device is
   connected.
2. **Token Authentication** — a token is generated on first startup at
   `/var/lib/tailscale/.webui_token` for scripted access or CLI integration.

Every `/api/*` route except the public ones (`GET /api/info`,
`GET /api/auth/status`, `POST /api/auth/setup`, `POST /api/auth/login`)
requires authentication via **either**:

- a **Bearer token** — `Authorization: Bearer <token>` matching the static
  token written to `/var/lib/tailscale/.webui_token` on first start; or
- a **session cookie** — `tailrelay_session` (HttpOnly, SameSite=Strict,
  `Secure` when served over TLS, 24h expiry). The cookie is set by
  `POST /api/auth/setup`, `POST /api/auth/login`, and by
  `GET /api/tailscale/poll` once the node is connected.

On failure, `/api/*` paths return `401` with a JSON body
`{"error":"unauthorized"}`; non-API paths receive a `303` redirect to
`/login`.

See the [API Reference](/docs/api/) for the full authentication scheme details
and per-endpoint requirements.

## Access

The Web UI is accessible on port `8021`:

- **Secure/Remote** — `https://your-hostname.your-tailnet.ts.net:8021` (once
  connected and HTTPS is enabled)
- **Local** — `http://localhost:8021`

## Retrieving the Token

```bash
docker exec tailrelay cat /var/lib/tailscale/.webui_token
```
