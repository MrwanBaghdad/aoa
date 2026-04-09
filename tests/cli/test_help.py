"""CLI smoke tests — no container runtime required."""

import pytest
from support.helpers import run_aoa


pytestmark = pytest.mark.cli


def test_root_help(aoa_binary):
    result = run_aoa(["--help"])
    assert result.returncode == 0
    assert "shell" in result.stdout
    assert "build" in result.stdout
    assert "list" in result.stdout
    assert "attach" in result.stdout
    assert "health" in result.stdout


def test_version(aoa_binary):
    result = run_aoa(["--version"])
    assert result.returncode == 0
    assert "0.1.0" in result.stdout


def test_shell_help(aoa_binary):
    result = run_aoa(["shell", "--help"])
    assert result.returncode == 0
    assert "--slot" in result.stdout
    assert "--resume" in result.stdout
    assert "--persistent" in result.stdout
    assert "--network" in result.stdout
    assert "--agent" in result.stdout


def test_build_help(aoa_binary):
    result = run_aoa(["build", "--help"])
    assert result.returncode == 0
    assert "--tag" in result.stdout
    assert "--target" in result.stdout


def test_list_help(aoa_binary):
    result = run_aoa(["list", "--help"])
    assert result.returncode == 0


def test_attach_help(aoa_binary):
    result = run_aoa(["attach", "--help"])
    assert result.returncode == 0
    assert "session-id" in result.stdout


def test_health_help(aoa_binary):
    result = run_aoa(["health", "--help"])
    assert result.returncode == 0


def test_unknown_command_fails(aoa_binary):
    result = run_aoa(["notacommand"])
    assert result.returncode != 0
    assert "unknown command" in result.stderr.lower() or "Error" in result.stderr


def test_attach_requires_arg(aoa_binary):
    result = run_aoa(["attach"])
    assert result.returncode != 0


def test_shell_invalid_dir(aoa_binary):
    result = run_aoa(["shell", "/nonexistent/path/that/does/not/exist"])
    assert result.returncode != 0
    assert "not found" in result.stderr.lower() or "no such" in result.stderr.lower()
