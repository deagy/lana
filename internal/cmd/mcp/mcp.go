// Package mcp provides MCP server interaction subcommands.
package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/deagy/lana/pkg/config"
)

// MCP server communication types
type mcpMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int            `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *mcpError       `json:"error,omitempty"`
}

type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type mcpResourceListResult struct {
	Resources []mcpResource `json:"resources"`
}

type mcpResource struct {
	Name        string `json:"name"`
	URI         string `json:"uri"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

type mcpResourceReadResult struct {
	Contents []mcpResourceContent `json:"contents"`
}

type mcpResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
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

			if jsonOutput {
				data, _ := json.MarshalIndent(config.RedactMCPServers(servers), "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Printf("MCP Servers (%d):\n\n", len(servers))
			for _, s := range config.RedactMCPServers(servers) {
				fmt.Printf("  Server: %s\n", s.Name)
				if s.URI != "" {
					fmt.Printf("    URI:    %s\n", s.URI)
				}
				if s.Stdio {
					fmt.Printf("    Transport: stdio\n")
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&server, "server", "s", "", "MCP server name")
	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Config file path")
	cmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Output in JSON format")
	return cmd
}

func readResourceCommand() *cobra.Command {
	var server, uri, configPath string

	cmd := &cobra.Command{
		Use:   "read-resource [flags]",
		Short: "Read an MCP resource",
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

			fmt.Printf("Resource: %s\n", config.RedactURI(uri))
			fmt.Printf("Server:   %s\n", server)
			fmt.Println("  (MCP client integration pending)")
			fmt.Println()
			fmt.Println("To configure MCP servers, add to ~/.config/lana/config.yaml:")
			fmt.Println("  mcp:")
			fmt.Println("    servers:")
			fmt.Println("      - name: my-server")
			fmt.Println("        uri: http://localhost:3000/mcp")
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
		Short: "List MCP resource templates",
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

			if jsonOutput {
				data, _ := json.MarshalIndent(config.RedactMCPServers(servers), "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Printf("MCP Servers (%d):\n\n", len(servers))
			for _, s := range config.RedactMCPServers(servers) {
				fmt.Printf("  Server: %s\n", s.Name)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&server, "server", "s", "", "MCP server name")
	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Config file path")
	cmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Output in JSON format")
	return cmd
}

func listToolsCommand() *cobra.Command {
	var server, configPath string

	cmd := &cobra.Command{
		Use:   "list-tools [flags]",
		Short: "List available MCP tools",
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

			fmt.Printf("MCP Servers (%d):\n\n", len(servers))
			for _, s := range config.RedactMCPServers(servers) {
				fmt.Printf("  Server: %s\n", s.Name)
				if s.URI != "" {
					fmt.Printf("    URI: %s\n", s.URI)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&server, "server", "s", "", "MCP server name")
	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Config file path")
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

			fmt.Printf("Calling tool %q on server %s...\n", toolName, server)
			if toolArgsStr != "" {
				fmt.Println("  Args: supplied (values withheld)")
			}
			fmt.Println("  (MCP tool calling integration pending)")
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

			fmt.Printf("MCP Servers (%d):\n\n", len(servers))
			for _, s := range config.RedactMCPServers(servers) {
				fmt.Printf("  Server: %s\n", s.Name)
				if s.URI != "" {
					fmt.Printf("    URI: %s\n", s.URI)
				}
				if s.Stdio {
					fmt.Printf("    Transport: stdio\n")
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

// mcpMessage, mcpError, mcpResourceListResult, mcpResource, mcpResourceReadResult, mcpResourceContent
// are used for MCP protocol support.
var _ = time.Now
var _ = strings.ToLower
