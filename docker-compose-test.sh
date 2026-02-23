#!/usr/bin/env bash
set -ex

# Load environment vars from .env
if [[ -f .env ]]; then
  set -o allexport   # automatically export all listed vars
  source .env
  set +o allexport
fi

docker compose -f ${COMPOSE_FILE} down
mkdir -p tailscale
docker buildx build \
  --build-arg VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev") \
  --build-arg COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "none") \
  --build-arg DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ) \
  --build-arg BRANCH=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown") \
  --build-arg BUILDER=$(whoami) \
  -t sudocarlos/tailrelay:dev --load .
docker compose -f ${COMPOSE_FILE} up -d
echo "Waiting for container to start..."
sleep 3
docker logs tailrelay-test | tail
docker exec tailrelay-test netstat -tulnp | grep LISTEN

docker exec tailrelay-test wget -qO-         http://127.0.0.1:8080 >/dev/null && echo success || echo fail
docker exec tailrelay-test wget -qO-         http://127.0.0.1:8081 >/dev/null && echo success || echo fail
docker exec tailrelay-test wget -qO- --no-check-certificate  https://127.0.0.1:8443 >/dev/null && echo success || echo fail
docker exec tailrelay-test wget -qO-         http://127.0.0.1:9002/healthz >/dev/null && echo success || echo fail
docker exec tailrelay-test wget -qO-         http://127.0.0.1:9002/metrics >/dev/null && echo success || echo fail

# Test Socat Relay
echo "Testing Socat Relay..."
docker exec tailrelay-test sh -c "echo '{\"relays\": [{\"id\": \"test-relay\", \"listen_port\": 8089, \"target_host\": \"whoami-test\", \"target_port\": 80, \"enabled\": true, \"autostart\": true}]}' > /var/lib/tailscale/relays.json"
docker restart tailrelay-test
sleep 5
docker exec tailrelay-test wget -qO- http://127.0.0.1:8089 | grep "Host: 127.0.0.1:8089" && echo "Socat Success" || echo "Socat Fail"

# stop the containers
docker compose -f ${COMPOSE_FILE} down