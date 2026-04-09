package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/marwan/aoa/internal/container"
)

var healthCmd = &cobra.Command{
	Use:          "health",
	Short:        "Verify security posture and dependencies",
	Long:         `Check that all required dependencies are installed and the security posture is correct.`,
	RunE:         runHealth,
	SilenceUsage: true,
}

func runHealth(cmd *cobra.Command, args []string) error {
	pass := 0
	fail := 0
	warn := 0

	required := func(name, desc string, fn func() error) {
		if err := fn(); err != nil {
			fmt.Printf("  FAIL  %s — %s\n", name, err)
			fail++
		} else {
			fmt.Printf("  OK    %s — %s\n", name, desc)
			pass++
		}
	}

	optional := func(name, desc string, fn func() error) {
		if err := fn(); err != nil {
			fmt.Printf("  WARN  %s — %s\n", name, err)
			warn++
		} else {
			fmt.Printf("  OK    %s — %s\n", name, desc)
			pass++
		}
	}

	fmt.Println("=== aoa health check ===")
	fmt.Println()

	fmt.Println("Dependencies:")
	required("apple/container", "container runtime", func() error {
		_, err := exec.LookPath("container")
		return err
	})
	required("tmux", "session management", func() error {
		_, err := exec.LookPath("tmux")
		return err
	})
	optional("secretspec", "multi-provider secrets (1Password, Keychain, Vault)", func() error {
		_, err := exec.LookPath("secretspec")
		return err
	})

	fmt.Println("\nEnvironment:")
	required("ANTHROPIC_API_KEY", "LLM API key present", func() error {
		if v := os.Getenv("ANTHROPIC_API_KEY"); v == "" {
			return fmt.Errorf("not set — export ANTHROPIC_API_KEY=sk-ant-...")
		}
		return nil
	})

	fmt.Println("\nImages:")
	optional("aoa-agent:latest", "agent sandbox image built", func() error {
		rt, err := container.NewRuntime()
		if err != nil {
			return err
		}
		if !rt.ImageExists("aoa-agent:latest") {
			return fmt.Errorf("not built — run: aoa build")
		}
		return nil
	})

	fmt.Println()
	if warn > 0 {
		fmt.Printf("Result: %d passed, %d failed, %d warnings\n", pass, fail, warn)
	} else {
		fmt.Printf("Result: %d passed, %d failed\n", pass, fail)
	}

	if fail > 0 {
		return fmt.Errorf("%d required check(s) failed", fail)
	}
	return nil
}
