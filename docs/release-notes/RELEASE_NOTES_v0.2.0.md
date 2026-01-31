# v0.2.0 - Web UI Integration

## 🎉 Major Release: Web UI Integration

This release adds a comprehensive **browser-based Web UI** for managing all aspects of tailrelay without manual configuration editing!

### 🌟 Key Features

#### Web UI Management
- **Dashboard** with Tailscale connection status and system health monitoring
- **Tailscale Management** - Connect/disconnect, view peers, check status
- **Caddy Proxy Management** - Add, edit, delete, and toggle HTTP/HTTPS reverse proxies
- **Socat Relay Management** - Full TCP relay process management
- **Backup & Restore** - Create, download, upload, and restore configuration backups
- **Dark Theme** with responsive design optimized for readability

#### Security & Authentication
- **Dual Authentication**:
  - Token-based authentication (auto-generated on first run)
  - Automatic Tailscale network authentication (no token needed from Tailnet)
- Session management with secure cookies
- Token stored at `/var/lib/tailscale/.webui_token`

#### Auto-Configuration
- Automatic migration from `RELAY_LIST` environment variable to JSON configs
- Auto-generated Caddyfile from Web UI proxy settings
- Process management with PID tracking for socat relays
- Configuration validation and error handling

### 📦 Installation & Access

**Docker Hub Images** (multi-platform: amd64, arm64):
```bash
docker pull sudocarlos/tailrelay:v0.2.0
# or
docker pull sudocarlos/tailrelay:latest
```

**Access Web UI**:
- Port: `8021` (expose with `-p 8021:8021`)
- URL: `http://localhost:8021` or `https://your-hostname.tailnet.ts.net:8021`

**Get Authentication Token**:
```bash
# Docker
docker exec <container> cat /var/lib/tailscale/.webui_token

# Podman
sudo podman exec start9.tailscale cat /var/lib/tailscale/.webui_token
```

### 🔧 Technical Details

- **Built with**: Go 1.21, embedded HTML/CSS/JS
- **Binary size**: ~14MB (includes all assets)
- **Multi-stage Docker build** for optimized image size
- **RESTful API** with 20+ endpoints
- **5,700+ lines** of new code and documentation

### 📚 Documentation

- Comprehensive [README](https://github.com/sudocarlos/tailscale-socaddy-proxy/blob/main/README.md) with Web UI sections
- Full [CHANGELOG](https://github.com/sudocarlos/tailscale-socaddy-proxy/blob/main/CHANGELOG.md) with API documentation
- Troubleshooting guide and upgrade instructions
- [AGENTS.md](https://github.com/sudocarlos/tailscale-socaddy-proxy/blob/main/AGENTS.md) for developers

### ⚠️ Breaking Changes

- `RELAY_LIST` environment variable is now **deprecated** (auto-migrated)
- Recommended to manage relays through Web UI instead
- Manual Caddyfile editing still supported but not recommended with Web UI

### 🔄 Upgrading from v0.1.1

1. Backup your configuration
2. Pull new image: `docker pull sudocarlos/tailrelay:v0.2.0`
3. Stop old container
4. Start new container with `-p 8021:8021` for Web UI access
5. Access Web UI and migrate configurations

See [CHANGELOG.md](https://github.com/sudocarlos/tailscale-socaddy-proxy/blob/main/CHANGELOG.md#upgrading-from-v011-to-v020) for detailed upgrade instructions.

### 🐛 Known Issues

- Permission warning for `/var/log/tailrelay-webui.log` (non-critical)
- Web UI requires root user in container (Tailscale requirement)

### 🙏 Feedback

Please report issues or suggestions on the [GitHub Issues](https://github.com/sudocarlos/tailscale-socaddy-proxy/issues) page.

---

**Full Changelog**: https://github.com/sudocarlos/tailscale-socaddy-proxy/blob/main/CHANGELOG.md

**Docker Hub**: https://hub.docker.com/r/sudocarlos/tailrelay
