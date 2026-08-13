package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/deagy/lana/internal/providers"
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

		// Check provider connectivity
		fmt.Println("Provider Connectivity:")
		if cfg != nil {
			factory := providers.NewFactory(
				cfg.Provider.Name,
				cfg.Provider.Model,
				cfg.Provider.Endpoint,
				cfg.Provider.APIKey,
			)

			client, err := factory.Create()
			if err != nil {
				fmt.Printf("  ✗ Error creating provider: %v\n", err)
			} else {
				models, err := client.SupportedModels(context.Background())
				if err != nil {
					fmt.Printf("  ✗ Connection failed: %v\n", err)
				} else {
					fmt.Printf("  ✓ Connected to %s\n", client.Name())
					fmt.Printf("    Current model: %s\n", client.Model())
					fmt.Printf("    Available models: %d\n", len(models))
				}
			}
		}
		fmt.Println()

		fmt.Println("✓ System health check complete")
		return nil
	},
}
