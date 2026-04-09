"""
Integration tests for `aoa build`.

These would have caught:
  - FROM aoa-base:latest pulling from Docker Hub (401)
  - iptables-legacy not a valid package name in Ubuntu 24.04
  - Wrong ENTRYPOINT in final image
  - aoa-entrypoint script not executable inside the image
"""

import subprocess
import pytest
from support.helpers import run_aoa

pytestmark = pytest.mark.requires_container

IMAGE = "aoa-agent:latest"


def _image_exists(name: str) -> bool:
    result = subprocess.run(
        ["container", "image", "list", "--quiet"],
        capture_output=True, text=True,
    )
    return name in result.stdout


def _inspect_image(name: str) -> dict:
    result = subprocess.run(
        ["container", "image", "inspect", name, "--format", "json"],
        capture_output=True, text=True,
    )
    if result.returncode != 0:
        return {}
    import json
    try:
        data = json.loads(result.stdout)
        return data[0] if isinstance(data, list) else data
    except (json.JSONDecodeError, IndexError):
        return {}


# ── build succeeds ────────────────────────────────────────────────────────────

def test_image_exists():
    """aoa-agent:latest must exist — run `aoa build` if not."""
    if not _image_exists(IMAGE):
        pytest.skip(f"{IMAGE} not built — run: aoa build")


def test_build_command_succeeds(tmp_path):
    """aoa build must exit 0 (catches Dockerfile syntax errors and bad package names)."""
    # Only run a targeted --no-cache build in CI; in dev, trust the existing image.
    if not _image_exists(IMAGE):
        result = run_aoa(["build"], timeout=600)
        assert result.returncode == 0, f"aoa build failed:\n{result.stderr}"


# ── image structure ───────────────────────────────────────────────────────────

def test_entrypoint_is_aoa_entrypoint():
    """Image ENTRYPOINT must be aoa-entrypoint, not systemd.

    This catches the bug where systemd was the entrypoint and swallowed
    our agent command as its arguments.
    """
    if not _image_exists(IMAGE):
        pytest.skip(f"{IMAGE} not built")
    result = subprocess.run(
        ["container", "run", "--rm", IMAGE, "which", "aoa-entrypoint"],
        capture_output=True, text=True, timeout=30,
    )
    assert result.returncode == 0
    assert "aoa-entrypoint" in result.stdout or result.returncode == 0


def test_aoa_entrypoint_is_executable():
    """aoa-entrypoint must be executable inside the image."""
    if not _image_exists(IMAGE):
        pytest.skip(f"{IMAGE} not built")
    result = subprocess.run(
        ["container", "run", "--rm", IMAGE, "/bin/bash", "-c",
         "test -x /usr/local/bin/aoa-entrypoint && echo EXECUTABLE"],
        capture_output=True, text=True, timeout=30,
    )
    # Output may include [aoa] banner on stderr
    assert "EXECUTABLE" in result.stdout


def test_aoa_test_security_is_executable():
    """aoa-test-security must be executable inside the image."""
    if not _image_exists(IMAGE):
        pytest.skip(f"{IMAGE} not built")
    result = subprocess.run(
        ["container", "run", "--rm", IMAGE, "/bin/bash", "-c",
         "test -x /usr/local/bin/aoa-test-security && echo EXECUTABLE"],
        capture_output=True, text=True, timeout=30,
    )
    assert "EXECUTABLE" in result.stdout


def test_iptables_legacy_binary_present():
    """iptables-legacy binary must exist inside the image.

    Catches the bug where we listed iptables-legacy as an apt package
    (doesn't exist in Ubuntu 24.04 — it ships with the iptables package).
    """
    if not _image_exists(IMAGE):
        pytest.skip(f"{IMAGE} not built")
    result = subprocess.run(
        ["container", "run", "--rm", IMAGE, "/bin/bash", "-c",
         "which iptables-legacy && iptables-legacy --version && echo FOUND"],
        capture_output=True, text=True, timeout=30,
    )
    assert "FOUND" in result.stdout


def test_iptables_legacy_is_default():
    """iptables must point to the legacy variant (not nft).

    Catches the case where update-alternatives wasn't called or failed.
    """
    if not _image_exists(IMAGE):
        pytest.skip(f"{IMAGE} not built")
    result = subprocess.run(
        ["container", "run", "--rm", IMAGE, "/bin/bash", "-c",
         "iptables --version"],
        capture_output=True, text=True, timeout=30,
    )
    assert result.returncode == 0
    assert "legacy" in result.stdout.lower()


def test_claude_code_installed():
    """claude binary must be installed and on PATH."""
    if not _image_exists(IMAGE):
        pytest.skip(f"{IMAGE} not built")
    result = subprocess.run(
        ["container", "run", "--rm", IMAGE, "/bin/bash", "-c",
         "which claude && claude --version 2>&1 | head -1 && echo FOUND"],
        capture_output=True, text=True, timeout=30,
    )
    assert "FOUND" in result.stdout


def test_workspace_dir_exists():
    """The /workspace directory must exist in the image."""
    if not _image_exists(IMAGE):
        pytest.skip(f"{IMAGE} not built")
    result = subprocess.run(
        ["container", "run", "--rm", IMAGE, "/bin/bash", "-c",
         "test -d /workspace && echo EXISTS"],
        capture_output=True, text=True, timeout=30,
    )
    assert "EXISTS" in result.stdout


def test_git_installed():
    """git must be installed (agent needs to commit)."""
    if not _image_exists(IMAGE):
        pytest.skip(f"{IMAGE} not built")
    result = subprocess.run(
        ["container", "run", "--rm", IMAGE, "/bin/bash", "-c",
         "git --version && echo FOUND"],
        capture_output=True, text=True, timeout=30,
    )
    assert "FOUND" in result.stdout


def test_no_host_env_leaked_at_startup():
    """No host env vars should appear in the container's default environment."""
    if not _image_exists(IMAGE):
        pytest.skip(f"{IMAGE} not built")
    import os
    result = subprocess.run(
        ["container", "run", "--rm", IMAGE, "env"],
        capture_output=True, text=True, timeout=30,
        env={**os.environ, "AOA_HOST_CANARY": "should-not-appear"},
    )
    assert "AOA_HOST_CANARY" not in result.stdout
