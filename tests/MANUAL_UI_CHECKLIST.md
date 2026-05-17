# Manual UI Test Checklist

Covers every area exercised by the integration tests and all routes exposed by
the server. Work through each section top-to-bottom against a running container.

---

## 1. Startup & Process Health

- [ ] Container starts without errors visible in `docker logs`
- [ ] Web UI loads at `http://localhost:8021` (or your host:port)
- [ ] Browser console shows no JS errors on initial load
- [ ] `tailscaled` health endpoint responds: `curl http://localhost:9002/healthz`
- [ ] Prometheus metrics endpoint responds: `curl http://localhost:9002/metrics` and contains `# HELP` lines

---

## 2. Authentication

- [ ] Unauthenticated visit to `/` redirects to the login page
- [ ] `GET /api/auth/status` returns JSON with a `needsSetup` key without authentication
- [ ] `GET /api/info` returns version/commit JSON without authentication
- [ ] Login with a wrong password shows an error and denies access
- [ ] Login with the correct token/password grants access and sets a session cookie
- [ ] Logout (`/logout`) clears the session and redirects back to login
- [ ] `POST /api/auth/logout` (authenticated) clears the session
- [ ] `POST /api/auth/change-password` (authenticated) accepts a new password and the new password works immediately after
- [ ] After logout, all protected `/api/*` routes return 401/redirect, not 200

---

## 3. Dashboard / Status

- [ ] Dashboard page (`/`) loads after login and shows Tailscale connection status
- [ ] `GET /api/status` returns a JSON object with system overview data
- [ ] `GET /api/tailscale/status` returns JSON with Tailscale connection info
- [ ] `GET /api/tailscale/peers` returns a JSON array (empty or populated)
- [ ] `GET /api/targets` returns a JSON array of available internal targets

---

## 4. Tailscale Controls

- [ ] Connect button (`POST /api/tailscale/connect`) triggers connection attempt; status updates in UI
- [ ] Disconnect button (`POST /api/tailscale/disconnect`) disconnects; status updates in UI
- [ ] Login with auth key (`POST /api/tailscale/login-with-key`) accepts a key and attempts authentication
- [ ] Login interactive (`POST /api/tailscale/login`) returns an auth URL for the browser flow
- [ ] Change hostname (`POST /api/tailscale/hostname`) updates the machine name; reflected in status
- [ ] Poll endpoint (`GET /api/tailscale/poll`) responds with current status (used for long-poll refresh)

---

## 5. TCP Relays (`/api/serve/tcp/*`)

- [ ] `GET /api/serve/tcp/list` returns a JSON array (empty on a fresh instance)
- [ ] Create a TCP relay via the UI form (id, listen_port, target_host, target_port); relay appears in the list immediately
- [ ] Created relay shows `running: false` when Tailscale is not connected, `running: true` when connected and active
- [ ] `GET /api/serve/tcp/get?id=<id>` returns the correct relay object
- [ ] Edit the relay (change target_port); saved values are reflected in the list without page reload
- [ ] Toggle relay **disabled** via the toggle control; relay shows as disabled in the list
- [ ] Toggle relay back **enabled**; relay shows as enabled and `running` reflects actual serve state
- [ ] Delete the relay; it disappears from the list immediately and does not reappear on refresh
- [ ] Attempt to create a relay with missing required fields (no target_host); UI shows a validation error
- [ ] Reload serve button (`POST /api/serve/reload`) completes without error

---

## 6. HTTPS Relays (`/api/serve/https/*`)

- [ ] `GET /api/serve/https/list` returns a JSON array
- [ ] Create an HTTPS relay (id, listen_port, target_host:target_port, optional hostname); relay appears in the list
- [ ] Relay `hostname` field is auto-populated with the MagicDNS name when Tailscale is connected
- [ ] Created relay shows `running: false/true` correctly based on serve status
- [ ] `GET /api/serve/https/get?id=<id>` returns the correct relay object
- [ ] Edit the relay (change listen_port or target); updated values persist
- [ ] Toggle HTTPS relay disabled; toggled state persists across page refresh
- [ ] Toggle HTTPS relay re-enabled
- [ ] Delete the HTTPS relay; removed from list immediately
- [ ] Attempt to create an HTTPS relay with an invalid target format (missing port); UI shows error

---

## 7. Backup & Restore (`/api/backup/*`)

- [ ] `GET /api/backup/list` returns a JSON array (empty initially)
- [ ] Create a backup via the UI; new entry appears in the backup list with a filename and size
- [ ] Rename a backup; renamed filename is reflected in the list
- [ ] Download a backup; the file downloads successfully and is a valid `.tar.gz`
- [ ] Upload a backup file; appears in the list after upload
- [ ] Restore a backup; after restore, relay config matches what was in the backup
- [ ] Delete a backup; entry disappears from the list
- [ ] Backup list persists across container restart (backed by volume)

---

## 8. Logs (`/api/logs/*`)

- [ ] Logs page (`/logs`) loads without error
- [ ] `GET /api/logs` returns recent log lines as JSON or plain text
- [ ] `GET /api/logs/stream` delivers a stream of log lines (SSE or chunked response)
- [ ] `POST /api/logs/level` changes the log verbosity level; subsequent log output reflects the new level

---

## 9. Legacy Endpoint Shims

- [ ] `GET /api/caddy/list` (with valid auth) returns **410 Gone**, not 404 or 200
- [ ] Response body for `/api/caddy/*` contains a `migrate` field pointing to the new endpoint
- [ ] `GET /api/socat/list` (with valid auth) returns **410 Gone**, not 404 or 200
- [ ] Response body for `/api/socat/*` contains a `migrate` field pointing to the new endpoint
- [ ] Any sub-path under `/api/caddy/` and `/api/socat/` also returns 410 (e.g. `/api/caddy/create`)

---

## 10. Persistence Across Restart

- [ ] Create a TCP relay, then restart the container; relay is still present in the list
- [ ] Create an HTTPS relay, restart; relay is still present
- [ ] Enabled/disabled state of relays is preserved across restart
- [ ] Autostart relays are reconciled with `tailscale serve` on container startup (check logs for "Successfully reconciled")

---

## 11. Auth Token File

- [ ] Token file exists at `/var/lib/tailscale/.webui_token` inside the container after first start
- [ ] Token value in the file grants access when sent as `Authorization: Bearer <token>`
- [ ] Requests without the header (and no session cookie) are denied
