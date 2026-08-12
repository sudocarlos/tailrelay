#!/bin/ash
trap 'shutdown' TERM INT
TAILRELAY_VERSION=v0.10.1

export TS_ENABLE_METRICS=true
export TS_ENABLE_HEALTH_CHECK=true

shutdown() {
   echo "Shutting down tailrelay..."
   if [ -n "$WEBUI_PID" ]; then
      kill -TERM "$WEBUI_PID" 2>/dev/null
   fi
   if [ -n "$TAILSCALED_PID" ]; then
      kill -TERM "$TAILSCALED_PID" 2>/dev/null
   fi
}

echo -n "Starting tailrelay ${TAILRELAY_VERSION} with Tailscale v"
tailscale --version | head -1

# Start Tailscale daemon manually (no containerboot)
TS_STATE_DIR=${TS_STATE_DIR:-/var/lib/tailscale}
TAILSCALED_STATE="${TS_STATE_DIR%/}/tailscaled.state"
TAILSCALED_SOCKET="/var/run/tailscale/tailscaled.sock"
mkdir -p /var/run/tailscale "$TS_STATE_DIR"
echo -n "Starting tailscaled in userspace networking mode... "
# Use userspace networking to avoid requiring NET_ADMIN or /dev/net/tun
tailscaled --state="$TAILSCALED_STATE" --socket="$TAILSCALED_SOCKET" --tun=userspace-networking --socks5-server=localhost:1055 > /var/log/tailscaled.log 2>&1 &
TAILSCALED_PID=$!
if [ $? -ne 0 ]; then
   echo "failed!"
else
   echo "success! (PID: $TAILSCALED_PID)"
fi

# Start Web UI
echo -n "Starting Tailrelay Web UI... "
/usr/bin/tailrelay-webui --config /etc/tailrelay/webui.yaml > /var/log/tailrelay-webui.log 2>&1 &
WEBUI_PID=$!
if [ $? -ne 0 ]; then
   echo "failed!"
else
   echo "success! (PID: $WEBUI_PID, available at http://0.0.0.0:8021)"
fi

wait $TAILSCALED_PID $WEBUI_PID
