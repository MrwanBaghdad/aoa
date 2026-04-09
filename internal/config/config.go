package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config is the top-level sandbox configuration.
type Config struct {
	Sandbox SandboxConfig `toml:"sandbox"`
	Network NetworkConfig `toml:"network"`
	Secrets SecretsConfig `toml:"secrets"`
}

type SandboxConfig struct {
	Image       string   `toml:"image"`
	WorkspaceDir string  `toml:"workspace_dir"`
	Persistent  bool     `toml:"persistent"`
	MaxSlots    int      `toml:"max_slots"`
	ExtraVolumes []string `toml:"extra_volumes"`
}

type NetworkConfig struct {
	// Mode: restricted | allowlist | open
	Mode       string   `toml:"mode"`
	Allowlist  []string `toml:"allowlist"`
}

type SecretsConfig struct {
	// Provider: env | secretspec
	Provider string `toml:"provider"`
	// Keys to inject from host environment
	EnvKeys  []string `toml:"env_keys"`
}

func DefaultConfig() *Config {
	return &Config{
		Sandbox: SandboxConfig{
			Image:        "aoa-agent:latest",
			WorkspaceDir: "/workspace",
			Persistent:   false,
			MaxSlots:     10,
		},
		Network: NetworkConfig{
			Mode: "restricted",
		},
		Secrets: SecretsConfig{
			Provider: "env",
			EnvKeys:  []string{"ANTHROPIC_API_KEY", "CLAUDE_CODE_OAUTH_TOKEN", "ANTHROPIC_AUTH_TOKEN", "GITHUB_TOKEN"},
		},
	}
}

// Load reads config from the given path, falling back to defaults for missing fields.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()
	if path == "" {
		path = defaultConfigPath()
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, nil
	}
	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func defaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "aoa", "config.toml")
}
