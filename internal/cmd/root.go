package cmd

import (
	"context"
	"os"
	"path/filepath"

	"github.com/deagy/lana/internal/config"
	"github.com/deagy/lana/internal/plugin"
	"github.com/spf13/cobra"
)

var (
	globalConfigPath string
	cfg              *config.Config
)

// Execute runs the root command.
func Execute() error {
	registerInstalledPlugins()
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

	rootCmd.AddCommand(chatCmd)
	rootCmd.AddCommand(runCmd)
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

// registerInstalledPlugins scans for installed plugins and registers them as subcommands.
// Non-fatal errors (e.g., one broken plugin) are logged to stderr but don't stop registration.
func registerInstalledPlugins() {
	pluginsDir := defaultPluginsPath()

	plugins, err := plugin.InstalledPlugins(pluginsDir)
	if err != nil {
		// Log but don't fail
		return
	}

	for _, p := range plugins {
		// Capture plugin reference for closure
		pCopy := p
		pluginDirCopy := filepath.Join(pluginsDir, p.Name)

		cmd := &cobra.Command{
			Use:                pCopy.Name,
			Short:              pCopy.Description,
			DisableFlagParsing: true,
			RunE: func(cmd *cobra.Command, args []string) error {
				ctx := context.Background()
				return plugin.Run(ctx, pluginDirCopy, pCopy, args, os.Stdin, os.Stdout, os.Stderr)
			},
		}

		rootCmd.AddCommand(cmd)
	}
}
