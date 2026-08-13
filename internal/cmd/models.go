package cmd

import (
	"context"
	"fmt"

	"github.com/deagy/lana/internal/providers"
	"github.com/spf13/cobra"
)

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "Manage models",
}

var modelsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available models for current provider",
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg == nil {
			fmt.Println("No configuration loaded")
			return nil
		}

		fmt.Printf("Provider: %s\n", cfg.Provider.Name)
		fmt.Printf("Current model: %s\n\n", cfg.Provider.Model)

		factory := providers.NewFactory(
			cfg.Provider.Name,
			cfg.Provider.Model,
			cfg.Provider.Endpoint,
			cfg.Provider.APIKey,
		)

		client, err := factory.Create()
		if err != nil {
			fmt.Printf("Error creating provider: %v\n", err)
			return nil
		}

		models, err := client.SupportedModels(context.Background())
		if err != nil {
			fmt.Printf("Error fetching models: %v\n", err)
			return nil
		}

		if len(models) == 0 {
			fmt.Println("No models available")
			return nil
		}

		fmt.Println("Available models:")
		for _, model := range models {
			marker := " "
			if model.ID == cfg.Provider.Model {
				marker = "*"
			}
			fmt.Printf("%s %s\n", marker, model.ID)
		}

		fmt.Println("\nTo set a model: lana config set provider.model <model>")
		return nil
	},
}

func init() {
	modelsCmd.AddCommand(modelsListCmd)
}
