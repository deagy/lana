// Package system provides system-level subcommands.
package system

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/deagy/lana/pkg/config"
	"github.com/deagy/lana/pkg/version"
)

// NewCommand creates the system command group.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "system",
		Short: "System commands",
	}
	cmd.AddCommand(versionCommand())
	cmd.AddCommand(healthCommand())
	cmd.AddCommand(schemaCommand())
	cmd.AddCommand(configCommand())
	cmd.AddCommand(envCommand())
	cmd.AddCommand(dirsCommand())
	return cmd
}

func versionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(version.Info())
		},
	}
}

func healthCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Show health status",
		RunE: func(cmd *cobra.Command, args []string) error {
			issues := []string{}

			if _, err := exec.Command("git", "rev-parse", "--is-inside-work-tree").Output(); err != nil {
				issues = append(issues, "not inside a git repository")
			}

			ws, _ := os.Getwd()

			fmt.Println("Health Check:")
			if len(issues) == 0 {
				fmt.Println("  Status: OK")
			} else {
				fmt.Println("  Status: ISSUES")
				for _, issue := range issues {
					fmt.Printf("  - %s\n", issue)
				}
			}

			fmt.Printf("\n  Go version: %s\n", runtime.Version())
			fmt.Printf("  OS/Arch:    %s/%s\n", runtime.GOOS, runtime.GOARCH)
			fmt.Printf("  CPUs:       %d\n", runtime.NumCPU())
			fmt.Printf("  Working dir: %s\n", ws)
			checkLanaHealth()
			return nil
		},
	}
}

func checkLanaHealth() {
	homeDir, _ := os.UserHomeDir()
	configPath := filepath.Join(homeDir, ".config/lana", "config.yaml")
	if _, err := os.Stat(configPath); err == nil {
		fmt.Printf("  Config:   %s (found)\n", configPath)
	} else {
		fmt.Printf("  Config:   %s (not found)\n", configPath)
	}

	dispatchPath := ".lana/dispatch-state.json"
	if _, err := os.Stat(dispatchPath); err == nil {
		fmt.Printf("  Dispatch: active\n")
	} else {
		fmt.Printf("  Dispatch: no active state\n")
	}

	sdlcPath := ".agentic-sdlc/runs"
	if _, err := os.Stat(sdlcPath); err == nil {
		entries, _ := os.ReadDir(sdlcPath)
		runCount := 0
		for _, e := range entries {
			if e.IsDir() {
				runCount++
			}
		}
		fmt.Printf("  SDLC:     %d runs\n", runCount)
	} else {
		fmt.Printf("  SDLC:     not initialized\n")
	}
}

func schemaCommand() *cobra.Command {
	var full bool

	cmd := &cobra.Command{
		Use:   "schema <type>",
		Short: "Output JSON schema for a type",
		Long: `Output JSON schema for a Lana type.

Supported types: dispatch-plan, run-record, goal, plan, config`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			schemaType := args[0]

			var schema map[string]interface{}

			switch schemaType {
			case "dispatch-plan":
				schema = map[string]interface{}{
					"$schema": "http://json-schema.org/draft-07/schema#",
					"type":    "object",
					"title":   "Dispatch Plan",
					"properties": map[string]interface{}{
						"schema_version": map[string]interface{}{"type": "integer"},
						"task_id":        map[string]interface{}{"type": "string"},
						"status":         map[string]interface{}{"type": "string"},
						"dispatch_plan":  map[string]interface{}{"type": "array"},
						"dependencies":   map[string]interface{}{"type": "array"},
					},
					"required": []string{"schema_version", "task_id", "status"},
				}

			case "run-record":
				schema = map[string]interface{}{
					"$schema": "http://json-schema.org/draft-07/schema#",
					"type":    "object",
					"title":   "Run Record",
					"properties": map[string]interface{}{
						"version":                 map[string]interface{}{"type": "integer"},
						"task_id":                 map[string]interface{}{"type": "string"},
						"current_lifecycle_phase": map[string]interface{}{"type": "string"},
						"lifecycle_gates":         map[string]interface{}{"type": "array"},
					},
					"required": []string{"version", "task_id", "current_lifecycle_phase"},
				}

			case "goal":
				schema = map[string]interface{}{
					"$schema": "http://json-schema.org/draft-07/schema#",
					"type":    "object",
					"title":   "Goal",
					"properties": map[string]interface{}{
						"id":           map[string]interface{}{"type": "string"},
						"objective":    map[string]interface{}{"type": "string"},
						"status":       map[string]interface{}{"type": "string"},
						"token_budget": map[string]interface{}{"type": "integer"},
						"created_at":   map[string]interface{}{"type": "string"},
						"updated_at":   map[string]interface{}{"type": "string"},
					},
					"required": []string{"id", "objective", "status"},
				}

			case "plan":
				schema = map[string]interface{}{
					"$schema": "http://json-schema.org/draft-07/schema#",
					"type":    "object",
					"title":   "Plan",
					"properties": map[string]interface{}{
						"id":         map[string]interface{}{"type": "string"},
						"title":      map[string]interface{}{"type": "string"},
						"steps":      map[string]interface{}{"type": "array"},
						"status":     map[string]interface{}{"type": "string"},
						"created_at": map[string]interface{}{"type": "string"},
						"updated_at": map[string]interface{}{"type": "string"},
					},
					"required": []string{"id", "steps", "status"},
				}

			case "config":
				schema = map[string]interface{}{
					"$schema": "http://json-schema.org/draft-07/schema#",
					"type":    "object",
					"title":   "Configuration",
					"properties": map[string]interface{}{
						"workspace": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"path": map[string]interface{}{"type": "string"},
							},
						},
						"logging": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"level":  map[string]interface{}{"type": "string"},
								"format": map[string]interface{}{"type": "string"},
							},
						},
						"mcp": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"rate_limit": map[string]interface{}{"type": "integer"},
							},
						},
						"exec": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"sandbox": map[string]interface{}{"type": "string"},
							},
						},
					},
					"required": []string{"workspace", "logging", "mcp", "exec"},
				}

			default:
				return fmt.Errorf("unknown schema type: %s (supported: dispatch-plan, run-record, goal, plan, config)", schemaType)
			}

			data, _ := json.MarshalIndent(schema, "", "  ")
			fmt.Println(string(data))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&full, "full", "f", false, "Include all properties")
	return cmd
}

func configCommand() *cobra.Command {
	var showDefaults, jsonOutput bool
	var configPath string

	cmd := &cobra.Command{
		Use:   "config",
		Short: "Show configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.DefaultConfig()

			if configPath == "" {
				homeDir, err := os.UserHomeDir()
				if err == nil {
					configPath = filepath.Join(homeDir, ".config/lana", "config.yaml")
				}
			}

			if configPath != "" {
				if loaded, err := config.Load(configPath); err == nil {
					cfg = loaded
				}
			}

			if jsonOutput {
				data, _ := json.MarshalIndent(cfg.Redacted(), "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Println("Lana Configuration:")
			fmt.Println("===================")
			fmt.Printf("\nWorkspace:\n")
			fmt.Printf("  Path:        %s\n", cfg.Workspace.Path)
			fmt.Printf("  MaxFileSize: %s\n", cfg.Workspace.MaxFileSize)

			fmt.Printf("\nLogging:\n")
			fmt.Printf("  Level:   %s\n", cfg.Logging.Level)
			fmt.Printf("  Format:  %s\n", cfg.Logging.Format)

			fmt.Printf("\nMCP:\n")
			fmt.Printf("  RateLimit: %d\n", cfg.MCP.RateLimit)
			fmt.Printf("  Timeout:   %s\n", cfg.MCP.Timeout)

			fmt.Printf("\nExec:\n")
			fmt.Printf("  Sandbox: %s\n", cfg.Exec.Sandbox)
			fmt.Printf("  Timeout: %s\n", cfg.Exec.Timeout)

			return nil
		},
	}

	cmd.Flags().BoolVarP(&showDefaults, "show-defaults", "d", false, "Show default values")
	cmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Output in JSON format")
	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Config file path")
	return cmd
}

func envCommand() *cobra.Command {
	var secret bool

	cmd := &cobra.Command{
		Use:   "env",
		Short: "Show environment variables",
		RunE: func(cmd *cobra.Command, args []string) error {
			env := os.Environ()
			for _, e := range env {
				parts := strings.SplitN(e, "=", 2)
				if len(parts) != 2 {
					continue
				}
				key := parts[0]
				value := parts[1]

				if config.IsSensitiveKey(key) {
					fmt.Fprintf(cmd.OutOrStdout(), "%s=***REDACTED***\n", key)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "%s=%s\n", key, value)
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&secret, "secret", "s", false, "Include secret variable names (values remain redacted)")
	return cmd
}

func dirsCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "dirs",
		Short: "Show directory paths",
		RunE: func(cmd *cobra.Command, args []string) error {
			homeDir, _ := os.UserHomeDir()
			cwd, _ := os.Getwd()

			fmt.Println("Directory Paths:")
			fmt.Printf("  Home:         %s\n", homeDir)
			fmt.Printf("  Working dir:  %s\n", cwd)
			fmt.Printf("  Config:       %s/.config/lana/\n", homeDir)
			fmt.Printf("  Data:         %s/.local/share/lana/\n", homeDir)
			fmt.Printf("  Plugins:      %s/.local/share/lana/plugins/\n", homeDir)
			fmt.Printf("  Skills:       %s/.local/share/lana/skills/\n", homeDir)
			fmt.Printf("  Local .lana:  %s/.lana/\n", cwd)
			fmt.Printf("  SDLC runs:    %s/.agentic-sdlc/runs/\n", cwd)
			fmt.Printf("  Knowledge:    %s/knowledge-store/\n", cwd)
			return nil
		},
	}
}
