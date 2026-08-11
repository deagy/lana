// Package plugin provides plugin management subcommands.
package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

const pluginsDir = ".lana/plugins"

// PluginManifest represents a plugin's manifest file.
type PluginManifest struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
	Path        string `json:"path"`
	Enabled     bool   `json:"enabled"`
}

// NewCommand creates the plugin command group.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Plugin management",
	}
	cmd.AddCommand(pluginListCommand())
	cmd.AddCommand(pluginInstallCommand())
	cmd.AddCommand(pluginEnableCommand())
	cmd.AddCommand(pluginDisableCommand())
	cmd.AddCommand(pluginRemoveCommand())
	cmd.AddCommand(pluginInfoCommand())
	return cmd
}

func pluginListCommand() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List installed plugins",
		RunE: func(cmd *cobra.Command, args []string) error {
			plugins, err := loadPlugins()
			if err != nil {
				fmt.Println("No plugins installed.")
				return nil
			}

			if len(plugins) == 0 {
				fmt.Println("No plugins installed.")
				fmt.Println("To install: lana plugin install <local-path>")
				return nil
			}

			if jsonOutput {
				data, _ := json.MarshalIndent(plugins, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Printf("Installed plugins (%d):\n\n", len(plugins))
			for _, p := range plugins {
				status := "on"
				if !p.Enabled {
					status = "off"
				}
				fmt.Printf("  [%s] %s v%s\n", status, p.Name, p.Version)
				if p.Description != "" {
					fmt.Printf("      %s\n", p.Description)
				}
				fmt.Printf("      Path: %s\n\n", p.Path)
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Output in JSON format")
	return cmd
}

func pluginInstallCommand() *cobra.Command {
	var pName, pVersion string

	cmd := &cobra.Command{
		Use:   "install <source>",
		Short: "Install a plugin from a local directory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			source := args[0]
			absSource, err := filepath.Abs(source)
			if err != nil {
				return fmt.Errorf("resolve source: %w", err)
			}

			info, err := os.Stat(absSource)
			if err != nil {
				return fmt.Errorf("stat source: %w", err)
			}
			if !info.IsDir() {
				return fmt.Errorf("source must be a directory: %s", source)
			}

			if pName == "" {
				pName = filepath.Base(absSource)
			}
			if pVersion == "" {
				pVersion = "0.0.1"
			}

			manifest := PluginManifest{
				Name:        pName,
				Version:     pVersion,
				Description: "Auto-detected plugin",
				Path:        absSource,
				Enabled:     true,
			}
			manifestData, _ := json.MarshalIndent(manifest, "", "  ")

			destDir := filepath.Join(pluginsDir, manifest.Name)
			if err := os.MkdirAll(destDir, 0755); err != nil {
				return fmt.Errorf("create plugin directory: %w", err)
			}
			if err := os.WriteFile(filepath.Join(destDir, "plugin.json"), manifestData, 0644); err != nil {
				return fmt.Errorf("save manifest: %w", err)
			}

			if err := copyDir(absSource, destDir); err != nil {
				return fmt.Errorf("copy plugin files: %w", err)
			}

			plugins, _ := loadPlugins()
			newPlugins := make([]PluginManifest, 0)
			for _, p := range plugins {
				if p.Name != manifest.Name {
					newPlugins = append(newPlugins, p)
				}
			}
			manifest.Path = destDir
			newPlugins = append(newPlugins, manifest)
			if err := savePlugins(newPlugins); err != nil {
				return fmt.Errorf("save plugins: %w", err)
			}

			fmt.Printf("Plugin installed: %s v%s\n", manifest.Name, manifest.Version)
			fmt.Printf("  Path: %s\n", destDir)
			return nil
		},
	}

	cmd.Flags().StringVarP(&pName, "name", "n", "", "Plugin name")
	cmd.Flags().StringVarP(&pVersion, "version", "v", "0.0.1", "Plugin version")
	return cmd
}

func pluginEnableCommand() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "enable <name>",
		Short: "Enable a plugin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name = args[0]
			return togglePlugin(name, true)
		},
	}
	return cmd
}

func pluginDisableCommand() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "disable <name>",
		Short: "Disable a plugin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name = args[0]
			return togglePlugin(name, false)
		},
	}
	return cmd
}

func pluginRemoveCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a plugin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			plugins, err := loadPlugins()
			if err != nil {
				return err
			}

			found := false
			var newPlugins []PluginManifest
			for _, p := range plugins {
				if p.Name == name {
					found = true
					if !force {
						return fmt.Errorf("plugin %s found. Use --force to remove", name)
					}
				} else {
					newPlugins = append(newPlugins, p)
				}
			}

			if !found {
				return fmt.Errorf("plugin not found: %s", name)
			}

			os.RemoveAll(filepath.Join(pluginsDir, name))
			if err := savePlugins(newPlugins); err != nil {
				return fmt.Errorf("save plugins: %w", err)
			}

			fmt.Printf("Plugin removed: %s\n", name)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation")
	return cmd
}

func pluginInfoCommand() *cobra.Command {
	var name string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "info <name>",
		Short: "Show plugin information",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name = args[0]

			plugins, err := loadPlugins()
			if err != nil {
				return err
			}

			for _, p := range plugins {
				if p.Name == name {
					if jsonOutput {
						data, _ := json.MarshalIndent(p, "", "  ")
						fmt.Println(string(data))
						return nil
					}

					fmt.Printf("Name:        %s\n", p.Name)
					fmt.Printf("Version:     %s\n", p.Version)
					fmt.Printf("Enabled:     %v\n", p.Enabled)
					fmt.Printf("Path:        %s\n", p.Path)
					if p.Description != "" {
						fmt.Printf("Description: %s\n", p.Description)
					}
					return nil
				}
			}
			return fmt.Errorf("plugin not found: %s", name)
		},
	}

	cmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Output in JSON format")
	return cmd
}

func loadPlugins() ([]PluginManifest, error) {
	pluginsPath := filepath.Join(pluginsDir, "plugins.json")
	data, err := os.ReadFile(pluginsPath)
	if err != nil {
		return nil, fmt.Errorf("no plugins found")
	}

	var plugins []PluginManifest
	if err := json.Unmarshal(data, &plugins); err != nil {
		return nil, fmt.Errorf("parse plugins: %w", err)
	}
	return plugins, nil
}

func savePlugins(plugins []PluginManifest) error {
	data, err := json.MarshalIndent(plugins, "", "  ")
	if err != nil {
		return err
	}

	pluginsDirAbs := pluginsDir
	if err := os.MkdirAll(pluginsDirAbs, 0755); err != nil {
		return fmt.Errorf("create plugins directory: %w", err)
	}

	return os.WriteFile(filepath.Join(pluginsDirAbs, "plugins.json"), data, 0644)
}

func togglePlugin(name string, enabled bool) error {
	plugins, err := loadPlugins()
	if err != nil {
		return err
	}

	found := false
	for i, p := range plugins {
		if p.Name == name {
			plugins[i].Enabled = enabled
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("plugin not found: %s", name)
	}

	if err := savePlugins(plugins); err != nil {
		return fmt.Errorf("save plugins: %w", err)
	}

	action := "disabled"
	if enabled {
		action = "enabled"
	}
	fmt.Printf("Plugin %s %s\n", name, action)
	return nil
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		if rel == ".git" || strings.HasPrefix(rel, ".git"+string(filepath.Separator)) || strings.HasPrefix(rel, ".git/") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		dstPath := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dstPath, data, info.Mode())
	})
}
