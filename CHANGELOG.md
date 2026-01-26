# Changelog

All notable changes to tailrelay will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v0.2.0] - 2026-01-26

### Added

#### Web UI
- **Comprehensive browser-based management interface** written in Go with embedded HTML/CSS/JS
- **Dashboard page** displaying Tailscale connection status and system health
- **Tailscale management page** for connecting/disconnecting and viewing peer information
- **Caddy proxy management page** with full CRUD operations for HTTP/HTTPS reverse proxies
- **Socat relay management page** with full CRUD operations for TCP relays
- **Backup & restore page** for creating, downloading, uploading, and restoring configuration backups
- **Dual authentication system**:
  - Token-based authentication (generated on first run)
  - Automatic Tailscale network authentication (IP-based)
- **Dark theme** with responsive design and modern UI
- **Auto-refresh** functionality on dashboard
- **Session management** with cookie-based login

#### Configuration Management
- Auto-migration from `RELAY_LIST` environment variable to JSON configuration
- JSON-based configuration storage for proxies and relays
- Automatic Caddyfile generation from Web UI proxy configurations
- Process management for socat relays with PID tracking
- Configuration validation and error handling

#### Docker Integration
- Multi-stage Docker build with Go 1.21
- Web UI binary embedded in final image (~14MB)
- Web UI runs on port 8021 (exposed by default)
- Configuration file at `/etc/tailrelay/webui.yaml`
- Authentication token stored at `/var/lib/tailscale/.webui_token`
- Backup directory at `/var/lib/tailscale/backups`

#### Documentation
- Comprehensive README update with Web UI sections
- Troubleshooting guide for common issues
- Web UI access and authentication documentation
- Docker and podman usage examples

### Changed
- Bumped version from v0.1.1 to v0.2.0
- Updated `start.sh` to launch Web UI on startup
- Updated `compose-test.yml` to expose Web UI port 8021
- Fixed `go.mod` version requirement (was incorrectly set to 1.25.6, now 1.21)
- Removed `user: 1000:1000` from compose-test.yml (requires root for Tailscale)
- `RELAY_LIST` environment variable now deprecated in favor of Web UI management

### Technical Details

#### Project Structure
```
webui/
├── cmd/webui/
│   ├── main.go                 # Entry point
│   └── web/
│       ├── templates/          # 7 HTML templates
│       └── static/             # CSS + JS
└── internal/
    ├── auth/                   # Authentication middleware
    ├── backup/                 # Backup/restore operations
    ├── caddy/                  # Caddy management & Caddyfile generation
    ├── config/                 # Configuration loading & migration
    ├── handlers/               # HTTP request handlers (6 files)
    ├── socat/                  # Socat process management
    ├── tailscale/              # Tailscale CLI wrapper
    └── web/                    # HTTP server
```

#### API Endpoints
- `GET /` - Dashboard (requires auth)
- `GET /login` - Login page
- `POST /login` - Login handler
- `GET /logout` - Logout handler
- `GET /tailscale` - Tailscale management page
- `POST /api/tailscale/*` - Tailscale operations (connect, disconnect, status)
- `GET /caddy` - Caddy proxy management page
- `GET /api/caddy/proxies` - List all proxies
- `POST /api/caddy/proxies` - Add new proxy
- `PUT /api/caddy/proxies/:id` - Update proxy
- `DELETE /api/caddy/proxies/:id` - Delete proxy
- `GET /socat` - Socat relay management page
- `GET /api/socat/relays` - List all relays
- `POST /api/socat/relays` - Add new relay
- `PUT /api/socat/relays/:id` - Update relay
- `DELETE /api/socat/relays/:id` - Delete relay
- `POST /api/socat/relays/:id/start` - Start relay process
- `POST /api/socat/relays/:id/stop` - Stop relay process
- `GET /backup` - Backup management page
- `POST /api/backup/create` - Create new backup
- `GET /api/backup/download/:filename` - Download backup
- `POST /api/backup/upload` - Upload backup
- `POST /api/backup/restore/:filename` - Restore from backup
- `GET /api/backup/list` - List all backups

#### File Formats
- **webui.yaml** - Main Web UI configuration (server, auth, paths, logging)
- **proxies.json** - Caddy proxy definitions (auto-generated Caddyfile)
- **relays.json** - Socat relay definitions (replaces RELAY_LIST)
- **backups/*.tar.gz** - Compressed backups with metadata.json

### Backward Compatibility
- Existing `RELAY_LIST` environment variable is automatically migrated to `relays.json` on first run
- Manual Caddyfile editing still supported (not recommended with Web UI)
- All previous command-line flags and environment variables remain functional

### Known Issues
- Permission denied warning for `/var/log/tailrelay-webui.log` (non-critical, Web UI still runs)
- LSP false positive warnings in `main.go` (embed patterns work correctly)

---

## [v0.1.1] - Previous Release

### Initial Features
- Tailscale integration for VPN connectivity
- Caddy reverse proxy for HTTP/HTTPS services
- Socat TCP relay for non-HTTP protocols
- Manual Caddyfile configuration
- Environment variable-based relay configuration (`RELAY_LIST`)
- Docker and Podman support
- Start9 compatibility

---

## Release Notes

### Upgrading from v0.1.1 to v0.2.0

1. **Backup your configuration**:
   ```bash
   sudo podman exec start9.tailscale tar czf /var/lib/tailscale/backup-v0.1.1.tar.gz \
     /etc/caddy/Caddyfile /var/lib/tailscale/
   sudo podman cp start9.tailscale:/var/lib/tailscale/backup-v0.1.1.tar.gz ./
   ```

2. **Pull the new image**:
   ```bash
   sudo podman pull docker.io/sudocarlos/tailrelay:latest
   ```

3. **Stop and remove the old container**:
   ```bash
   sudo podman stop start9.tailscale
   sudo podman rm start9.tailscale
   ```

4. **Run the new container with Web UI port**:
   ```bash
   sudo podman run --name start9.tailscale \
     -v /home/start9/tailscale/:/var/lib/tailscale \
     -v /home/start9/tailscale/Caddyfile:/etc/caddy/Caddyfile \
     -e TS_HOSTNAME=start9 \
     -p 8021:8021 \
     --net start9 \
     docker.io/sudocarlos/tailrelay:latest
   ```

5. **Access the Web UI**:
   ```bash
   # Get the auth token
   sudo podman exec start9.tailscale cat /var/lib/tailscale/.webui_token
   
   # Access at http://localhost:8021
   ```

6. **Migrate existing relays** (optional):
   - Your `RELAY_LIST` will be automatically migrated to JSON format
   - You can now manage relays through the Web UI instead

### Future Plans
- [ ] Real-time log viewer in Web UI
- [ ] Metrics and statistics dashboard
- [ ] Email/webhook notifications for status changes
- [ ] Multi-user authentication with roles
- [ ] API key management for programmatic access
- [ ] Configuration import/export wizard
- [ ] Integration tests for Web UI endpoints
