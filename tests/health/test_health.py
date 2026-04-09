"""Tests for `aoa health` — checks dependency detection and env var checks."""

import os
import pytest
from support.helpers import run_aoa


pytestmark = pytest.mark.cli


def test_health_runs(aoa_binary):
    """health should always run without panicking."""
    result = run_aoa(["health"])
    # It may fail if dependencies are missing, but it must not crash
    assert result.returncode in (0, 1)
    assert "aoa health check" in result.stdout


def test_health_shows_container_runtime(aoa_binary):
    result = run_aoa(["health"])
    assert "apple/container" in result.stdout


def test_health_shows_tmux(aoa_binary):
    result = run_aoa(["health"])
    assert "tmux" in result.stdout


def test_health_shows_secretspec(aoa_binary):
    result = run_aoa(["health"])
    assert "secretspec" in result.stdout


def test_health_shows_auth_fail_when_no_credentials(aoa_binary):
    """When no credentials at all are available, health must report FAIL for auth."""
    env = {k: v for k, v in os.environ.items()
           if k not in ("ANTHROPIC_API_KEY", "CLAUDE_CODE_OAUTH_TOKEN", "ANTHROPIC_AUTH_TOKEN")}
    # Patch out the Keychain by pointing HOME somewhere empty
    import tempfile
    with tempfile.TemporaryDirectory() as fake_home:
        result = run_aoa(["health"], env={**env, "HOME": fake_home})
    assert "LLM credentials" in result.stdout
    assert "FAIL" in result.stdout


def test_health_shows_auth_ok_when_api_key_set(aoa_binary):
    """When ANTHROPIC_API_KEY is set, health must show OK for auth."""
    result = run_aoa(["health"], env={**os.environ, "ANTHROPIC_API_KEY": "sk-ant-test"})
    assert "LLM credentials" in result.stdout
    assert "OK" in result.stdout


def test_health_shows_auth_ok_when_oauth_token_set(aoa_binary):
    """When CLAUDE_CODE_OAUTH_TOKEN is set, health must show OK for auth."""
    result = run_aoa(["health"], env={**os.environ, "CLAUDE_CODE_OAUTH_TOKEN": "sk-ant-oat-test"})
    assert "LLM credentials" in result.stdout
    assert "OK" in result.stdout


def test_health_summary_line(aoa_binary):
    result = run_aoa(["health"])
    assert "passed" in result.stdout
    assert "failed" in result.stdout
