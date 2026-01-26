# syntax=docker/dockerfile:1
# check=skip=SecretsUsedInArgOrEnv
ARG TAILSCALE_VERSION=v1.92.5
ARG TAILRELAY_VERSION=v0.2.0
ARG GO_VERSION=1.21

# Build stage for Web UI
FROM golang:${GO_VERSION}-alpine AS webui-builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY webui/go.mod webui/go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY webui/ ./

# Build the Web UI binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o tailrelay-webui ./cmd/webui

# Main image
FROM tailscale/tailscale:$TAILSCALE_VERSION

LABEL maintainer="carlos@sudocarlos.com"

ENV RELAY_LIST=
ENV TS_HOSTNAME=
ENV TS_EXTRA_FLAGS=
ENV TS_STATE_DIR=/var/lib/tailscale/ 
ENV TS_AUTH_ONCE=true
ENV TS_ENABLE_METRICS=true
ENV TS_ENABLE_HEALTH_CHECK=true

RUN apk update && \
    apk upgrade --no-cache && \
    apk add --no-cache ca-certificates mailcap caddy socat && \
    caddy upgrade

# Copy Web UI binary from builder
COPY --from=webui-builder /build/tailrelay-webui /usr/bin/tailrelay-webui

# Copy Web UI configuration
COPY webui.yaml /etc/tailrelay/webui.yaml

COPY start.sh /usr/bin/start.sh
RUN chmod +x /usr/bin/start.sh && \
    mkdir --parents /var/run/tailscale && \
    mkdir --parents /var/lib/tailscale/backups && \
    ln -s /tmp/tailscaled.sock /var/run/tailscale/tailscaled.sock && \
    touch /etc/caddy/Caddyfile

# Expose Web UI port
EXPOSE 8021

CMD  [ "start.sh" ]
