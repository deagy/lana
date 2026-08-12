// Package mcp provides a high-level MCP client that wraps the JSON-RPC transport.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
)

const defaultProtocolVersion = "2024-11-05"

// Client is a high-level MCP client.
type MCPClient struct {
	client       *Client
	initialized  bool
	serverInfo   ServerInfo
	capabilities ServerCapabilities
}

// NewMCPClient creates a new MCP client with the given transport.
func NewMCPClient(transport Transport) *MCPClient {
	return &MCPClient{
		client: NewClient(transport),
	}
}

// NewMCPClientWithInfo creates a new MCP client with client identification.
func NewMCPClientWithInfo(transport Transport, name, version string) *MCPClient {
	return &MCPClient{
		client:     NewClient(transport),
		serverInfo: ServerInfo{Name: name, Version: version},
	}
}

// Connect initializes the MCP connection.
func (c *MCPClient) Connect(ctx context.Context) (*InitializeResult, error) {
	info := ClientInfo{
		Name:    c.serverInfo.Name,
		Version: c.serverInfo.Version,
	}
	if info.Name == "" {
		info.Name = "lana"
	}
	result, err := c.client.Initialize(ctx, defaultProtocolVersion, info)
	if err != nil {
		return nil, fmt.Errorf("MCP initialize: %w", err)
	}
	c.initialized = true
	c.serverInfo = result.ServerInfo
	c.capabilities = result.Capabilities
	return result, nil
}

// IsConnected reports whether the client has been initialized.
func (c *MCPClient) IsConnected() bool { return c.initialized }

// ServerInfo returns information about the connected server.
func (c *MCPClient) ServerInfo() ServerInfo { return c.serverInfo }

// Capabilities returns the server capabilities.
func (c *MCPClient) Capabilities() ServerCapabilities { return c.capabilities }

// ListTools lists available tools from the MCP server.
func (c *MCPClient) ListTools(ctx context.Context) ([]ToolDefinition, error) {
	if !c.initialized {
		return nil, fmt.Errorf("MCP client not connected")
	}
	resp, err := c.client.Call(ctx, "tools/list", nil)
	if err != nil {
		return nil, fmt.Errorf("tools/list: %w", err)
	}
	var result ListToolsResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("decode tools list: %w", err)
	}
	return result.Tools, nil
}

// CallTool calls a tool on the MCP server.
func (c *MCPClient) CallTool(ctx context.Context, name string, args any) (*CallToolResult, error) {
	if !c.initialized {
		return nil, fmt.Errorf("MCP client not connected")
	}
	params := map[string]any{"name": name}
	if args != nil {
		params["arguments"] = args
	}
	resp, err := c.client.Call(ctx, "tools/call", params)
	if err != nil {
		return nil, fmt.Errorf("tools/call: %w", err)
	}
	var result CallToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("decode tool call result: %w", err)
	}
	return &result, nil
}

// ListResources lists available resources from the MCP server.
func (c *MCPClient) ListResources(ctx context.Context) ([]ResourceDefinition, error) {
	if !c.initialized {
		return nil, fmt.Errorf("MCP client not connected")
	}
	resp, err := c.client.Call(ctx, "resources/list", nil)
	if err != nil {
		return nil, fmt.Errorf("resources/list: %w", err)
	}
	var result ListResourcesResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("decode resources list: %w", err)
	}
	return result.Resources, nil
}

// ReadResource reads a resource from the MCP server.
func (c *MCPClient) ReadResource(ctx context.Context, uri string) (*ReadResourceResult, error) {
	if !c.initialized {
		return nil, fmt.Errorf("MCP client not connected")
	}
	params := map[string]any{"uri": uri}
	resp, err := c.client.Call(ctx, "resources/read", params)
	if err != nil {
		return nil, fmt.Errorf("resources/read: %w", err)
	}
	var result ReadResourceResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("decode resource read: %w", err)
	}
	return &result, nil
}

// ListPrompts lists available prompts from the MCP server.
func (c *MCPClient) ListPrompts(ctx context.Context) ([]PromptDefinition, error) {
	if !c.initialized {
		return nil, fmt.Errorf("MCP client not connected")
	}
	resp, err := c.client.Call(ctx, "prompts/list", nil)
	if err != nil {
		return nil, fmt.Errorf("prompts/list: %w", err)
	}
	var result ListPromptsResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("decode prompts list: %w", err)
	}
	return result.Prompts, nil
}

// Disconnect closes the MCP connection.
func (c *MCPClient) Disconnect(ctx context.Context) error {
	c.initialized = false
	return c.client.Close(ctx)
}
