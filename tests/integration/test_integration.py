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

import pytest

from conftest import container_exec, container_exec_check, CONTAINER_NAME

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

WEBUI_ADDR = "http://127.0.0.1:8021"
HEALTHZ_ADDR = "http://127.0.0.1:9002/healthz"
METRICS_ADDR = "http://127.0.0.1:9002/metrics"
CADDY_HTTP_ADDR = "http://127.0.0.1:8080"
CADDY_HTTP2_ADDR = "http://127.0.0.1:8081"
CADDY_HTTPS_ADDR = "https://127.0.0.1:8443"
SOCAT_RELAY_PORT = 8089


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

    def test_listening_ports_include_caddy(self, running_container: str) -> None:
        result = container_exec_check(
            running_container, "netstat -tulnp 2>/dev/null || ss -tulnp"
        )
        listening = result.stdout
        assert ":8080" in listening or ":8081" in listening, (
            f"Expected Caddy listen ports in netstat output:\n{listening}"
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
    """Tailscale health check and Prometheus metrics endpoints."""

    def test_healthz_returns_200(self, running_container: str) -> None:
        body = wget_ok(running_container, HEALTHZ_ADDR)
        # tailscale /healthz returns plain text or JSON; just check it's reachable
        assert body is not None, "healthz returned empty response"

    def test_metrics_returns_prometheus_format(self, running_container: str) -> None:
        body = wget_ok(running_container, METRICS_ADDR)
        # Prometheus format always starts metric lines with "# HELP" or a metric name
        assert "tailscale" in body.lower() or "go_" in body or "# HELP" in body, (
            f"Unexpected metrics response body (first 200 chars):\n{body[:200]}"
        )


# ---------------------------------------------------------------------------
# Caddy proxy ports
# ---------------------------------------------------------------------------


class TestCaddyPorts:
    """Verify Caddy is listening and responding on its configured ports."""

    def test_http_port_8080_responds(self, running_container: str) -> None:
        # Caddy with no routes returns a 4xx/5xx but still connects.
        # We only check that wget can connect (exit 0 = success, 8 = server error is fine).
        result = container_exec(
            running_container, f"wget -qO- --timeout=5 --tries=1 {CADDY_HTTP_ADDR}"
        )
        # wget exits 8 for server-issued errors (40x/50x) — connection worked.
        assert result.returncode in (0, 8), (
            f"Expected wget to connect to {CADDY_HTTP_ADDR} (exit 0 or 8), "
            f"got {result.returncode}:\n{result.stderr}"
        )

    def test_http_port_8081_responds(self, running_container: str) -> None:
        result = container_exec(
            running_container, f"wget -qO- --timeout=5 --tries=1 {CADDY_HTTP2_ADDR}"
        )
        assert result.returncode in (0, 8), (
            f"Expected wget to connect to {CADDY_HTTP2_ADDR} (exit 0 or 8), "
            f"got {result.returncode}:\n{result.stderr}"
        )

    def test_https_port_8443_responds(self, running_container: str) -> None:
        result = container_exec(
            running_container,
            f"wget -qO- --timeout=5 --tries=1 --no-check-certificate {CADDY_HTTPS_ADDR}",
        )
        assert result.returncode in (0, 8), (
            f"Expected wget to connect to {CADDY_HTTPS_ADDR} (exit 0 or 8), "
            f"got {result.returncode}:\n{result.stderr}"
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
        try:
            data = json.loads(result.stdout)
        except json.JSONDecodeError:
            pytest.fail(f"/api/auth/status returned non-JSON: {result.stdout!r}")
        assert "needsSetup" in data, (
            f"Expected 'needsSetup' key in auth status response: {data}"
        )

    def test_webui_caddy_api_list(self, running_container: str) -> None:
        result = container_exec(
            running_container,
            f"wget -qO- --timeout=5 --tries=1 {WEBUI_ADDR}/api/caddy",
        )
        assert result.returncode == 0, (
            f"/api/caddy should return 200, got exit {result.returncode}:\n{result.stderr}"
        )
        # Should be a JSON array (may be empty).
        try:
            data = json.loads(result.stdout)
        except json.JSONDecodeError:
            pytest.fail(f"/api/caddy returned non-JSON: {result.stdout!r}")
        assert isinstance(data, list), (
            f"Expected JSON array from /api/caddy, got {type(data)}"
        )

    def test_webui_socat_api_list(self, running_container: str) -> None:
        result = container_exec(
            running_container,
            f"wget -qO- --timeout=5 --tries=1 {WEBUI_ADDR}/api/socat",
        )
        assert result.returncode == 0, (
            f"/api/socat should return 200, got exit {result.returncode}:\n{result.stderr}"
        )
        try:
            data = json.loads(result.stdout)
        except json.JSONDecodeError:
            pytest.fail(f"/api/socat returned non-JSON: {result.stdout!r}")
        assert isinstance(data, list), (
            f"Expected JSON array from /api/socat, got {type(data)}"
        )

    def test_webui_backup_api_list(self, running_container: str) -> None:
        result = container_exec(
            running_container,
            f"wget -qO- --timeout=5 --tries=1 {WEBUI_ADDR}/api/backup",
        )
        assert result.returncode == 0, (
            f"/api/backup should return 200, got exit {result.returncode}:\n{result.stderr}"
        )
        try:
            data = json.loads(result.stdout)
        except json.JSONDecodeError:
            pytest.fail(f"/api/backup returned non-JSON: {result.stdout!r}")
        assert isinstance(data, list), (
            f"Expected JSON array from /api/backup, got {type(data)}"
        )


# ---------------------------------------------------------------------------
# Socat relay forwarding
# ---------------------------------------------------------------------------


class TestSocatRelay:
    """
    Write a relay config into the container and verify that socat starts
    and forwards TCP traffic to the whoami service on the test network.
    """

    RELAY_JSON = json.dumps(
        {
            "relays": [
                {
                    "id": "test-relay",
                    "listen_port": SOCAT_RELAY_PORT,
                    "target_host": "whoami-test",
                    "target_port": 80,
                    "enabled": True,
                    "autostart": True,
                }
            ]
        }
    )
    RELAY_PATH = "/var/lib/tailscale/relays.json"

    def test_socat_relay_forwards_http(self, running_container: str) -> None:
        # Write relay config and restart the container to trigger autostart.
        container_exec_check(
            running_container,
            f"echo '{self.RELAY_JSON}' > {self.RELAY_PATH}",
        )

        # Restart container to pick up the new relay config.
        result = container_exec(running_container, "kill -HUP 1")
        # HUP may not be honoured; fall back to a short sleep for socat to start.
        time.sleep(3)

        # Verify socat is listening on the configured port.
        net_result = container_exec(
            running_container, "netstat -tulnp 2>/dev/null || ss -tulnp"
        )
        assert f":{SOCAT_RELAY_PORT}" in net_result.stdout, (
            f"Expected socat to be listening on port {SOCAT_RELAY_PORT}:\n{net_result.stdout}"
        )

    def test_socat_relay_responds_to_http(self, running_container: str) -> None:
        # The relay should already be running from test_socat_relay_forwards_http.
        # whoami returns a small HTTP response containing the request Host header.
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
