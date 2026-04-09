package secrets

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// SecretSpec represents a parsed secretspec.toml file.
type SecretSpec struct {
	Project  ProjectSpec             `toml:"project"`
	Profiles map[string]ProfileSpec  `toml:"profiles"`
}

type ProjectSpec struct {
	Name string `toml:"name"`
}

type ProfileSpec struct {
	Secrets map[string]SecretDecl
}

// SecretDecl is decoded manually because TOML keys are dynamic.
type SecretDecl struct {
	Description string `toml:"description"`
	Required    bool   `toml:"required"`
	Default     string `toml:"default"`
	AsPath      bool   `toml:"as_path"`
}

// LoadSecretSpec parses secretspec.toml from the given directory.
func LoadSecretSpec(dir string) (*SecretSpec, error) {
	path := filepath.Join(dir, "secretspec.toml")
	var spec struct {
		Project  ProjectSpec `toml:"project"`
		Profiles map[string]map[string]SecretDecl `toml:"profiles"`
	}
	if _, err := toml.DecodeFile(path, &spec); err != nil {
		return nil, err
	}
	ss := &SecretSpec{
		Project:  spec.Project,
		Profiles: make(map[string]ProfileSpec),
	}
	for name, decls := range spec.Profiles {
		ss.Profiles[name] = ProfileSpec{Secrets: decls}
	}
	return ss, nil
}

// Resolve resolves secrets using the `secretspec` CLI if available, otherwise
// falls back to env vars and declared defaults. Profile is typically "default".
func Resolve(spec *SecretSpec, profile string, bundle *Bundle) error {
	p, ok := spec.Profiles[profile]
	if !ok {
		return fmt.Errorf("profile %q not found in secretspec.toml", profile)
	}

	// Try secretspec CLI first (it handles 1Password, Keychain, Vault, etc.)
	if bin, err := exec.LookPath("secretspec"); err == nil {
		return resolveViaSecretspecCLI(bin, profile, p, bundle)
	}

	// Fallback: env vars + defaults
	return resolveFromEnvAndDefaults(p, bundle)
}

func resolveViaSecretspecCLI(bin, profile string, p ProfileSpec, bundle *Bundle) error {
	// secretspec exec --profile <profile> -- env  prints resolved env vars
	out, err := exec.Command(bin, "exec", "--profile", profile, "--", "env").Output()
	if err != nil {
		return fmt.Errorf("secretspec exec: %w", err)
	}

	var lines []string
	resolved := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			key := parts[0]
			if _, declared := p.Secrets[key]; declared {
				resolved[key] = parts[1]
				lines = append(lines, line)
			}
		}
	}

	// Handle as_path secrets separately
	for key, decl := range p.Secrets {
		if decl.AsPath {
			val, ok := resolved[key]
			if !ok {
				continue
			}
			hostPath, err := bundle.WritePathSecret(key, val)
			if err != nil {
				return err
			}
			if bundle.PathMounts == nil {
				bundle.PathMounts = map[string]string{}
			}
			containerPath := fmt.Sprintf("/run/secrets/%s", strings.ToLower(key))
			bundle.PathMounts[containerPath] = hostPath
			// Remove from env lines since it's injected as path
			var filtered []string
			for _, l := range lines {
				if !strings.HasPrefix(l, key+"=") {
					filtered = append(filtered, l)
				}
			}
			lines = filtered
		}
	}

	f, err := os.CreateTemp("", "aoa-secrets-*")
	if err != nil {
		return err
	}
	f.WriteString(strings.Join(lines, "\n") + "\n")
	f.Close()
	os.Chmod(f.Name(), 0600)
	bundle.EnvFile = f.Name()
	bundle.cleanup = append(bundle.cleanup, f.Name())
	return nil
}

func resolveFromEnvAndDefaults(p ProfileSpec, bundle *Bundle) error {
	var lines []string
	for key, decl := range p.Secrets {
		val := os.Getenv(key)
		if val == "" {
			val = decl.Default
		}
		if val == "" && decl.Required {
			return fmt.Errorf("required secret %q is not set (secretspec CLI not found; set env var or install secretspec)", key)
		}
		if val == "" {
			continue
		}
		if decl.AsPath {
			hostPath, err := bundle.WritePathSecret(key, val)
			if err != nil {
				return err
			}
			if bundle.PathMounts == nil {
				bundle.PathMounts = map[string]string{}
			}
			containerPath := fmt.Sprintf("/run/secrets/%s", strings.ToLower(key))
			bundle.PathMounts[containerPath] = hostPath
		} else {
			lines = append(lines, fmt.Sprintf("%s=%s", key, val))
		}
	}

	f, err := os.CreateTemp("", "aoa-secrets-*")
	if err != nil {
		return err
	}
	f.WriteString(strings.Join(lines, "\n") + "\n")
	f.Close()
	os.Chmod(f.Name(), 0600)
	bundle.EnvFile = f.Name()
	bundle.cleanup = append(bundle.cleanup, f.Name())
	return nil
}
