package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Sandbox.Image == "" {
		t.Error("default image should not be empty")
	}
	if cfg.Sandbox.WorkspaceDir == "" {
		t.Error("default workspace dir should not be empty")
	}
	if cfg.Network.Mode == "" {
		t.Error("default network mode should not be empty")
	}
	if cfg.Network.Mode != "restricted" {
		t.Errorf("default network mode should be 'restricted', got %q", cfg.Network.Mode)
	}
	if len(cfg.Secrets.EnvKeys) == 0 {
		t.Error("default env keys should not be empty")
	}
}

func TestDefaultConfigContainsAnthropicKey(t *testing.T) {
	cfg := DefaultConfig()
	found := false
	for _, k := range cfg.Secrets.EnvKeys {
		if k == "ANTHROPIC_API_KEY" {
			found = true
		}
	}
	if !found {
		t.Error("default config should include ANTHROPIC_API_KEY in env_keys")
	}
}

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.toml")
	if err != nil {
		t.Fatalf("Load with missing file should return defaults, got error: %v", err)
	}
	def := DefaultConfig()
	if cfg.Network.Mode != def.Network.Mode {
		t.Errorf("expected default network mode %q, got %q", def.Network.Mode, cfg.Network.Mode)
	}
}

func TestLoadValidConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := `
[sandbox]
image = "my-custom-image:v2"
persistent = true

[network]
mode = "open"

[secrets]
env_keys = ["MY_SECRET"]
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Sandbox.Image != "my-custom-image:v2" {
		t.Errorf("expected image 'my-custom-image:v2', got %q", cfg.Sandbox.Image)
	}
	if !cfg.Sandbox.Persistent {
		t.Error("expected persistent=true")
	}
	if cfg.Network.Mode != "open" {
		t.Errorf("expected network mode 'open', got %q", cfg.Network.Mode)
	}
	if len(cfg.Secrets.EnvKeys) != 1 || cfg.Secrets.EnvKeys[0] != "MY_SECRET" {
		t.Errorf("unexpected env keys: %v", cfg.Secrets.EnvKeys)
	}
}

func TestLoadInvalidTomlReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.toml")
	if err := os.WriteFile(path, []byte("[[[[invalid toml"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Error("expected error for invalid TOML, got nil")
	}
}
