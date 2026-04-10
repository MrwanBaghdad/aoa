"""Security configuration tests — Dockerfiles, protected paths, and audit log."""

from pathlib import Path

import pytest

pytestmark = pytest.mark.cli

REPO_ROOT = Path(__file__).parent.parent.parent
DOCKERFILE_BASE = str(REPO_ROOT / "images" / "Dockerfile.base")
DOCKERFILE_AGENT = str(REPO_ROOT / "images" / "Dockerfile.agent")
PATHS_GO = str(REPO_ROOT / "internal" / "security" / "paths.go")
AUDIT_GO = str(REPO_ROOT / "internal" / "security" / "audit.go")


# ---------------------------------------------------------------------------
# Dockerfile.base — security configuration baked into the base image
# ---------------------------------------------------------------------------

def test_base_dockerfile_uses_iptables_legacy(local_host):
    assert local_host.file(DOCKERFILE_BASE).contains("iptables-legacy")


def test_base_dockerfile_sets_iptables_alternative(local_host):
    """iptables-legacy must be set as the system default via update-alternatives."""
    f = local_host.file(DOCKERFILE_BASE)
    assert f.contains("update-alternatives")
    assert f.contains("iptables-legacy")


def test_base_dockerfile_installs_libcap2_bin(local_host):
    """libcap2-bin (capsh) must be installed — required for capability dropping."""
    assert local_host.file(DOCKERFILE_BASE).contains("libcap2-bin")


def test_base_dockerfile_has_workspace_dir(local_host):
    assert local_host.file(DOCKERFILE_BASE).contains("/workspace")


def test_base_dockerfile_uses_systemd_entrypoint(local_host):
    """systemd as PID 1 gives full OS behaviour inside the VM."""
    f = local_host.file(DOCKERFILE_BASE)
    assert f.contains("systemd")
    assert f.contains("ENTRYPOINT")


# ---------------------------------------------------------------------------
# Dockerfile.agent — agent image structure
# ---------------------------------------------------------------------------

def test_agent_dockerfile_copies_entrypoint(local_host):
    f = local_host.file(DOCKERFILE_AGENT)
    assert f.contains("entrypoint.sh")
    assert f.contains("aoa-entrypoint")


def test_agent_dockerfile_copies_security_tests(local_host):
    assert local_host.file(DOCKERFILE_AGENT).contains("test-security.sh")


def test_agent_dockerfile_installs_claude_code(local_host):
    assert local_host.file(DOCKERFILE_AGENT).contains("@anthropic-ai/claude-code")


# ---------------------------------------------------------------------------
# internal/security/paths.go — protected volume mounts
# ---------------------------------------------------------------------------

def test_protected_paths_source_exists(local_host):
    assert local_host.file(PATHS_GO).is_file


def test_protected_paths_covers_git_hooks(local_host):
    assert local_host.file(PATHS_GO).contains(r"\.git/hooks")


def test_protected_paths_marks_read_only(local_host):
    f = local_host.file(PATHS_GO)
    assert f.contains("ReadOnly") or f.contains(":ro")


def test_protected_paths_volume_args(local_host):
    f = local_host.file(PATHS_GO)
    assert f.contains("ToVolumeArgs")
    assert f.contains(":ro")


# ---------------------------------------------------------------------------
# internal/security/audit.go — JSONL audit log
# ---------------------------------------------------------------------------

def test_audit_source_exists(local_host):
    assert local_host.file(AUDIT_GO).is_file


def test_audit_uses_jsonl(local_host):
    f = local_host.file(AUDIT_GO)
    assert f.contains("json") or f.contains("jsonl")


def test_audit_has_severity_levels(local_host):
    f = local_host.file(AUDIT_GO)
    for level in ["INFO", "LOW", "MEDIUM", "HIGH", "CRITICAL"]:
        assert f.contains(level), f"Audit log missing severity level: {level}"


def test_audit_has_session_id(local_host):
    f = local_host.file(AUDIT_GO)
    assert f.contains("SessionID") or f.contains("session_id")
