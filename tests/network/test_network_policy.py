"""Network policy tests — assert entrypoint script structure via testinfra."""

from pathlib import Path

import pytest

pytestmark = pytest.mark.cli

REPO_ROOT = Path(__file__).parent.parent.parent
ENTRYPOINT = str(REPO_ROOT / "scripts" / "entrypoint.sh")
SECURITY_TEST = str(REPO_ROOT / "scripts" / "test-security.sh")


# ---------------------------------------------------------------------------
# entrypoint.sh — iptables configuration
# ---------------------------------------------------------------------------

def test_entrypoint_uses_iptables_legacy(local_host):
    f = local_host.file(ENTRYPOINT)
    assert f.contains("iptables-legacy")
    assert f.contains("IPT=iptables-legacy")


def test_entrypoint_blocks_private_networks_in_restricted_mode(local_host):
    f = local_host.file(ENTRYPOINT)
    assert f.contains(r"10\.0\.0\.0/8")
    assert f.contains(r"172\.16\.0\.0/12")
    assert f.contains(r"192\.168\.0\.0/16")


def test_entrypoint_blocks_metadata_endpoint(local_host):
    assert local_host.file(ENTRYPOINT).contains(r"169\.254\.169\.254")


def test_entrypoint_allows_dns(local_host):
    assert local_host.file(ENTRYPOINT).contains("--dport 53")


def test_entrypoint_allows_https(local_host):
    assert local_host.file(ENTRYPOINT).contains("--dport 443")


def test_entrypoint_has_open_mode(local_host):
    f = local_host.file(ENTRYPOINT)
    assert f.contains("open")
    assert f.contains("unrestricted")


def test_entrypoint_has_allowlist_mode(local_host):
    f = local_host.file(ENTRYPOINT)
    assert f.contains("allowlist")
    assert f.contains("AOA_ALLOWLIST")


def test_entrypoint_defaults_to_restricted(local_host):
    # Use content_string + plain `in` — avoids BSD grep regex quirks with {}.
    content = local_host.file(ENTRYPOINT).content_string
    assert "AOA_NETWORK_MODE" in content and ":-restricted}" in content


def test_entrypoint_drops_output_by_default_in_restricted(local_host):
    assert local_host.file(ENTRYPOINT).contains("OUTPUT DROP")


def test_entrypoint_sets_loopback_accept(local_host):
    assert local_host.file(ENTRYPOINT).contains(r"-o lo -j ACCEPT")


# ---------------------------------------------------------------------------
# entrypoint.sh — capability drop (iptables rule immutability)
# ---------------------------------------------------------------------------

def test_entrypoint_exec_handoff(local_host):
    """Agent must be launched via capsh so caps are dropped at exec."""
    assert local_host.file(ENTRYPOINT).contains("exec capsh")


def test_entrypoint_drops_net_admin_cap(local_host):
    """CAP_NET_ADMIN must be in the drop list so agent cannot flush iptables."""
    assert local_host.file(ENTRYPOINT).contains("cap_net_admin")


# ---------------------------------------------------------------------------
# entrypoint.sh — file properties
# ---------------------------------------------------------------------------

def test_entrypoint_is_executable(local_host):
    assert local_host.file(ENTRYPOINT).is_file
    # host.file().mode uses Linux stat format and fails on macOS; use test -x instead
    assert local_host.run(f"test -x {ENTRYPOINT}").rc == 0, "entrypoint.sh must have execute bits set"


# ---------------------------------------------------------------------------
# test-security.sh — coverage checks
# ---------------------------------------------------------------------------

def test_security_test_script_exists(local_host):
    f = local_host.file(SECURITY_TEST)
    assert f.is_file
    assert local_host.run(f"test -s {SECURITY_TEST}").rc == 0, "test-security.sh must not be empty"


def test_security_test_covers_credential_isolation(local_host):
    f = local_host.file(SECURITY_TEST)
    assert f.contains("ssh") or f.contains("SSH")
    assert f.contains("ANTHROPIC_API_KEY")


def test_security_test_covers_vm_isolation(local_host):
    assert local_host.file(SECURITY_TEST).contains("/Users")


def test_security_test_covers_network(local_host):
    f = local_host.file(SECURITY_TEST)
    assert f.contains(r"192\.168")
    assert f.contains(r"169\.254")
