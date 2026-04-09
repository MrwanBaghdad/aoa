package cmd

import (
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/marwan/aoa/internal/container"
)

var (
	buildDockerfile string
	buildTag        string
	buildTarget     string
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build the agent sandbox container image",
	Long: `Build the aoa-agent container image using apple/container's BuildKit.

Examples:
  aoa build                         # build default agent image
  aoa build --target base           # build only the base image
  aoa build --tag my-agent:latest   # custom tag
`,
	RunE: runBuild,
}

func init() {
	buildCmd.Flags().StringVar(&buildDockerfile, "file", "", "Dockerfile path (default: images/Dockerfile.agent)")
	buildCmd.Flags().StringVar(&buildTag, "tag", "aoa-agent:latest", "image tag")
	buildCmd.Flags().StringVar(&buildTarget, "target", "agent", "build target: base|agent")
}

func runBuild(cmd *cobra.Command, args []string) error {
	rt, err := container.NewRuntime()
	if err != nil {
		return err
	}

	// Find the images directory relative to the binary or source
	imagesDir := findImagesDir()

	dockerfile := buildDockerfile
	if dockerfile == "" {
		switch buildTarget {
		case "base":
			dockerfile = filepath.Join(imagesDir, "Dockerfile.base")
			if buildTag == "aoa-agent:latest" {
				buildTag = "aoa-base:latest"
			}
		default:
			dockerfile = filepath.Join(imagesDir, "Dockerfile.agent")
		}
	}

	fmt.Printf("Building %s from %s...\n", buildTag, dockerfile)
	return rt.Build(dockerfile, buildTag, ".")
}

func findImagesDir() string {
	// Try relative to binary location, then current dir
	_, file, _, ok := runtime.Caller(0)
	if ok {
		// In dev: file is .../cmd/build.go, images is .../images/
		return filepath.Join(filepath.Dir(filepath.Dir(file)), "images")
	}
	return "images"
}
