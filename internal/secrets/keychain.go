package secrets

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

const claudeKeychainService = "Claude Code-credentials"

// claudeCredentials mirrors the JSON stored by Claude Code in the macOS Keychain.
type claudeCredentials struct {
	ClaudeAiOauth *oauthCredential `json:"claudeAiOauth"`
}

type oauthCredential struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresAt    int64  `json:"expiresAt"`
}

// FromClaudeKeychain reads Claude Code's OAuth credentials from the macOS
// Keychain and returns a Bundle with CLAUDE_CODE_OAUTH_TOKEN set.
// This mirrors exactly how `claude` CLI authenticates — no token generation needed.
func FromClaudeKeychain() (*Bundle, error) {
	raw, err := readKeychain(claudeKeychainService)
	if err != nil {
		return nil, fmt.Errorf("could not read Claude credentials from Keychain: %w\n  Have you run `claude` and logged in on this machine?", err)
	}

	var creds claudeCredentials
	if err := json.Unmarshal([]byte(raw), &creds); err != nil {
		return nil, fmt.Errorf("parse Claude Keychain credentials: %w", err)
	}

	if creds.ClaudeAiOauth == nil || creds.ClaudeAiOauth.AccessToken == "" {
		return nil, fmt.Errorf("no OAuth credentials found in Keychain — run `claude` to log in first")
	}

	return fromMap(map[string]string{
		"CLAUDE_CODE_OAUTH_TOKEN": creds.ClaudeAiOauth.AccessToken,
	})
}

// ClaudeOAuthToken reads just the access token string from the Keychain.
// Returns ("", nil) if not found (so callers can fall back gracefully).
func ClaudeOAuthToken() (string, error) {
	raw, err := readKeychain(claudeKeychainService)
	if err != nil {
		return "", nil // not logged in — caller decides what to do
	}
	var creds claudeCredentials
	if err := json.Unmarshal([]byte(raw), &creds); err != nil {
		return "", fmt.Errorf("parse Claude Keychain credentials: %w", err)
	}
	if creds.ClaudeAiOauth == nil {
		return "", nil
	}
	return creds.ClaudeAiOauth.AccessToken, nil
}

// readKeychain uses the macOS `security` CLI to read a generic password.
func readKeychain(service string) (string, error) {
	out, err := exec.Command("security", "find-generic-password", "-s", service, "-w").Output()
	if err != nil {
		return "", fmt.Errorf("security find-generic-password: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
