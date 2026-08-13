package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/deagy/lana/internal/tools"
)

// Server represents an MCP server that exposes Lana's tools.
type Server struct {
	registry *tools.Registry
	idMutex  sync.Mutex
	nextID   int

	// Callbacks for sending responses/notifications
	sendResponse     func(*Response) error
	sendNotification func(*Notification) error
}

// NewServer creates a new MCP server.
func NewServer(registry *tools.Registry) *Server {
	return &Server{
		registry: registry,
		nextID:   1,
	}
}

// SetCallbacks sets the send functions for responses and notifications.
func (s *Server) SetCallbacks(sendResp func(*Response) error, sendNotif func(*Notification) error) {
	s.sendResponse = sendResp
	s.sendNotification = sendNotif
}

// HandleRequest processes an incoming JSON-RPC request.
func (s *Server) HandleRequest(ctx context.Context, req *Request) (*Response, error) {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(ctx, req)
	case "tools/list":
		return s.handleListTools(ctx, req)
	case "tools/call":
		return s.handleCallTool(ctx, req)
	default:
		return s.errorResponse(req.ID, -32601, "Method not found"), nil
	}
}

// handleInitialize processes the initialize method.
func (s *Server) handleInitialize(ctx context.Context, req *Request) (*Response, error) {
	var params InitializeParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return s.errorResponse(req.ID, -32700, "Parse error"), nil
	}

	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: ServerCapabilities{
			Tools: ToolCapability{
				ListChanged: false,
			},
		},
		ServerInfo: ServerInfo{
			Name:    "Lana",
			Version: "0.2.0",
		},
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return s.errorResponse(req.ID, -32603, "Internal error"), nil
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  resultJSON,
	}, nil
}

// handleListTools processes the tools/list method.
func (s *Server) handleListTools(ctx context.Context, req *Request) (*Response, error) {
	toolDefs := s.registry.List()
	var toolSpecs []ToolSpec

	for _, td := range toolDefs {
		spec := ToolSpec{
			Name:        td.Name(),
			Description: td.Description(),
			InputSchema: td.InputSchema(),
		}
		toolSpecs = append(toolSpecs, spec)
	}

	result := ListToolsResult{
		Tools: toolSpecs,
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return s.errorResponse(req.ID, -32603, "Internal error"), nil
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  resultJSON,
	}, nil
}

// handleCallTool processes the tools/call method.
func (s *Server) handleCallTool(ctx context.Context, req *Request) (*Response, error) {
	var params CallToolParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return s.errorResponse(req.ID, -32700, "Parse error"), nil
	}

	// Find the tool
	toolDef, err := s.registry.Get(params.Name)
	if err != nil {
		return s.errorResponse(req.ID, -32602, fmt.Sprintf("Tool not found: %s", params.Name)), nil
	}

	// Execute the tool
	result, err := toolDef.Execute(ctx, params.Arguments)
	if err != nil {
		content := []ContentBlock{
			{
				Type: "text",
				Text: fmt.Sprintf("Error: %v", err),
			},
		}
		callResult := CallToolResult{
			Content: content,
			IsError: true,
		}
		resultJSON, _ := json.Marshal(callResult)
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  resultJSON,
		}, nil
	}

	// Format result
	content := []ContentBlock{
		{
			Type: "text",
			Text: result,
		},
	}
	callResult := CallToolResult{
		Content: content,
		IsError: false,
	}

	resultJSON, err := json.Marshal(callResult)
	if err != nil {
		return s.errorResponse(req.ID, -32603, "Internal error"), nil
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  resultJSON,
	}, nil
}

// errorResponse creates an error response.
func (s *Server) errorResponse(id int, code int, message string) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &ResponseError{
			Code:    code,
			Message: message,
		},
	}
}

// ReadLoop reads JSON-RPC requests from an io.Reader.
func (s *Server) ReadLoop(ctx context.Context, reader io.Reader) error {
	decoder := json.NewDecoder(reader)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var req Request
		if err := decoder.Decode(&req); err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("decode error: %w", err)
		}

		// Handle request (spawn goroutine to avoid blocking)
		go func(r Request) {
			resp, err := s.HandleRequest(ctx, &r)
			if err == nil && resp != nil && s.sendResponse != nil {
				_ = s.sendResponse(resp)
			}
		}(req)
	}
}
