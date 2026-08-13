package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
)

// Client wraps a transport and handles JSON-RPC communication.
type Client struct {
	transport     io.ReadWriteCloser
	requestID     atomic.Int64
	responses     map[int64]chan *Response
	responsesMu   sync.RWMutex
	readLoopDone  chan struct{}
	closeOnce     sync.Once
}

// NewClient creates a new MCP client over the given transport.
func NewClient(transport io.ReadWriteCloser) *Client {
	c := &Client{
		transport:    transport,
		responses:    make(map[int64]chan *Response),
		readLoopDone: make(chan struct{}),
	}

	// Start background read loop
	go c.readLoop()

	return c
}

// readLoop reads JSON-RPC messages from the transport and dispatches responses.
func (c *Client) readLoop() {
	defer close(c.readLoopDone)

	scanner := bufio.NewScanner(c.transport)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Try to parse as Response (with ID field)
		var resp Response
		if err := json.Unmarshal(line, &resp); err != nil {
			// Not a valid JSON-RPC message, skip
			continue
		}

		// If it has an ID, dispatch to the waiting goroutine
		if resp.ID != 0 {
			c.responsesMu.RLock()
			ch, ok := c.responses[int64(resp.ID)]
			c.responsesMu.RUnlock()

			if ok {
				select {
				case ch <- &resp:
				default:
					// Channel full or closed, skip
				}
			}
		}
		// Notifications (no ID) are currently ignored in this simple implementation
	}

	if err := scanner.Err(); err != nil {
		// Read error, but we're shutting down anyway
	}
}

// nextRequestID generates a unique request ID.
func (c *Client) nextRequestID() int64 {
	return c.requestID.Add(1)
}

// sendRequest sends a JSON-RPC request and waits for the response.
func (c *Client) sendRequest(ctx context.Context, method string, params interface{}) (*Response, error) {
	id := c.nextRequestID()

	// Marshal params
	var paramBytes json.RawMessage
	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("marshal params: %w", err)
		}
		paramBytes = json.RawMessage(data)
	}

	// Create request
	req := Request{
		JSONRPC: "2.0",
		ID:      int(id),
		Method:  method,
		Params:  paramBytes,
	}

	// Create response channel
	respCh := make(chan *Response, 1)
	c.responsesMu.Lock()
	c.responses[id] = respCh
	c.responsesMu.Unlock()

	defer func() {
		c.responsesMu.Lock()
		delete(c.responses, id)
		c.responsesMu.Unlock()
		close(respCh)
	}()

	// Send request
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	reqBytes = append(reqBytes, '\n')
	if _, err := c.transport.Write(reqBytes); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}

	// Wait for response
	select {
	case resp := <-respCh:
		if resp == nil {
			return nil, fmt.Errorf("response channel closed")
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("rpc error: %s (code %d)", resp.Error.Message, resp.Error.Code)
		}
		return resp, nil

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Initialize performs the MCP initialization handshake.
func (c *Client) Initialize(ctx context.Context, name, version string) error {
	params := InitializeParams{
		ProtocolVersion: "2024-11-05",
		Capabilities: Capabilities{
			Experimental: make(map[string]interface{}),
		},
		ClientInfo: ClientInfo{
			Name:    name,
			Version: version,
		},
	}

	resp, err := c.sendRequest(ctx, "initialize", params)
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}

	// Parse and validate the response
	var initResp InitializeResult
	if err := json.Unmarshal(resp.Result, &initResp); err != nil {
		return fmt.Errorf("parse initialize response: %w", err)
	}

	// The spec also expects a notifications/initialized message, but we'll accept
	// it if we get it; this is a simplified implementation
	return nil
}

// ListTools requests the list of available tools from the server.
func (c *Client) ListTools(ctx context.Context) ([]ToolSpec, error) {
	resp, err := c.sendRequest(ctx, "tools/list", nil)
	if err != nil {
		return nil, fmt.Errorf("list tools: %w", err)
	}

	var result ListToolsResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("parse tools list response: %w", err)
	}

	return result.Tools, nil
}

// CallTool invokes a tool with the given arguments.
func (c *Client) CallTool(ctx context.Context, toolName string, arguments json.RawMessage) (string, bool, error) {
	params := CallToolParams{
		Name:      toolName,
		Arguments: arguments,
	}

	resp, err := c.sendRequest(ctx, "tools/call", params)
	if err != nil {
		return "", false, fmt.Errorf("call tool: %w", err)
	}

	var result CallToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", false, fmt.Errorf("parse tool call response: %w", err)
	}

	// Concatenate all content blocks into a single result string
	var output string
	for _, block := range result.Content {
		if block.Text != "" {
			output += block.Text
		}
	}

	return output, result.IsError, nil
}

// Close closes the client and underlying transport.
func (c *Client) Close() error {
	var err error
	c.closeOnce.Do(func() {
		err = c.transport.Close()
		// Wait for read loop to finish
		<-c.readLoopDone
	})
	return err
}
