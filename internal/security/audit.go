package security

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Severity levels for audit events.
type Severity string

const (
	SeverityInfo     Severity = "INFO"
	SeverityLow      Severity = "LOW"
	SeverityMedium   Severity = "MEDIUM"
	SeverityHigh     Severity = "HIGH"
	SeverityCritical Severity = "CRITICAL"
)

// AuditEvent is one line in the JSONL audit log.
type AuditEvent struct {
	Timestamp   time.Time         `json:"timestamp"`
	SessionID   string            `json:"session_id"`
	ContainerID string            `json:"container_id"`
	Severity    Severity          `json:"severity"`
	Event       string            `json:"event"`
	Details     map[string]string `json:"details,omitempty"`
}

// Auditor writes JSONL audit logs.
type Auditor struct {
	file      *os.File
	sessionID string
}

func NewAuditor(sessionID string) (*Auditor, error) {
	logDir, err := auditLogDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(logDir, fmt.Sprintf("session-%s.jsonl", sessionID))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("open audit log: %w", err)
	}
	return &Auditor{file: f, sessionID: sessionID}, nil
}

func (a *Auditor) Log(containerID string, severity Severity, event string, details map[string]string) {
	ev := AuditEvent{
		Timestamp:   time.Now().UTC(),
		SessionID:   a.sessionID,
		ContainerID: containerID,
		Severity:    severity,
		Event:       event,
		Details:     details,
	}
	line, _ := json.Marshal(ev)
	fmt.Fprintf(a.file, "%s\n", line)
}

func (a *Auditor) Close() error {
	return a.file.Close()
}

func auditLogDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".local", "share", "aoa", "audit")
	return dir, os.MkdirAll(dir, 0700)
}
