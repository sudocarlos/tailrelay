# tailrelay

A Docker container that exposes local services to your Tailscale network. Combines **Tailscale VPN**, **Tailscale Serve** (HTTPS + TCP relays), and a **Web UI** for browser-based management.

[![Docker Pulls](https://img.shields.io/docker/pulls/sudocarlos/tailrelay)](https://hub.docker.com/r/sudocarlos/tailrelay)
[![GitHub Release](https://img.shields.io/github/v/release/sudocarlos/tailrelay)](https://github.com/sudocarlos/tailrelay/releases)
[![License](https://img.shields.io/github/license/sudocarlos/tailrelay)](https://github.com/sudocarlos/tailrelay/blob/main/LICENSE)

📖 **[Full documentation and API reference](https://sudocarlos.github.io/tailrelay/)**

## Quick Start

```bash
docker pull sudocarlos/tailrelay:latest
# or: docker pull ghcr.io/sudocarlos/tailrelay:latest

docker run -d --name tailrelay \
  -v /path/to/data:/var/lib/tailscale \
  -e TS_HOSTNAME=myserver \
  -p 8021:8021 \
  --net bridge \
  sudocarlos/tailrelay:latest
```

Open [http://localhost:8021](http://localhost:8021) and follow the Tailscale login link.

---

- [Issues & Pull Requests](https://github.com/sudocarlos/tailrelay) — contributions welcome
- [CHANGELOG](CHANGELOG.md) — release history
- [License](LICENSE) — BSD 3-Clause
