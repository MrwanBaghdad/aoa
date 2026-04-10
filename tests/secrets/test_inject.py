"""Tests for secret injection configuration and CLI smoke test."""

import os
import sys
from pathlib import Path

import pytest

if sys.version_info >= (3, 11):
    import tomllib
else:
    import tomli as tomllib

pytestmark = pytest.mark.cli

REPO_ROOT = Path(__file__).parent.parent.parent
SECRETSPEC = str(REPO_ROOT / "config" / "secretspec.toml")
DEFAULT_CFG = str(REPO_ROOT / "config" / "default.toml")


# ---------------------------------------------------------------------------
# config/secretspec.toml — structure validation via TOML parse
# ---------------------------------------------------------------------------

def test_example_secretspec_toml_exists(local_host):
    assert local_host.file(SECRETSPEC).is_file


def test_example_secretspec_has_project_section(local_host):
    cfg = tomllib.loads(local_host.file(SECRETSPEC).content_string)
    assert "project" in cfg
    assert "name" in cfg["project"]


def test_example_secretspec_has_default_profile(local_host):
    cfg = tomllib.loads(local_host.file(SECRETSPEC).content_string)
    assert "profiles" in cfg
    assert "default" in cfg["profiles"]


def test_example_secretspec_has_anthropic_key(local_host):
    cfg = tomllib.loads(local_host.file(SECRETSPEC).content_string)
    profile = cfg["profiles"]["default"]
    assert "ANTHROPIC_API_KEY" in profile


def test_example_secretspec_has_required_field(local_host):
    cfg = tomllib.loads(local_host.file(SECRETSPEC).content_string)
    profile = cfg["profiles"]["default"]
    assert profile["ANTHROPIC_API_KEY"].get("required") is True


def test_example_secretspec_has_development_profile(local_host):
    cfg = tomllib.loads(local_host.file(SECRETSPEC).content_string)
    assert "development" in cfg["profiles"]


def test_example_secretspec_has_default_value(local_host):
    cfg = tomllib.loads(local_host.file(SECRETSPEC).content_string)
    # At least one key in any profile should have a default value
    profiles = cfg.get("profiles", {})
    has_default = any(
        isinstance(v, dict) and "default" in v
        for profile in profiles.values()
        for v in profile.values()
        if isinstance(v, dict)
    )
    assert has_default, "No key with 'default' value found in secretspec.toml"


# ---------------------------------------------------------------------------
# config/default.toml — structure validation via TOML parse
# ---------------------------------------------------------------------------

def test_default_config_toml_exists(local_host):
    assert local_host.file(DEFAULT_CFG).is_file


def test_default_config_has_image(local_host):
    cfg = tomllib.loads(local_host.file(DEFAULT_CFG).content_string)
    assert "sandbox" in cfg
    assert "image" in cfg["sandbox"]
    assert "aoa-agent" in cfg["sandbox"]["image"]


def test_default_config_has_network_mode(local_host):
    cfg = tomllib.loads(local_host.file(DEFAULT_CFG).content_string)
    assert "network" in cfg
    assert cfg["network"]["mode"] == "restricted"


def test_default_config_has_env_keys(local_host):
    cfg = tomllib.loads(local_host.file(DEFAULT_CFG).content_string)
    assert "secrets" in cfg
    env_keys = cfg["secrets"].get("env_keys", [])
    assert "ANTHROPIC_API_KEY" in env_keys


def test_default_config_network_section(local_host):
    cfg = tomllib.loads(local_host.file(DEFAULT_CFG).content_string)
    assert "network" in cfg


def test_default_config_secrets_section(local_host):
    cfg = tomllib.loads(local_host.file(DEFAULT_CFG).content_string)
    assert "secrets" in cfg


# ---------------------------------------------------------------------------
# CLI smoke test — aoa shell must not panic when secrets are missing
# ---------------------------------------------------------------------------

def test_shell_does_not_panic_without_api_key(aoa_binary, workspace_dir):
    """aoa shell must not panic when ANTHROPIC_API_KEY is unset."""
    from support.helpers import run_aoa
    env = {k: v for k, v in os.environ.items() if k != "ANTHROPIC_API_KEY"}
    result = run_aoa(["shell", str(workspace_dir), "--agent", "echo ok"], env=env, timeout=15)
    combined = result.stdout + result.stderr
    assert "goroutine" not in combined and "runtime error" not in combined
