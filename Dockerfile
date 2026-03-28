# syntax=docker/dockerfile:1
# check=skip=SecretsUsedInArgOrEnv
ARG TAILSCALE_VERSION=v1.92.5
ARG CADDY_VERSION=2.11.2
ARG GO_VERSION=1.26.1
ARG NODE_VERSION=24
ARG ALPINE_VERSION=3.22
ARG MAILCAP_VERSION=2.1.54
ARG SOCAT_VERSION=1.8.0.3
ARG WEBUI_SOURCE=webui-builder

# Frontend build stage — Vite + Svelte + Tailwind
FROM node:${NODE_VERSION}-alpine AS frontend-builder

WORKDIR /build/webui/frontend

# Copy package files first for layer caching
COPY webui/frontend/package.json webui/frontend/package-lock.json* ./

RUN npm ci --ignore-scripts

# Copy frontend source and build
COPY webui/frontend/ ./

ARG VERSION=dev
RUN npm version --no-git-tag-version --allow-same-version ${VERSION} 2>/dev/null || true
RUN npm run build

# Caddy build stage — cloned and compiled from source at the pinned version tag
FROM golang:${GO_VERSION}-alpine AS caddy-builder

ARG CADDY_VERSION
RUN apk add --no-cache git && \
    git clone --depth 1 --branch v${CADDY_VERSION} https://github.com/caddyserver/caddy.git /caddy-src && \
    cd /caddy-src/cmd/caddy && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-w -s" -o /caddy .

# Build stage for Web UI
FROM golang:${GO_VERSION}-alpine AS webui-builder

WORKDIR /build

# Copy go mod files
COPY webui/go.mod webui/go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY webui/ ./

# Copy Vite dist output from frontend stage
COPY --from=frontend-builder /build/webui/cmd/webui/web/dist/ ./cmd/webui/web/dist/

# Build metadata arguments
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown
ARG BRANCH=unknown
ARG BUILDER=docker

# Build the Web UI binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s \
    -X main.Version=${VERSION} \
    -X main.Commit=${COMMIT} \
    -X main.BuildTime=${DATE} \
    -X main.branch=${BRANCH} \
    -X main.builtBy=${BUILDER}" \
    -o /tailrelay-webui ./cmd/webui

# Build Tailscale binaries from source at the pinned version tag
FROM golang:${GO_VERSION}-alpine AS tailscale-builder

ARG TAILSCALE_VERSION
RUN go install -ldflags="-w -s" \
      tailscale.com/cmd/tailscale@${TAILSCALE_VERSION} \
      tailscale.com/cmd/tailscaled@${TAILSCALE_VERSION} \
      tailscale.com/cmd/containerboot@${TAILSCALE_VERSION}

# Dev binary stage — copies pre-built binary from local ./data
FROM scratch AS binary-dev
COPY data/tailrelay-webui /tailrelay-webui

# Select binary source: webui-builder (default) or binary-dev (--build-arg WEBUI_SOURCE=binary-dev)
FROM ${WEBUI_SOURCE} AS binary-source

# Main image — matches the base used by the official tailscale/tailscale Docker image
ARG ALPINE_VERSION
FROM alpine:${ALPINE_VERSION}

LABEL maintainer="carlos@sudocarlos.com"

ENV RELAY_LIST=
ENV TS_HOSTNAME=
ENV TS_EXTRA_FLAGS=
ENV TS_STATE_DIR=/var/lib/tailscale/
ENV TS_AUTH_ONCE=true
ENV TS_ENABLE_METRICS=true
ENV TS_ENABLE_HEALTH_CHECK=true

ARG MAILCAP_VERSION
ARG SOCAT_VERSION
RUN apk update && \
    apk upgrade --no-cache && \
    apk add --no-cache \
      ca-certificates \
      iptables \
      iproute2 \
      ip6tables \
      mailcap~=${MAILCAP_VERSION} \
      socat~=${SOCAT_VERSION}

# Alpine 3.19+ replaced legacy iptables with nftables. Some hosts don't support
# nftables (e.g. Synology), so restore legacy symlinks for broader compat.
# See: https://github.com/tailscale/tailscale/issues/17854
RUN rm /usr/sbin/iptables && ln -s /usr/sbin/iptables-legacy /usr/sbin/iptables && \
    rm /usr/sbin/ip6tables && ln -s /usr/sbin/ip6tables-legacy /usr/sbin/ip6tables

# Tailscale binaries built from source
COPY --from=tailscale-builder /go/bin/tailscale       /usr/local/bin/tailscale
COPY --from=tailscale-builder /go/bin/tailscaled      /usr/local/bin/tailscaled
COPY --from=tailscale-builder /go/bin/containerboot   /usr/local/bin/containerboot

# Compat symlink (mirrors official tailscale/tailscale image layout)
RUN mkdir /tailscale && ln -s /usr/local/bin/containerboot /tailscale/run.sh

# Copy Caddy binary built from source
COPY --from=caddy-builder /caddy /usr/bin/caddy

# Copy Web UI binary
COPY --from=binary-source /tailrelay-webui /usr/bin/tailrelay-webui

# Copy Web UI configuration
COPY webui.yaml /etc/tailrelay/webui.yaml

COPY start.sh /usr/bin/start.sh
RUN chmod +x /usr/bin/start.sh && \
    mkdir --parents /var/run/tailscale && \
    mkdir --parents /var/lib/tailscale/backups && \
    ln -s /tmp/tailscaled.sock /var/run/tailscale/tailscaled.sock && \
    mkdir --parents /etc/caddy && \
    touch /etc/caddy/Caddyfile

# Expose Web UI port
EXPOSE 8021

CMD  [ "start.sh" ]
