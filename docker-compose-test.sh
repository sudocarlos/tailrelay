#!/usr/bin/env bash
set -ex

# Load environment vars from .env
if [[ -f .env ]]; then
  set -o allexport   # automatically export all listed vars
  source .env
  set +o allexport
fi

docker compose -f ${COMPOSE_FILE} down
docker buildx build -t sudocarlos/tailrelay:dev --load .
docker compose -f ${COMPOSE_FILE} up -d
echo "Waiting for container to start..."
sleep 3
docker logs tailrelay-test | tail
docker exec -it tailrelay-test netstat -tulnp | grep LISTEN

curl -sSL http://${TAILRELAY_HOST}:8080 && echo success || echo fail
curl -sSL http://${TAILRELAY_HOST}:8081 && echo success || echo fail
curl -sSL https://${TAILRELAY_HOST}.${TAILNET_DOMAIN}:8443 && echo success || echo fail
curl -sSL http://${TAILRELAY_HOST}:9002/healthz && echo success || echo fail
curl -sSL http://${TAILRELAY_HOST}:9002/metrics && echo success || echo fail

# stop the containers
docker compose -f ${COMPOSE_FILE} down