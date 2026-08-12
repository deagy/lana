// Package mcp provides a high-level MCP client that wraps the JSON-RPC transport.
package mcp

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ServerManager manages the lifecycle of MCP server connections. It spawns
// stdio child processes as needed, builds transports, initializes the MCP
// handshake, and caches connected clients so repeated calls reuse the same
// underlying transport.
type ServerManager struct {
	mu      sync.Mutex
	servers map[string]*connectedServer
	timeout time.Duration
	closed  bool
}

type connectedServer struct {
	client    *MCPClient
	transport Transport
}

// NewServerManager creates a ServerManager with the given default timeout.
// A zero timeout means no deadline is applied to individual requests.
func NewServerManager(timeout time.Duration) *ServerManager {
	return &ServerManager{
		servers: make(map[string]*connectedServer),
		timeout: timeout,
	}
}

// Connect establishes a connection to the named MCP server described by the
// given config. The returned MCPClient is initialized and ready to use.
// Connections are cached by server name; subsequent calls with the same name
// return the existing connection unless forceReconnect is true.
func (m *ServerManager) Connect(ctx context.Context, name string, cfg ServerConfig, forceReconnect bool) (*MCPClient, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil, fmt.Errorf("server manager closed")
	}

	if !forceReconnect {
		if cs, ok := m.servers[name]; ok {
			return cs.client, nil
		}
	}

	transport, err := NewTransport(cfg)
	if err != nil {
		return nil, fmt.Errorf("build transport for %q: %w", name, err)
	}

	client := NewMCPClient(transport)
	if _, err := client.Connect(ctx); err != nil {
		_ = transport.Close()
		return nil, fmt.Errorf("initialize %q: %w", name, err)
	}

	m.servers[name] = &connectedServer{
		client:    client,
		transport: transport,
	}
	return client, nil
}

// Get returns a previously connected client for the named server, or nil if
// no connection exists.
func (m *ServerManager) Get(name string) (*MCPClient, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cs, ok := m.servers[name]
	if !ok {
		return nil, false
	}
	return cs.client, true
}

// Close releases all managed connections.
func (m *ServerManager) Close(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	var firstErr error
	for _, cs := range m.servers {
		if err := cs.client.Disconnect(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	m.servers = make(map[string]*connectedServer)
	return firstErr
}
