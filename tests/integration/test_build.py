"""
Integration tests for the aoa-agent image build.
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
        pytest.skip("trivy is not installed")

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
