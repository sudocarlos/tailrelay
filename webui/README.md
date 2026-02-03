# Tailrelay Web UI

A lightweight web interface for managing Tailscale, Caddy reverse proxies, and socat TCP relays in the tailrelay container.

## Features

- **Dashboard**: System status overview
- **Tailscale Management**: Login, status, device list
- **Caddy Proxy Management**: Add/edit/delete HTTP/HTTPS reverse proxies via Caddy Admin API
- **Socat Relay Management**: Add/edit/delete TCP relays
- **Backup & Restore**: Full configuration and certificate backup
- **Authentication**: Tailscale login link + token-based access for scripts

## Recent Updates (v0.3.0)

### Caddy API Integration

The Web UI now uses **Caddy's Admin API** directly instead of file-based Caddyfile management. This provides:

- ✅ **Zero-downtime configuration changes** - No reload/restart needed
- ✅ **5-10x faster operations** - Direct API calls vs file regeneration
- ✅ **Atomic updates** - Changes apply instantly and safely
- ✅ **Better error handling** - Immediate feedback from Caddy
- ✅ **No file system dependencies** - Pure HTTP-based management

See `CADDY_API_GUIDE.md` for detailed documentation and `MIGRATION_SUMMARY.md` for migration information.

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
- **paths.**: File paths for configurations and state

## Authentication

The Web UI supports two authentication methods:

1. **Tailscale Network Authentication**: Automatic authentication from Tailscale IPs (100.x.y.z). If the device is not connected, the login page shows a Tailscale login link and polls until connected.
2. **Token Authentication**: Token-based access for scripted or legacy flows (token generated on first run and saved to the configured token file).

## Migration from RELAY_LIST

On first startup, if the `RELAY_LIST` environment variable is set and `relays.json` doesn't exist, the Web UI will automatically migrate the relay configuration to JSON format.

Format: `RELAY_LIST=port:host:port,port:host:port`

After migration, you can remove the `RELAY_LIST` environment variable and manage relays through the Web UI.

## Development

### Bootstrap Icons (SPA)

The SPA uses a lightweight Bootstrap Icons SVG sprite stored at:

- webui/cmd/webui/web/static/vendor/bootstrap-icons/bootstrap-icons.svg

If you want to swap in the full Bootstrap Icons distribution, keep the sprite in the same path or update the template references accordingly.

### Project Structure

```
webui/
├── cmd/webui/          # Main application entry point
│   └── web/            # Embedded static assets and templates
├── internal/
│   ├── config/         # Configuration management
│   ├── tailscale/      # Tailscale CLI integration
│   ├── caddy/          # Caddy API integration
│   │   ├── api_client.go      # HTTP client for Caddy Admin API
│   │   ├── api_types.go       # Caddy JSON config structures
│   │   ├── proxy_manager.go   # High-level proxy management
│   │   ├── manager.go          # Simplified manager interface
│   │   ├── migration.go        # Migration utilities
│   │   └── caddyfile.go        # Legacy Caddyfile support
│   ├── socat/          # Socat process management
│   ├── auth/           # Authentication middleware
│   ├── handlers/       # HTTP request handlers
│   └── web/            # HTTP server and routing
├── config/             # Example configuration files
├── examples/           # Usage examples
└── docs/
    ├── CADDY_API_GUIDE.md      # Comprehensive API documentation
    └── MIGRATION_SUMMARY.md    # Migration guide
```

### Dependencies

- Go 1.21+
- `gopkg.in/yaml.v3` - YAML configuration parsing

All other functionality uses the Go standard library.

## Testing

```bash
# Run tests
go test ./...

# Build and test locally
go build -o tailrelay-webui ./cmd/webui
./tailrelay-webui --config ./config/webui.yaml
```

## Docker Integration

The Web UI is built as part of the tailrelay Docker image and starts automatically with the container.

See the main project README for Docker usage instructions.
