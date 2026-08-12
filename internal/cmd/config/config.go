// Package config provides configuration management subcommands.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/deagy/lana/pkg/config"
)

// NewCommand creates the config command group.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "View and modify Lana configuration",
		Long: `View and modify Lana configuration.

Subcommands:
  show    Show the current configuration
  get     Get a specific configuration value
  set     Set a configuration value
  path    Show the configuration file path

Examples:
  lana config show
  lana config get logging.level
  lana config set logging.level debug
  lana config path
`,
	}
	cmd.AddCommand(configShowCommand())
	cmd.AddCommand(configGetCommand())
	cmd.AddCommand(configSetCommand())
	cmd.AddCommand(configPathCommand())
	return cmd
}

func loadConfig(path string) (*config.Config, error) {
	if path == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get home directory: %w", err)
		}
		path = homeDir + "/.config/lana/config.yaml"
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return config.DefaultConfig(), nil
	}
	cfg, err := config.Load(path)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	return cfg, nil
}

func configShowCommand() *cobra.Command {
	var configPath string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show the current configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(configPath)
			if err != nil {
				return err
			}
			if jsonOutput {
				data, err := json.MarshalIndent(cfg, "", "  ")
				if err != nil {
					return fmt.Errorf("encode JSON: %w", err)
				}
				fmt.Println(string(data))
				return nil
			}
			printConfigText(cmd.OutOrStdout(), cfg)
			return nil
		},
	}
	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Config file path")
	cmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Output in JSON format")
	return cmd
}

func printConfigText(w interface{ Write([]byte) (int, error) }, cfg *config.Config) {
	fmt.Fprintf(w, "Lana Configuration\n")
	fmt.Fprintf(w, "==================\n\n")
	fmt.Fprintf(w, "Workspace:\n")
	fmt.Fprintf(w, "  Path: %s\n", cfg.Workspace.Path)
	fmt.Fprintf(w, "  Max file size: %s\n\n", cfg.Workspace.MaxFileSize)
	fmt.Fprintf(w, "Logging:\n")
	fmt.Fprintf(w, "  Level: %s\n", cfg.Logging.Level)
	fmt.Fprintf(w, "  Format: %s\n", cfg.Logging.Format)
	fmt.Fprintf(w, "  To file: %v\n", cfg.Logging.ToFile)
	if cfg.Logging.ToFile {
		fmt.Fprintf(w, "  File path: %s\n", cfg.Logging.FilePath)
	}
	fmt.Fprintf(w, "\nMCP:\n")
	fmt.Fprintf(w, "  Rate limit: %d\n", cfg.MCP.RateLimit)
	fmt.Fprintf(w, "  Timeout: %s\n", cfg.MCP.Timeout)
	if len(cfg.MCP.Servers) > 0 {
		fmt.Fprintf(w, "  Servers (%d):\n", len(cfg.MCP.Servers))
		for _, s := range cfg.MCP.Servers {
			fmt.Fprintf(w, "    - %s", s.Name)
			if s.URI != "" {
				fmt.Fprintf(w, " (%s)", s.URI)
			}
			if s.Stdio {
				fmt.Fprint(w, " [stdio]")
			}
			fmt.Fprintln(w)
		}
	}
	fmt.Fprintf(w, "\nExec:\n")
	fmt.Fprintf(w, "  Sandbox: %s\n", cfg.Exec.Sandbox)
	fmt.Fprintf(w, "  Timeout: %s\n", cfg.Exec.Timeout)
	if len(cfg.Exec.AllowedEnvPrefixes) > 0 {
		fmt.Fprintf(w, "  Allowed env prefixes: %s\n", strings.Join(cfg.Exec.AllowedEnvPrefixes, ", "))
	}
	if cfg.KnowledgeStore != nil {
		fmt.Fprintf(w, "\nKnowledge Store:\n")
		fmt.Fprintf(w, "  Path: %s\n", cfg.KnowledgeStore.Path)
	}
	if cfg.PluginDir != "" {
		fmt.Fprintf(w, "\nPlugin Dir: %s\n", cfg.PluginDir)
	}
	if cfg.SkillDir != "" {
		fmt.Fprintf(w, "Skill Dir: %s\n", cfg.SkillDir)
	}
	fmt.Fprintf(w, "\nDispatch:\n")
	fmt.Fprintf(w, "  Max parallel: %d\n", cfg.Dispatch.MaxParallel)
	fmt.Fprintf(w, "  Default timeout: %s\n", cfg.Dispatch.DefaultTimeout)
}

func configGetCommand() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Get a specific configuration value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(configPath)
			if err != nil {
				return err
			}
			value, err := config.GetNestedValue(cfg, args[0])
			if err != nil {
				return err
			}
			fmt.Println(value)
			return nil
		},
	}
	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Config file path")
	return cmd
}

func configSetCommand() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if configPath == "" {
				homeDir, err := os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("get home directory: %w", err)
				}
				configPath = homeDir + "/.config/lana/config.yaml"
			}
			cfg, err := loadConfig("")
			if err != nil {
				return err
			}
			if err := config.SetNestedValue(cfg, args[0], args[1]); err != nil {
				return err
			}
			if err := cfg.WriteToPath(configPath); err != nil {
				return fmt.Errorf("write config: %w", err)
			}
			fmt.Printf("Set %s = %s\n", args[0], args[1])
			return nil
		},
	}
	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Config file path")
	return cmd
}

func configPathCommand() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "path",
		Short: "Show the configuration file path",
		RunE: func(cmd *cobra.Command, args []string) error {
			if configPath != "" {
				fmt.Println(configPath)
				return nil
			}
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("get home directory: %w", err)
			}
			path := homeDir + "/.config/lana/config.yaml"
			fmt.Println(path)
			return nil
		},
	}
	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Config file path")
	return cmd
}
