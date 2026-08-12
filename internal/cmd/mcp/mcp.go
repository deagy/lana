// Package mcp provides MCP server interaction subcommands.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/deagy/lana/internal/mcp"
	"github.com/deagy/lana/pkg/config"
)

// serverTimeout is the default per-request timeout for MCP calls.
const serverTimeout = 30 * time.Second

// serverManagerKey is the context key used to store the ServerManager in the
// cobra command context so all subcommands share a single manager.
type serverManagerKey struct{}

// newServerManager builds a fresh ServerManager and stores it on the command
// context so every subcommand reuses the same connection pool.
func newServerManager() *mcp.ServerManager {
	return mcp.NewServerManager(serverTimeout)
}

// withServerManager returns the ServerManager from the command context, or a
// fresh one if none was set.
func withServerManager(cmd *cobra.Command) *mcp.ServerManager {
	if mgr, ok := cmd.Context().Value(serverManagerKey{}).(*mcp.ServerManager); ok && mgr != nil {
		return mgr
	}
	return newServerManager()
}

// NewCommand creates the mcp command group.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "MCP server operations",
		Long: `Interact with MCP (Model Context Protocol) servers.

Subcommands:
  list-resources      List available resources
  read-resource       Read a resource
  list-templates      List resource templates
  list-tools          List available tools
  call-tool           Call a tool
  server-info         Show server information

Configure MCP servers in ~/.config/lana/config.yaml:
  mcp:
    servers:
      - name: my-server
        uri: http://localhost:3000/mcp
        stdio: true
        command: my-mcp-server
`,
	}

	cmd.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		mgr := newServerManager()
		cmd.SetContext(context.WithValue(cmd.Context(), serverManagerKey{}, mgr))
		return nil
	}
	cmd.PersistentPostRunE = func(cmd *cobra.Command, _ []string) error {
		mgr := withServerManager(cmd)
		return mgr.Close(context.Background())
	}

	cmd.AddCommand(listResourcesCommand())
	cmd.AddCommand(readResourceCommand())
	cmd.AddCommand(listTemplatesCommand())
	cmd.AddCommand(listToolsCommand())
	cmd.AddCommand(callToolCommand())
	cmd.AddCommand(serverInfoCommand())
	return cmd
}

func listResourcesCommand() *cobra.Command {
	var server, configPath string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "list-resources [flags]",
		Short: "List MCP server resources",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := getDefaultConfig(configPath)

			var servers []config.MCPServerConfig
			if server != "" {
				s := cfg.GetMCPServer(server)
				if s == nil {
					return fmt.Errorf("MCP server not found: %s", server)
				}
				servers = append(servers, *s)
			} else {
				servers = cfg.MCP.Servers
			}

			if len(servers) == 0 {
				fmt.Println("No MCP servers configured.")
				fmt.Println("Add servers to ~/.config/lana/config.yaml:")
				fmt.Println("  mcp:")
				fmt.Println("    servers:")
				fmt.Println("      - name: my-server")
				fmt.Println("        uri: http://localhost:3000/mcp")
				return nil
			}

			mgr := withServerManager(cmd)
			ctx, cancel := context.WithTimeout(cmd.Context(), serverTimeout)
			defer cancel()

			for _, s := range servers {
				srvCfg := mcp.ServerConfig{
					Name: s.Name, URI: s.URI, Stdio: s.Stdio,
					Command: s.Command, Args: s.Args,
				}

				client, err := mgr.Connect(ctx, s.Name, srvCfg, false)
				if err != nil {
					fmt.Fprintf(os.Stderr, "  [%s] connect failed: %v\n", s.Name, redactErr(err))
					continue
				}

				resources, err := client.ListResources(ctx)
				if err != nil {
					fmt.Fprintf(os.Stderr, "  [%s] list-resources failed: %v\n", s.Name, err)
					continue
				}

				if jsonOutput {
					data, _ := json.MarshalIndent(resources, "", "  ")
					fmt.Println(string(data))
					continue
				}

				fmt.Printf("Server: %s\n", s.Name)
				if s.URI != "" {
					fmt.Printf("  URI: %s\n", config.RedactURI(s.URI))
				}
				if len(resources) == 0 {
					fmt.Println("  Resources: (none)")
				} else {
					fmt.Printf("  Resources (%d):\n", len(resources))
					for _, r := range resources {
						fmt.Printf("    - %s (%s)\n", r.Name, r.URI)
						if r.Description != "" {
							fmt.Printf("      %s\n", r.Description)
						}
					}
				}
				fmt.Println()
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&server, "server", "s", "", "MCP server name")
	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Config file path")
	cmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Output as JSON")
	return cmd
}

func readResourceCommand() *cobra.Command {
	var server, uri, configPath string

	cmd := &cobra.Command{
		Use:   "read-resource [flags]",
		Short: "Read an MCP server resource",
		RunE: func(cmd *cobra.Command, args []string) error {
			if server == "" {
				return fmt.Errorf("--server is required")
			}
			if uri == "" {
				return fmt.Errorf("--uri is required")
			}

			cfg := getDefaultConfig(configPath)
			s := cfg.GetMCPServer(server)
			if s == nil {
				return fmt.Errorf("MCP server not found: %s", server)
			}

			mgr := withServerManager(cmd)
			ctx, cancel := context.WithTimeout(cmd.Context(), serverTimeout)
			defer cancel()

			srvCfg := mcp.ServerConfig{
				Name: s.Name, URI: s.URI, Stdio: s.Stdio,
				Command: s.Command, Args: s.Args,
			}

			client, err := mgr.Connect(ctx, s.Name, srvCfg, false)
			if err != nil {
				return fmt.Errorf("connect to %s: %w", server, redactErr(err))
			}

			result, err := client.ReadResource(ctx, uri)
			if err != nil {
				return fmt.Errorf("read resource %q: %w", uri, err)
			}

			for _, c := range result.Contents {
				if c.Text != "" {
					fmt.Print(c.Text)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&server, "server", "s", "", "MCP server name (required)")
	cmd.Flags().StringVarP(&uri, "uri", "u", "", "Resource URI (required)")
	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Config file path")
	return cmd
}

func listTemplatesCommand() *cobra.Command {
	var server, configPath string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "list-templates [flags]",
		Short: "List MCP server resource templates (prompts)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := getDefaultConfig(configPath)

			var servers []config.MCPServerConfig
			if server != "" {
				s := cfg.GetMCPServer(server)
				if s == nil {
					return fmt.Errorf("MCP server not found: %s", server)
				}
				servers = append(servers, *s)
			} else {
				servers = cfg.MCP.Servers
			}

			if len(servers) == 0 {
				fmt.Println("No MCP servers configured.")
				return nil
			}

			mgr := withServerManager(cmd)
			ctx, cancel := context.WithTimeout(cmd.Context(), serverTimeout)
			defer cancel()

			for _, s := range servers {
				srvCfg := mcp.ServerConfig{
					Name: s.Name, URI: s.URI, Stdio: s.Stdio,
					Command: s.Command, Args: s.Args,
				}

				client, err := mgr.Connect(ctx, s.Name, srvCfg, false)
				if err != nil {
					fmt.Fprintf(os.Stderr, "  [%s] connect failed: %v\n", s.Name, redactErr(err))
					continue
				}

				prompts, err := client.ListPrompts(ctx)
				if err != nil {
					fmt.Fprintf(os.Stderr, "  [%s] list-prompts failed: %v\n", s.Name, err)
					continue
				}

				if jsonOutput {
					data, _ := json.MarshalIndent(prompts, "", "  ")
					fmt.Println(string(data))
					continue
				}

				fmt.Printf("Server: %s\n", s.Name)
				if len(prompts) == 0 {
					fmt.Println("  Templates: (none)")
				} else {
					fmt.Printf("  Templates (%d):\n", len(prompts))
					for _, p := range prompts {
						fmt.Printf("    - %s\n", p.Name)
						if p.Description != "" {
							fmt.Printf("      %s\n", p.Description)
						}
						if len(p.Arguments) > 0 {
							fmt.Println("      Arguments:")
							for _, a := range p.Arguments {
								req := ""
								if a.Required {
									req = " (required)"
								}
								fmt.Printf("        - %s%s\n", a.Name, req)
							}
						}
					}
				}
				fmt.Println()
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&server, "server", "s", "", "MCP server name")
	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Config file path")
	cmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Output as JSON")
	return cmd
}

func listToolsCommand() *cobra.Command {
	var server, configPath string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "list-tools [flags]",
		Short: "List MCP server tools",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := getDefaultConfig(configPath)

			var servers []config.MCPServerConfig
			if server != "" {
				s := cfg.GetMCPServer(server)
				if s == nil {
					return fmt.Errorf("MCP server not found: %s", server)
				}
				servers = append(servers, *s)
			} else {
				servers = cfg.MCP.Servers
			}

			if len(servers) == 0 {
				fmt.Println("No MCP servers configured.")
				return nil
			}

			mgr := withServerManager(cmd)
			ctx, cancel := context.WithTimeout(cmd.Context(), serverTimeout)
			defer cancel()

			for _, s := range servers {
				srvCfg := mcp.ServerConfig{
					Name: s.Name, URI: s.URI, Stdio: s.Stdio,
					Command: s.Command, Args: s.Args,
				}

				client, err := mgr.Connect(ctx, s.Name, srvCfg, false)
				if err != nil {
					fmt.Fprintf(os.Stderr, "  [%s] connect failed: %v\n", s.Name, redactErr(err))
					continue
				}

				tools, err := client.ListTools(ctx)
				if err != nil {
					fmt.Fprintf(os.Stderr, "  [%s] list-tools failed: %v\n", s.Name, err)
					continue
				}

				if jsonOutput {
					data, _ := json.MarshalIndent(tools, "", "  ")
					fmt.Println(string(data))
					continue
				}

				fmt.Printf("Server: %s\n", s.Name)
				if len(tools) == 0 {
					fmt.Println("  Tools: (none)")
				} else {
					fmt.Printf("  Tools (%d):\n", len(tools))
					for _, t := range tools {
						fmt.Printf("    - %s\n", t.Name)
						if t.Description != "" {
							fmt.Printf("      %s\n", t.Description)
						}
					}
				}
				fmt.Println()
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&server, "server", "s", "", "MCP server name")
	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Config file path")
	cmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Output as JSON")
	return cmd
}

func callToolCommand() *cobra.Command {
	var server, toolName, toolArgsStr, configPath string

	cmd := &cobra.Command{
		Use:   "call-tool [flags]",
		Short: "Call an MCP tool",
		RunE: func(cmd *cobra.Command, args []string) error {
			if server == "" {
				return fmt.Errorf("--server is required")
			}
			if toolName == "" {
				return fmt.Errorf("--tool is required")
			}

			cfg := getDefaultConfig(configPath)
			s := cfg.GetMCPServer(server)
			if s == nil {
				return fmt.Errorf("MCP server not found: %s", server)
			}

			mgr := withServerManager(cmd)
			ctx, cancel := context.WithTimeout(cmd.Context(), serverTimeout)
			defer cancel()

			srvCfg := mcp.ServerConfig{
				Name: s.Name, URI: s.URI, Stdio: s.Stdio,
				Command: s.Command, Args: s.Args,
			}

			client, err := mgr.Connect(ctx, s.Name, srvCfg, false)
			if err != nil {
				return fmt.Errorf("connect to %s: %w", server, redactErr(err))
			}

			var argsMap any
			if toolArgsStr != "" {
				if err := json.Unmarshal([]byte(toolArgsStr), &argsMap); err != nil {
					return fmt.Errorf("parse tool args: %w", err)
				}
			}

			result, err := client.CallTool(ctx, toolName, argsMap)
			if err != nil {
				return fmt.Errorf("call tool %q: %w", toolName, err)
			}

			for _, c := range result.Content {
				if c.Type == "text" && c.Text != "" {
					fmt.Println(c.Text)
				} else if c.Type == "image" && c.Image != nil {
					fmt.Printf("[image: %s %s]\n", c.Image.MIMEType, c.Image.Data)
				} else if c.Text != "" {
					fmt.Println(c.Text)
				}
			}
			if result.IsError {
				return fmt.Errorf("tool %q reported an error", toolName)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&server, "server", "s", "", "MCP server name (required)")
	cmd.Flags().StringVarP(&toolName, "tool", "t", "", "Tool name (required)")
	cmd.Flags().StringVarP(&toolArgsStr, "args", "a", "", "Tool arguments as JSON string")
	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Config file path")
	return cmd
}

func serverInfoCommand() *cobra.Command {
	var server, configPath string

	cmd := &cobra.Command{
		Use:   "server-info [flags]",
		Short: "Show server information",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := getDefaultConfig(configPath)

			var servers []config.MCPServerConfig
			if server != "" {
				s := cfg.GetMCPServer(server)
				if s == nil {
					return fmt.Errorf("MCP server not found: %s", server)
				}
				servers = append(servers, *s)
			} else {
				servers = cfg.MCP.Servers
			}

			if len(servers) == 0 {
				fmt.Println("No MCP servers configured.")
				return nil
			}

			mgr := withServerManager(cmd)
			ctx, cancel := context.WithTimeout(cmd.Context(), serverTimeout)
			defer cancel()

			for _, s := range servers {
				srvCfg := mcp.ServerConfig{
					Name: s.Name, URI: s.URI, Stdio: s.Stdio,
					Command: s.Command, Args: s.Args,
				}

				client, err := mgr.Connect(ctx, s.Name, srvCfg, false)
				if err != nil {
					fmt.Fprintf(os.Stderr, "  [%s] connect failed: %v\n", s.Name, redactErr(err))
					continue
				}

				info := client.ServerInfo()
				caps := client.Capabilities()

				fmt.Printf("Server: %s\n", s.Name)
				if s.URI != "" {
					fmt.Printf("  URI: %s\n", config.RedactURI(s.URI))
				}
				if s.Stdio {
					fmt.Println("  Transport: stdio")
				}
				fmt.Printf("  Protocol: %s\n", info.Name)
				fmt.Printf("  Version: %s\n", info.Version)
				fmt.Println()
				fmt.Println("  Capabilities:")
				if caps.Tools != nil {
					fmt.Println("    tools: yes")
				}
				if caps.Resources != nil {
					fmt.Println("    resources: yes")
				}
				if caps.Prompts != nil {
					fmt.Println("    prompts: yes")
				}
				if caps.Tools == nil && caps.Resources == nil && caps.Prompts == nil {
					fmt.Println("    (none advertised)")
				}
				fmt.Println()
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&server, "server", "s", "", "MCP server name")
	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Config file path")
	return cmd
}

func getDefaultConfig(path string) *config.Config {
	if path == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return config.DefaultConfig()
		}
		path = homeDir + "/.config/lana/config.yaml"
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return config.DefaultConfig()
	}

	cfg, err := config.Load(path)
	if err != nil {
		return config.DefaultConfig()
	}
	return cfg
}

// redactErr wraps an error message, redacting any embedded URIs so credentials
// never reach the user. It is a no-op when err is nil.
func redactErr(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	// Extract and redact any URI found in the error message.
	for _, scheme := range []string{"https://", "http://"} {
		for strings.Contains(msg, scheme) {
			idx := strings.Index(msg, scheme)
			rest := msg[idx+len(scheme):]
			// Find end of URI (space, quote, or end of string)
			endIdx := len(rest)
			for i, c := range rest {
				if c == ' ' || c == '"' || c == '`' {
					endIdx = i
					break
				}
			}
			uri := rest[:endIdx]
			redacted := config.RedactURI(uri)
			msg = msg[:idx] + scheme + redacted + msg[idx+len(scheme)+endIdx:]
		}
	}
	return fmt.Errorf("%s", msg)
}
