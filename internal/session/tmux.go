package session

import (
	"fmt"
	"os"
	"os/exec"
)

// TmuxSession wraps tmux lifecycle for agent sessions.
type TmuxSession struct {
	Name string
}

// New creates a new detached tmux session.
func NewTmuxSession(name string) (*TmuxSession, error) {
	if _, err := exec.LookPath("tmux"); err != nil {
		return nil, fmt.Errorf("tmux not found — install with: brew install tmux")
	}
	cmd := exec.Command("tmux", "new-session", "-d", "-s", name)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("tmux new-session: %w: %s", err, out)
	}
	return &TmuxSession{Name: name}, nil
}

// Attach attaches the current terminal to the named tmux session.
// This replaces the current process.
func (t *TmuxSession) Attach() error {
	cmd := exec.Command("tmux", "attach-session", "-t", t.Name)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// SendKeys sends a command string to the tmux session and presses Enter.
func (t *TmuxSession) SendKeys(command string) error {
	return exec.Command("tmux", "send-keys", "-t", t.Name, command, "Enter").Run()
}

// Kill terminates the tmux session.
func (t *TmuxSession) Kill() error {
	return exec.Command("tmux", "kill-session", "-t", t.Name).Run()
}

// Exists returns true if a tmux session with this name is currently running.
func (t *TmuxSession) Exists() bool {
	err := exec.Command("tmux", "has-session", "-t", t.Name).Run()
	return err == nil
}

// AttachByName attaches to a tmux session by name (used for resume).
func AttachByName(name string) error {
	cmd := exec.Command("tmux", "attach-session", "-t", name)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
