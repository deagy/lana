package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/deagy/lana/internal/config"
	"github.com/deagy/lana/internal/mcp"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Manage MCP (Model Context Protocol) server integrations",
	Long: `Configure and manage external MCP servers that provide tools.

MCP servers extend Lana's capabilities by providing additional tools.
You can connect to local servers (via stdio) or remote servers (via HTTP/SSE).`,
}

var mcpListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured MCP servers",
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg == nil {
			fmt.Fprintf(os.Stderr, "Error: configuration not loaded\n")
			os.Exit(1)
		}

		if len(cfg.MCP.Servers) == 0 {
			fmt.Println("No MCP servers configured.")
			fmt.Println("Use 'lana mcp add' to add a server.")
			return nil
		}

		fmt.Println("Configured MCP Servers:")
		fmt.Println()

		for i, srv := range cfg.MCP.Servers {
			status := "enabled"
			if srv.Disabled {
				status = "disabled"
			}

			location := srv.URL
			if srv.Transport == "stdio" || srv.Transport == "" {
				location = fmt.Sprintf("%s %v", srv.Command, srv.Args)
			}

			fmt.Printf("%d. %s (%s) [%s]\n", i+1, srv.Name, srv.Transport, status)
			fmt.Printf("   Location: %s\n", location)
			fmt.Printf("   Risk Level: %s\n", srv.RiskLevel)
			fmt.Println()
		}

		return nil
	},
}

var mcpAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a new MCP server",
	Long: `Add a new MCP server configuration.

Examples:
  # Add a local stdio-based server
  lana mcp add myserver --command npx --arg @anthropic/resources

  # Add an HTTP-based server
  lana mcp add remoteserver --url http://localhost:3000 --transport http`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg == nil {
			fmt.Fprintf(os.Stderr, "Error: configuration not loaded\n")
			os.Exit(1)
		}

		name := args[0]

		// Get flags
		transport, _ := cmd.Flags().GetString("transport")
		command, _ := cmd.Flags().GetString("command")
		commandArgs, _ := cmd.Flags().GetStringSlice("arg")
		envVars, _ := cmd.Flags().GetStringToString("env")
		url, _ := cmd.Flags().GetString("url")
		headers, _ := cmd.Flags().GetStringToString("header")
		riskLevel, _ := cmd.Flags().GetString("risk")
		disabled, _ := cmd.Flags().GetBool("disabled")
		startTimeout, _ := cmd.Flags().GetInt("start-timeout")
		callTimeout, _ := cmd.Flags().GetInt("call-timeout")

		// Default transport to stdio
		if transport == "" {
			transport = "stdio"
		}

		// Validate transport
		if transport != "stdio" && transport != "http" {
			return fmt.Errorf("invalid transport: %s (must be 'stdio' or 'http')", transport)
		}

		// Validate based on transport
		if transport == "stdio" {
			if command == "" {
				return fmt.Errorf("stdio transport requires --command")
			}
		} else if transport == "http" {
			if url == "" {
				return fmt.Errorf("http transport requires --url")
			}
		}

		// Default risk level
		if riskLevel == "" {
			riskLevel = "medium"
		}

		// Create server config
		serverCfg := config.MCPServerConfig{
			Name:                name,
			Transport:           transport,
			Command:             command,
			Args:                commandArgs,
			Env:                 envVars,
			URL:                 url,
			Headers:             headers,
			Disabled:            disabled,
			RiskLevel:           riskLevel,
			StartTimeoutSeconds: startTimeout,
			CallTimeoutSeconds:  callTimeout,
		}

		// Check for duplicate name
		for _, existing := range cfg.MCP.Servers {
			if existing.Name == name {
				return fmt.Errorf("server '%s' already exists", name)
			}
		}

		// Add to config
		cfg.MCP.Servers = append(cfg.MCP.Servers, serverCfg)

		// Write config
		loader := config.NewLoader()
		_, err := loader.Load(globalConfigPath)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		loader.Set("mcp.servers", cfg.MCP.Servers)
		if err := loader.WriteGlobal(globalConfigPath); err != nil {
			return fmt.Errorf("write config: %w", err)
		}

		fmt.Printf("Added MCP server '%s'\n", name)
		return nil
	},
}

var mcpRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove an MCP server",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg == nil {
			fmt.Fprintf(os.Stderr, "Error: configuration not loaded\n")
			os.Exit(1)
		}

		name := args[0]

		// Find and remove the server
		found := false
		newServers := []config.MCPServerConfig{}
		for _, srv := range cfg.MCP.Servers {
			if srv.Name == name {
				found = true
			} else {
				newServers = append(newServers, srv)
			}
		}

		if !found {
			return fmt.Errorf("server '%s' not found", name)
		}

		cfg.MCP.Servers = newServers

		// Write config
		loader := config.NewLoader()
		_, err := loader.Load(globalConfigPath)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		loader.Set("mcp.servers", cfg.MCP.Servers)
		if err := loader.WriteGlobal(globalConfigPath); err != nil {
			return fmt.Errorf("write config: %w", err)
		}

		fmt.Printf("Removed MCP server '%s'\n", name)
		return nil
	},
}

var mcpToolsCmd = &cobra.Command{
	Use:   "tools [server]",
	Short: "List tools from MCP servers",
	Long: `Discover and list tools available from MCP servers.

If a server name is provided, only tools from that server are listed.
Otherwise, all configured servers are queried.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg == nil {
			fmt.Fprintf(os.Stderr, "Error: configuration not loaded\n")
			os.Exit(1)
		}

		// Determine which servers to query
		var serversToQuery []config.MCPServerConfig
		if len(args) > 0 {
			// Query specific server
			name := args[0]
			found := false
			for _, srv := range cfg.MCP.Servers {
				if srv.Name == name {
					serversToQuery = append(serversToQuery, srv)
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("server '%s' not found", name)
			}
		} else {
			// Query all servers
			serversToQuery = cfg.MCP.Servers
		}

		if len(serversToQuery) == 0 {
			fmt.Println("No MCP servers configured.")
			return nil
		}

		// Convert to mcp.ServerConfig
		mcpConfigs := make([]mcp.ServerConfig, len(serversToQuery))
		for i, cfg := range serversToQuery {
			mcpConfigs[i] = mcp.ServerConfig{
				Name:                cfg.Name,
				Transport:           cfg.Transport,
				Command:             cfg.Command,
				Args:                cfg.Args,
				Env:                 cfg.Env,
				URL:                 cfg.URL,
				Headers:             cfg.Headers,
				Disabled:            cfg.Disabled,
				RiskLevel:           cfg.RiskLevel,
				StartTimeoutSeconds: cfg.StartTimeoutSeconds,
				CallTimeoutSeconds:  cfg.CallTimeoutSeconds,
			}
		}

		// Create manager and start
		mgr := mcp.NewManager(mcpConfigs)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		errs := mgr.Start(ctx)
		for _, err := range errs {
			fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
		}
		defer mgr.Close()

		// List tools
		tools := mgr.Tools()
		if len(tools) == 0 {
			fmt.Println("No tools discovered.")
			return nil
		}

		fmt.Println("Discovered MCP Tools:")
		fmt.Println()

		// Group by server
		toolsByServer := make(map[string][]mcp.NamedTool)
		for _, tool := range tools {
			toolsByServer[tool.ServerName] = append(toolsByServer[tool.ServerName], tool)
		}

		for serverName, serverTools := range toolsByServer {
			fmt.Printf("Server: %s\n", serverName)
			for i, tool := range serverTools {
				fmt.Printf("  %d. %s\n", i+1, tool.ToolName)
				fmt.Printf("     %s\n", tool.Spec.Description)
			}
			fmt.Println()
		}

		return nil
	},
}

func init() {
	// Add subcommands to mcp command
	mcpCmd.AddCommand(mcpListCmd)
	mcpCmd.AddCommand(mcpAddCmd)
	mcpCmd.AddCommand(mcpRemoveCmd)
	mcpCmd.AddCommand(mcpToolsCmd)

	// mcpAddCmd flags
	mcpAddCmd.Flags().String("transport", "stdio", "Transport type: 'stdio' (default) or 'http'")
	mcpAddCmd.Flags().String("command", "", "Command to run (stdio transport)")
	mcpAddCmd.Flags().StringSlice("arg", []string{}, "Arguments to command (can be used multiple times)")
	mcpAddCmd.Flags().StringToString("env", make(map[string]string), "Environment variables (e.g., KEY=VALUE)")
	mcpAddCmd.Flags().String("url", "", "Server URL (http transport)")
	mcpAddCmd.Flags().StringToString("header", make(map[string]string), "HTTP headers (e.g., Authorization:Bearer)")
	mcpAddCmd.Flags().String("risk", "medium", "Risk level: 'low', 'medium' (default), or 'high'")
	mcpAddCmd.Flags().Bool("disabled", false, "Disable this server")
	mcpAddCmd.Flags().Int("start-timeout", 10, "Timeout for server startup (seconds)")
	mcpAddCmd.Flags().Int("call-timeout", 60, "Timeout for tool calls (seconds)")

	// Add mcp command to root
	rootCmd.AddCommand(mcpCmd)
}
