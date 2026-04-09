"""Tests for protected path logic and the security test suite script."""

from pathlib import Path
import pytest

REPO_ROOT = Path(__file__).parent.parent.parent

pytestmark = pytest.mark.cli


# ---------------------------------------------------------------------------
# Dockerfile checks — security configuration baked into images
# ---------------------------------------------------------------------------

def test_base_dockerfile_uses_iptables_legacy():
    content = (REPO_ROOT / "images" / "Dockerfile.base").read_text()
    assert "iptables-legacy" in content


def test_base_dockerfile_sets_alternative():
    """iptables-legacy must be set as the system default."""
    content = (REPO_ROOT / "images" / "Dockerfile.base").read_text()
    assert "update-alternatives" in content
    assert "iptables-legacy" in content


def test_agent_dockerfile_copies_entrypoint():
    content = (REPO_ROOT / "images" / "Dockerfile.agent").read_text()
    assert "entrypoint.sh" in content
    assert "aoa-entrypoint" in content


def test_agent_dockerfile_copies_security_tests():
    content = (REPO_ROOT / "images" / "Dockerfile.agent").read_text()
    assert "test-security.sh" in content


def test_agent_dockerfile_installs_claude_code():
    content = (REPO_ROOT / "images" / "Dockerfile.agent").read_text()
    assert "@anthropic-ai/claude-code" in content


def test_base_dockerfile_has_workspace_dir():
    content = (REPO_ROOT / "images" / "Dockerfile.base").read_text()
    assert "/workspace" in content


def test_base_dockerfile_uses_systemd_entrypoint():
    """systemd as PID 1 gives full OS behavior inside the VM."""
    content = (REPO_ROOT / "images" / "Dockerfile.base").read_text()
    assert "systemd" in content
    assert "ENTRYPOINT" in content


# ---------------------------------------------------------------------------
# Protected paths logic
# ---------------------------------------------------------------------------

def test_protected_paths_source_exists():
    assert (REPO_ROOT / "internal" / "security" / "paths.go").exists()


def test_protected_paths_covers_git_hooks():
    content = (REPO_ROOT / "internal" / "security" / "paths.go").read_text()
    assert ".git/hooks" in content


def test_protected_paths_marks_read_only():
    content = (REPO_ROOT / "internal" / "security" / "paths.go").read_text()
    assert "ReadOnly" in content or "ro" in content


def test_protected_paths_volume_args():
    content = (REPO_ROOT / "internal" / "security" / "paths.go").read_text()
    assert "ToVolumeArgs" in content
    assert ":ro" in content


# ---------------------------------------------------------------------------
# Audit log
# ---------------------------------------------------------------------------

def test_audit_source_exists():
    assert (REPO_ROOT / "internal" / "security" / "audit.go").exists()


def test_audit_uses_jsonl():
    content = (REPO_ROOT / "internal" / "security" / "audit.go").read_text()
    assert "json" in content.lower()
    assert "jsonl" in content.lower() or ".jsonl" in content


def test_audit_has_severity_levels():
    content = (REPO_ROOT / "internal" / "security" / "audit.go").read_text()
    for level in ["INFO", "LOW", "MEDIUM", "HIGH", "CRITICAL"]:
        assert level in content


def test_audit_has_session_id():
    content = (REPO_ROOT / "internal" / "security" / "audit.go").read_text()
    assert "SessionID" in content or "session_id" in content
