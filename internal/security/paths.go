package security

// ProtectedMount describes a host path to mount read-only inside the container.
type ProtectedMount struct {
	HostPath      string
	ContainerPath string
	ReadOnly      bool
}

// DefaultProtectedPaths returns mounts that should always be read-only
// to prevent supply-chain attacks (e.g., agent modifying git hooks).
func DefaultProtectedPaths(workspaceDir string) []ProtectedMount {
	return []ProtectedMount{
		{
			HostPath:      workspaceDir + "/.git/hooks",
			ContainerPath: "/workspace/.git/hooks",
			ReadOnly:      true,
		},
	}
}

// ToVolumeArgs converts protected mounts to --volume flag values.
func ToVolumeArgs(mounts []ProtectedMount) []string {
	args := make([]string, 0, len(mounts))
	for _, m := range mounts {
		v := m.HostPath + ":" + m.ContainerPath
		if m.ReadOnly {
			v += ":ro"
		}
		args = append(args, v)
	}
	return args
}
