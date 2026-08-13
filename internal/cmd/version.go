package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Lana v0.1.0\n")
		fmt.Printf("Build: Phase 1 Scaffolding\n")
		return nil
	},
}
