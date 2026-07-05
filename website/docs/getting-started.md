---
id: getting-started
title: Getting Started
---

# Getting Started

## Quick Start

```bash
# Pull the image
docker pull sudocarlos/tailrelay:latest

# Run the container
docker run -d --name tailrelay \
  -v /path/to/data:/var/lib/tailscale \
  -e TS_HOSTNAME=myserver \
  -p 8021:8021 \
  --net bridge \
  sudocarlos/tailrelay:latest

# Access the Web UI and follow the Tailscale login link
open http://localhost:8021
```

## Prerequisites

1. A Tailscale account with an active Tailnet ([tailscale.com](https://tailscale.com))
2. [HTTPS certificates enabled](https://tailscale.com/kb/1153/enabling-https) in the Tailscale Admin console
3. Docker or Podman installed

## Tailscale Setup

1. Log into the Tailscale Admin console and open [DNS](https://login.tailscale.com/admin/dns) to enable MagicDNS.
   - Tailnets created on or after October 20, 2022 have MagicDNS enabled by default.
2. Review [MagicDNS](https://tailscale.com/kb/1081/magicdns) to understand how it works.
3. Verify or set your [Tailnet name](https://tailscale.com/kb/1217/tailnet-name).
4. Scroll down and enable HTTPS under HTTPS Certificates.

## StartOS Deployment

tailrelay is available as a StartOS package via
[sudocarlos/tailrelay-startos](https://github.com/sudocarlos/tailrelay-startos).

**Sideloading:**

1. Download the latest `tailrelay.s9pk` from the
   [tailrelay-startos releases page](https://github.com/sudocarlos/tailrelay-startos/releases),
   or clone the repo and run `make` to build it yourself.
2. In the StartOS web UI menu, navigate to **System → Sideload Service**.
3. Drag and drop or select the `tailrelay.s9pk` file to install.
4. Once installed, navigate to **Services → Tailrelay** and click **Start**.

## Next Steps

- [Authentication](./authentication.md) — how the Web UI is secured.
- [API Reference](/docs/api/) — the full HTTP/JSON API.
- [Troubleshooting](./troubleshooting.md) — common issues and fixes.
