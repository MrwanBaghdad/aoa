package container

import (
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
	Remove      bool     // --rm
	Entrypoint  string
	Cmd         []string
}

// Run executes `container run` with the given options, replacing the current process (exec).
func (r *Runtime) Run(opts RunOptions) error {
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

// Exec runs a command inside an existing container, replacing the current process.
func (r *Runtime) Exec(containerID string, interactive bool, cmd []string) error {
	args := []string{"exec"}
	if interactive {
		args = append(args, "-it")
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
	args := []string{"build", "-f", dockerfile, "-t", tag, contextDir}
	cmd := exec.Command(r.binary, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// List returns running container IDs and names.
func (r *Runtime) List() ([]ContainerInfo, error) {
	out, err := exec.Command(r.binary, "ps", "--format", "{{.ID}}\t{{.Names}}\t{{.Status}}").Output()
	if err != nil {
		return nil, err
	}
	var infos []ContainerInfo
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		info := ContainerInfo{}
		if len(parts) > 0 {
			info.ID = parts[0]
		}
		if len(parts) > 1 {
			info.Name = parts[1]
		}
		if len(parts) > 2 {
			info.Status = parts[2]
		}
		infos = append(infos, info)
	}
	return infos, nil
}

type ContainerInfo struct {
	ID     string
	Name   string
	Status string
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
