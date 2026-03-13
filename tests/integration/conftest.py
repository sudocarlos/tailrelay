"""
pytest fixtures for tailrelay integration tests.

The fixture lifecycle:
  - session-scoped `docker_image` builds the dev image once per run
  - session-scoped `running_container` starts the stack, waits for services,
    and tears down after all tests complete
  - function-scoped helpers (http_get, etc.) are thin wrappers

Environment variables (from .env or shell):
  COMPOSE_FILE     path to compose file  (default: compose-test.yml)
  TAILRELAY_HOST   container name        (default: tailrelay-test)
  TAILNET_DOMAIN   tailnet domain        (default: example.com)
  BUILD_IMAGE      whether to build the image before tests (default: true)
  IMAGE_TAG        Docker image tag to use (default: sudocarlos/tailrelay:dev)
  STARTUP_WAIT     seconds to wait after container start (default: 8)
"""

import os
import subprocess
import time
from pathlib import Path
from typing import Generator

import pytest

# ---------------------------------------------------------------------------
# Configuration helpers
# ---------------------------------------------------------------------------

ROOT = Path(__file__).parent.parent.parent  # repo root


def _env(key: str, default: str) -> str:
    return os.environ.get(key, default)


COMPOSE_FILE = _env("COMPOSE_FILE", "compose-test.yml")
CONTAINER_NAME = _env("TAILRELAY_HOST", "tailrelay-test")
IMAGE_TAG = _env("IMAGE_TAG", "sudocarlos/tailrelay:dev")
BUILD_IMAGE = _env("BUILD_IMAGE", "true").lower() not in ("false", "0", "no")
STARTUP_WAIT = int(_env("STARTUP_WAIT", "8"))


def _run(args: list[str], **kwargs) -> subprocess.CompletedProcess:
    """Run a subprocess, capturing output. Raise on non-zero exit."""
    return subprocess.run(
        args,
        capture_output=True,
        text=True,
        cwd=str(ROOT),
        **kwargs,
    )


def _run_check(args: list[str], **kwargs) -> subprocess.CompletedProcess:
    result = _run(args, **kwargs)
    if result.returncode != 0:
        raise RuntimeError(
            f"Command failed: {' '.join(args)}\n"
            f"stdout: {result.stdout}\n"
            f"stderr: {result.stderr}"
        )
    return result


# ---------------------------------------------------------------------------
# Session-scoped fixtures
# ---------------------------------------------------------------------------


@pytest.fixture(scope="session")
def docker_image() -> str:
    """
    Build the dev Docker image (if BUILD_IMAGE=true) and return the tag.
    Skipped when BUILD_IMAGE is false — useful in CI where the image is
    pre-built in an earlier step.
    """
    if not BUILD_IMAGE:
        return IMAGE_TAG

    import subprocess as sp

    git_version = (
        sp.run(
            ["git", "describe", "--tags", "--always", "--dirty"],
            capture_output=True,
            text=True,
            cwd=str(ROOT),
        ).stdout.strip()
        or "dev"
    )

    git_commit = (
        sp.run(
            ["git", "rev-parse", "--short", "HEAD"],
            capture_output=True,
            text=True,
            cwd=str(ROOT),
        ).stdout.strip()
        or "none"
    )

    _run_check(
        [
            "docker",
            "buildx",
            "build",
            "--build-arg",
            f"VERSION={git_version}",
            "--build-arg",
            f"COMMIT={git_commit}",
            "-t",
            IMAGE_TAG,
            "--load",
            ".",
        ]
    )
    return IMAGE_TAG


@pytest.fixture(scope="session")
def running_container(docker_image: str) -> Generator[str, None, None]:
    """
    Start the Compose stack and yield the container name.
    Tears down the stack when the session ends.
    """
    # Ensure clean state
    _run(["docker", "compose", "-f", COMPOSE_FILE, "down", "--remove-orphans"])

    # Ensure the state dir exists (needed for volume mount)
    (ROOT / "tailscale").mkdir(exist_ok=True)

    _run_check(["docker", "compose", "-f", COMPOSE_FILE, "up", "-d"])

    time.sleep(STARTUP_WAIT)

    yield CONTAINER_NAME

    # Teardown
    _run(["docker", "compose", "-f", COMPOSE_FILE, "down", "--remove-orphans"])


# ---------------------------------------------------------------------------
# Helper: exec a command inside the container
# ---------------------------------------------------------------------------


def container_exec(container: str, cmd: str) -> subprocess.CompletedProcess:
    """Run `cmd` inside `container` via docker exec (shell -c)."""
    return _run(["docker", "exec", container, "sh", "-c", cmd])


def container_exec_check(container: str, cmd: str) -> subprocess.CompletedProcess:
    result = container_exec(container, cmd)
    if result.returncode != 0:
        raise AssertionError(
            f"docker exec failed (exit {result.returncode}):\n"
            f"  cmd: {cmd}\n"
            f"  stdout: {result.stdout}\n"
            f"  stderr: {result.stderr}"
        )
    return result
