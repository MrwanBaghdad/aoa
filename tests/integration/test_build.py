"""
Integration tests for the aoa-agent image structure.

Uses testinfra via a running container so we're asserting actual system
state rather than grepping script output for sentinel strings.
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


# ---------------------------------------------------------------------------
# Build
# ---------------------------------------------------------------------------

def test_build_command_succeeds():
    """aoa build must exit 0 — catches Dockerfile syntax errors and bad package names."""
    if not _image_exists(IMAGE):
        result = run_aoa(["build"], timeout=600)
        assert result.returncode == 0, f"aoa build failed:\n{result.stderr}"


# ---------------------------------------------------------------------------
# Image structure — asserted via testinfra against a running container
# ---------------------------------------------------------------------------

def test_aoa_entrypoint_is_executable(image_host):
    assert image_host.file("/usr/local/bin/aoa-entrypoint").is_file
    assert image_host.run("test -x /usr/local/bin/aoa-entrypoint").rc == 0


def test_security_test_script_is_executable(image_host):
    assert image_host.file("/usr/local/bin/aoa-test-security").is_file
    assert image_host.run("test -x /usr/local/bin/aoa-test-security").rc == 0


def test_iptables_legacy_binary_present(image_host):
    """Catches the bug where iptables-legacy was listed as an apt package name
    (doesn't exist in Ubuntu 24.04 — it ships with the iptables package)."""
    assert image_host.file("/usr/sbin/iptables-legacy").exists


def test_iptables_legacy_is_default(image_host):
    """iptables must point to the legacy variant, not nft."""
    result = image_host.run("iptables --version")
    assert result.rc == 0
    assert "legacy" in result.stdout.lower()


def test_capsh_is_installed(image_host):
    """capsh (libcap2-bin) must be installed — required to drop CAP_NET_ADMIN."""
    result = image_host.run("capsh --version")
    assert result.rc == 0


def test_claude_code_installed(image_host):
    result = image_host.run("which claude")
    assert result.rc == 0, "claude binary not found on PATH"


def test_workspace_dir_exists(image_host):
    assert image_host.file("/workspace").is_directory


def test_git_installed(image_host):
    result = image_host.run("git --version")
    assert result.rc == 0


def test_tmux_installed(image_host):
    result = image_host.run("which tmux")
    assert result.rc == 0


def test_no_host_env_leaked(image_host):
    """Arbitrary host env vars must not appear in the container environment."""
    result = image_host.run("env")
    assert result.rc == 0
    assert "AOA_HOST_CANARY" not in result.stdout
