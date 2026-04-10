"""Unit-style tests for network policy script generation (no container needed)."""

import subprocess
import sys
from pathlib import Path

import pytest


# Import the network policy logic by invoking a small Go test helper.
# For pure Python validation we test the generated shell script content directly.

REPO_ROOT = Path(__file__).parent.parent.parent


def _get_network_script(mode: str, allowlist=None) -> str:
    """
    Call a small Go helper that prints the network policy script for a given mode.
    Falls back to reading scripts/entrypoint.sh for pattern validation.
    """
    # We validate via the entrypoint script patterns
    entrypoint = (REPO_ROOT / "scripts" / "entrypoint.sh").read_text()
    return entrypoint


pytestmark = pytest.mark.cli


def test_entrypoint_uses_iptables_legacy():
    """Must use iptables-legacy — nftables not supported in apple/container VMs."""
    script = (REPO_ROOT / "scripts" / "entrypoint.sh").read_text()
    assert "iptables-legacy" in script
    # Must NOT set iptables (non-legacy) as the command
    assert "IPT=iptables-legacy" in script


def test_entrypoint_blocks_private_networks_in_restricted_mode():
    script = (REPO_ROOT / "scripts" / "entrypoint.sh").read_text()
    assert "10.0.0.0/8" in script
    assert "172.16.0.0/12" in script
    assert "192.168.0.0/16" in script


def test_entrypoint_blocks_metadata_endpoint():
    script = (REPO_ROOT / "scripts" / "entrypoint.sh").read_text()
    assert "169.254.169.254" in script


def test_entrypoint_allows_dns():
    script = (REPO_ROOT / "scripts" / "entrypoint.sh").read_text()
    assert "--dport 53" in script


def test_entrypoint_allows_https():
    script = (REPO_ROOT / "scripts" / "entrypoint.sh").read_text()
    assert "--dport 443" in script


def test_entrypoint_has_open_mode():
    script = (REPO_ROOT / "scripts" / "entrypoint.sh").read_text()
    assert "open" in script
    assert "unrestricted" in script


def test_entrypoint_has_allowlist_mode():
    script = (REPO_ROOT / "scripts" / "entrypoint.sh").read_text()
    assert "allowlist" in script
    assert "AOA_ALLOWLIST" in script


def test_entrypoint_defaults_to_restricted():
    script = (REPO_ROOT / "scripts" / "entrypoint.sh").read_text()
    # The default case (*)  should apply restricted rules
    assert 'AOA_NETWORK_MODE="${AOA_NETWORK_MODE:-restricted}"' in script


def test_entrypoint_drops_output_by_default_in_restricted():
    script = (REPO_ROOT / "scripts" / "entrypoint.sh").read_text()
    assert "OUTPUT DROP" in script


def test_entrypoint_sets_loopback_accept():
    script = (REPO_ROOT / "scripts" / "entrypoint.sh").read_text()
    assert "-o lo -j ACCEPT" in script


def test_entrypoint_exec_handoff():
    """Script must exec the agent via capsh to drop caps before handoff."""
    script = (REPO_ROOT / "scripts" / "entrypoint.sh").read_text()
    assert "exec capsh" in script


def test_entrypoint_drops_net_admin_cap():
    """CAP_NET_ADMIN must be dropped so the agent cannot modify iptables rules."""
    script = (REPO_ROOT / "scripts" / "entrypoint.sh").read_text()
    assert "cap_net_admin" in script


def test_entrypoint_is_executable():
    import os
    path = REPO_ROOT / "scripts" / "entrypoint.sh"
    assert os.access(path, os.X_OK), "entrypoint.sh must be executable"


def test_security_test_script_exists():
    path = REPO_ROOT / "scripts" / "test-security.sh"
    assert path.exists()
    assert path.read_text().strip() != ""


def test_security_test_covers_credential_isolation():
    script = (REPO_ROOT / "scripts" / "test-security.sh").read_text()
    assert "SSH" in script or "ssh" in script
    assert "ANTHROPIC_API_KEY" in script


def test_security_test_covers_vm_isolation():
    script = (REPO_ROOT / "scripts" / "test-security.sh").read_text()
    assert "/Users" in script


def test_security_test_covers_network():
    script = (REPO_ROOT / "scripts" / "test-security.sh").read_text()
    assert "192.168" in script
    assert "169.254" in script
