"""
Shared helpers for tailrelay integration tests.

This module is the single source of truth for all test utilities. It is
imported by both conftest.py (fixtures) and test_integration.py (tests).

Public API
----------
ROOT            : Path  — repo root directory
CONTAINER_NAME  : str   — Docker container name (from TAILRELAY_HOST env var)
get_env         : read an environment variable with a default
run_cmd         : run a subprocess, capturing stdout/stderr
run_cmd_check   : same, but raise RuntimeError on non-zero exit
container_exec  : run a shell command inside a Docker container
container_exec_check : same, but raise AssertionError on non-zero exit
"""

import os
import subprocess
from pathlib import Path

ROOT: Path = Path(__file__).parent.parent.parent  # repo root


def get_env(key: str, default: str) -> str:
    """Return the value of environment variable *key*, or *default*."""
    return os.environ.get(key, default)


CONTAINER_NAME: str = get_env("TAILRELAY_HOST", "tailrelay-test")


# ---------------------------------------------------------------------------
# Low-level subprocess helpers
# ---------------------------------------------------------------------------


def run_cmd(args: list[str], **kwargs) -> subprocess.CompletedProcess:
    """Run *args* as a subprocess from the repo root, capturing output."""
    return subprocess.run(
        args,
        capture_output=True,
        text=True,
        cwd=str(ROOT),
        **kwargs,
    )


def run_cmd_check(args: list[str], **kwargs) -> subprocess.CompletedProcess:
    """Like run_cmd, but raise RuntimeError on non-zero exit."""
    result = run_cmd(args, **kwargs)
    if result.returncode != 0:
        raise RuntimeError(
            f"Command failed: {' '.join(args)}\n"
            f"stdout: {result.stdout}\n"
            f"stderr: {result.stderr}"
        )
    return result


# ---------------------------------------------------------------------------
# Docker container helpers
# ---------------------------------------------------------------------------


def container_exec(container: str, cmd: str) -> subprocess.CompletedProcess:
    """Run *cmd* inside *container* via docker exec (shell -c)."""
    return run_cmd(["docker", "exec", container, "sh", "-c", cmd])


def container_exec_check(container: str, cmd: str) -> subprocess.CompletedProcess:
    """Like container_exec, but raise AssertionError on non-zero exit."""
    result = container_exec(container, cmd)
    if result.returncode != 0:
        raise AssertionError(
            f"docker exec failed (exit {result.returncode}):\n"
            f"  cmd: {cmd}\n"
            f"  stdout: {result.stdout}\n"
            f"  stderr: {result.stderr}"
        )
    return result
