package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const version = "0.1.0"

var cfgFile string

var rootCmd = &cobra.Command{
	Use:          "aoa",
	Short:        "Agent on apple/container — secure AI coding agent sandbox for macOS",
	Long: `aoa runs AI coding agents (Claude Code, opencode, Aider) in isolated VMs
using apple/container (Virtualization.framework). Each agent gets hardware-level
isolation with controlled secrets injection and network egress filtering.`,
	Version:      version,
	SilenceUsage: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ~/.config/aoa/config.toml)")
	rootCmd.AddCommand(shellCmd)
	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(attachCmd)
	rootCmd.AddCommand(healthCmd)
}
