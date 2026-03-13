"""
pytest fixtures for tailrelay integration tests.

The fixture lifecycle:
  - session-scoped `docker_image` builds the dev image once per run
  - session-scoped `running_container` starts the stack, waits for services,
    and tears down after all tests complete

Environment variables (from .env or shell):
  COMPOSE_FILE     path to compose file  (default: compose-test.yml)
  TAILRELAY_HOST   container name        (default: tailrelay-test)
  TAILNET_DOMAIN   tailnet domain        (default: example.com)
  BUILD_IMAGE      whether to build the image before tests (default: true)
  IMAGE_TAG        Docker image tag to use (default: sudocarlos/tailrelay:dev)
  STARTUP_WAIT     seconds to wait after container start (default: 8)
"""

import time
from typing import Generator

import pytest

from tests.integration.helpers import (
    ROOT,
    CONTAINER_NAME,
    get_env,
    run_cmd,
    run_cmd_check,
)

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------

COMPOSE_FILE = get_env("COMPOSE_FILE", "compose-test.yml")
IMAGE_TAG = get_env("IMAGE_TAG", "sudocarlos/tailrelay:dev")
BUILD_IMAGE = get_env("BUILD_IMAGE", "true").lower() not in ("false", "0", "no")
STARTUP_WAIT = int(get_env("STARTUP_WAIT", "8"))


# ---------------------------------------------------------------------------
# Session-scoped fixtures
# ---------------------------------------------------------------------------


@pytest.fixture(scope="session")
def docker_image() -> str:
    """
    Build the dev Docker image (if BUILD_IMAGE=true) and return the tag.
    Set BUILD_IMAGE=false to skip the build — useful in CI where the image
    is pre-built in an earlier step.
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

    run_cmd_check(
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
    run_cmd(["docker", "compose", "-f", COMPOSE_FILE, "down", "--remove-orphans"])

    # Ensure the state dir exists (needed for volume mount)
    (ROOT / "tailscale").mkdir(exist_ok=True)

    run_cmd_check(["docker", "compose", "-f", COMPOSE_FILE, "up", "-d"])

    time.sleep(STARTUP_WAIT)

    yield CONTAINER_NAME

    # Teardown
    run_cmd(["docker", "compose", "-f", COMPOSE_FILE, "down", "--remove-orphans"])
