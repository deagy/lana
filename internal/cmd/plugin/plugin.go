// Package plugin provides plugin management subcommands.
package plugin

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	internalplugin "github.com/deagy/lana/internal/plugin"
)

var pluginsDir = ".lana/plugins"

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
	cmd.AddCommand(pluginGitHubSearchCommand())
	cmd.AddCommand(pluginGitHubInstallCommand())
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
				fmt.Fprintln(cmd.OutOrStdout(), "No plugins installed.")
				return nil
			}

			if len(plugins) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No plugins installed.")
				fmt.Fprintln(cmd.OutOrStdout(), "To install: lana plugin install <local-path>")
				return nil
			}

			if jsonOutput {
				data, _ := json.MarshalIndent(plugins, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Installed plugins (%d):\n\n", len(plugins))
			for _, p := range plugins {
				status := "on"
				if !p.Enabled {
					status = "off"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  [%s] %s v%s\n", status, p.Name, p.Version)
				if p.Description != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "      %s\n", p.Description)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "      Path: %s\n\n", p.Path)
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

			fmt.Fprintf(cmd.OutOrStdout(), "Plugin installed: %s v%s\n", manifest.Name, manifest.Version)
			fmt.Fprintf(cmd.OutOrStdout(), "  Path: %s\n", destDir)
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
			return togglePlugin(cmd.OutOrStdout(), name, true)
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
			return togglePlugin(cmd.OutOrStdout(), name, false)
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

			fmt.Fprintf(cmd.OutOrStdout(), "Plugin removed: %s\n", name)
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
						fmt.Fprintln(cmd.OutOrStdout(), string(data))
						return nil
					}

					fmt.Fprintf(cmd.OutOrStdout(), "Name:        %s\n", p.Name)
					fmt.Fprintf(cmd.OutOrStdout(), "Version:     %s\n", p.Version)
					fmt.Fprintf(cmd.OutOrStdout(), "Enabled:     %v\n", p.Enabled)
					fmt.Fprintf(cmd.OutOrStdout(), "Path:        %s\n", p.Path)
					if p.Description != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "Description: %s\n", p.Description)
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

func togglePlugin(w io.Writer, name string, enabled bool) error {
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
	fmt.Fprintf(w, "Plugin %s %s\n", name, action)
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

func pluginGitHubSearchCommand() *cobra.Command {
	var limit int
	var query string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "github-search [query]",
		Short: "Search for plugins on GitHub",
		Long:  "Search for Lana plugins available on GitHub repositories",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				query = args[0]
			}

			token := os.Getenv("GITHUB_TOKEN")
			client := internalplugin.NewGitHubPluginClient(token)

			plugins, err := client.SearchPlugins(query, limit)
			if err != nil {
				return fmt.Errorf("search plugins: %w", err)
			}

			if len(plugins) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No plugins found.")
				return nil
			}

			if jsonOutput {
				data, _ := json.MarshalIndent(plugins, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Found %d plugins:\n\n", len(plugins))
			for _, p := range plugins {
				fmt.Fprintf(cmd.OutOrStdout(), "  [%s]\n", p.Name)
				if p.Description != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "    %s\n", p.Description)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "    Repository: %s\n\n", p.Repository)
			}
			return nil
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "l", 10, "Maximum results (1-100)")
	cmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Output in JSON format")
	return cmd
}

func pluginGitHubInstallCommand() *cobra.Command {
	var version string

	cmd := &cobra.Command{
		Use:   "github-install <owner/repo> [version]",
		Short: "Install a plugin from GitHub",
		Long:  "Install a Lana plugin from a GitHub repository release",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo := args[0]
			parts := strings.SplitN(repo, "/", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid repository format: %s (expected owner/repo)", repo)
			}

			owner, repoName := parts[0], parts[1]
			if version == "" {
				version = "latest"
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Installing plugin from %s@%s...\n", repo, version)

			token := os.Getenv("GITHUB_TOKEN")
			client := internalplugin.NewGitHubPluginClient(token)

			destDir := filepath.Join(pluginsDir, repoName)
			archivePath, err := client.DownloadPluginArchive(owner, repoName, version, destDir)
			if err != nil {
				return fmt.Errorf("download plugin: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Plugin downloaded: %s\n", archivePath)
			fmt.Fprintf(cmd.OutOrStdout(), "Note: Extract the archive and run 'lana plugin install %s' to complete installation\n", destDir)
			return nil
		},
	}

	cmd.Flags().StringVarP(&version, "version", "v", "", "Specific version to install (default: latest)")
	return cmd
}
