package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// Status of a session.
type Status string

const (
	StatusRunning Status = "running"
	StatusStopped Status = "stopped"
)

// Session tracks one agent sandbox session.
type Session struct {
	ID          string    `json:"id"`
	Slot        int       `json:"slot"`
	ContainerID string    `json:"container_id"`
	TmuxSession string    `json:"tmux_session"`
	WorkspaceDir string   `json:"workspace_dir"`
	Image       string    `json:"image"`
	Status      Status    `json:"status"`
	Persistent  bool      `json:"persistent"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Manager persists session state to ~/.local/share/aoa/sessions/.
type Manager struct {
	dir string
}

func NewManager() (*Manager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".local", "share", "aoa", "sessions")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	return &Manager{dir: dir}, nil
}

func (m *Manager) Create(slot int, workspaceDir, image string, persistent bool) (*Session, error) {
	s := &Session{
		ID:           uuid.New().String(),
		Slot:         slot,
		WorkspaceDir: workspaceDir,
		Image:        image,
		Status:       StatusRunning,
		Persistent:   persistent,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	s.TmuxSession = fmt.Sprintf("aoa-%s", s.ID[:8])
	return s, m.save(s)
}

func (m *Manager) Update(s *Session) error {
	s.UpdatedAt = time.Now()
	return m.save(s)
}

func (m *Manager) Get(id string) (*Session, error) {
	path := filepath.Join(m.dir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Session
	return &s, json.Unmarshal(data, &s)
}

func (m *Manager) List() ([]*Session, error) {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		return nil, err
	}
	var sessions []*Session
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		id := e.Name()[:len(e.Name())-5]
		s, err := m.Get(id)
		if err != nil {
			continue
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

func (m *Manager) Delete(id string) error {
	return os.Remove(filepath.Join(m.dir, id+".json"))
}

// FindBySlot returns the most recent running session for the given slot and workspace.
func (m *Manager) FindBySlot(slot int, workspaceDir string) (*Session, error) {
	all, err := m.List()
	if err != nil {
		return nil, err
	}
	var found *Session
	for _, s := range all {
		if s.Slot == slot && s.WorkspaceDir == workspaceDir && s.Status == StatusRunning {
			if found == nil || s.CreatedAt.After(found.CreatedAt) {
				found = s
			}
		}
	}
	if found == nil {
		return nil, fmt.Errorf("no running session for slot %d in %s", slot, workspaceDir)
	}
	return found, nil
}

// NextSlot returns the next available slot number for the given workspace.
func (m *Manager) NextSlot(workspaceDir string) (int, error) {
	all, err := m.List()
	if err != nil {
		return 1, err
	}
	used := map[int]bool{}
	for _, s := range all {
		if s.WorkspaceDir == workspaceDir && s.Status == StatusRunning {
			used[s.Slot] = true
		}
	}
	for i := 1; i <= 10; i++ {
		if !used[i] {
			return i, nil
		}
	}
	return 0, fmt.Errorf("all slots in use for workspace %s", workspaceDir)
}

func (m *Manager) save(s *Session) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(m.dir, s.ID+".json"), data, 0600)
}
