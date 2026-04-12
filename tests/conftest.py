"""Pytest configuration and shared fixtures for aoa tests."""

from __future__ import annotations

import json
import os
import subprocess
import time
import uuid
from pathlib import Path
from typing import Generator

import pytest
import testinfra
import testinfra.backend.base

from support.helpers import (
    cleanup_sessions,
    get_aoa_binary,
    get_session_dir,
    make_workspace,
)


# ---------------------------------------------------------------------------
# Custom testinfra backend for apple/container
# ---------------------------------------------------------------------------

class ContainerExecBackend(testinfra.backend.base.BaseBackend):
    """Runs testinfra commands via `container exec <id>`.

    apple/container has the same exec interface as docker but uses a different
    binary name, so we can't use testinfra's built-in docker backend directly.
    """

    NAME = "container-exec"

    def __init__(self, container_id: str, *args, **kwargs):
        self._container_id = container_id
        super().__init__(container_id, *args, **kwargs)

    def run(self, command: str, *args, **kwargs):
        cmd = ["container", "exec", self._container_id, "sh", "-c", command]
        r = subprocess.run(cmd, capture_output=True, timeout=30)
        return self.result(r.returncode, command, r.stdout, r.stderr)


# ---------------------------------------------------------------------------
# Session-scoped fixtures
# ---------------------------------------------------------------------------

@pytest.fixture(scope="session")
def local_host():
    """Testinfra host on the local machine — for asserting source file structure."""
    return testinfra.get_host("local://")


@pytest.fixture(scope="module")
def image_host(has_container_runtime):
    """Testinfra host running inside aoa-agent:latest.

    Starts a single long-lived container per test module, yields a testinfra
    Host backed by `container exec`, then tears the container down.
    """
    if not has_container_runtime:
        pytest.skip("apple/container not installed")

    image = "aoa-agent:latest"
    result = subprocess.run(
        ["container", "image", "list", "--quiet"],
        capture_output=True, text=True,
    )
    if image not in result.stdout:
        pytest.skip(f"{image} not built — run: aoa build")

    name = f"testinfra-{uuid.uuid4().hex[:8]}"
    subprocess.run(
        ["container", "run", "-d", "--name", name, "--entrypoint", "sleep", image, "3600"],
        check=True, capture_output=True,
    )

    backend = ContainerExecBackend(name)
    host = testinfra.host.Host(backend)

    yield host

    subprocess.run(["container", "stop", name], capture_output=True)
    subprocess.run(["container", "rm", name], capture_output=True)


@pytest.fixture(scope="session")
def aoa_binary() -> str:
    """Resolve the aoa binary once per test session."""
    try:
        binary = get_aoa_binary()
    except FileNotFoundError as e:
        pytest.skip(str(e))
    return binary


@pytest.fixture(scope="session")
def has_container_runtime() -> bool:
    """True if apple/container CLI is available."""
    result = subprocess.run(["which", "container"], capture_output=True)
    return result.returncode == 0


# ---------------------------------------------------------------------------
# Per-test fixtures
# ---------------------------------------------------------------------------

@pytest.fixture
def workspace_dir(tmp_path: Path) -> Path:
    """A fresh workspace directory for each test."""
    return make_workspace(tmp_path)


@pytest.fixture
def git_workspace(tmp_path: Path) -> Path:
    """A fresh workspace with an initialized git repo."""
    return make_workspace(tmp_path, git=True)


@pytest.fixture(autouse=True)
def clean_test_sessions(tmp_path: Path) -> Generator:
    """Remove session state created during a test."""
    yield
    # Clean up sessions for any workspace under this test's tmp_path
    session_dir = get_session_dir()
    if not session_dir.exists():
        return
    import json
    for f in session_dir.glob("*.json"):
        try:
            data = json.loads(f.read_text())
            ws = data.get("workspace_dir", "")
            if ws and str(tmp_path) in ws:
                f.unlink(missing_ok=True)
        except (json.JSONDecodeError, KeyError):
            pass


# ---------------------------------------------------------------------------
# Markers
# ---------------------------------------------------------------------------

def pytest_configure(config: pytest.Config) -> None:
    config.addinivalue_line("markers", "requires_container: test requires apple/container installed")
    config.addinivalue_line("markers", "slow: test is slow (spawns real containers)")
    config.addinivalue_line("markers", "cli: test only exercises the CLI binary (no container needed)")


def pytest_collection_modifyitems(config: pytest.Config, items: list[pytest.Item]) -> None:
    # Load optional skip list
    skip_file = Path(__file__).parent / "pytest_skip_list.txt"
    if skip_file.exists():
        skipped = {line.strip() for line in skip_file.read_text().splitlines() if line.strip() and not line.startswith("#")}
    else:
        skipped = set()

    for item in items:
        if item.nodeid in skipped:
            item.add_marker(pytest.mark.skip(reason="listed in pytest_skip_list.txt"))

        # Auto-skip container tests if runtime not available
        if item.get_closest_marker("requires_container"):
            result = subprocess.run(["which", "container"], capture_output=True)
            if result.returncode != 0:
                item.add_marker(pytest.mark.skip(reason="apple/container not installed"))


# ---------------------------------------------------------------------------
# Hooks
# ---------------------------------------------------------------------------

def pytest_runtest_makereport(item: pytest.Item, call):
    """Append duration to test report for slow test visibility."""
    pass  # pytest handles this natively with -v


def pytest_sessionfinish(session: pytest.Session, exitstatus: int) -> None:
    """Clean up any orphaned session state after the test run."""
    cleanup_sessions()
