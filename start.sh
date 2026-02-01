#!/bin/ash
trap 'kill -TERM $PID' TERM INT
TAILRELAY_VERSION=v0.3.0

# Accept a single comma‑separated list of port:target pairs
# Each item in the list represents one socat relay
# Example:
#   RELAY_LIST=50001:electrs.embassy:50001,21004:lnd.embassy:10009
RELAY_LIST=${RELAY_LIST:-}

export TS_ENABLE_METRICS=true
export TS_ENABLE_HEALTH_CHECK=true

echo -n "Starting tailrelay ${TAILRELAY_VERSION} with Tailscale v"
tailscale --version | head -1


# Start Web UI
echo -n "Starting Tailrelay Web UI... "
/usr/bin/tailrelay-webui --config /etc/tailrelay/webui.yaml > /var/log/tailrelay-webui.log 2>&1 &
WEBUI_PID=$!
if [ $? -ne 0 ]; then
   echo "failed!"
else
   echo "success! (PID: $WEBUI_PID, available at http://0.0.0.0:8021)"
fi

# Spawn socat instances if RELAY_LIST is provided
if [ ! -z "$RELAY_LIST" ]; then
   # Split the comma‑separated list into individual items
   set -- ${RELAY_LIST//,/ }
   echo "Starting socat..."
   for ITEM in "$@"; do
      # Example ITEM: 50002:electrs.embassy:50001
      LISTENING_PORT=${ITEM%%:*}      # 50002
      REST=${ITEM#*:}                 # electrs.embassy:50001
      TARGET_HOST=${REST%%:*}         # electrs.embassy
      TARGET_PORT=${REST#*:}          # 50001

      # Basic sanity check
      if [ -z "$LISTENING_PORT" ] || [ -z "$TARGET_HOST" ] || [ -z "$TARGET_PORT" ]; then
         echo "Error: '$ITEM' must be in 'port:TARGET_HOST:TARGET_PORT' format"
         exit 1
      fi

      echo -n "Relaying $TARGET_HOST:$TARGET_PORT to listening port $LISTENING_PORT... "
      socat tcp-listen:$LISTENING_PORT,fork,reuseaddr tcp:$TARGET_HOST:$TARGET_PORT < /dev/null &
      if [ $? -ne 0 ]; then
         echo "failed!"
      else
         echo "success!"
      fi

   done
fi

# Start Caddy
echo -n "Starting Caddy... "
CADDY_STATUS=$(caddy start --config /etc/caddy/Caddyfile >/dev/null)
# echo success or fail + stderr
if [ $? -ne 0 ]; then
  echo "failed!"
  echo $CADDY_STATUS
else
  echo "success!"
fi

exec /usr/local/bin/containerboot