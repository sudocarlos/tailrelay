# Agent Development Guide

This guide provides coding agents with essential information for working with the tailrelay codebase.

## Project Overview

**tailrelay** is a Docker container that combines Tailscale, Caddy, and socat to expose local services (particularly Start9 services) to a Tailscale network. The project includes:

- Docker image building with Tailscale, Caddy, and socat
- Shell script orchestration for service startup
- Python-based integration testing
- Docker Compose for development and testing

## Build, Test & Development Commands

### Build Docker Image

```bash
# Build development image
docker buildx build -t sudocarlos/tailrelay:dev --load .

# Build production image (with version tag)
docker buildx build -t sudocarlos/tailrelay:latest .
```

### Run Tests

```bash
# Full integration test suite (Python)
python docker-compose-test.py

# Full integration test suite (Bash)
./docker-compose-test.sh

# Prerequisites: Create .env file from template
cp .env.example .env
# Edit .env with your TAILRELAY_HOST, TAILNET_DOMAIN, COMPOSE_FILE
```

### Development Environment

```bash
# Start test environment
docker compose -f compose-test.yml up -d

# View logs
docker compose -f compose-test.yml logs tailrelay-test

# Stop test environment
docker compose -f compose-test.yml down

# Check listening ports
docker exec -it tailrelay-test netstat -tulnp | grep LISTEN

# Manual health checks
curl -sSL http://tailrelay-dev:8080  # Test HTTP proxy
curl -sSL http://tailrelay-dev:9002/healthz  # Health endpoint
curl -sSL http://tailrelay-dev:9002/metrics  # Metrics endpoint
```

### Running Single Tests

Since this project uses integration tests rather than unit tests, there's no single test runner. Tests are curl-based health checks defined in the test scripts:

```bash
# Run a single health check manually
curl -sSL http://${TAILRELAY_HOST}:8080 && echo success || echo fail
curl -sSL http://${TAILRELAY_HOST}:9002/healthz && echo success || echo fail
```

## Code Style Guidelines

### Shell Scripts (Bash/sh)

**File Extensions & Shebangs:**
- Use `.sh` extension for shell scripts
- Use `#!/usr/bin/env bash` for Bash scripts
- Use `#!/bin/ash` for Alpine Linux scripts (Dockerfile entrypoint)

**Style Conventions:**
- Use 4-space indentation (not tabs)
- Variable names in UPPER_SNAKE_CASE for environment variables
- Variable names in lower_snake_case for local variables
- Use `${VAR}` syntax for variable expansion (prefer braces)
- Quote all variables: `"$VAR"` unless word splitting is intended
- Use `set -e` for error handling (fail fast)
- Use `set -x` for debugging (show commands)

**Error Handling:**
```bash
# Check command exit codes
if [ $? -ne 0 ]; then
    echo "failed!"
    exit 1
fi

# Alternative with conditional execution
command || { echo "failed!"; exit 1; }
```

**Comments:**
- Use `#` for single-line comments
- Add descriptive comments for complex logic
- Document environment variables at the top of scripts

### Python Scripts

**Imports:**
- Standard library imports first
- Third-party imports second
- Separate groups with blank lines
- Alphabetically sorted within groups when practical

```python
import subprocess
import time
import sys
from pathlib import Path
from typing import List, Tuple

from dotenv import load_dotenv
```

**Type Hints:**
- Use type hints for function parameters and return values
- Use `typing` module types: `List`, `Tuple`, `Dict`, etc.
- Example: `def run(cmd: str, *, capture_output=False) -> Tuple[int, str, str]:`

**Naming Conventions:**
- Variables and functions: `lower_snake_case`
- Constants: `UPPER_SNAKE_CASE`
- Classes: `PascalCase`
- Private functions/vars: prefix with `_`

**String Formatting:**
- Prefer f-strings for string interpolation
- Example: `f"❌ Build failed:\n{err}"`

**Error Handling:**
- Use try-except blocks for subprocess timeouts
- Return error codes instead of raising exceptions when appropriate
- Provide descriptive error messages with emoji indicators (✅, ❌, ⚠️)

**Docstrings:**
- Use triple-quoted strings for function documentation
- Include parameter and return value descriptions

```python
def run(cmd: str, *, capture_output=False, timeout=None) -> Tuple[int, str, str]:
    """Run a shell command and return (returncode, stdout, stderr).
    If the process times out, return rc=124 and a timeout message instead of raising."""
```

### Dockerfile

**Best Practices:**
- Use `ARG` for build-time variables
- Use `ENV` for runtime variables
- Combine `RUN` commands to reduce layers
- Use `--no-cache` with apk/apt for smaller images
- Add `LABEL` for maintainer information
- Use multi-stage builds when appropriate

**Version Pinning:**
- Pin versions for base images: `tailscale/tailscale:$TAILSCALE_VERSION`
- Pin versions for Alpine packages when stability is critical

### Caddyfile Configuration

**Style:**
- Use tabs for indentation (Caddy convention)
- One site block per listening address
- Group related directives together
- Always specify full domain with port: `host.domain.ts.net:port`

**Common Patterns:**
```
hostname.tailnet.ts.net:port {
    reverse_proxy target:port {
        header_up Host {upstream_hostport}
        trusted_proxies private_ranges
    }
}
```

## Environment Variables

**Required Variables:**
- `TS_HOSTNAME` - Tailscale machine name (must match Caddyfile)
- `TS_STATE_DIR` - Tailscale state directory (default: `/var/lib/tailscale/`)

**Optional Variables:**
- `RELAY_LIST` - Comma-separated socat relay definitions: `port:host:port`
  - Example: `50001:electrs.embassy:50001,21004:lnd.embassy:10009`
- `TS_EXTRA_FLAGS` - Additional Tailscale flags
- `TS_AUTH_ONCE` - Authenticate once (default: `true`)
- `TS_ENABLE_METRICS` - Enable metrics endpoint (default: `true`)
- `TS_ENABLE_HEALTH_CHECK` - Enable health check endpoint (default: `true`)

**Test Environment Variables (.env file):**
- `TAILRELAY_HOST` - Test container hostname
- `TAILNET_DOMAIN` - Tailscale domain for testing
- `COMPOSE_FILE` - Path to docker-compose test file

## File Structure

```
.
├── Dockerfile              # Multi-service container definition
├── start.sh                # Container entrypoint script
├── Caddyfile.example       # Example Caddy configuration
├── docker-compose-test.py  # Python integration tests
├── docker-compose-test.sh  # Bash integration tests
├── compose-test.yml        # Docker Compose test configuration
├── requirements.txt        # Python dependencies (python-dotenv)
├── .env.example            # Environment variable template
└── README.md               # User documentation
```

## Testing Strategy

**Integration Tests:**
- Build Docker image with dev tag
- Start containers with docker-compose
- Wait for services to initialize (3 second delay)
- Run health checks via curl
- Verify listening ports
- Clean up containers

**Health Check Endpoints:**
- HTTP proxy: `:8080`, `:8081` (proxied services)
- HTTPS with TLS: `:8443`
- Tailscale health: `:9002/healthz`
- Tailscale metrics: `:9002/metrics`

## Common Pitfalls & Notes

1. **File Persistence**: Start9 removes files on reboot - always backup `/home/start9/tailscale`
2. **Hostname Matching**: `TS_HOSTNAME` must match the hostname in Caddyfile
3. **Tailnet Domain**: Must be exact Tailnet name from Tailscale admin console
4. **RELAY_LIST Format**: Strict format `port:host:port` - parsing is fragile
5. **Docker Network**: Use `--net start9` for Start9 deployments
6. **TLS Certificates**: Require HTTPS enabled in Tailscale admin console
7. **Container Execution**: Uses `exec` to replace shell with containerboot (PID 1 handling)

## Version Information

- Current Version: `v0.1.1` (defined in Dockerfile ARG and start.sh)
- Tailscale Base: `v1.92.5` (Dockerfile ARG)
- Alpine Linux with Caddy and socat

## Making Changes

When modifying the project:

1. Update version in both `Dockerfile` ARG and `start.sh`
2. Test with both Python and Bash test scripts
3. Verify all health check endpoints respond
4. Update README.md if user-facing changes
5. Rebuild image before testing: `docker buildx build`
6. Test in isolated network: use compose-test.yml
