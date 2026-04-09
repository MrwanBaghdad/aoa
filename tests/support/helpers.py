"""Shared test utilities for aoa integration tests."""

from __future__ import annotations

import hashlib
import os
import subprocess
import time
from pathlib import Path
from typing import Optional

import pexpect
import pyte


# ---------------------------------------------------------------------------
# Binary resolution
# ---------------------------------------------------------------------------

def get_aoa_binary() -> str:
    """Return path to the aoa binary, preferring AOA_BINARY env var."""
    if env := os.environ.get("AOA_BINARY"):
        return env
    # Assume tests are run from repo root
    local = Path(__file__).parent.parent.parent / "aoa"
    if local.exists():
        return str(local)
    raise FileNotFoundError(
        "aoa binary not found. Build with `devbox run go build -o aoa .` "
        "or set AOA_BINARY env var."
    )


# ---------------------------------------------------------------------------
# Terminal emulator — handles ANSI/VT100 codes for screen capture
# ---------------------------------------------------------------------------

class TerminalEmulator:
    """Wraps pyte screen + pexpect child for testing interactive CLI output."""

    COLS = 220
    ROWS = 50

    def __init__(self, child: pexpect.spawn):
        self.child = child
        self.screen = pyte.Screen(self.COLS, self.ROWS)
        self.stream = pyte.ByteStream(self.screen)

    def feed(self) -> None:
        """Feed available output from child into the pyte screen."""
        try:
            data = self.child.read_nonblocking(size=4096, timeout=0.1)
            self.stream.feed(data)
        except (pexpect.TIMEOUT, pexpect.EOF):
            pass

    def get_screen_text(self) -> str:
        """Return current screen contents as a string (strips trailing spaces)."""
        self.feed()
        lines = [row.rstrip() for row in self.screen.display]
        return "\n".join(lines).rstrip()

    def wait_for_text(self, text: str, timeout: float = 10.0) -> bool:
        """Poll until text appears on screen or timeout expires."""
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            if text in self.get_screen_text():
                return True
            time.sleep(0.1)
        return False

    def send(self, text: str) -> None:
        self.child.send(text)

    def sendline(self, text: str) -> None:
        self.child.sendline(text)


# ---------------------------------------------------------------------------
# CLI runner — for non-interactive commands
# ---------------------------------------------------------------------------

def run_aoa(args: list[str], *, env: Optional[dict] = None, timeout: int = 30) -> subprocess.CompletedProcess:
    """Run aoa with the given args and return CompletedProcess."""
    binary = get_aoa_binary()
    merged_env = {**os.environ, **(env or {})}
    return subprocess.run(
        [binary, *args],
        capture_output=True,
        text=True,
        timeout=timeout,
        env=merged_env,
    )


def aoa_output(args: list[str], **kwargs) -> str:
    """Run aoa and return stdout. Raises on non-zero exit."""
    result = run_aoa(args, **kwargs)
    assert result.returncode == 0, (
        f"aoa {args} failed (rc={result.returncode}):\n{result.stderr}"
    )
    return result.stdout


# ---------------------------------------------------------------------------
# Workspace helpers
# ---------------------------------------------------------------------------

def make_workspace(tmp_path: Path, *, git: bool = False) -> Path:
    """Create a minimal workspace directory for testing."""
    ws = tmp_path / "workspace"
    ws.mkdir(exist_ok=True)
    (ws / "hello.py").write_text('print("hello from sandbox")\n')
    if git:
        subprocess.run(["git", "init", str(ws)], check=True, capture_output=True)
        subprocess.run(["git", "-C", str(ws), "config", "user.email", "test@aoa"], check=True, capture_output=True)
        subprocess.run(["git", "-C", str(ws), "config", "user.name", "aoa-test"], check=True, capture_output=True)
        (ws / ".git" / "hooks").mkdir(exist_ok=True)
    return ws


def workspace_hash(path: Path) -> str:
    return hashlib.sha256(str(path).encode()).hexdigest()[:8]


# ---------------------------------------------------------------------------
# Session state helpers
# ---------------------------------------------------------------------------

def get_session_dir() -> Path:
    home = Path.home()
    return home / ".local" / "share" / "aoa" / "sessions"


def cleanup_sessions(workspace_dir: Optional[str] = None) -> None:
    """Remove persisted session JSON files (optionally scoped to a workspace)."""
    session_dir = get_session_dir()
    if not session_dir.exists():
        return
    import json
    for f in session_dir.glob("*.json"):
        if workspace_dir is None:
            f.unlink(missing_ok=True)
        else:
            try:
                data = json.loads(f.read_text())
                if data.get("workspace_dir") == workspace_dir:
                    f.unlink(missing_ok=True)
            except (json.JSONDecodeError, KeyError):
                pass
