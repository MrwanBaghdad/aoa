package cmd

import (
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"
)

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Verify security posture and dependencies",
	Long:  `Check that all required dependencies are installed and the security posture is correct.`,
	RunE:  runHealth,
}

func runHealth(cmd *cobra.Command, args []string) error {
	pass := 0
	fail := 0

	check := func(name, desc string, fn func() error) {
		if err := fn(); err != nil {
			fmt.Printf("  FAIL  %s — %s\n", name, err)
			fail++
		} else {
			fmt.Printf("  OK    %s — %s\n", name, desc)
			pass++
		}
	}

	fmt.Println("=== aoa health check ===")
	fmt.Println()

	fmt.Println("Dependencies:")
	check("apple/container", "container runtime", func() error {
		_, err := exec.LookPath("container")
		return err
	})
	check("tmux", "session management", func() error {
		_, err := exec.LookPath("tmux")
		return err
	})
	check("secretspec", "secrets management (optional)", func() error {
		_, err := exec.LookPath("secretspec")
		return err
	})

	fmt.Println("\nEnvironment:")
	check("ANTHROPIC_API_KEY", "LLM API key present", func() error {
		return checkEnvVar("ANTHROPIC_API_KEY")
	})

	fmt.Printf("\nResult: %d passed, %d failed\n", pass, fail)
	if fail > 0 {
		return fmt.Errorf("%d check(s) failed", fail)
	}
	return nil
}

func checkEnvVar(key string) error {
	cmd := exec.Command("bash", "-c", fmt.Sprintf("echo ${%s}", key))
	out, err := cmd.Output()
	if err != nil {
		return err
	}
	if len(out) <= 1 { // just newline
		return fmt.Errorf("not set")
	}
	return nil
}
