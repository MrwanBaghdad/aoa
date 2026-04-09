package session

import (
	"os"
	"path/filepath"
	"testing"
)

// newTestManager creates a Manager backed by a temp directory.
func newTestManager(t *testing.T) *Manager {
	t.Helper()
	dir := t.TempDir()
	return &Manager{dir: dir}
}

func TestCreateSession(t *testing.T) {
	m := newTestManager(t)
	s, err := m.Create(1, "/tmp/test-ws", "aoa-agent:latest", false)
	if err != nil {
		t.Fatal(err)
	}
	if s.ID == "" {
		t.Error("session ID should not be empty")
	}
	if s.Slot != 1 {
		t.Errorf("expected slot 1, got %d", s.Slot)
	}
	if s.Status != StatusRunning {
		t.Errorf("expected running, got %q", s.Status)
	}
	if s.TmuxSession == "" {
		t.Error("tmux session name should not be empty")
	}
}

func TestGetSession(t *testing.T) {
	m := newTestManager(t)
	created, err := m.Create(2, "/tmp/ws", "aoa-agent:latest", true)
	if err != nil {
		t.Fatal(err)
	}
	got, err := m.Get(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != created.ID {
		t.Errorf("ID mismatch: %q vs %q", got.ID, created.ID)
	}
	if got.Persistent != true {
		t.Error("expected persistent=true")
	}
}

func TestGetMissingSessionReturnsError(t *testing.T) {
	m := newTestManager(t)
	_, err := m.Get("does-not-exist")
	if err == nil {
		t.Error("expected error for missing session")
	}
}

func TestListSessions(t *testing.T) {
	m := newTestManager(t)
	if _, err := m.Create(1, "/tmp/ws", "img:latest", false); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Create(2, "/tmp/ws", "img:latest", false); err != nil {
		t.Fatal(err)
	}

	sessions, err := m.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(sessions))
	}
}

func TestListEmptyDir(t *testing.T) {
	m := newTestManager(t)
	sessions, err := m.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestUpdateSession(t *testing.T) {
	m := newTestManager(t)
	s, _ := m.Create(1, "/tmp/ws", "img:latest", false)
	s.Status = StatusStopped
	s.ContainerID = "abc123"
	if err := m.Update(s); err != nil {
		t.Fatal(err)
	}
	got, _ := m.Get(s.ID)
	if got.Status != StatusStopped {
		t.Errorf("expected stopped, got %q", got.Status)
	}
	if got.ContainerID != "abc123" {
		t.Errorf("expected container ID 'abc123', got %q", got.ContainerID)
	}
}

func TestDeleteSession(t *testing.T) {
	m := newTestManager(t)
	s, _ := m.Create(1, "/tmp/ws", "img:latest", false)
	if err := m.Delete(s.ID); err != nil {
		t.Fatal(err)
	}
	_, err := m.Get(s.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestNextSlot(t *testing.T) {
	m := newTestManager(t)
	ws := "/tmp/unique-ws"

	slot, err := m.NextSlot(ws)
	if err != nil {
		t.Fatal(err)
	}
	if slot != 1 {
		t.Errorf("first slot should be 1, got %d", slot)
	}

	if _, err := m.Create(1, ws, "img:latest", false); err != nil {
		t.Fatal(err)
	}
	slot2, _ := m.NextSlot(ws)
	if slot2 != 2 {
		t.Errorf("second slot should be 2, got %d", slot2)
	}
}

func TestNextSlotSkipsUsed(t *testing.T) {
	m := newTestManager(t)
	ws := "/tmp/slot-skip-ws"
	// Use slots 1 and 2
	if _, err := m.Create(1, ws, "img:latest", false); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Create(2, ws, "img:latest", false); err != nil {
		t.Fatal(err)
	}
	slot, _ := m.NextSlot(ws)
	if slot != 3 {
		t.Errorf("expected slot 3, got %d", slot)
	}
}

func TestFindBySlot(t *testing.T) {
	m := newTestManager(t)
	ws := "/tmp/find-ws"
	s, _ := m.Create(3, ws, "img:latest", false)

	found, err := m.FindBySlot(3, ws)
	if err != nil {
		t.Fatal(err)
	}
	if found.ID != s.ID {
		t.Errorf("ID mismatch: %q vs %q", found.ID, s.ID)
	}
}

func TestFindBySlotNotFound(t *testing.T) {
	m := newTestManager(t)
	_, err := m.FindBySlot(99, "/tmp/ws")
	if err == nil {
		t.Error("expected error for missing slot")
	}
}

func TestFindBySlotIgnoresStopped(t *testing.T) {
	m := newTestManager(t)
	ws := "/tmp/stopped-ws"
	s, _ := m.Create(1, ws, "img:latest", false)
	s.Status = StatusStopped
	if err := m.Update(s); err != nil {
		t.Fatal(err)
	}

	_, err := m.FindBySlot(1, ws)
	if err == nil {
		t.Error("FindBySlot should not return stopped sessions")
	}
}

func TestSessionPersistenceToFile(t *testing.T) {
	m := newTestManager(t)
	s, _ := m.Create(1, "/tmp/ws", "img:latest", false)

	path := filepath.Join(m.dir, s.ID+".json")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("session file should exist at %s", path)
	}
}
