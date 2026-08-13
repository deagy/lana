package cmd

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/deagy/lana/internal/config"
)

var (
	globalConfigPath string
	cfg              *config.Config
)

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

var rootCmd = &cobra.Command{
	Use:   "lana",
	Short: "A terminal-first coding agent",
	Long: `Lana is a local coding agent CLI that provides interactive terminal conversations,
noninteractive prompt execution, and structured agent orchestration.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Load configuration
		loader := config.NewLoader()
		var err error
		cfg, err = loader.Load(globalConfigPath)
		if err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(
		&globalConfigPath,
		"config",
		defaultGlobalConfigPath(),
		"path to global config file",
	)

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(providersCmd)
	rootCmd.AddCommand(modelsCmd)
	rootCmd.AddCommand(sessionsCmd)
	rootCmd.AddCommand(doctorCmd)
}

func defaultGlobalConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/.lana/config.yaml"
	}
	return filepath.Join(home, ".lana", "config.yaml")
}
