"""Tests for `aoa attach` — session attachment validation."""

import json
from pathlib import Path

import pytest
from support.helpers import get_session_dir, run_aoa


pytestmark = pytest.mark.cli


def _write_fake_session(session_id: str, status: str = "running", workspace: str = "/tmp/test") -> None:
    session_dir = get_session_dir()
    session_dir.mkdir(parents=True, exist_ok=True)
    data = {
        "id": session_id,
        "slot": 1,
        "container_id": f"aoa-{session_id[:8]}",
        "tmux_session": f"aoa-{session_id[:8]}",
        "workspace_dir": workspace,
        "image": "aoa-agent:latest",
        "status": status,
        "persistent": False,
        "created_at": "2026-04-09T10:00:00Z",
        "updated_at": "2026-04-09T10:00:00Z",
    }
    (session_dir / f"{session_id}.json").write_text(json.dumps(data))


def _remove(sid: str) -> None:
    (get_session_dir() / f"{sid}.json").unlink(missing_ok=True)


def test_attach_no_args_fails(aoa_binary):
    result = run_aoa(["attach"])
    assert result.returncode != 0


def test_attach_unknown_id_fails(aoa_binary):
    result = run_aoa(["attach", "zzzzzzzz"])
    assert result.returncode != 0
    assert "not found" in result.stderr.lower() or "no session" in result.stderr.lower()


def test_attach_stopped_session_fails(aoa_binary):
    sid = "11223344-0000-0000-0000-000000000000"
    _write_fake_session(sid, status="stopped")
    try:
        result = run_aoa(["attach", sid[:8]])
        assert result.returncode != 0
        assert "not running" in result.stderr.lower() or "stopped" in result.stderr.lower()
    finally:
        _remove(sid)


def test_attach_ambiguous_prefix_fails(aoa_binary):
    """Two sessions sharing a prefix should return an error."""
    sid1 = "aabbccdd-1111-0000-0000-000000000000"
    sid2 = "aabbccdd-2222-0000-0000-000000000000"
    _write_fake_session(sid1)
    _write_fake_session(sid2)
    try:
        result = run_aoa(["attach", "aabbccdd"])
        assert result.returncode != 0
        assert "ambiguous" in result.stderr.lower()
    finally:
        _remove(sid1)
        _remove(sid2)
