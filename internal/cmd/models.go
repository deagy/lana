package cmd

import (
	"fmt"

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
		fmt.Printf("Models for provider '%s':\n", cfg.Provider.Name)
		fmt.Println("  (provider not yet initialized)")
		return nil
	},
}

func init() {
	modelsCmd.AddCommand(modelsListCmd)
}
