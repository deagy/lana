package cmd

import (
	"fmt"

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
		providers := []struct {
			name        string
			description string
		}{
			{"openai", "OpenAI-compatible API provider"},
			{"ollama", "Local Ollama endpoint"},
		}
		fmt.Println("Available providers:")
		for _, p := range providers {
			fmt.Printf("  %s: %s\n", p.name, p.description)
		}
		return nil
	},
}

func init() {
	providersCmd.AddCommand(providersListCmd)
}
