---
name: socat-relay-management
description: socat TCP relay management for forwarding non-HTTP protocols. Use when working with TCP relays, the RELAY_LIST environment variable, socat process management, or the relay management Go code in internal/socat/.
---

# socat Relay Management

## Overview

socat provides TCP relay (port forwarding) for non-HTTP protocols like electrs, LND gRPC, or any raw TCP service. Relays are managed through the Web UI or via the legacy `RELAY_LIST` environment variable.

## How Relays Work

Each relay runs as a background `socat` process:

```bash
socat tcp-listen:$LISTEN_PORT,fork,reuseaddr tcp:$TARGET_HOST:$TARGET_PORT
```

- `fork` — handles multiple concurrent connections
- `reuseaddr` — allows port reuse after restart

## Relay Sources

### 1. Web UI (Preferred)

The Web UI manages relays via `webui/internal/socat/`:
- Relay configs stored in `relays.json`
- CRUD operations through the dashboard
- Process lifecycle management (start, stop, restart)

### 2. RELAY_LIST Environment Variable (Legacy)

```bash
RELAY_LIST=50001:electrs.embassy:50001,21004:lnd.embassy:10009
```

Format: `listen_port:target_host:target_port` (comma-separated)

Parsed in `start.sh`:
```bash
LISTENING_PORT=${ITEM%%:*}      # First field
REST=${ITEM#*:}
TARGET_HOST=${REST%%:*}         # Second field
TARGET_PORT=${REST#*:}          # Third field
```

### Migration

On first startup, if `RELAY_LIST` is set and `relays.json` doesn't exist, the Web UI auto-migrates to JSON format. After migration, remove `RELAY_LIST` and manage through the Web UI.

## Go Package: `internal/socat/`

Key responsibilities:
- **Process management**: Start/stop socat subprocesses
- **State tracking**: Monitor running relay PIDs
- **Configuration**: Read/write `relays.json`
- **Status polling**: Check if relay processes are alive

### Relay Config Structure

```json
[
  {
    "name": "electrs",
    "listen_port": 50001,
    "target_host": "electrs.embassy",
    "target_port": 50001,
    "enabled": true
  }
]
```

## Web UI Endpoints

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api/socat/relays` | GET | List all relays with status |
| `/api/socat/relays` | POST | Add new relay |
| `/api/socat/relays` | PUT | Update relay |
| `/api/socat/relays` | DELETE | Delete relay |
| `/api/socat/relays/toggle` | POST | Start/stop relay |

## Testing

```bash
# Check running relays
docker exec tailrelay ps aux | grep socat

# Verify listening ports
docker exec tailrelay netstat -tulnp | grep socat

# Test target connectivity
docker exec tailrelay nc -zv target-host target-port
```

## Troubleshooting

### Relay not starting
- Check port conflicts: `netstat -tulnp | grep $PORT`
- Verify target is reachable: `nc -zv $HOST $PORT`
- Check relay config in `relays.json`

### Relay shows "Running" but connection fails
- Verify the target service is actually listening
- Check Docker network mode (`--net start9` for Start9)
- Look for socat error output in container logs

### Port already in use
- Another relay or service may be using the port
- Stop conflicting process or choose a different listen port
