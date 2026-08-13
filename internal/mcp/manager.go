package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"
)

// NamedTool represents a tool with its server name for namespacing.
type NamedTool struct {
	ServerName string
	ToolName   string
	Spec       ToolSpec
}

// ServerConfig represents configuration for an MCP server.
type ServerConfig struct {
	Name                 string
	Transport            string // "stdio" or "http"
	Command              string
	Args                 []string
	Env                  map[string]string
	URL                  string
	Headers              map[string]string
	Disabled             bool
	RiskLevel            string
	StartTimeoutSeconds  int
	CallTimeoutSeconds   int
}

// Manager manages multiple MCP server connections.
type Manager struct {
	configs map[string]ServerConfig
	clients map[string]*Client
	tools   map[string][]NamedTool // keyed by server name
	toolsMu sync.RWMutex
}

// NewManager creates a new MCP manager with the given server configs.
func NewManager(configs []ServerConfig) *Manager {
	configMap := make(map[string]ServerConfig)
	for _, cfg := range configs {
		if cfg.StartTimeoutSeconds == 0 {
			cfg.StartTimeoutSeconds = 10
		}
		if cfg.CallTimeoutSeconds == 0 {
			cfg.CallTimeoutSeconds = 60
		}
		configMap[cfg.Name] = cfg
	}

	return &Manager{
		configs: configMap,
		clients: make(map[string]*Client),
		tools:   make(map[string][]NamedTool),
	}
}

// Start connects to all configured servers and discovers their tools.
// Returns a slice of errors (one per failed server), but does not abort on failures.
func (m *Manager) Start(ctx context.Context) []error {
	var errors []error
	var wg sync.WaitGroup
	var mu sync.Mutex

	for name, cfg := range m.configs {
		if cfg.Disabled {
			continue
		}

		wg.Add(1)
		go func(serverName string, config ServerConfig) {
			defer wg.Done()

			// Create a context with a timeout for this server's startup
			startCtx, cancel := context.WithTimeout(ctx, time.Duration(config.StartTimeoutSeconds)*time.Second)
			defer cancel()

			if err := m.startServer(startCtx, serverName, config); err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("start %s: %w", serverName, err))
				mu.Unlock()
			}
		}(name, cfg)
	}

	wg.Wait()
	return errors
}

// startServer connects to and initializes a single server.
func (m *Manager) startServer(ctx context.Context, name string, cfg ServerConfig) error {
	// Create transport based on config
	var transport interface {
		io.ReadWriteCloser
	}
	var err error

	switch cfg.Transport {
	case "http", "":
		if cfg.URL == "" {
			return fmt.Errorf("HTTP transport requires URL")
		}
		transport, err = NewHTTPTransport(cfg.URL, cfg.Headers)
	case "stdio":
		if cfg.Command == "" {
			return fmt.Errorf("stdio transport requires command")
		}
		transport, err = NewStdioTransport(cfg.Command, cfg.Args, cfg.Env)
	default:
		return fmt.Errorf("unknown transport: %s", cfg.Transport)
	}

	if err != nil {
		return fmt.Errorf("create transport: %w", err)
	}

	// Create and initialize client
	client := NewClient(transport)
	if err := client.Initialize(ctx, "lana", "0.2.0"); err != nil {
		client.Close()
		return fmt.Errorf("initialize: %w", err)
	}

	// List tools
	tools, err := client.ListTools(ctx)
	if err != nil {
		client.Close()
		return fmt.Errorf("list tools: %w", err)
	}

	// Store client and tools
	m.clients[name] = client

	namedTools := make([]NamedTool, len(tools))
	for i, spec := range tools {
		namedTools[i] = NamedTool{
			ServerName: name,
			ToolName:   spec.Name,
			Spec:       spec,
		}
	}

	m.toolsMu.Lock()
	m.tools[name] = namedTools
	m.toolsMu.Unlock()

	return nil
}

// Tools returns all discovered tools across all servers.
func (m *Manager) Tools() []NamedTool {
	m.toolsMu.RLock()
	defer m.toolsMu.RUnlock()

	var result []NamedTool
	for _, serverTools := range m.tools {
		result = append(result, serverTools...)
	}
	return result
}

// CallTool invokes a tool on a specific server.
func (m *Manager) CallTool(ctx context.Context, serverName, toolName string, input json.RawMessage) (string, error) {
	client, ok := m.clients[serverName]
	if !ok {
		return "", fmt.Errorf("server not found or not connected: %s", serverName)
	}

	cfg, ok := m.configs[serverName]
	if !ok {
		return "", fmt.Errorf("server config not found: %s", serverName)
	}

	// Create a context with a timeout for the tool call
	callCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.CallTimeoutSeconds)*time.Second)
	defer cancel()

	output, isError, err := client.CallTool(callCtx, toolName, input)
	if err != nil {
		return "", fmt.Errorf("call tool: %w", err)
	}

	if isError {
		return output, fmt.Errorf("tool error: %s", output)
	}

	return output, nil
}

// Close closes all server connections.
func (m *Manager) Close() error {
	var lastErr error
	for name, client := range m.clients {
		if err := client.Close(); err != nil {
			lastErr = fmt.Errorf("close %s: %w", name, err)
		}
	}
	return lastErr
}
