package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/marwan/aoa/internal/session"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List sandbox sessions",
	Long:  `List all aoa sandbox sessions (running and stopped).`,
	RunE:  runList,
}

func runList(cmd *cobra.Command, args []string) error {
	mgr, err := session.NewManager()
	if err != nil {
		return err
	}

	sessions, err := mgr.List()
	if err != nil {
		return err
	}

	if len(sessions) == 0 {
		fmt.Println("No sessions found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "ID\tSLOT\tSTATUS\tIMAGE\tWORKSPACE\tAGE"); err != nil {
		return err
	}
	for _, s := range sessions {
		age := time.Since(s.CreatedAt).Round(time.Second)
		if _, err := fmt.Fprintf(w, "%s\t%d\t%s\t%s\t%s\t%s\n",
			s.ID[:8],
			s.Slot,
			s.Status,
			s.Image,
			truncate(s.WorkspaceDir, 40),
			age,
		); err != nil {
			return err
		}
	}
	return w.Flush()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "..." + s[len(s)-n+3:]
}
