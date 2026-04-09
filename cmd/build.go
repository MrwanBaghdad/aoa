package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/marwan/aoa/internal/container"
)

var (
	buildTag        string
	buildTarget     string
	buildNoCache    bool
)

var buildCmd = &cobra.Command{
	Use:          "build",
	Short:        "Build the agent sandbox container image",
	SilenceUsage: true,
	Long: `Build the aoa-agent container image using apple/container's BuildKit.

The default build produces aoa-agent:latest using images/Dockerfile (multi-stage).
Use --target base to build only the base layer.

Examples:
  aoa build                         # build aoa-agent:latest
  aoa build --target base           # build only the base image (aoa-base:latest)
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

	imagesDir := findImagesDir()
	dockerfile := filepath.Join(imagesDir, "Dockerfile")

	// Verify the Dockerfile exists
	if _, err := os.Stat(dockerfile); err != nil {
		return fmt.Errorf("dockerfile not found at %s — are you running from the aoa repo root?", dockerfile)
	}

	// Default tag based on target
	tag := buildTag
	if tag == "" {
		switch buildTarget {
		case "base":
			tag = "aoa-base:latest"
		default:
			tag = "aoa-agent:latest"
		}
	}

	// Context dir is the repo root (scripts/ must be accessible from Dockerfile)
	contextDir := findRepoRoot()

	fmt.Printf("Building %s (target: %s)...\n", tag, buildTarget)
	return rt.BuildWithTarget(dockerfile, tag, buildTarget, contextDir, buildNoCache)
}

func findImagesDir() string {
	// In dev: file is .../cmd/build.go → images/ is one level up
	_, file, _, ok := runtime.Caller(0)
	if ok {
		return filepath.Join(filepath.Dir(filepath.Dir(file)), "images")
	}
	return "images"
}

func findRepoRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if ok {
		return filepath.Dir(filepath.Dir(file))
	}
	wd, _ := os.Getwd()
	return wd
}
