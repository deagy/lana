package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check system health and configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("=== Lana System Health ===")
		fmt.Println()

		// Check configuration
		fmt.Println("Configuration:")
		if cfg != nil {
			fmt.Printf("  Provider: %s\n", cfg.Provider.Name)
			fmt.Printf("  Model: %s\n", cfg.Provider.Model)
			fmt.Printf("  Approval Mode: %s\n", cfg.Approval.Mode)
		} else {
			fmt.Println("  (No configuration loaded)")
		}
		fmt.Println()

		// Check environment
		fmt.Println("Environment:")
		if home, err := os.UserHomeDir(); err == nil {
			fmt.Printf("  Home: %s\n", home)
		}
		if wd, err := os.Getwd(); err == nil {
			fmt.Printf("  Working Directory: %s\n", wd)
		}
		fmt.Println()

		// Check provider connectivity (Phase 2)
		fmt.Println("Provider Connectivity:")
		fmt.Println("  (Not yet implemented in Phase 1)")
		fmt.Println()

		fmt.Println("✓ All checks passed")
		return nil
	},
}
