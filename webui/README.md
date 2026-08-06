# Tailrelay Web UI

A lightweight web interface for managing Tailscale and `tailscale serve`/`tailscale funnel` HTTPS/TCP relays in the tailrelay container.

## Features

- **Dashboard**: System status overview
- **Tailscale Management**: Login, status, device list, custom control server (Headscale) support, networking preferences (advertise/accept routes, exit-node advertisement, SSH)
  - *(Note: under tailrelay's userspace-networking mode, routing this node's own traffic through a peer exit node isn't supported; only "Run as exit node" is offered.)*
- **HTTPS Relay Management**: Add/edit/delete HTTPS relays via `tailscale serve`
- **TCP Relay Management**: Add/edit/delete TCP relays via `tailscale serve`
- **Funnel Management**: Expose a service on the public internet on port `443`, `8443`, or `10000` via `tailscale funnel`
- **Backup & Restore**: Full configuration and certificate backup
- **Authentication**: Tailscale login link + token-based access for scripts

## Building

```bash
go build -o tailrelay-webui ./cmd/webui
```

## Running

```bash
# With default config (/var/lib/tailscale/webui.yaml)
./tailrelay-webui

# With custom config
./tailrelay-webui --config /path/to/webui.yaml

# Show version
./tailrelay-webui --version
```

## Configuration

See `config/webui.yaml` for an example configuration file.

### Key Settings

- **server.port**: Web UI port (default: 8021)
- **auth.enable_tailscale_auth**: Allow auth from Tailscale network IPs
- **auth.enable_token_auth**: Require authentication token
- **paths.serve_relay_config**: Path for persisted serve relay metadata (default: `/var/lib/tailscale/serve_relays.json`)
- **paths.**: Other file paths for configurations and state
- **tailscale.control_server**: Custom control server URL for a self-hosted Headscale instance (default: empty, meaning Tailscale's default control plane); also settable via the Control Server field on the Tailscale page

## Authentication

The Web UI supports two authentication methods:

1. **Tailscale Network Authentication**: Automatic authentication from Tailscale IPs (100.x.y.z). If the device is not connected, the login page shows a Tailscale login link and polls until connected.
2. **Token Authentication**: Token-based access for scripted or legacy flows (token generated on first run and saved to the configured token file).

## Legacy Migration

On first startup, the Web UI migrates legacy `relays.json` and `proxies.json` metadata into `serve_relays.json` and reconciles active relays through `tailscale serve`.

## Development

### Project Structure

```
webui/
├── cmd/webui/          # Main application entry point
│   └── web/            # Embedded static assets and templates
├── internal/
│   ├── config/         # Configuration management
│   ├── tailscale/      # Tailscale CLI integration
│   ├── serve/          # tailscale serve relay management
│   ├── auth/           # Authentication middleware
│   ├── handlers/       # HTTP request handlers
│   └── web/            # HTTP server and routing
├── config/             # Example configuration files
├── examples/           # Usage examples
```

### Dependencies

- Go 1.26.1+
- `gopkg.in/yaml.v3` - YAML configuration parsing

All other functionality uses the Go standard library.

## Testing

```bash
# Run Go unit tests
make test

# Run specific package tests
go test ./internal/backup/...
go test ./internal/web/...

# Integration tests (requires Docker; see compose-test.yml)
make integration-test
```

## Docker Integration

The Web UI is built as part of the tailrelay Docker image and starts automatically with the container.

See the main project README for Docker usage instructions.

## Review Status

<!-- reviewed_at: f2c24a0 | paths: webui/ -->
Last full review completed at commit `f2c24a0`. To check what has changed since:
```bash
git log --oneline f2c24a0..HEAD -- webui/
```
