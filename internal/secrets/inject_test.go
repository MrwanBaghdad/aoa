package secrets

import (
	"os"
	"strings"
	"testing"
)

func TestFromEnvBasic(t *testing.T) {
	t.Setenv("TEST_KEY_ONE", "value-one")
	t.Setenv("TEST_KEY_TWO", "value-two")

	bundle, err := FromEnv([]string{"TEST_KEY_ONE", "TEST_KEY_TWO"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer bundle.Cleanup()

	if bundle.EnvFile == "" {
		t.Fatal("expected non-empty EnvFile path")
	}

	data, err := os.ReadFile(bundle.EnvFile)
	if err != nil {
		t.Fatalf("could not read env file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "TEST_KEY_ONE=value-one") {
		t.Errorf("env file missing TEST_KEY_ONE: %s", content)
	}
	if !strings.Contains(content, "TEST_KEY_TWO=value-two") {
		t.Errorf("env file missing TEST_KEY_TWO: %s", content)
	}
}

func TestFromEnvMissingKeyWarns(t *testing.T) {
	os.Unsetenv("DEFINITELY_NOT_SET_XYZ")
	// Should not return an error — missing keys are warned, not fatal
	bundle, err := FromEnv([]string{"DEFINITELY_NOT_SET_XYZ"})
	if err != nil {
		t.Fatalf("expected no error for missing optional key, got: %v", err)
	}
	defer bundle.Cleanup()
}

func TestFromEnvFilePermissions(t *testing.T) {
	t.Setenv("TEST_SECRET_PERM", "supersecret")
	bundle, err := FromEnv([]string{"TEST_SECRET_PERM"})
	if err != nil {
		t.Fatal(err)
	}
	defer bundle.Cleanup()

	info, err := os.Stat(bundle.EnvFile)
	if err != nil {
		t.Fatal(err)
	}
	mode := info.Mode().Perm()
	if mode != 0600 {
		t.Errorf("expected permissions 0600, got %04o", mode)
	}
}

func TestCleanupRemovesFile(t *testing.T) {
	t.Setenv("TEST_CLEANUP_KEY", "val")
	bundle, err := FromEnv([]string{"TEST_CLEANUP_KEY"})
	if err != nil {
		t.Fatal(err)
	}
	path := bundle.EnvFile
	if _, err := os.Stat(path); err != nil {
		t.Fatal("file should exist before cleanup")
	}

	bundle.Cleanup()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file should not exist after cleanup")
	}
}

func TestCleanupIdempotent(t *testing.T) {
	t.Setenv("TEST_IDEMPOTENT_KEY", "val")
	bundle, err := FromEnv([]string{"TEST_IDEMPOTENT_KEY"})
	if err != nil {
		t.Fatal(err)
	}
	bundle.Cleanup()
	// Should not panic on second call
	bundle.Cleanup()
}

func TestWritePathSecret(t *testing.T) {
	bundle := &Bundle{}
	defer bundle.Cleanup()

	path, err := bundle.WritePathSecret("DB_URL", "postgres://localhost/dev")
	if err != nil {
		t.Fatal(err)
	}
	if path == "" {
		t.Fatal("expected non-empty path")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "postgres://localhost/dev" {
		t.Errorf("unexpected content: %q", string(data))
	}
}

func TestWritePathSecretPermissions(t *testing.T) {
	bundle := &Bundle{}
	defer bundle.Cleanup()

	path, err := bundle.WritePathSecret("MY_CERT", "cert-data")
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("expected 0600, got %04o", info.Mode().Perm())
	}
}

func TestVolumesReturnsROMount(t *testing.T) {
	bundle := &Bundle{
		PathMounts: map[string]string{
			"/run/secrets/db_url": "/tmp/some-secret-file",
		},
	}
	vols := bundle.Volumes()
	if len(vols) != 1 {
		t.Fatalf("expected 1 volume, got %d", len(vols))
	}
	if !strings.HasSuffix(vols[0], ":ro") {
		t.Errorf("volume should be read-only: %s", vols[0])
	}
}
