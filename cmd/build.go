package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/marwan/aoa/internal/container"
)

var (
	buildTag     string
	buildTarget  string
	buildNoCache bool

	// buildAssets is set by main via SetBuildAssets before Execute() is called.
	// It holds the embedded images/ and scripts/ directories.
	buildAssets fs.FS
)

// SetBuildAssets wires the embedded build assets (Dockerfiles + scripts) into
// the build command. Called once from main() before cmd.Execute().
func SetBuildAssets(f fs.FS) {
	buildAssets = f
}

var buildCmd = &cobra.Command{
	Use:          "build",
	Short:        "Build the agent sandbox container image",
	SilenceUsage: true,
	Long: `Build the aoa-agent container image using apple/container's BuildKit.

The Dockerfile and entrypoint scripts are embedded in the aoa binary, so
this command works whether aoa was installed via Homebrew or built from source.

Examples:
  aoa build                         # build aoa-agent:latest
  aoa build --target base           # build only the base layer (aoa-base:latest)
  aoa build --tag my-agent:v2       # custom tag
  aoa build --no-cache              # force full rebuild
`,
	RunE: runBuild,
}

func init() {
	buildCmd.Flags().StringVar(&buildTag, "tag", "", "image tag (default: aoa-base:latest or aoa-agent:latest)")
	buildCmd.Flags().StringVar(&buildTarget, "target", "agent", "build stage: base|agent")
	buildCmd.Flags().BoolVar(&buildNoCache, "no-cache", false, "do not use layer cache")
}

func runBuild(cmd *cobra.Command, args []string) error {
	rt, err := container.NewRuntime()
	if err != nil {
		return err
	}

	// Write the embedded build context (images/ + scripts/) to a temp directory.
	// apple/container's BuildKit needs real files on disk.
	buildCtx, err := extractBuildAssets()
	if err != nil {
		return fmt.Errorf("extracting build assets: %w", err)
	}
	defer os.RemoveAll(buildCtx)

	tag := buildTag
	if tag == "" {
		switch buildTarget {
		case "base":
			tag = "aoa-base:latest"
		default:
			tag = "aoa-agent:latest"
		}
	}

	dockerfile := filepath.Join(buildCtx, "images", "Dockerfile")
	fmt.Printf("Building %s (target: %s)...\n", tag, buildTarget)
	return rt.BuildWithTarget(dockerfile, tag, buildTarget, buildCtx, buildNoCache)
}

// extractBuildAssets writes the embedded images/ and scripts/ directories to
// a temporary directory and returns its path. The caller is responsible for
// removing it.
func extractBuildAssets() (string, error) {
	tmp, err := os.MkdirTemp("", "aoa-build-*")
	if err != nil {
		return "", err
	}

	err = fs.WalkDir(buildAssets, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		dest := filepath.Join(tmp, path)
		if d.IsDir() {
			return os.MkdirAll(dest, 0755)
		}
		data, err := fs.ReadFile(buildAssets, path)
		if err != nil {
			return err
		}
		// Preserve executable bits for scripts
		mode := fs.FileMode(0644)
		if info, err := d.Info(); err == nil {
			mode = info.Mode()
		}
		return os.WriteFile(dest, data, mode)
	})
	if err != nil {
		os.RemoveAll(tmp)
		return "", err
	}
	return tmp, nil
}
