# tailrelay

A container image designed to run on [Start9](https://start9.com) that exposes
local services to your Tailscale network, using **Caddy** as an HTTP reverse 
proxy and **socat** for other non‑HTTP protocols.

**New in v0.2.0:** Now includes a comprehensive **Web UI** for managing Tailscale, Caddy proxies, socat relays, and backups through your browser!

![](images/tailrelay.svg)

## Table of Contents

- [Why?](#why)
- [Technology Stack](#technology-stack)
- [Web UI](#web-ui)
    - [Features](#features)
    - [Access](#access)
    - [Authentication](#authentication)
- [Getting Started](#getting-started)
    - [Prerequisites](#prerequisites)
    - [Tailscale](#tailscale)
    - [Caddy (Manual Configuration)](#caddy-manual-configuration)
        - [Caddyfile](#caddyfile)
    - [Start9](#start9)
- [Testing with Docker‑Compose](#testing-with-docker-compose)


## Why?

Accessing **Start9** services such as **BTCPayServer** and **electrs RPC** requires Tor today.
**Tailscale** lets you privately and securely expose your services, while 
**Caddy** takes care of obtaining and renewing TLS certificates.
**`socat`** relays the non-HTTP ports Caddy can’t reverse‑proxy.

This container image combines them 


## Technology Stack

| Component | Purpose | Docs |
|-----------|---------|------|
| **Start9** | Local container orchestration & file persistence | [Start9 docs](https://docs.start9.com/0.3.5.x/user-manual/) |
| **Tailscale** | Zero‑configuration VPN, MagicDNS, and device authentication | [Tailscale docs](https://tailscale.com/kb) |
| **Caddy** | Modern HTTP/2 reverse proxy, automatic Let's Encrypt integration | [Caddy docs](https://caddyserver.com/docs) |
| **socat** | One‑shot TCP relay for non‑HTTP services | [socat manual](https://linux.die.net/man/1/socat) |
| **Web UI** | Browser-based management interface (Go, HTML/CSS/JS) | *See below* |


## Web UI

The Web UI provides a comprehensive browser-based interface for managing all aspects of tailrelay without manual configuration file editing.

### Features

- **Dashboard**: Overview of Tailscale connection status and system health
- **Tailscale Management**: Connect/disconnect, view peers, check status
- **Caddy Proxy Management**: Add, edit, delete, and toggle HTTP/HTTPS reverse proxies
- **Socat Relay Management**: Add, edit, delete, and manage TCP relay processes
- **Backup & Restore**: Create, download, upload, and restore configuration backups
- **Auto-configuration**: Automatically generates Caddyfile from GUI settings
- **Dark Theme**: Modern, responsive interface optimized for readability

### Access

The Web UI runs on **port 8021** by default. After starting the container:

```bash
# Access via Tailscale hostname (if HTTPS is enabled)
https://your-hostname.your-tailnet.ts.net:8021

# Or via local IP
http://localhost:8021
```

### Authentication

The Web UI uses dual authentication:

1. **Token Authentication**: A random token is generated on first startup and stored at `/var/lib/tailscale/.webui_token`. You can view it with:
   ```bash
   # For podman
   sudo podman exec start9.tailscale cat /var/lib/tailscale/.webui_token
   
   # For docker
   docker exec <container-name> cat /var/lib/tailscale/.webui_token
   ```

2. **Tailscale Network Authentication**: If you access the Web UI from a device on your Tailscale network, you're automatically authenticated (no token needed).

**Security Note**: The Web UI is designed to be accessed from your Tailscale network. If exposing port 8021 externally, ensure proper firewall rules are in place.


## Getting Started

### Prerequisites

1. A [Start9](https://docs.start9.com/0.3.5.x/user-manual/) server
2. A [Tailscale](https://tailscale.com/kb/1017/install) with an active [Tailnet](https://tailscale.com/kb/1217/tailnet-name)
3. [HTTPS certificates](https://tailscale.com/kb/1153/enabling-https) enabled
 in Tailscale **Admin console > DNS**

### Tailscale

1. Log into the Tailscale **Admin console** and click [**DNS**](https://login.tailscale.com/admin/dns)
1. Verify or set your [**Tailnet name**](https://tailscale.com/kb/1217/tailnet-name)
1. Scroll down and **Enable HTTPS** under **HTTPS Certificates**

### Caddy (Manual Configuration)

**Note**: With the Web UI, you can manage Caddy proxies through the browser instead of manually editing the Caddyfile. This section is for advanced users who prefer manual configuration.

1. Login to your Start9, see https://docs.start9.com/0.3.5.x/user-manual/ssh

        ssh start9@SERVER-HOSTNAME

1. Create a directory to persist Tailscale and Caddy files

        mkdir -p /home/start9/tailscale

1. Create the [Caddyfile](#caddyfile) below or [`Caddyfile.example`](/Caddyfile.example)

        nano /home/start9/tailscale/Caddyfile

1. ⚠️ Files are removed by Start9 on reboot. **Back up `/home/start9/tailscale`** ⚠️

#### Caddyfile

```
# Caddyfile
start9.your-tailnet.ts.net:21000 {
	reverse_proxy https://lnd.embassy:8080 {
		header_up Host {upstream_hostport}
		transport http {
			tls_trust_pool file /var/lib/tailscale/tls.cert
		}
	}
}

start9.your-tailnet.ts.net:21001 {
	reverse_proxy http://mempool.embassy:8080 {
		header_up Host {upstream_hostport}
	}
}

start9.your-tailnet.ts.net:21002 {
	reverse_proxy http://btcpayserver.embassy:80 {
		header_up Host {upstream_hostport}
		trusted_proxies private_ranges
	}
}

start9.your-tailnet.ts.net:21003 {
	reverse_proxy http://jam.embassy:80 {
		header_up Host {upstream_hostport}
	}
}

```
- Replace `start9` with the Tailscale machine name you want,
 this must match `-e TS_HOSTNAME=` in `podman run`
- Replace `your-tailnet.ts.net` with your [**Tailnet name**](https://tailscale.com/kb/1217/tailnet-name)
- This example lists some common services. Feel free to discover and add more
- See https://caddyserver.com/docs/caddyfile/patterns#reverse-proxy for more info

⚠️ Files are removed by Start9 on reboot. **Back up `/home/start9/tailscale`** ⚠️


### Start9

1. Run the container with Web UI port exposed:

```bash
sudo podman run --name start9.tailscale \
 -v /home/start9/tailscale/:/var/lib/tailscale \
 -v /home/start9/tailscale/Caddyfile:/etc/caddy/Caddyfile \
 -e TS_HOSTNAME=start9 \
 -e RELAY_LIST=50001:electrs.embassy:50001,21004:lnd.embassy:10009 \
 -p 8021:8021 \
 --net start9 \
 docker.io/sudocarlos/tailrelay:latest
```

**Environment Variables:**
- `TS_HOSTNAME` - your desired Tailnet machine name. This should match in your [Caddyfile](#caddyfile)
- `RELAY_LIST` - **(Optional, deprecated)** comma‑separated `listener_port:target_host:target_port` pairs for socat listeners  
  Example: `50001:electrs.embassy:50001,21004:lnd.embassy:10009`  
  **Recommendation**: Use the Web UI to manage relays instead.

**Volume Mounts:**
- `-v /home/start9/tailscale/:/var/lib/tailscale` - Tailscale state, Web UI configs, backups
- `-v /home/start9/tailscale/Caddyfile:/etc/caddy/Caddyfile` - Caddy configuration (optional if using Web UI)

**Port Mappings:**
- `-p 8021:8021` - Web UI (add this if you want browser access)
- Additional ports: Add `-p` flags for any custom Caddy proxies or socat relays

**Network:**
- `--net start9` - Required to access Start9 services

See https://tailscale.com/kb/1282/docker for more info on Tailscale Docker options.

2. View the Web UI authentication token:

```bash
sudo podman exec start9.tailscale cat /var/lib/tailscale/.webui_token
```

3. Access the Web UI at `http://localhost:8021` or `https://start9.your-tailnet.ts.net:8021`


## Testing with Docker‑Compose

The repository includes two helper scripts that build the image, launch a test
environment with `docker‑compose`, run a series of health checks, and then shut
down the containers again.

```.env.example```
This file contains the environment variables required for the test container
to connect to a running Tailscale network.  
Copy it to a local ```.env``` file and edit the values as needed, e.g.:

```bash
cp .env.example .env
# Edit variables (TAILRELAY_HOST, TAILNET_DOMAIN, COMPOSE_FILE)
```

Once the `.env` file is set, any of the following scripts will pick it up:
   * `docker-compose-test.py` – pure Python implementation (requires `docker`
     and `docker‑compose` Python packages).
   * `docker-compose-test.sh` – Bash wrapper that reads the same environment
     variables.

```bash
# From the repository root
# 1. Test with Python script
python docker-compose-test.py

# 2. Test with Bash script
./docker-compose-test.sh
```

## Troubleshooting

### Web UI Not Accessible

1. Check if the container is running:
   ```bash
   sudo podman ps | grep tailscale
   ```

2. Verify the Web UI port is mapped:
   ```bash
   sudo podman port start9.tailscale
   ```

3. Check Web UI logs:
   ```bash
   sudo podman logs start9.tailscale | grep -i webui
   ```

4. Verify the Web UI is listening inside the container:
   ```bash
   sudo podman exec start9.tailscale netstat -tulnp | grep 8021
   ```

### Cannot Log In to Web UI

1. Retrieve the authentication token:
   ```bash
   sudo podman exec start9.tailscale cat /var/lib/tailscale/.webui_token
   ```

2. If accessing from a Tailscale device, ensure you're using the Tailscale IP or hostname

3. Clear browser cache/cookies and try again

### Caddy Proxy Not Working

1. Check Caddyfile syntax in Web UI or manually:
   ```bash
   sudo podman exec start9.tailscale caddy validate --config /etc/caddy/Caddyfile
   ```

2. Verify Caddy is running:
   ```bash
   sudo podman exec start9.tailscale caddy list-modules
   ```

3. Check Caddy logs:
   ```bash
   sudo podman logs start9.tailscale | grep -i caddy
   ```

### Socat Relay Not Working

1. Check relay status in Web UI or manually:
   ```bash
   sudo podman exec start9.tailscale ps aux | grep socat
   ```

2. Verify listening ports:
   ```bash
   sudo podman exec start9.tailscale netstat -tulnp | grep socat
   ```

3. Test connectivity to target service:
   ```bash
   sudo podman exec start9.tailscale nc -zv target-host target-port
   ```

## Version History

- **v0.2.0** (2026-01-26)
  - Added comprehensive Web UI for browser-based management
  - Web UI features: Dashboard, Tailscale control, Caddy proxy management, socat relay management, backup/restore
  - Auto-migration from RELAY_LIST environment variable to JSON configuration
  - Dual authentication (token + Tailscale network)
  - Dark theme with responsive design

- **v0.1.1** (Previous)
  - Initial release with Tailscale, Caddy, and socat integration
  - Manual Caddyfile configuration
  - Environment variable-based socat relay configuration

## Contributing

Contributions are welcome! Please submit issues or pull requests on the GitHub repository.

## License

This project is open source. See the repository for license details.