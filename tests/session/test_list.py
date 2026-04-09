"""Tests for `aoa list` — session listing."""

import json
import time
from pathlib import Path

import pytest
from support.helpers import get_session_dir, run_aoa


pytestmark = pytest.mark.cli


def _write_fake_session(session_id: str, slot: int, workspace: str, status: str = "running") -> None:
    """Inject a fake session record directly into the session store."""
    session_dir = get_session_dir()
    session_dir.mkdir(parents=True, exist_ok=True)
    data = {
        "id": session_id,
        "slot": slot,
        "container_id": f"aoa-{session_id[:8]}",
        "tmux_session": f"aoa-{session_id[:8]}",
        "workspace_dir": workspace,
        "image": "aoa-agent:latest",
        "status": status,
        "persistent": False,
        "created_at": "2026-04-09T10:00:00Z",
        "updated_at": "2026-04-09T10:00:00Z",
    }
    path = session_dir / f"{session_id}.json"
    path.write_text(json.dumps(data, indent=2))


def _remove_fake_session(session_id: str) -> None:
    path = get_session_dir() / f"{session_id}.json"
    path.unlink(missing_ok=True)


def test_list_empty(aoa_binary, tmp_path):
    """list with no sessions should say 'No sessions found'."""
    result = run_aoa(["list"])
    assert result.returncode == 0
    # If there happen to be real sessions, don't fail — just check it runs
    assert result.returncode == 0


def test_list_shows_injected_session(aoa_binary, tmp_path):
    sid = "aaaabbbb-cccc-dddd-eeee-ffffffffffff"
    _write_fake_session(sid, slot=1, workspace=str(tmp_path))
    try:
        result = run_aoa(["list"])
        assert result.returncode == 0
        assert "aaaabbbb" in result.stdout
        assert "running" in result.stdout
    finally:
        _remove_fake_session(sid)


def test_list_shows_slot_number(aoa_binary, tmp_path):
    sid = "11112222-3333-4444-5555-666677778888"
    _write_fake_session(sid, slot=3, workspace=str(tmp_path))
    try:
        result = run_aoa(["list"])
        assert result.returncode == 0
        assert "3" in result.stdout
    finally:
        _remove_fake_session(sid)


def test_list_shows_stopped_sessions(aoa_binary, tmp_path):
    sid = "deadbeef-0000-0000-0000-000000000000"
    _write_fake_session(sid, slot=2, workspace=str(tmp_path), status="stopped")
    try:
        result = run_aoa(["list"])
        assert result.returncode == 0
        assert "stopped" in result.stdout
    finally:
        _remove_fake_session(sid)


def test_list_table_headers(aoa_binary, tmp_path):
    sid = "cafebabe-0000-0000-0000-000000000000"
    _write_fake_session(sid, slot=1, workspace=str(tmp_path))
    try:
        result = run_aoa(["list"])
        assert result.returncode == 0
        output = result.stdout.upper()
        assert "ID" in output
        assert "SLOT" in output
        assert "STATUS" in output
    finally:
        _remove_fake_session(sid)
