package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/deagy/lana/internal/config"
	"github.com/deagy/lana/internal/plugin"
	"github.com/spf13/cobra"
)

var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Manage Lana CLI plugins",
	Long: `Install and manage CLI plugins that extend Lana with custom commands.

Plugins are executables that become new 'lana <name>' subcommands.
You can discover plugins locally and install them to extend Lana's functionality.`,
}

var pluginListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed plugins",
	RunE: func(cmd *cobra.Command, args []string) error {
		pluginsDir := defaultPluginsPath()

		plugins, err := plugin.InstalledPlugins(pluginsDir)
		if err != nil {
			return fmt.Errorf("scan plugins: %w", err)
		}

		if len(plugins) == 0 {
			fmt.Println("No plugins installed.")
			fmt.Println("Use 'lana plugin install <path>' to install a plugin.")
			return nil
		}

		fmt.Println("Installed Plugins:")
		fmt.Println()

		for i, p := range plugins {
			fmt.Printf("%d. %s (%s)\n", i+1, p.Name, p.Version)
			if p.Description != "" {
				fmt.Printf("   %s\n", p.Description)
			}
			fmt.Printf("   Entrypoint: %s\n", p.Entrypoint)
			fmt.Println()
		}

		return nil
	},
}

var pluginInstallCmd = &cobra.Command{
	Use:   "install <path>",
	Short: "Install a plugin from a directory",
	Long: `Install a plugin from a local directory.

The directory must contain a manifest.yaml file that defines the plugin.
Plugins become available as 'lana <name>' subcommands.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg == nil {
			fmt.Fprintf(os.Stderr, "Error: configuration not loaded\n")
			os.Exit(1)
		}

		sourcePath := args[0]

		// Resolve absolute path
		absPath, err := filepath.Abs(sourcePath)
		if err != nil {
			return fmt.Errorf("resolve path: %w", err)
		}

		// Define reserved built-in command names
		reserved := map[string]bool{
			"chat":      true,
			"run":       true,
			"version":   true,
			"config":    true,
			"providers": true,
			"models":    true,
			"sessions":  true,
			"doctor":    true,
			"mcp":       true,
			"plugin":    true, // Prevent plugin named "plugin"
		}

		// Install the plugin
		pluginsDir := defaultPluginsPath()
		manifest, err := plugin.Install(absPath, pluginsDir, reserved)
		if err != nil {
			return fmt.Errorf("install plugin: %w", err)
		}

		fmt.Printf("Installed plugin '%s' (v%s)\n", manifest.Name, manifest.Version)

		// If the plugin declares MCP servers, register them
		if len(manifest.MCPServers) > 0 {
			fmt.Printf("Registering %d MCP server(s)...\n", len(manifest.MCPServers))

			loader := config.NewLoader()
			_, err := loader.Load(globalConfigPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			// Rewrite MCP server commands to absolute paths inside the plugin directory
			pluginDir := filepath.Join(pluginsDir, manifest.Name)
			for i := range manifest.MCPServers {
				if manifest.MCPServers[i].Command != "" {
					manifest.MCPServers[i].Command = filepath.Join(pluginDir, manifest.MCPServers[i].Command)
				}
			}

			// Merge with existing MCP servers
			cfg.MCP.Servers = append(cfg.MCP.Servers, manifest.MCPServers...)

			loader.Set("mcp.servers", cfg.MCP.Servers)
			if err := loader.WriteGlobal(globalConfigPath); err != nil {
				return fmt.Errorf("write config: %w", err)
			}

			fmt.Printf("Registered %d MCP server(s)\n", len(manifest.MCPServers))
		}

		return nil
	},
}

var pluginRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Uninstall a plugin",
	Long: `Remove an installed plugin.

This deletes the plugin's directory. If the plugin registered MCP servers,
you may want to remove them manually with 'lana mcp remove'.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		pluginsDir := defaultPluginsPath()

		if err := plugin.Remove(pluginsDir, name); err != nil {
			return fmt.Errorf("remove plugin: %w", err)
		}

		fmt.Printf("Removed plugin '%s'\n", name)
		return nil
	},
}

var pluginInfoCmd = &cobra.Command{
	Use:   "info <name>",
	Short: "Show information about a plugin",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		pluginsDir := defaultPluginsPath()

		p, err := plugin.FindPlugin(pluginsDir, name)
		if err != nil {
			return fmt.Errorf("find plugin: %w", err)
		}

		fmt.Printf("Plugin: %s\n", p.Name)
		fmt.Printf("Version: %s\n", p.Version)
		if p.Description != "" {
			fmt.Printf("Description: %s\n", p.Description)
		}
		fmt.Printf("Entrypoint: %s\n", p.Entrypoint)

		if len(p.MCPServers) > 0 {
			fmt.Printf("\nMCP Servers: %d\n", len(p.MCPServers))
			for i, srv := range p.MCPServers {
				fmt.Printf("  %d. %s (%s)\n", i+1, srv.Name, srv.Transport)
			}
		}

		return nil
	},
}

func defaultPluginsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".lana", "plugins")
	}
	return filepath.Join(home, ".lana", "plugins")
}

func init() {
	// Add subcommands to plugin command
	pluginCmd.AddCommand(pluginListCmd)
	pluginCmd.AddCommand(pluginInstallCmd)
	pluginCmd.AddCommand(pluginRemoveCmd)
	pluginCmd.AddCommand(pluginInfoCmd)

	// Add plugin command to root
	rootCmd.AddCommand(pluginCmd)
}
