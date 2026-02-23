#!/usr/bin/env python3
# compose_test.py

import subprocess
import time
import sys
from pathlib import Path
from typing import List, Tuple
import os
import json
from dotenv import load_dotenv  # <‑‑ added

# Load environment variables from .env
load_dotenv()
# Use these env vars instead of hard‑coded host/domain
TAILRELAY_HOST = os.getenv("TAILRELAY_HOST", "tailrelay-dev")
TAILNET_DOMAIN = os.getenv("TAILNET_DOMAIN", "my-tailnet.ts.net")
COMPOSE_FILE = os.getenv("COMPOSE_FILE", "./compose-test.yml")

# --------------------------------------------------------------------------- #
# Helper functions

def run(cmd: str, *, capture_output=False, timeout=None) -> Tuple[int, str, str]:
    """Run a shell command and return (returncode, stdout, stderr).
    If the process times out, return rc=124 and a timeout message instead of raising."""
    try:
        result = subprocess.run(
            cmd,
            shell=True,
            capture_output=capture_output,
            text=True,
            timeout=timeout,
        )
        return result.returncode, result.stdout or "", result.stderr or ""
    except subprocess.TimeoutExpired:
        # Return a non‑zero rc and a descriptive stderr; stdout is empty
        return 124, "", f"Timeout expired for command: {cmd}"

def docker_compose(cmd: str, compose_file:str=COMPOSE_FILE) -> Tuple[int, str, str]:
    return run(f"docker compose -f {compose_file} {cmd}", capture_output=True)

def docker_build() -> Tuple[int, str, str]:
    return run("docker buildx build -t sudocarlos/tailrelay:dev --load .", capture_output=True)

# --------------------------------------------------------------------------- #
# 1. Clean start

rc, _, _ = docker_compose("down")
if rc:
    print("⚠️  docker compose down failed – continuing anyway")

mkdir_rc, _, _ = run("mkdir -p tailscale")

# 2. Build image

print("\nBuilding image…")
rc, out, err = docker_build()
if rc:
    print(f"❌ Build failed:\n{err}")
    sys.exit(1)
print(out.strip())

# 3. Start containers

print("\nStarting containers…")
rc, out, err = docker_compose("up -d")
if rc:
    print(f"❌ docker compose up failed:\n{err}")
    sys.exit(1)

# give the containers a moment to spin up
print("\nWaiting for container to start…")
time.sleep(3)

# 4. (Optional) Show logs & listening sockets

print("\nContainer logs tail (last 10 lines):")
_, logs, _ = docker_compose("logs tailrelay-test | tail")
print(logs)

print("\nListening sockets:")
_, sockets, _ = run("docker exec tailrelay-test netstat -tulnp | grep LISTEN", capture_output=True)
print(sockets)

# --------------------------------------------------------------------------- #
# 5. Curl tests

curl_tests = [
    ("http://127.0.0.1:8080",   "Health / 8080"),
    ("http://127.0.0.1:8081",   "Health / 8081"),
    ("https://127.0.0.1:8443", "TLS / 8443"),
    ("http://127.0.0.1:9002/healthz",   "Health endpoint / 9002"),
    ("http://127.0.0.1:9002/metrics",   "Metrics endpoint / 9002"),
]

results: List[Tuple[str, str, str]] = []

for url, desc in curl_tests:
    rc, _, _ = run(f"docker exec tailrelay-test wget -qO- --no-check-certificate {url}", capture_output=True, timeout=10)
    status = "✅ success" if rc == 0 else "❌ fail"
    results.append((desc, url, status))

# --------------------------------------------------------------------------- #
# 5b. Socat tests

print("\nSetting up Socat relay...")
# Create a socat relay: Listen on 8089 -> whoami-test:80
relay_config = {
    "relays": [
        {
            "id": "test-relay",
            "listen_port": 8089,
            "target_host": "whoami-test",
            "target_port": 80,
            "enabled": True,
            "autostart": True
        }
    ]
}

# Write config to mapped volume
relays_file = Path("tailscale/relays.json")
try:
    with open(relays_file, "w") as f:
        json.dump(relay_config, f, indent=2)
    print(f"Created {relays_file}")
except Exception as e:
    print(f"❌ Failed to write relays file: {e}")
    sys.exit(1)

# Restart container to pick up config
print("Restarting tailrelay container to apply config...")
run("docker restart tailrelay-test")
time.sleep(5) # Wait for startup

print("Testing socat relay connection...")
# We run wget inside the container to test the local listener
rc, out, err = run("docker exec tailrelay-test wget -qO- http://127.0.0.1:8089", capture_output=True)
if rc == 0 and "Host: 127.0.0.1:8089" in out:
     print("✅ Socat relay working (found 'Host: 127.0.0.1:8089')")
     results.append(("Socat Relay / 8089", "http://127.0.0.1:8089", "✅ success"))
else:
     print(f"❌ Socat relay failed. RC={rc}, Out='{out}', Err='{err}'")
     results.append(("Socat Relay / 8089", "http://127.0.0.1:8089", "❌ fail"))

# --------------------------------------------------------------------------- #
# 6. Pretty‑print the table

def format_table(rows: List[Tuple[str, str, str]]) -> str:
    # Determine column widths
    widths = [max(len(row[i]) for row in rows) for i in range(3)]
    header = ("Description", "URL", "Result")
    header_widths = [len(header[i]) for i in range(3)]
    widths = [max(widths[i], header_widths[i]) for i in range(3)]

    sep = "+-" + "-+-".join("-" * w for w in widths) + "-+"
    # ────────────────←←──────←←──────←←──────
    # Use a single format string instead of nested f‑strings
    row_fmt = "| {{:{}}} | {{:{}}} | {{:{}}} |".format(widths[0], widths[1], widths[2])

    lines = [sep,
             row_fmt.format(*header),
             sep]
    for r in rows:
        lines.append(row_fmt.format(*r))
    lines.append(sep)
    return "\n".join(lines)

print("\nCurl test results:")
print(format_table(results))

# --------------------------------------------------------------------------- #
# 7. Clean shutdown

print("\nShutting down containers…")
docker_compose("down")

print("\nAll done!")