package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/marwan/aoa/internal/session"
)

var attachCmd = &cobra.Command{
	Use:   "attach <session-id>",
	Short: "Attach to a running sandbox session",
	Long: `Attach to a running sandbox session by ID (or ID prefix).

Example:
  aoa list              # find session IDs
  aoa attach a1b2c3d4   # attach to session
`,
	Args: cobra.ExactArgs(1),
	RunE: runAttach,
}

func runAttach(cmd *cobra.Command, args []string) error {
	prefix := args[0]

	mgr, err := session.NewManager()
	if err != nil {
		return err
	}

	sessions, err := mgr.List()
	if err != nil {
		return err
	}

	var target *session.Session
	for _, s := range sessions {
		if len(s.ID) >= len(prefix) && s.ID[:len(prefix)] == prefix {
			if target != nil {
				return fmt.Errorf("ambiguous session ID prefix %q — be more specific", prefix)
			}
			target = s
		}
	}

	if target == nil {
		return fmt.Errorf("no session found with ID prefix %q", prefix)
	}

	if target.Status != session.StatusRunning {
		return fmt.Errorf("session %s is not running (status: %s)", target.ID[:8], target.Status)
	}

	if target.TmuxSession == "" {
		return fmt.Errorf("session %s has no tmux session", target.ID[:8])
	}

	fmt.Printf("Attaching to session %s (slot %d)...\n", target.ID[:8], target.Slot)
	return session.AttachByName(target.TmuxSession)
}
