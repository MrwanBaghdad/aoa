package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"golang.org/x/term"

	"github.com/spf13/cobra"

	"github.com/marwan/aoa/internal/config"
	"github.com/marwan/aoa/internal/container"
	"github.com/marwan/aoa/internal/secrets"
	"github.com/marwan/aoa/internal/security"
	"github.com/marwan/aoa/internal/session"
)

var (
	shellSlot       int
	shellResume     bool
	shellPersistent bool
	shellImage      string
	shellNetwork    string
	shellAgent      string
)

var shellCmd = &cobra.Command{
	Use:   "shell [project-dir]",
	Short: "Launch an agent sandbox session",
	Long: `Launch a new agent sandbox session in an isolated apple/container VM.

The project directory is mounted at /workspace inside the VM. Secrets are
injected via tmpfiles (cleaned up on exit). Network egress is filtered according
to the configured policy.

Examples:
  aoa shell                        # launch in current directory
  aoa shell ~/myproject            # launch with specific project
  aoa shell --slot 2               # use slot 2 (parallel sessions)
  aoa shell --resume               # resume last session in this dir
  aoa shell --persistent           # keep container after exit
  aoa shell --network open         # no network restrictions
`,
	Args: cobra.MaximumNArgs(1),
	RunE: runShell,
}

func init() {
	shellCmd.Flags().IntVar(&shellSlot, "slot", 0, "session slot number (0 = auto-assign)")
	shellCmd.Flags().BoolVar(&shellResume, "resume", false, "resume the last session for this directory")
	shellCmd.Flags().BoolVar(&shellPersistent, "persistent", false, "keep container alive after exit")
	shellCmd.Flags().StringVar(&shellImage, "image", "", "override container image")
	shellCmd.Flags().StringVar(&shellNetwork, "network", "", "network mode: restricted|allowlist|open (overrides config)")
	shellCmd.Flags().StringVar(&shellAgent, "agent", "claude", "agent to launch: claude|opencode|bash")
}

func runShell(cmd *cobra.Command, args []string) error {
	// Resolve project directory
	projectDir, err := resolveProjectDir(args)
	if err != nil {
		return err
	}

	// Load config
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Apply flag overrides
	if shellImage != "" {
		cfg.Sandbox.Image = shellImage
	}
	if shellNetwork != "" {
		cfg.Network.Mode = shellNetwork
	}
	if shellPersistent {
		cfg.Sandbox.Persistent = true
	}

	// Session management
	mgr, err := session.NewManager()
	if err != nil {
		return fmt.Errorf("session manager: %w", err)
	}

	// Resume mode: reattach to existing tmux session
	if shellResume {
		slot := shellSlot
		if slot == 0 {
			slot = 1
		}
		s, err := mgr.FindBySlot(slot, projectDir)
		if err != nil {
			return fmt.Errorf("resume: %w", err)
		}
		fmt.Printf("Resuming session %s (slot %d)\n", s.ID[:8], s.Slot)
		return session.AttachByName(s.TmuxSession)
	}

	// Assign slot
	slot := shellSlot
	if slot == 0 {
		slot, err = mgr.NextSlot(projectDir)
		if err != nil {
			return err
		}
	}

	// Initialize apple/container runtime
	rt, err := container.NewRuntime()
	if err != nil {
		return err
	}

	// Resolve secrets
	bundle, err := resolveSecrets(cfg, projectDir)
	if err != nil {
		return fmt.Errorf("secrets: %w", err)
	}
	defer bundle.Cleanup()

	// Ensure cleanup on interrupt
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigs
		bundle.Cleanup()
		os.Exit(0)
	}()

	// Create session record
	s, err := mgr.Create(slot, projectDir, cfg.Sandbox.Image, cfg.Sandbox.Persistent)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	// Build volumes
	volumes := []string{
		fmt.Sprintf("%s:/workspace", projectDir),
	}
	// Protected paths (read-only .git/hooks)
	gitHooksDir := filepath.Join(projectDir, ".git", "hooks")
	if _, err := os.Stat(gitHooksDir); err == nil {
		protectedMounts := security.DefaultProtectedPaths(projectDir)
		volumes = append(volumes, security.ToVolumeArgs(protectedMounts)...)
	}
	// as_path secret mounts
	volumes = append(volumes, bundle.Volumes()...)

	// Build container name from session
	containerName := fmt.Sprintf("aoa-%s", s.ID[:8])

	// Determine agent command inside container
	agentCmd := agentCommand(shellAgent)

	fmt.Printf("Starting aoa session %s (slot %d) in %s\n", s.ID[:8], slot, projectDir)
	fmt.Printf("Image: %s | Network: %s | Agent: %s\n", cfg.Sandbox.Image, cfg.Network.Mode, shellAgent)

	isTTY := term.IsTerminal(int(os.Stdin.Fd()))

	opts := container.RunOptions{
		Name:        containerName,
		Image:       cfg.Sandbox.Image,
		Volumes:     volumes,
		EnvFiles:    []string{bundle.EnvFile},
		Interactive: true,
		TTY:         isTTY,
		Remove:      !cfg.Sandbox.Persistent,
		Env: []string{
			fmt.Sprintf("AOA_NETWORK_MODE=%s", cfg.Network.Mode),
			fmt.Sprintf("AOA_SESSION_ID=%s", s.ID),
		},
		Cmd: agentCmd,
	}

	s.ContainerID = containerName
	mgr.Update(s)

	// Run the container (blocks until exit)
	runErr := rt.Run(opts)

	// Mark session stopped
	s.Status = session.StatusStopped
	mgr.Update(s)

	if runErr != nil {
		// Exit codes from the container are normal — don't treat as error
		if isExitError(runErr) {
			return nil
		}
		return runErr
	}
	return nil
}

func resolveProjectDir(args []string) (string, error) {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(abs); err != nil {
		return "", fmt.Errorf("project directory not found: %s", abs)
	}
	return abs, nil
}

func resolveSecrets(cfg *config.Config, projectDir string) (*secrets.Bundle, error) {
	// 1. Project-level secretspec.toml takes priority.
	specPath := filepath.Join(projectDir, "secretspec.toml")
	if _, err := os.Stat(specPath); err == nil {
		spec, err := secrets.LoadSecretSpec(projectDir)
		if err != nil {
			return nil, fmt.Errorf("parse secretspec.toml: %w", err)
		}
		bundle := &secrets.Bundle{}
		if err := secrets.Resolve(spec, "default", bundle); err != nil {
			return nil, err
		}
		return bundle, nil
	}

	// 2. Explicit env vars from config (e.g. ANTHROPIC_API_KEY set in shell).
	bundle, err := secrets.FromEnv(cfg.Secrets.EnvKeys)
	if err != nil {
		return nil, err
	}

	// 3. If no LLM auth token ended up in the bundle, fall back to
	//    the Claude Code credentials already stored in the macOS Keychain.
	//    This means `aoa shell` just works if you've already run `claude login`.
	if !bundle.HasLLMAuth() {
		if keychainBundle, err := secrets.FromClaudeKeychain(); err == nil {
			bundle.Cleanup()
			fmt.Println("Auth: using Claude credentials from macOS Keychain")
			return keychainBundle, nil
		}
		// Nothing worked — tell the user what to do.
		fmt.Fprintln(os.Stderr, "warning: no LLM credentials found — set ANTHROPIC_API_KEY, or log in with `claude`")
	}

	return bundle, nil
}

func agentCommand(agent string) []string {
	switch agent {
	case "claude":
		return []string{"/bin/bash", "-c", "cd /workspace && claude --dangerously-skip-permissions"}
	case "opencode":
		return []string{"/bin/bash", "-c", "cd /workspace && opencode"}
	case "bash":
		return []string{"/bin/bash"}
	default:
		return []string{"/bin/bash", "-c", fmt.Sprintf("cd /workspace && %s", agent)}
	}
}

func isExitError(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(interface{ ExitCode() int })
	return ok
}
