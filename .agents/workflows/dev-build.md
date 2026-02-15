---
description: Build the Web UI binary locally and restart the container for testing
---

## Dev Build Workflow

// turbo-all

1. Build frontend assets (if frontend code changed):
```bash
make frontend-build
```

2. Build the Go binary with build metadata:
```bash
make dev-build
```

3. Restart the container to pick up the new binary:
```bash
docker compose -f compose-test.yml restart tailrelay
```

4. Verify the Web UI is responding:
```bash
curl -sSL http://localhost:8021 && echo "✅ Web UI OK" || echo "❌ Web UI failed"
```

5. Check container health endpoints:
```bash
curl -sSL http://localhost:9002/healthz && echo "✅ Health OK" || echo "❌ Health failed"
```
