package container

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Runtime wraps the apple/container CLI.
type Runtime struct {
	binary string // path to `container` binary
}

func NewRuntime() (*Runtime, error) {
	bin, err := exec.LookPath("container")
	if err != nil {
		return nil, fmt.Errorf("apple/container not found in PATH — install from https://github.com/apple/container: %w", err)
	}
	return &Runtime{binary: bin}, nil
}

// RunOptions holds configuration for `container run`.
type RunOptions struct {
	Name        string
	Image       string
	Volumes     []string // host:container[:ro]
	EnvFiles    []string // paths to env files
	Env         []string // KEY=VALUE pairs
	Interactive bool
	TTY         bool
	Detach      bool
	Remove      bool // --rm
	Entrypoint  string
	Cmd         []string
}

// Run executes `container run` with the given options.
func (r *Runtime) Run(opts RunOptions) error {
	args := buildRunArgs(opts)
	cmd := exec.Command(r.binary, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// RunDetached starts a container in the background and returns its ID.
func (r *Runtime) RunDetached(opts RunOptions) (string, error) {
	opts.Detach = true
	opts.Interactive = false
	opts.TTY = false

	args := buildRunArgs(opts)
	out, err := exec.Command(r.binary, args...).Output()
	if err != nil {
		return "", fmt.Errorf("container run: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// Exec runs a command inside an existing container.
func (r *Runtime) Exec(containerID string, interactive bool, cmd []string) error {
	args := []string{"exec"}
	if interactive {
		args = append(args, "-i")
		args = append(args, "-t")
	}
	args = append(args, containerID)
	args = append(args, cmd...)

	c := exec.Command(r.binary, args...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

// Stop stops a running container.
func (r *Runtime) Stop(containerID string) error {
	return exec.Command(r.binary, "stop", containerID).Run()
}

// Remove removes a container.
func (r *Runtime) Remove(containerID string) error {
	return exec.Command(r.binary, "rm", containerID).Run()
}

// Build runs `container build`.
func (r *Runtime) Build(dockerfile, tag, contextDir string) error {
	return r.BuildWithTarget(dockerfile, tag, "", contextDir, false)
}

// BuildWithTarget runs `container build` with an optional --target stage and --no-cache.
func (r *Runtime) BuildWithTarget(dockerfile, tag, target, contextDir string, noCache bool) error {
	args := []string{"build", "-f", dockerfile, "-t", tag}
	if target != "" {
		args = append(args, "--target", target)
	}
	if noCache {
		args = append(args, "--no-cache")
	}
	args = append(args, contextDir)
	cmd := exec.Command(r.binary, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ContainerInfo holds basic info about a running container.
type ContainerInfo struct {
	ID     string
	Name   string
	Status string
}

// List returns running container IDs and names using JSON output.
func (r *Runtime) List() ([]ContainerInfo, error) {
	out, err := exec.Command(r.binary, "list", "--all", "--format", "json").Output()
	if err != nil {
		return nil, err
	}

	// apple/container outputs a JSON array of container objects
	var raw []struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parse container list output: %w", err)
	}

	infos := make([]ContainerInfo, 0, len(raw))
	for _, r := range raw {
		infos = append(infos, ContainerInfo{
			ID:     r.ID,
			Name:   r.Name,
			Status: r.Status,
		})
	}
	return infos, nil
}

// ImageExists returns true if an image with the given name is present locally.
func (r *Runtime) ImageExists(name string) bool {
	out, err := exec.Command(r.binary, "image", "list", "--quiet").Output()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.TrimSpace(line) == name {
			return true
		}
	}
	return false
}

func buildRunArgs(opts RunOptions) []string {
	args := []string{"run"}
	if opts.Interactive {
		args = append(args, "-i")
	}
	if opts.TTY {
		args = append(args, "-t")
	}
	if opts.Detach {
		args = append(args, "-d")
	}
	if opts.Remove {
		args = append(args, "--rm")
	}
	if opts.Name != "" {
		args = append(args, "--name", opts.Name)
	}
	for _, v := range opts.Volumes {
		args = append(args, "--volume", v)
	}
	for _, ef := range opts.EnvFiles {
		args = append(args, "--env-file", ef)
	}
	for _, e := range opts.Env {
		args = append(args, "--env", e)
	}
	if opts.Entrypoint != "" {
		args = append(args, "--entrypoint", opts.Entrypoint)
	}
	args = append(args, opts.Image)
	args = append(args, opts.Cmd...)
	return args
}
