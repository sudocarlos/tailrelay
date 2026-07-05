---
id: intro
title: Introduction
---

# tailrelay

A Docker container that exposes local services to your Tailscale network.
Combines **Tailscale VPN**, **Tailscale Serve** (HTTPS + TCP relays), **Tailscale
Funnel** (public relays), and a **Web UI** for browser-based management.

This site documents how to deploy, configure, and develop tailrelay, and hosts
the generated [API Reference](/api/) sourced directly from the project's
OpenAPI specification.

## Features

- **Browser-Based Management** — a responsive Web UI running on port `8021` to
  manage your relays.
- **Automatic TLS** — Tailscale Serve terminates TLS for HTTPS relays with
  automatic MagicDNS hostnames.
- **HTTPS Relays** — configure HTTPS reverse proxies/relays through the UI.
- **TCP Relays** — forward non-HTTP protocols through Tailscale Serve.
- **Funnel** — expose a service to the public internet on port `443`, `8443`,
  or `10000` via Tailscale Funnel.
- **Backup & Restore** — save, download, upload, and restore configurations.
- **Dual Authentication** — authenticate via Tailscale network identity (peer
  IP) or secure token.
- **Multi-Platform** — multi-arch Docker images built for `amd64` and `arm64`.

This makes tailrelay useful for exposing local or self-hosted services (like
BTCPayServer, LND, electrs, and Mempool) to your Tailnet securely without Tor.

## Web UI

The frontend is a single-page application built with **Svelte 5** (runes
mode), **Tailwind CSS v4**, and **Vite**. All assets (JS, CSS, icons) are
bundled locally for zero external CDN requests at runtime.

- **Dashboard** — real-time Tailscale status, search filtering, and system
  health info.
- **Tailscale Management** — easily connect, disconnect, or deauthorize the
  node.
- **Relay Configuration** — add, edit, delete, toggle, and auto-reconcile
  HTTPS and TCP relays.
- **Funnel Configuration** — dedicated dashboard section for the three
  funnel-eligible ports (443, 8443, 10000), showing each as configured,
  in-use-by-a-relay, or available to configure.
- **Live Log Viewer** — streaming container logs (SSE) with live log-level
  control.
- **UX Conveniences** — keyboard shortcuts (`n` for new relay, `r` to
  refresh, `b` for backups, `l` for logs, `t` for theme), local-storage
  persisted dark mode.

Continue to [Getting Started](./getting-started.md) to deploy your first
container, or jump straight to the [API Reference](/api/).
