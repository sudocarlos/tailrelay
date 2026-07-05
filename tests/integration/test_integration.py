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
RELAY_PORT = 8089

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

    def test_webui_process_running(self, running_container: str) -> None:
        result = container_exec(running_container, "pgrep -f tailrelay-webui")
        assert result.returncode == 0, "tailrelay-webui process should be running"

    def test_tailscaled_process_running(self, running_container: str) -> None:
        result = container_exec(running_container, "pgrep -x tailscaled")
        assert result.returncode == 0, "tailscaled process should be running"

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

    def test_webui_https_api_list(self, running_container: str) -> None:
        token = get_webui_token(running_container)
        body = wget_authed_ok(
            running_container, f"{WEBUI_ADDR}/api/serve/https/list", token
        )
        data = parse_json_body(body, "/api/serve/https/list")
        assert isinstance(data, list), (
            f"Expected JSON array from /api/serve/https/list, got {type(data)}"
        )

    def test_webui_tcp_api_list(self, running_container: str) -> None:
        token = get_webui_token(running_container)
        body = wget_authed_ok(
            running_container, f"{WEBUI_ADDR}/api/serve/tcp/list", token
        )
        data = parse_json_body(body, "/api/serve/tcp/list")
        assert isinstance(data, list), (
            f"Expected JSON array from /api/serve/tcp/list, got {type(data)}"
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


class TestTCPRelay:
    """
    Create a TCP relay via the Web UI API and verify the relay appears in the list.
    Uses the new /api/serve/tcp/* endpoints backed by `tailscale serve`.
    """

    RELAY_ID = "test-relay"

    def _create_relay_payload(self) -> str:
        payload = {
            "id": self.RELAY_ID,
            "type": "tcp",
            "listen_port": RELAY_PORT,
            "target_host": "whoami-test",
            "target_port": 80,
            "enabled": True,
            "autostart": True,
        }
        return json.dumps(payload)

    def test_tcp_relay_create_and_list(self, running_container: str) -> None:
        token = get_webui_token(running_container)

        # Create the relay via the Web UI API.
        payload = self._create_relay_payload()
        create_result = container_exec(
            running_container,
            f"wget -qO- --timeout=10 --tries=1 "
            f'--header="Authorization: Bearer {token}" '
            f'--header="Content-Type: application/json" '
            f"--post-data='{payload}' "
            f"{WEBUI_ADDR}/api/serve/tcp/create",
        )
        assert create_result.returncode == 0, (
            f"POST /api/serve/tcp/create failed (exit {create_result.returncode}):\n"
            f"stdout: {create_result.stdout}\nstderr: {create_result.stderr}"
        )

        # Give tailscale serve reconcile a moment.
        time.sleep(2)

        # Verify relay appears in API list after creation.
        body = wget_authed_ok(
            running_container, f"{WEBUI_ADDR}/api/serve/tcp/list", token
        )
        data = parse_json_body(body, "/api/serve/tcp/list")
        ids = [item.get("relay", {}).get("id") for item in data]
        assert self.RELAY_ID in ids, f"Created relay {self.RELAY_ID} not found in API list"

    def test_tcp_relay_delete_removes_from_list(self, running_container: str) -> None:
        """Create a relay, delete it, and verify it no longer appears in the list."""
        token = get_webui_token(running_container)

        # Ensure the relay exists first.
        payload = self._create_relay_payload()
        container_exec(
            running_container,
            f"wget -qO- --timeout=10 --tries=1 "
            f'--header="Authorization: Bearer {token}" '
            f'--header="Content-Type: application/json" '
            f"--post-data='{payload}' "
            f"{WEBUI_ADDR}/api/serve/tcp/create",
        )
        time.sleep(1)

        # Delete the relay.
        delete_result = container_exec(
            running_container,
            f"wget -qO- --timeout=10 --tries=1 "
            f'--header="Authorization: Bearer {token}" '
            f"--post-data='' "
            f'"{WEBUI_ADDR}/api/serve/tcp/delete?id={self.RELAY_ID}"',
        )
        assert delete_result.returncode == 0, (
            f"DELETE /api/serve/tcp/delete failed (exit {delete_result.returncode}):\n"
            f"stdout: {delete_result.stdout}\nstderr: {delete_result.stderr}"
        )

        # Verify relay no longer appears in API list.
        body = wget_authed_ok(
            running_container, f"{WEBUI_ADDR}/api/serve/tcp/list", token
        )
        data = parse_json_body(body, "/api/serve/tcp/list")
        ids = [item.get("relay", {}).get("id") for item in data]
        assert self.RELAY_ID not in ids, (
            f"Deleted relay {self.RELAY_ID} still appears in API list"
        )


# ---------------------------------------------------------------------------
# Funnel relays
# ---------------------------------------------------------------------------


class TestFunnelRelay:
    """
    Create a Funnel relay via the Web UI API and verify it appears in the
    list. Uses /api/serve/funnel/* endpoints backed by `tailscale funnel`.

    Live public exposure requires the tailnet policy file to grant the
    `funnel` node attribute, which is outside the container's control, so
    these tests only assert config persistence and API behavior.
    """

    FUNNEL_ID = "test-funnel"
    FUNNEL_PORT = 10000

    def _create_funnel_payload(self, port: int = FUNNEL_PORT) -> str:
        payload = {
            "id": self.FUNNEL_ID,
            "type": "funnel",
            "funnel_transport": "https",
            "listen_port": port,
            "target_host": "whoami-test",
            "target_port": 80,
            "enabled": True,
            "autostart": True,
        }
        return json.dumps(payload)

    def test_funnel_relay_create_and_list(self, running_container: str) -> None:
        token = get_webui_token(running_container)

        payload = self._create_funnel_payload()
        create_result = container_exec(
            running_container,
            f"wget -qO- --timeout=10 --tries=1 "
            f'--header="Authorization: Bearer {token}" '
            f'--header="Content-Type: application/json" '
            f"--post-data='{payload}' "
            f"{WEBUI_ADDR}/api/serve/funnel/create",
        )
        assert create_result.returncode == 0, (
            f"POST /api/serve/funnel/create failed (exit {create_result.returncode}):\n"
            f"stdout: {create_result.stdout}\nstderr: {create_result.stderr}"
        )

        time.sleep(2)

        body = wget_authed_ok(
            running_container, f"{WEBUI_ADDR}/api/serve/funnel/list", token
        )
        data = parse_json_body(body, "/api/serve/funnel/list")
        ids = [item.get("id") for item in data]
        assert self.FUNNEL_ID in ids, f"Created funnel {self.FUNNEL_ID} not found in API list"

    def test_funnel_relay_delete_removes_from_list(self, running_container: str) -> None:
        """Create a funnel, delete it, and verify it no longer appears in the list."""
        token = get_webui_token(running_container)

        payload = self._create_funnel_payload()
        container_exec(
            running_container,
            f"wget -qO- --timeout=10 --tries=1 "
            f'--header="Authorization: Bearer {token}" '
            f'--header="Content-Type: application/json" '
            f"--post-data='{payload}' "
            f"{WEBUI_ADDR}/api/serve/funnel/create",
        )
        time.sleep(1)

        delete_result = container_exec(
            running_container,
            f"wget -qO- --timeout=10 --tries=1 "
            f'--header="Authorization: Bearer {token}" '
            f"--post-data='' "
            f'"{WEBUI_ADDR}/api/serve/funnel/delete?id={self.FUNNEL_ID}"',
        )
        assert delete_result.returncode == 0, (
            f"DELETE /api/serve/funnel/delete failed (exit {delete_result.returncode}):\n"
            f"stdout: {delete_result.stdout}\nstderr: {delete_result.stderr}"
        )

        body = wget_authed_ok(
            running_container, f"{WEBUI_ADDR}/api/serve/funnel/list", token
        )
        data = parse_json_body(body, "/api/serve/funnel/list")
        ids = [item.get("id") for item in data]
        assert self.FUNNEL_ID not in ids, (
            f"Deleted funnel {self.FUNNEL_ID} still appears in API list"
        )

    def test_funnel_relay_rejects_disallowed_port(self, running_container: str) -> None:
        """Funnel only permits ports 443, 8443, and 10000; other ports must be rejected."""
        token = get_webui_token(running_container)

        payload = self._create_funnel_payload(port=9999)
        result = container_exec(
            running_container,
            f"wget -qO- --server-response --timeout=10 --tries=1 "
            f'--header="Authorization: Bearer {token}" '
            f'--header="Content-Type: application/json" '
            f"--post-data='{payload}' "
            f"{WEBUI_ADDR}/api/serve/funnel/create 2>&1",
        )
        assert "400" in result.stdout or "400" in result.stderr, (
            f"Expected 400 Bad Request for disallowed funnel port, got:\n"
            f"stdout: {result.stdout}\nstderr: {result.stderr}"
        )


# ---------------------------------------------------------------------------
# Legacy endpoint shims
# ---------------------------------------------------------------------------


class TestLegacyEndpointShims:
    """Removed /api/caddy/* and /api/socat/* endpoints return 410 Gone."""

    def _wget_status(self, container: str, url: str, token: str) -> int:
        """Return the HTTP status code from wget's server-response header."""
        result = container_exec(
            container,
            f"wget -S --spider --timeout=5 --tries=1 "
            f'--header="Authorization: Bearer {token}" '
            f"{url} 2>&1",
        )
        output = result.stdout + result.stderr
        for line in output.splitlines():
            line = line.strip()
            if line.startswith("HTTP/") and len(line.split()) >= 2:
                try:
                    return int(line.split()[1])
                except ValueError:
                    pass
        return -1

    def test_caddy_endpoint_returns_410(self, running_container: str) -> None:
        token = get_webui_token(running_container)
        status = self._wget_status(
            running_container,
            f"{WEBUI_ADDR}/api/caddy/list",
            token,
        )
        assert status == 410, (
            f"Expected 410 Gone from /api/caddy/list, got {status}"
        )

    def test_socat_endpoint_returns_410(self, running_container: str) -> None:
        token = get_webui_token(running_container)
        status = self._wget_status(
            running_container,
            f"{WEBUI_ADDR}/api/socat/list",
            token,
        )
        assert status == 410, (
            f"Expected 410 Gone from /api/socat/list, got {status}"
        )

