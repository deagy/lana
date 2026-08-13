package mcp

import (
	"encoding/json"
)

// JSON-RPC 2.0 protocol structures

// Request is a JSON-RPC 2.0 request.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response is a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *ResponseError  `json:"error,omitempty"`
}

// ResponseError is a JSON-RPC error object.
type ResponseError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// Notification is a JSON-RPC 2.0 notification (request without ID).
type Notification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// MCP-specific message types

// InitializeParams are parameters for the initialize method.
type InitializeParams struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    Capabilities `json:"capabilities"`
	ClientInfo      ClientInfo   `json:"clientInfo"`
}

// Capabilities describes client capabilities.
type Capabilities struct {
	Experimental map[string]interface{} `json:"experimental,omitempty"`
}

// ClientInfo describes the client.
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeResult is the result of an initialize call.
type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ServerInfo         `json:"serverInfo"`
}

// ServerCapabilities describes server capabilities.
type ServerCapabilities struct {
	Logging      Logging                `json:"logging,omitempty"`
	Tools        ToolCapability         `json:"tools,omitempty"`
	Experimental map[string]interface{} `json:"experimental,omitempty"`
}

// Logging describes logging capability.
type Logging struct {
	Level string `json:"level,omitempty"`
}

// ToolCapability indicates tool support.
type ToolCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ServerInfo describes the server.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ToolSpec describes a tool that the server provides.
type ToolSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"` // JSON Schema
}

// ListToolsResult is the result of listing tools.
type ListToolsResult struct {
	Tools []ToolSpec `json:"tools"`
}

// ListToolsParams are parameters for listing tools.
type ListToolsParams struct {
	Cursor string `json:"cursor,omitempty"`
}

// CallToolParams are parameters for calling a tool.
type CallToolParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// CallToolResult is the result of calling a tool.
type CallToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock represents a content block in a tool result.
type ContentBlock struct {
	Type     string `json:"type"` // "text", "image", etc.
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
}

// InitializedNotification is sent after successful initialization.
type InitializedNotification struct{}

// Helper function to unmarshal a ToolSpec from raw JSON
func UnmarshalToolSpec(data json.RawMessage) (*ToolSpec, error) {
	var spec ToolSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, err
	}
	return &spec, nil
}
