package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/deagy/lana/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg == nil {
			fmt.Println("No configuration loaded")
			return nil
		}
		fmt.Printf("Provider: %s\n", cfg.Provider.Name)
		fmt.Printf("Model: %s\n", cfg.Provider.Model)
		fmt.Printf("Endpoint: %s\n", cfg.Provider.Endpoint)
		fmt.Printf("Approval Mode: %s\n", cfg.Approval.Mode)
		fmt.Printf("Session Store: %s\n", cfg.Session.StorePath)
		return nil
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a configuration value",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		loader := config.NewLoader()
		if _, err := loader.Load(globalConfigPath); err != nil {
			return err
		}
		value := loader.Get(key)
		fmt.Printf("%s = %v\n", key, value)
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		value := args[1]
		loader := config.NewLoader()
		if _, err := loader.Load(globalConfigPath); err != nil {
			return err
		}
		loader.Set(key, value)

		dir := filepath.Dir(globalConfigPath)
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}

		if err := loader.WriteGlobal(globalConfigPath); err != nil {
			return err
		}

		fmt.Printf("Set %s = %s\n", key, value)
		return nil
	},
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show configuration file path",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(globalConfigPath)
		return nil
	},
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configPathCmd)
}
