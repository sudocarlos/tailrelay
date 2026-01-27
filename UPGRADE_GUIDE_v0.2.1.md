# Quick Start Guide - v0.2.1 Upgrade

## What's New in v0.2.1?

**Caddy API Integration** - We've replaced file-based Caddy management with direct Admin API integration for:
- ⚡ 5-10x faster operations
- 🚀 Zero-downtime configuration changes  
- 🔒 Atomic updates with better error handling
- 🔄 Automatic migration from old system

## Upgrade Steps

### For Docker Users

```bash
# Stop current container
docker stop tailrelay
docker rm tailrelay

# Pull new version
docker pull sudocarlos/tailrelay:v0.2.1

# Start with same configuration
docker run -d --name tailrelay \
  -v /path/to/data:/var/lib/tailscale \
  -e TS_HOSTNAME=myserver \
  -p 8021:8021 \
  --net bridge \
  sudocarlos/tailrelay:v0.2.1
```

### For Start9 Users

```bash
# SSH into Start9
ssh start9@your-server

# Stop and remove old container
sudo podman stop start9.tailscale
sudo podman rm start9.tailscale

# Pull new version
sudo podman pull docker.io/sudocarlos/tailrelay:v0.2.1

# Start with your existing configuration
sudo podman run --name start9.tailscale \
  -v /home/start9/tailscale/:/var/lib/tailscale \
  -e TS_HOSTNAME=start9 \
  -p 8021:8021 \
  --net start9 \
  docker.io/sudocarlos/tailrelay:v0.2.1
```

## What Happens on First Startup?

1. ✅ System detects existing `proxies.json` (if present)
2. ✅ All proxies automatically migrated to Caddy API
3. ✅ Old file backed up as `proxies.json.migrated.bak`
4. ✅ Migration validated to ensure success
5. ✅ Web UI works exactly as before

**No manual intervention required!**

## Verification

After upgrade, verify everything works:

```bash
# Check container logs
docker logs tailrelay
# OR for Start9
sudo podman logs start9.tailscale

# Look for these success messages:
# ✓ "Migrated X proxies to Caddy API"
# ✓ "Web UI started successfully"
# ✓ "Caddy started successfully"

# Access Web UI
curl http://localhost:8021
# OR via Tailscale hostname
https://your-hostname.tailnet.ts.net:8021

# Check Caddy API (inside container)
docker exec tailrelay curl http://localhost:2019/config/ | jq
```

## Troubleshooting

### Issue: "Caddy API not accessible"

**Solution**: Ensure Caddy is running with admin API enabled (default port 2019)

```bash
# Check Caddy status inside container
docker exec tailrelay ps aux | grep caddy

# Test API
docker exec tailrelay curl -s http://localhost:2019/config/ | jq
```

### Issue: "Proxies not working after upgrade"

**Solution**: Check migration logs and verify proxies in Caddy

```bash
# View migration logs
docker logs tailrelay | grep -i migration

# Check proxies in Caddy
docker exec tailrelay curl -s \
  http://localhost:2019/config/apps/http/servers/tailrelay/routes | jq

# Via Web UI
curl http://localhost:8021/api/caddy/proxies | jq
```

### Issue: "Web UI shows no proxies"

**Solution**: Proxies may not have been migrated if they were disabled

```bash
# Check backup file
docker exec tailrelay cat /var/lib/tailscale/proxies.json.migrated.bak

# Manually add proxies via Web UI or API
curl -X POST http://localhost:8021/api/caddy/proxies \
  -H "Content-Type: application/json" \
  -d '{
    "id": "my-proxy",
    "hostname": "myserver.tailnet.ts.net",
    "port": 8080,
    "target": "localhost:9000",
    "enabled": true
  }'
```

## Key Differences from v0.2.0

| Feature | v0.2.0 | v0.2.1 |
|---------|--------|--------|
| Configuration Method | File-based (JSON → Caddyfile) | API-based (direct) |
| Reload Required | Yes (~100-200ms downtime) | No (zero downtime) |
| Operation Speed | 200-500ms | 10-50ms |
| Error Handling | Delayed (after reload) | Immediate |
| Configuration Drift | Possible | Impossible |

## Getting Help

- **Documentation**: `webui/CADDY_API_GUIDE.md`
- **Migration Guide**: `webui/MIGRATION_SUMMARY.md`
- **Release Notes**: `RELEASE_NOTES_v0.2.1.md`
- **GitHub Issues**: https://github.com/sudocarlos/tailscale-socaddy-proxy/issues

## Rollback (if needed)

If you encounter issues, you can rollback to v0.2.0:

```bash
# For Docker
docker pull sudocarlos/tailrelay:v0.2.0
docker stop tailrelay && docker rm tailrelay
# ... run with v0.2.0

# Restore old proxies.json if needed
docker exec tailrelay mv \
  /var/lib/tailscale/proxies.json.migrated.bak \
  /var/lib/tailscale/proxies.json
```

However, rollback should not be necessary - the new system is more robust!

## Advanced Features

### Using the API Directly

```bash
# Add a proxy via API
curl -X POST http://localhost:2019/config/apps/http/servers/tailrelay/routes \
  -H "Content-Type: application/json" \
  -d '{
    "@id": "my-proxy",
    "match": [{"host": ["myhost.tailnet.ts.net:8080"]}],
    "handle": [{
      "handler": "reverse_proxy",
      "upstreams": [{"dial": "localhost:9000"}]
    }]
  }'

# Get proxy by ID
curl http://localhost:2019/id/my-proxy | jq

# Delete proxy by ID
curl -X DELETE http://localhost:2019/id/my-proxy
```

### Export Current Configuration

```bash
# Export all proxies to backup
docker exec tailrelay curl -s http://localhost:2019/config/ > caddy-config.json

# View just the routes
docker exec tailrelay curl -s \
  http://localhost:2019/config/apps/http/servers/tailrelay/routes \
  | jq > routes.json
```

## Next Steps

1. ✅ Upgrade completed - verify everything works
2. 📖 Read the comprehensive guide: `webui/CADDY_API_GUIDE.md`
3. 🧪 Test adding/updating/deleting proxies via Web UI
4. 🔍 Monitor performance improvements
5. 💾 Set up automated backups (Web UI → Backup & Restore)

---

**Welcome to v0.2.1!** Enjoy faster, more reliable proxy management with zero downtime. 🎉
