package cmd

import (
	"context"
	"fmt"

	"github.com/deagy/lana/internal/providers"
	"github.com/spf13/cobra"
)

var providersCmd = &cobra.Command{
	Use:   "providers",
	Short: "Manage AI providers",
}

var providersListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available providers",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Available providers:")
		for _, name := range providers.AvailableProviders() {
			fmt.Printf("  %s: %s\n", name, providers.ProviderDescription(name))
		}
		fmt.Println()
		fmt.Println("Configure with: lana config set provider.name <name>")
		return nil
	},
}

var providersStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check provider connectivity",
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg == nil {
			fmt.Println("No configuration loaded")
			return nil
		}

		fmt.Printf("Checking provider: %s\n", cfg.Provider.Name)

		factory := providers.NewFactory(
			cfg.Provider.Name,
			cfg.Provider.Model,
			cfg.Provider.Endpoint,
			cfg.Provider.APIKey,
		)

		client, err := factory.Create()
		if err != nil {
			fmt.Printf("✗ Error creating provider: %v\n", err)
			return nil
		}

		// Try to get supported models as a connectivity check
		models, err := client.SupportedModels(context.Background())
		if err != nil {
			fmt.Printf("✗ Connection failed: %v\n", err)
			return nil
		}

		fmt.Printf("✓ Connected to %s\n", client.Name())
		fmt.Printf("  Current model: %s\n", client.Model())
		fmt.Printf("  Available models: %d\n", len(models))
		if len(models) > 0 && len(models) <= 5 {
			for _, m := range models {
				fmt.Printf("    - %s\n", m.ID)
			}
		}

		return nil
	},
}

func init() {
	providersCmd.AddCommand(providersListCmd)
	providersCmd.AddCommand(providersStatusCmd)
}
