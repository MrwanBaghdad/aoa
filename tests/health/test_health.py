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


def test_health_shows_api_key_missing_when_unset(aoa_binary):
    """When ANTHROPIC_API_KEY is absent, health should report FAIL for it."""
    env = {k: v for k, v in os.environ.items() if k != "ANTHROPIC_API_KEY"}
    result = run_aoa(["health"], env=env)
    assert "ANTHROPIC_API_KEY" in result.stdout
    # Should report either FAIL or not set
    assert "FAIL" in result.stdout or "not set" in result.stdout


def test_health_shows_api_key_ok_when_set(aoa_binary):
    result = run_aoa(["health"], env={**os.environ, "ANTHROPIC_API_KEY": "sk-ant-test"})
    assert "ANTHROPIC_API_KEY" in result.stdout
    assert "OK" in result.stdout


def test_health_summary_line(aoa_binary):
    result = run_aoa(["health"])
    assert "passed" in result.stdout
    assert "failed" in result.stdout
