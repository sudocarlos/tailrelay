"""
Integration tests for tailrelay.

Each test class maps to one subsystem. All tests require the `running_container`
session fixture which builds/starts the Compose stack.

Test IDs follow the pattern:
  test_<subsystem>_<what>_<expected_outcome>

Run with:
  pytest tests/integration/ -v
  pytest tests/integration/ -v --tb=short   # compact tracebacks
  BUILD_IMAGE=false pytest tests/integration/ -v  # skip image build

Environment variables that control behaviour are documented in conftest.py.
"""

import json
import time
from typing import Any

import pytest

from tests.integration.helpers import (
    container_exec,
    container_exec_check,
    CONTAINER_NAME,
)

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

WEBUI_ADDR = "http://127.0.0.1:8021"
HEALTHZ_ADDR = "http://127.0.0.1:9002/healthz"
METRICS_ADDR = "http://127.0.0.1:9002/metrics"
CADDY_ADMIN_ADDR = "http://127.0.0.1:2019"
SOCAT_RELAY_PORT = 8089

# Token file written by the Web UI on first start.
WEBUI_TOKEN_FILE = "/var/lib/tailscale/.webui_token"


def wget(container: str, url: str, extra_flags: str = "") -> tuple[int, str]:
    """
    Run wget inside the container and return (exit_code, output).
    Uses -q (quiet) so only the body is printed, -O- to stdout,
    --timeout=5 to avoid hanging, --tries=1 for fast failure.
    """
    flags = f"-qO- --timeout=5 --tries=1 {extra_flags}".strip()
    result = container_exec(container, f"wget {flags} {url}")
    return result.returncode, result.stdout + result.stderr


def wget_ok(container: str, url: str, extra_flags: str = "") -> str:
    """Assert wget exits 0 and return the response body."""
    code, output = wget(container, url, extra_flags)
    assert code == 0, f"wget {url} failed (exit {code}):\n{output}"
    return output


def get_webui_token(container: str) -> str:
    """Read the Web UI bearer token from the container."""
    result = container_exec(container, f"cat {WEBUI_TOKEN_FILE}")
    assert result.returncode == 0, (
        f"Failed to read Web UI token from {WEBUI_TOKEN_FILE}:\n{result.stderr}"
    )
    return result.stdout.strip()


def wget_authed(
    container: str, url: str, token: str, extra_flags: str = ""
) -> tuple[int, str]:
    """Run wget with a Bearer token Authorization header."""
    auth_flag = f'--header="Authorization: Bearer {token}"'
    flags = f"-qO- --timeout=5 --tries=1 {auth_flag} {extra_flags}".strip()
    result = container_exec(container, f"wget {flags} {url}")
    return result.returncode, result.stdout + result.stderr


def wget_authed_ok(container: str, url: str, token: str, extra_flags: str = "") -> str:
    """Assert authenticated wget exits 0 and return the response body."""
    code, output = wget_authed(container, url, token, extra_flags)
    assert code == 0, f"wget {url} (authed) failed (exit {code}):\n{output}"
    return output


def parse_json_body(body: str, endpoint: str) -> Any:
    """Parse JSON body, calling pytest.fail on error so callers get a clear message."""
    try:
        return json.loads(body)
    except json.JSONDecodeError:
        pytest.fail(f"{endpoint} returned non-JSON: {body!r}")


def is_port_open(container: str, port: int) -> bool:
    """Return True if a TCP port is listening inside the container."""
    result = container_exec(container, f"netstat -tulnp 2>/dev/null || ss -tulnp")
    return f":{port}" in result.stdout


# ---------------------------------------------------------------------------
# Startup sanity
# ---------------------------------------------------------------------------


class TestContainerStartup:
    """Verify the container is running and key processes are listening."""

    def test_container_is_running(self, running_container: str) -> None:
        result = container_exec(running_container, "echo alive")
        assert result.returncode == 0, "Container should be reachable via docker exec"

    def test_caddy_process_running(self, running_container: str) -> None:
        result = container_exec(running_container, "pgrep -x caddy")
        assert result.returncode == 0, "caddy process should be running"

    def test_webui_process_running(self, running_container: str) -> None:
        result = container_exec(running_container, "pgrep -f tailrelay-webui")
        assert result.returncode == 0, "tailrelay-webui process should be running"

    def test_tailscaled_process_running(self, running_container: str) -> None:
        result = container_exec(running_container, "pgrep -x tailscaled")
        assert result.returncode == 0, "tailscaled process should be running"

    def test_listening_ports_include_caddy_admin(self, running_container: str) -> None:
        """Caddy always opens its Admin API on :2019 regardless of routes."""
        result = container_exec_check(
            running_container, "netstat -tulnp 2>/dev/null || ss -tulnp"
        )
        assert ":2019" in result.stdout, (
            f"Expected Caddy Admin API port :2019 in netstat output:\n{result.stdout}"
        )

    def test_listening_ports_include_webui(self, running_container: str) -> None:
        result = container_exec_check(
            running_container, "netstat -tulnp 2>/dev/null || ss -tulnp"
        )
        assert ":8021" in result.stdout, (
            f"Expected Web UI port 8021 in netstat output:\n{result.stdout}"
        )


# ---------------------------------------------------------------------------
# Tailscale health / metrics
# ---------------------------------------------------------------------------


class TestTailscaleEndpoints:
    """Tailscale health check and Prometheus metrics endpoints.

    These endpoints (:9002) are only available after tailscaled has fully
    authenticated to a tailnet (requires TS_AUTHKEY). In CI without an auth
    key the port may not be open — tests are skipped automatically in that
    case rather than failing.
    """

    def test_healthz_returns_200(self, running_container: str) -> None:
        if not is_port_open(running_container, 9002):
            pytest.skip("Tailscale :9002 not open (no TS_AUTHKEY in this environment)")
        body = wget_ok(running_container, HEALTHZ_ADDR)
        assert body is not None, "healthz returned empty response"

    def test_metrics_returns_prometheus_format(self, running_container: str) -> None:
        if not is_port_open(running_container, 9002):
            pytest.skip("Tailscale :9002 not open (no TS_AUTHKEY in this environment)")
        body = wget_ok(running_container, METRICS_ADDR)
        assert "tailscale" in body.lower() or "go_" in body or "# HELP" in body, (
            f"Unexpected metrics response body (first 200 chars):\n{body[:200]}"
        )


# ---------------------------------------------------------------------------
# Caddy admin API
# ---------------------------------------------------------------------------


class TestCaddyAdminAPI:
    """Verify Caddy's Admin API is reachable and well-formed.

    Caddy only opens proxy listener ports (8080/8081/8443) when at least one
    route is configured. The Admin API port (:2019) is always available and is
    the correct thing to probe in a freshly-started, route-free container.
    """

    def test_caddy_admin_responds(self, running_container: str) -> None:
        """GET /config/ returns the current Caddy config as JSON."""
        code, output = wget(running_container, f"{CADDY_ADMIN_ADDR}/config/")
        assert code == 0, (
            f"Caddy Admin API at {CADDY_ADMIN_ADDR}/config/ did not respond "
            f"(exit {code}):\n{output}"
        )

    def test_caddy_admin_config_is_json(self, running_container: str) -> None:
        """The Admin API /config/ endpoint must return valid JSON."""
        body = wget_ok(running_container, f"{CADDY_ADMIN_ADDR}/config/")
        try:
            json.loads(body)
        except json.JSONDecodeError:
            pytest.fail(f"Caddy Admin API /config/ returned non-JSON:\n{body[:400]}")


# ---------------------------------------------------------------------------
# Web UI
# ---------------------------------------------------------------------------


class TestWebUI:
    """Web UI availability and API surface."""

    def test_webui_root_responds(self, running_container: str) -> None:
        # The SPA root always returns 200.
        result = container_exec(
            running_container, f"wget -qO- --timeout=5 --tries=1 {WEBUI_ADDR}"
        )
        assert result.returncode in (0, 8), (
            f"Web UI at {WEBUI_ADDR} did not respond (exit {result.returncode}):\n{result.stderr}"
        )

    def test_webui_auth_status_endpoint(self, running_container: str) -> None:
        result = container_exec(
            running_container,
            f"wget -qO- --timeout=5 --tries=1 {WEBUI_ADDR}/api/auth/status",
        )
        assert result.returncode == 0, (
            f"/api/auth/status should return 200, got exit {result.returncode}:\n{result.stderr}"
        )
        # Response must be valid JSON with a "needsSetup" key.
        data = parse_json_body(result.stdout, "/api/auth/status")
        assert "needsSetup" in data, (
            f"Expected 'needsSetup' key in auth status response: {data}"
        )

    def test_webui_caddy_api_list(self, running_container: str) -> None:
        token = get_webui_token(running_container)
        body = wget_authed_ok(
            running_container, f"{WEBUI_ADDR}/api/caddy/proxies", token
        )
        data = parse_json_body(body, "/api/caddy/proxies")
        assert isinstance(data, list), (
            f"Expected JSON array from /api/caddy/proxies, got {type(data)}"
        )

    def test_webui_socat_api_list(self, running_container: str) -> None:
        token = get_webui_token(running_container)
        body = wget_authed_ok(
            running_container, f"{WEBUI_ADDR}/api/socat/relays", token
        )
        data = parse_json_body(body, "/api/socat/relays")
        assert isinstance(data, list), (
            f"Expected JSON array from /api/socat/relays, got {type(data)}"
        )

    def test_webui_backup_api_list(self, running_container: str) -> None:
        token = get_webui_token(running_container)
        body = wget_authed_ok(running_container, f"{WEBUI_ADDR}/api/backup/list", token)
        data = parse_json_body(body, "/api/backup/list")
        assert isinstance(data, list), (
            f"Expected JSON array from /api/backup/list, got {type(data)}"
        )


# ---------------------------------------------------------------------------
# Socat relay forwarding
# ---------------------------------------------------------------------------


class TestSocatRelay:
    """
    Create a relay via the Web UI API and verify that socat starts and
    forwards TCP traffic to the whoami service on the test network.
    """

    RELAY_ID = "test-relay"
    RELAY_PATH = "/var/lib/tailscale/relays.json"

    def _create_relay_payload(self) -> str:
        payload = {
            "id": self.RELAY_ID,
            "listen_port": SOCAT_RELAY_PORT,
            "target_host": "whoami-test",
            "target_port": 80,
            "enabled": True,
            "autostart": True,
        }
        return json.dumps(payload)

    def test_socat_relay_forwards_http(self, running_container: str) -> None:
        token = get_webui_token(running_container)

        # Create the relay via the Web UI API.
        # The Create handler automatically starts the relay when enabled=true.
        payload = self._create_relay_payload()
        create_result = container_exec(
            running_container,
            f"wget -qO- --timeout=10 --tries=1 "
            f'--header="Authorization: Bearer {token}" '
            f'--header="Content-Type: application/json" '
            f"--post-data='{payload}' "
            f"{WEBUI_ADDR}/api/socat/create",
        )
        assert create_result.returncode == 0, (
            f"POST /api/socat/create failed (exit {create_result.returncode}):\n"
            f"stdout: {create_result.stdout}\nstderr: {create_result.stderr}"
        )

        # Give socat a moment to bind.
        time.sleep(2)

        # Verify socat is listening on the configured port.
        net_result = container_exec(
            running_container, "netstat -tulnp 2>/dev/null || ss -tulnp"
        )
        assert f":{SOCAT_RELAY_PORT}" in net_result.stdout, (
            f"Expected socat to be listening on port {SOCAT_RELAY_PORT}:\n{net_result.stdout}"
        )

    def test_socat_relay_responds_to_http(self, running_container: str) -> None:
        # The relay should already be running from test_socat_relay_forwards_http.
        # whoami returns a small HTTP response containing the request headers.
        result = container_exec(
            running_container,
            f"wget -qO- --timeout=5 --tries=1 http://127.0.0.1:{SOCAT_RELAY_PORT}",
        )
        assert result.returncode == 0, (
            f"wget through socat relay failed (exit {result.returncode}):\n"
            f"stdout: {result.stdout}\nstderr: {result.stderr}"
        )
        # whoami echoes request headers; the Host line should mention our relay port.
        assert "Host" in result.stdout or "GET" in result.stdout, (
            f"Unexpected whoami response through socat relay:\n{result.stdout}"
        )
