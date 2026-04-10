"""
Integration tests for the aoa-agent image structure.

Uses testinfra via a running container so we're asserting actual system
state rather than grepping script output for sentinel strings.
"""

import shutil
import subprocess
import tempfile
from pathlib import Path
import pytest
from support.helpers import run_aoa

REPO_ROOT = Path(__file__).parent.parent.parent

pytestmark = pytest.mark.requires_container

IMAGE = "aoa-agent:latest"


@pytest.fixture(scope="module", autouse=True)
def built_image():
    """Always build aoa-agent:latest before any test in this module runs."""
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


# ---------------------------------------------------------------------------
# CVE scanning — Trivy against the built image tarball
# ---------------------------------------------------------------------------

def test_image_has_no_critical_or_high_cves():
    """aoa-agent:latest must have no unfixed CRITICAL or HIGH CVEs.

    Exports the image as an OCI tarball via `container image save`, then
    scans it with Trivy. Findings are suppressed via .trivyignore — each
    suppressed entry must have a reason comment and an expiry date.
    """
    trivy = shutil.which("trivy")
    if trivy is None:
        pytest.fail(
            "trivy is not installed. Add it with: devbox install\n"
            "Or: brew install trivy"
        )

    with tempfile.NamedTemporaryFile(suffix=".tar", delete=False) as f:
        tarball = f.name

    try:
        save = subprocess.run(
            ["container", "image", "save", IMAGE, "--output", tarball],
            capture_output=True, text=True, timeout=120,
        )
        assert save.returncode == 0, f"container image save failed:\n{save.stderr}"

        scan = subprocess.run(
            [
                trivy, "image",
                "--input", tarball,
                "--severity", "CRITICAL,HIGH",
                "--exit-code", "1",
                "--ignorefile", str(REPO_ROOT / ".trivyignore"),
                "--no-progress",
                "--format", "table",
            ],
            capture_output=True, text=True, timeout=300,
        )

        if scan.returncode != 0:
            pytest.fail(
                f"Trivy found CRITICAL or HIGH CVEs in {IMAGE}:\n\n"
                f"{scan.stdout}\n{scan.stderr}\n\n"
                f"To suppress a finding, add an entry to .trivyignore with a\n"
                f"reason comment and an expiry date."
            )
    finally:
        Path(tarball).unlink(missing_ok=True)
