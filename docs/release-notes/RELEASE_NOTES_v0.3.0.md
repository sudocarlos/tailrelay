````markdown
# Release Notes - v0.3.0

**Release Date:** February 1, 2026

## 🚀 Highlights

This release focuses on stronger observability and more robust proxy management:
- Live log streaming in the Web UI
- Custom CA certificate uploads for TLS-verified upstreams
- More consistent and stable Caddy proxy configuration handling

## ✨ Key Features

### Web UI Logging (Live Streaming)
- Comprehensive logging system added to the Web UI
- Live streaming of logs to the browser for faster troubleshooting
- Improved visibility into proxy, relay, and service activity

### Custom CA Certificate Uploads
- Upload custom CA certificates for upstream TLS verification
- Enables secure connections to services using internal or private PKI
- Reduces the need for insecure TLS bypasses

### Proxy Management Improvements
- Aligned Caddy route structure for consistent configuration output
- Persisted server mappings per proxy to minimize churn
- Refactored proxy manager for clearer configuration handling

## 🔧 Technical Changes

### Added
- Logging subsystem with live-streaming endpoints in the Web UI
- Custom CA certificate upload pipeline for proxy TLS
- Local Web UI development workflow for rapid iteration

### Changed
- Caddy route generation to maintain consistent proxy structure
- Caddy server mappings stored per proxy for stability
- Internal proxy manager refactors to match new configuration flows

### Removed
- Legacy file-based proxy handling (Caddyfile/JSON-based proxy CRUD)

## 📦 Upgrade Instructions

### Docker Users

```bash
# Pull the latest image
docker pull sudocarlos/tailrelay:v0.3.0
# or
docker pull sudocarlos/tailrelay:latest

# Restart your container
docker restart tailrelay
```

### Start9 Users

1. Stop the current container:
   ```bash
   sudo podman stop start9.tailscale
   sudo podman rm start9.tailscale
   ```
2. Pull the new version:
   ```bash
   sudo podman pull docker.io/sudocarlos/tailrelay:v0.3.0
   ```
3. Start with the same configuration:
   ```bash
   sudo podman run --name start9.tailscale \
     -v /home/start9/tailscale/:/var/lib/tailscale \
     -e TS_HOSTNAME=start9 \
     -p 8021:8021 \
     --net start9 \
     docker.io/sudocarlos/tailrelay:v0.3.0
   ```

## 🔍 Verification

After upgrading, verify the new features:

```bash
# Web UI health (should load and show logs)
curl http://localhost:8021

# Caddy config routes
tailrelay_caddy_routes=$(curl -s http://localhost:2019/config/apps/http/servers/tailrelay/routes)
echo "$tailrelay_caddy_routes" | jq
```

## ⚠️ Notes

- Proxy configuration is now fully API-driven; legacy file-based proxy handling has been removed.
- If you use a private CA for upstream TLS, upload it through the Web UI before enabling verification.

## 📝 Changelog

### Added
- Live log streaming in the Web UI
- Custom CA certificate upload support for upstream TLS verification
- Local Web UI development workflow

### Changed
- Consistent Caddy proxy route structure
- Persisted Caddy server mappings per proxy
- Refactored proxy manager and configuration handling

### Removed
- Legacy proxy file handling

---

**Full Changelog**: https://github.com/sudocarlos/tailscale-socaddy-proxy/compare/v0.2.1...v0.3.0
````
