package security

import (
	"strings"
	"testing"
)

func TestDefaultProtectedPaths(t *testing.T) {
	mounts := DefaultProtectedPaths("/home/user/myproject")
	if len(mounts) == 0 {
		t.Fatal("expected at least one protected mount")
	}

	// .git/hooks must always be protected
	found := false
	for _, m := range mounts {
		if strings.Contains(m.HostPath, ".git/hooks") {
			found = true
			if !m.ReadOnly {
				t.Error(".git/hooks must be read-only")
			}
			if m.ContainerPath == "" {
				t.Error("container path must not be empty")
			}
		}
	}
	if !found {
		t.Error("default protected paths must include .git/hooks")
	}
}

func TestToVolumeArgsReadOnly(t *testing.T) {
	mounts := []ProtectedMount{
		{
			HostPath:      "/host/path",
			ContainerPath: "/container/path",
			ReadOnly:      true,
		},
	}
	args := ToVolumeArgs(mounts)
	if len(args) != 1 {
		t.Fatalf("expected 1 arg, got %d", len(args))
	}
	if !strings.HasSuffix(args[0], ":ro") {
		t.Errorf("expected :ro suffix, got %q", args[0])
	}
	if !strings.Contains(args[0], "/host/path") {
		t.Error("missing host path")
	}
	if !strings.Contains(args[0], "/container/path") {
		t.Error("missing container path")
	}
}

func TestToVolumeArgsReadWrite(t *testing.T) {
	mounts := []ProtectedMount{
		{HostPath: "/h", ContainerPath: "/c", ReadOnly: false},
	}
	arg := ToVolumeArgs(mounts)[0]
	if strings.HasSuffix(arg, ":ro") {
		t.Error("read-write mount should not have :ro suffix")
	}
}

func TestToVolumeArgsEmpty(t *testing.T) {
	args := ToVolumeArgs(nil)
	if len(args) != 0 {
		t.Errorf("expected 0 args, got %d", len(args))
	}
}

func TestDefaultProtectedPathsWorkspaceScoped(t *testing.T) {
	ws := "/Users/marwan/myproject"
	mounts := DefaultProtectedPaths(ws)
	for _, m := range mounts {
		if !strings.HasPrefix(m.HostPath, ws) {
			t.Errorf("host path %q should be under workspace %q", m.HostPath, ws)
		}
	}
}
