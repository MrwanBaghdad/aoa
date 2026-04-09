package secrets

import (
	"fmt"
	"os"
	"strings"
)

// Bundle holds resolved secrets ready to inject into a container.
type Bundle struct {
	// EnvFile is a path to a tmpfile with KEY=VALUE lines (deleted on Cleanup).
	EnvFile string
	// PathMounts maps container path -> tmpfile path for as_path secrets.
	PathMounts map[string]string
	// cleanup holds all tmpfiles created.
	cleanup []string
}

// FromEnv resolves the given keys from the host environment and writes them
// to a tmpfile. Returns a Bundle with the env file path.
// Missing keys are silently skipped — call HasLLMAuth() to check if auth was found.
func FromEnv(keys []string) (*Bundle, error) {
	var lines []string
	for _, key := range keys {
		val := os.Getenv(key)
		if val == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s=%s", key, val))
	}

	f, err := os.CreateTemp("", "aoa-secrets-*")
	if err != nil {
		return nil, fmt.Errorf("create secret tmpfile: %w", err)
	}
	if _, err := f.WriteString(strings.Join(lines, "\n") + "\n"); err != nil {
		f.Close()
		os.Remove(f.Name())
		return nil, fmt.Errorf("write secret tmpfile: %w", err)
	}
	f.Close()

	// Restrict permissions so only owner can read.
	if err := os.Chmod(f.Name(), 0600); err != nil {
		os.Remove(f.Name())
		return nil, err
	}

	return &Bundle{
		EnvFile: f.Name(),
		cleanup: []string{f.Name()},
	}, nil
}

// HasLLMAuth returns true if the bundle contains a recognised LLM auth token.
func (b *Bundle) HasLLMAuth() bool {
	if b.EnvFile == "" {
		return false
	}
	data, err := os.ReadFile(b.EnvFile)
	if err != nil {
		return false
	}
	content := string(data)
	for _, key := range []string{"ANTHROPIC_API_KEY", "CLAUDE_CODE_OAUTH_TOKEN", "ANTHROPIC_AUTH_TOKEN"} {
		if strings.Contains(content, key+"=") {
			return true
		}
	}
	return false
}

// Cleanup removes all tmpfiles created by this bundle.
// Safe to call multiple times.
func (b *Bundle) Cleanup() {
	for _, path := range b.cleanup {
		os.Remove(path)
	}
	b.cleanup = nil
}

// Volumes returns volume mount strings for PathMounts (host:container:ro).
func (b *Bundle) Volumes() []string {
	var vols []string
	for containerPath, hostPath := range b.PathMounts {
		vols = append(vols, fmt.Sprintf("%s:%s:ro", hostPath, containerPath))
	}
	return vols
}

// fromMap writes the given key=value pairs to a tmpfile bundle.
func fromMap(m map[string]string) (*Bundle, error) {
	var lines []string
	for k, v := range m {
		lines = append(lines, fmt.Sprintf("%s=%s", k, v))
	}
	f, err := os.CreateTemp("", "aoa-secrets-*")
	if err != nil {
		return nil, fmt.Errorf("create secret tmpfile: %w", err)
	}
	if _, err := f.WriteString(strings.Join(lines, "\n") + "\n"); err != nil {
		f.Close()
		os.Remove(f.Name())
		return nil, err
	}
	f.Close()
	if err := os.Chmod(f.Name(), 0600); err != nil {
		os.Remove(f.Name())
		return nil, err
	}
	return &Bundle{EnvFile: f.Name(), cleanup: []string{f.Name()}}, nil
}

// WritePathSecret writes a secret value to a tmpfile and registers it for cleanup.
// Returns the host path to the tmpfile.
func (b *Bundle) WritePathSecret(name, value string) (string, error) {
	f, err := os.CreateTemp("", fmt.Sprintf("aoa-secret-%s-*", name))
	if err != nil {
		return "", err
	}
	if _, err := f.WriteString(value); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", err
	}
	f.Close()
	if err := os.Chmod(f.Name(), 0600); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	b.cleanup = append(b.cleanup, f.Name())
	return f.Name(), nil
}
