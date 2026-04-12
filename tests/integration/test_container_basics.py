"""
Integration tests — require apple/container and aoa-agent:latest.

These tests actually run containers and verify behaviour inside them.
"""

import os
import subprocess
from pathlib import Path

import pytest
from support.helpers import make_workspace, run_aoa

pytestmark = pytest.mark.requires_container

IMAGE = "aoa-agent:latest"


def shell_run(
    workspace: Path, cmd: str, network: str = "open", env: dict = None
) -> subprocess.CompletedProcess:
    """Run `aoa shell <workspace> --agent <cmd>` non-interactively and return result."""
    extra = {**(env or {}), "CLAUDE_CODE_OAUTH_TOKEN": "test-token"}
    return run_aoa(
        ["shell", str(workspace), "--agent", cmd, "--network", network, "--image", IMAGE],
        env={**os.environ, **extra},
        timeout=60,
    )


# ── entrypoint ────────────────────────────────────────────────────────────────


def test_container_starts(workspace_dir):
    """Container must start and run a command — catches wrong ENTRYPOINT."""
    result = shell_run(workspace_dir, "echo hello-from-container")
    assert result.returncode == 0
    assert "hello-from-container" in result.stdout


def test_entrypoint_script_runs(workspace_dir):
    """aoa-entrypoint must print its [aoa] banner — proves it's not systemd."""
    result = shell_run(workspace_dir, "echo ok")
    assert "[aoa] Network:" in result.stderr or "[aoa] Network:" in result.stdout


def test_workdir_is_workspace(workspace_dir):
    """Working directory inside the container must be /workspace."""
    result = shell_run(workspace_dir, "pwd")
    assert result.returncode == 0
    assert "/workspace" in result.stdout


def test_runs_as_non_root(workspace_dir):
    """Agent must run as non-root — Claude Code rejects --dangerously-skip-permissions as root."""
    result = shell_run(workspace_dir, "id -u")
    assert result.returncode == 0
    assert result.stdout.strip().split()[-1] != "0", "Agent must not run as root (UID 0)"


# ── workspace mount ───────────────────────────────────────────────────────────


def test_workspace_mounted(workspace_dir):
    """Project directory must be mounted at /workspace."""
    result = shell_run(workspace_dir, "ls /workspace")
    assert result.returncode == 0
    # workspace_dir contains hello.py (created by make_workspace)
    assert "hello.py" in result.stdout


def test_workspace_is_writable(workspace_dir):
    """Agent must be able to write files to /workspace."""
    result = shell_run(workspace_dir, "touch /workspace/aoa-test-write && echo write-ok")
    assert result.returncode == 0
    assert "write-ok" in result.stdout
    # File should appear on the host filesystem
    assert (workspace_dir / "aoa-test-write").exists()


def test_workspace_changes_persist_to_host(workspace_dir):
    """Files created inside the container must appear on the host."""
    result = shell_run(workspace_dir, "echo persistent > /workspace/persist-test.txt")
    assert result.returncode == 0
    assert (workspace_dir / "persist-test.txt").exists()
    assert "persistent" in (workspace_dir / "persist-test.txt").read_text()


def test_host_filesystem_not_accessible(workspace_dir):
    """Host /Users directory must NOT be visible inside the VM."""
    result = shell_run(workspace_dir, "ls /Users 2>/dev/null && echo FAIL || echo PASS")
    assert "PASS" in result.stdout


def test_separate_kernel(workspace_dir):
    """/proc/version must exist — container has its own kernel."""
    result = shell_run(workspace_dir, "cat /proc/version")
    assert result.returncode == 0
    assert "Linux" in result.stdout


# ── secret injection ──────────────────────────────────────────────────────────


def test_oauth_token_injected(workspace_dir):
    """CLAUDE_CODE_OAUTH_TOKEN must be set inside the container."""
    result = run_aoa(
        ["shell", str(workspace_dir), "--agent", "env", "--network", "open", "--image", IMAGE],
        env={**os.environ, "CLAUDE_CODE_OAUTH_TOKEN": "sk-ant-test-token-xyz"},
        timeout=30,
    )
    assert result.returncode == 0
    assert "CLAUDE_CODE_OAUTH_TOKEN=sk-ant-test-token-xyz" in result.stdout


def test_api_key_injected(workspace_dir):
    """ANTHROPIC_API_KEY must be set inside the container when provided."""
    result = run_aoa(
        ["shell", str(workspace_dir), "--agent", "env", "--network", "open", "--image", IMAGE],
        env={**os.environ, "ANTHROPIC_API_KEY": "sk-ant-api-key-test"},
        timeout=30,
    )
    assert result.returncode == 0
    assert "ANTHROPIC_API_KEY=sk-ant-api-key-test" in result.stdout


def test_host_env_vars_not_leaked(workspace_dir):
    """Arbitrary host env vars must NOT appear inside the container."""
    result = run_aoa(
        ["shell", str(workspace_dir), "--agent", "env", "--network", "open", "--image", IMAGE],
        env={**os.environ, "CLAUDE_CODE_OAUTH_TOKEN": "test", "AOA_SECRET_CANARY": "do-not-leak"},
        timeout=30,
    )
    assert result.returncode == 0
    assert "AOA_SECRET_CANARY" not in result.stdout


def test_no_ssh_keys_in_container(workspace_dir):
    """Host SSH keys must never appear inside the container."""
    result = shell_run(
        workspace_dir, "ls ~/.ssh/id_rsa ~/.ssh/id_ed25519 2>/dev/null && echo FAIL || echo PASS"
    )
    assert "PASS" in result.stdout


def test_secret_tmpfile_cleaned_up(workspace_dir):
    """Secret tmpfiles must be removed from the host after the session."""
    import glob

    before = set(glob.glob("/tmp/aoa-secrets-*"))
    shell_run(workspace_dir, "echo done")
    after = set(glob.glob("/tmp/aoa-secrets-*"))
    new_files = after - before
    assert len(new_files) == 0, f"Secret tmpfiles not cleaned up: {new_files}"


# ── network policy ────────────────────────────────────────────────────────────


def test_restricted_mode_logs_drop_policy(workspace_dir):
    """Entrypoint must log that OUTPUT policy is DROP in restricted mode."""
    result = shell_run(workspace_dir, "echo done", network="restricted")
    output = result.stdout + result.stderr
    assert "-P OUTPUT DROP" in output


def test_restricted_mode_allows_public_https(workspace_dir):
    """Port 443 must be reachable in restricted mode."""
    result = shell_run(
        workspace_dir,
        "curl -sfo /dev/null --max-time 10 https://example.com && echo HTTPS-OK",
        network="restricted",
    )
    assert "HTTPS-OK" in result.stdout


def test_restricted_mode_allows_dns(workspace_dir):
    """DNS resolution must work in restricted mode."""
    result = shell_run(
        workspace_dir,
        "nslookup example.com >/dev/null 2>&1 && echo DNS-OK",
        network="restricted",
    )
    assert "DNS-OK" in result.stdout


def test_open_mode_no_drop_policy(workspace_dir):
    """Open mode must not apply any DROP policy."""
    result = shell_run(workspace_dir, "echo done", network="open")
    output = result.stdout + result.stderr
    assert "-P OUTPUT DROP" not in output


def test_net_caps_dropped(workspace_dir):
    """Agent must not be able to modify iptables — CAP_NET_ADMIN is dropped."""
    result = shell_run(
        workspace_dir,
        "iptables-legacy -L OUTPUT 2>&1 && echo FAIL || echo CAPS-DROPPED",
        network="restricted",
    )
    assert "CAPS-DROPPED" in result.stdout



# ── supply-chain protection ───────────────────────────────────────────────────


def test_git_hooks_read_only(git_workspace):
    """The .git/hooks directory must be mounted read-only."""
    result = run_aoa(
        [
            "shell",
            str(git_workspace),
            "--agent",
            "touch /workspace/.git/hooks/pwned 2>/dev/null && echo FAIL || echo PASS",
            "--network",
            "open",
            "--image",
            IMAGE,
        ],
        env={**os.environ, "CLAUDE_CODE_OAUTH_TOKEN": "test"},
        timeout=30,
    )
    assert "PASS" in result.stdout


def test_workspace_git_dir_writable_except_hooks(git_workspace):
    """The rest of .git must be writable (agent needs to commit)."""
    result = run_aoa(
        [
            "shell",
            str(git_workspace),
            "--agent",
            "touch /workspace/.git/aoa-test && echo WRITE-OK",
            "--network",
            "open",
            "--image",
            IMAGE,
        ],
        env={**os.environ, "CLAUDE_CODE_OAUTH_TOKEN": "test"},
        timeout=30,
    )
    assert "WRITE-OK" in result.stdout


# ── session lifecycle ─────────────────────────────────────────────────────────


def test_session_record_created(workspace_dir):
    """A session JSON file must be created when aoa shell runs."""
    from support.helpers import get_session_dir

    before = set(get_session_dir().glob("*.json"))
    shell_run(workspace_dir, "echo done")
    after = set(get_session_dir().glob("*.json"))
    assert len(after) > len(before), "No session record was created"


def test_session_marked_stopped_after_exit(workspace_dir):
    """Session status must be 'stopped' after the container exits."""
    import json

    from support.helpers import get_session_dir

    before_ids = {f.stem for f in get_session_dir().glob("*.json")}
    shell_run(workspace_dir, "echo done")
    after_files = {f for f in get_session_dir().glob("*.json") if f.stem not in before_ids}
    assert after_files, "No new session file found"
    data = json.loads(next(iter(after_files)).read_text())
    assert data["status"] == "stopped", f"Expected stopped, got {data['status']}"


def test_container_removed_after_ephemeral_session(workspace_dir):
    """Container must be removed after an ephemeral session (--rm behaviour).

    Checks by running a command and verifying the named container disappears.
    """
    import json
    import time

    from support.helpers import get_session_dir

    before_ids = {f.stem for f in get_session_dir().glob("*.json")}
    result = shell_run(workspace_dir, "echo done")
    assert result.returncode == 0

    after_files = {f for f in get_session_dir().glob("*.json") if f.stem not in before_ids}
    assert after_files, "No session record created"

    data = json.loads(next(iter(after_files)).read_text())
    container_id = data.get("container_id", "")
    if not container_id:
        pytest.skip("No container_id in session record")

    # Give container runtime a moment to remove the container
    time.sleep(1)

    list_result = subprocess.run(
        ["container", "list", "--all", "--format", "json"],
        capture_output=True,
        text=True,
    )
    if list_result.returncode != 0:
        pytest.skip("Could not list containers")

    containers = json.loads(list_result.stdout)
    ids = [c.get("configuration", {}).get("id", "") for c in containers]
    assert container_id not in ids, f"Container {container_id} still exists after ephemeral session"
