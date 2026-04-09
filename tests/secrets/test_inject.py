"""Tests for secret injection via tmpfiles."""

import os
import stat
import tempfile
from pathlib import Path

import pytest


# We test the inject logic by invoking a Go test binary or by importing
# patterns from the source. For Python-level tests we validate the behaviour
# indirectly through the CLI and directly through file-level checks.

REPO_ROOT = Path(__file__).parent.parent.parent

pytestmark = pytest.mark.cli


# ---------------------------------------------------------------------------
# secretspec.toml parsing / structure tests
# ---------------------------------------------------------------------------

def test_example_secretspec_toml_exists():
    path = REPO_ROOT / "config" / "secretspec.toml"
    assert path.exists()


def test_example_secretspec_has_project_section():
    path = REPO_ROOT / "config" / "secretspec.toml"
    content = path.read_text()
    assert "[project]" in content
    assert "name" in content


def test_example_secretspec_has_default_profile():
    content = (REPO_ROOT / "config" / "secretspec.toml").read_text()
    assert "[profiles.default" in content


def test_example_secretspec_has_anthropic_key():
    content = (REPO_ROOT / "config" / "secretspec.toml").read_text()
    assert "ANTHROPIC_API_KEY" in content


def test_example_secretspec_has_required_field():
    content = (REPO_ROOT / "config" / "secretspec.toml").read_text()
    assert "required = true" in content


def test_example_secretspec_has_development_profile():
    content = (REPO_ROOT / "config" / "secretspec.toml").read_text()
    assert "[profiles.development" in content


def test_example_secretspec_has_default_value():
    content = (REPO_ROOT / "config" / "secretspec.toml").read_text()
    assert "default = " in content


# ---------------------------------------------------------------------------
# default config validation
# ---------------------------------------------------------------------------

def test_default_config_toml_exists():
    assert (REPO_ROOT / "config" / "default.toml").exists()


def test_default_config_has_image():
    content = (REPO_ROOT / "config" / "default.toml").read_text()
    assert "image" in content
    assert "aoa-agent" in content


def test_default_config_has_network_mode():
    content = (REPO_ROOT / "config" / "default.toml").read_text()
    assert "mode" in content
    assert "restricted" in content


def test_default_config_has_env_keys():
    content = (REPO_ROOT / "config" / "default.toml").read_text()
    assert "env_keys" in content
    assert "ANTHROPIC_API_KEY" in content


def test_default_config_network_section():
    content = (REPO_ROOT / "config" / "default.toml").read_text()
    assert "[network]" in content


def test_default_config_secrets_section():
    content = (REPO_ROOT / "config" / "default.toml").read_text()
    assert "[secrets]" in content


# ---------------------------------------------------------------------------
# Shell → secret injection smoke test (no container, just checks the CLI
# doesn't panic when secrets are missing and gives a clear error)
# ---------------------------------------------------------------------------

def test_shell_warns_on_missing_env_key(aoa_binary, workspace_dir):
    """When ANTHROPIC_API_KEY is unset, aoa should warn (not crash silently)."""
    from support.helpers import run_aoa
    env = {k: v for k, v in os.environ.items() if k != "ANTHROPIC_API_KEY"}
    result = run_aoa(["shell", str(workspace_dir)], env=env, timeout=10)
    combined = result.stdout + result.stderr
    # Must not panic
    assert "panic" not in combined.lower()
    # Must emit a warning about the missing key
    assert "ANTHROPIC_API_KEY" in combined
