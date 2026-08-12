// Package mcp provides the Model Context Protocol (MCP) JSON-RPC transport
// layer. It supports both stdio (child-process) and HTTP transports, and
// handles request/response correlation via monotonically increasing IDs.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Transport is the interface implemented by both stdio and HTTP transports.
type Transport interface {
	Send(ctx context.Context, msg *Message) (*Message, error)
	Close() error
	IsClosed() bool
}

// Message is a JSON-RPC 2.0 message.
type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError is a JSON-RPC 2.0 error.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("MCP error %d: %s", e.Code, e.Message)
}

const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

// StdioTransport communicates with an MCP server over its stdin/stdout.
type StdioTransport struct {
	stdin  io.WriteCloser
	stdout *bufio.Reader
	closed atomic.Bool
	done   chan struct{}
}

// NewStdioTransport creates a transport around the given stdin/stdout streams.
func NewStdioTransport(stdin io.WriteCloser, stdout io.Reader) *StdioTransport {
	return &StdioTransport{
		stdin:  stdin,
		stdout: bufio.NewReader(stdout),
		done:   make(chan struct{}),
	}
}

func (t *StdioTransport) Send(ctx context.Context, msg *Message) (*Message, error) {
	if t.closed.Load() {
		return nil, errors.New("transport closed")
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal message: %w", err)
	}
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	if _, err := io.WriteString(t.stdin, header+string(data)); err != nil {
		return nil, fmt.Errorf("write message: %w", err)
	}
	type respResult struct {
		msg *Message
		err error
	}
	ch := make(chan respResult, 1)
	go func() {
		m, err := t.readMessage()
		ch <- respResult{msg: m, err: err}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-ch:
		return r.msg, r.err
	}
}

func (t *StdioTransport) readMessage() (*Message, error) {
	line, err := t.stdout.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("read header line: %w", err)
	}
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(strings.ToLower(line), "content-length:") {
		return nil, fmt.Errorf("missing Content-Length header: %q", line)
	}
	var length int
	if _, err := fmt.Sscanf(line, "Content-Length: %d", &length); err != nil {
		return nil, fmt.Errorf("parse Content-Length: %w", err)
	}
	if _, err := t.stdout.ReadString('\n'); err != nil {
		return nil, fmt.Errorf("read blank line: %w", err)
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(t.stdout, body); err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	var msg Message
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, fmt.Errorf("unmarshal message: %w", err)
	}
	return &msg, nil
}

func (t *StdioTransport) Close() error {
	if t.closed.CompareAndSwap(false, true) {
		close(t.done)
		if t.stdin != nil {
			return t.stdin.Close()
		}
	}
	return nil
}

func (t *StdioTransport) IsClosed() bool { return t.closed.Load() }

// ChildStdioTransport wraps StdioTransport around a spawned child process.
type ChildStdioTransport struct {
	*StdioTransport
	cmd *exec.Cmd
}

// Close closes the transport and waits for the child process to exit.
func (t *ChildStdioTransport) Close() error {
	if err := t.StdioTransport.Close(); err != nil {
		return err
	}
	if t.cmd != nil && t.cmd.Process != nil {
		_ = t.cmd.Wait()
	}
	return nil
}

// NewChildStdioTransport spawns the given command with the given args and
// returns a Transport backed by the child process's stdin/stdout.
func NewChildStdioTransport(command string, args []string) (*ChildStdioTransport, error) {
	if command == "" {
		return nil, fmt.Errorf("stdio transport: command must not be empty")
	}
	cmd := exec.Command(command, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", command, err)
	}
	return &ChildStdioTransport{
		StdioTransport: NewStdioTransport(stdin, stdout),
		cmd:            cmd,
	}, nil
}

// HTTPTransport communicates with an MCP server over HTTP.
type HTTPTransport struct {
	client  *http.Client
	baseURL *url.URL
	closed  atomic.Bool
	done    chan struct{}
}

// NewHTTPTransport creates a transport for the given server URL.
func NewHTTPTransport(serverURL string) (*HTTPTransport, error) {
	u, err := url.Parse(serverURL)
	if err != nil {
		return nil, fmt.Errorf("parse server URL: %w", err)
	}
	return &HTTPTransport{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: u,
		done:    make(chan struct{}),
	}, nil
}

// NewHTTPTransportWithClient creates a transport with a custom HTTP client.
func NewHTTPTransportWithClient(serverURL string, client *http.Client) (*HTTPTransport, error) {
	u, err := url.Parse(serverURL)
	if err != nil {
		return nil, fmt.Errorf("parse server URL: %w", err)
	}
	return &HTTPTransport{
		client:  client,
		baseURL: u,
		done:    make(chan struct{}),
	}, nil
}

func (t *HTTPTransport) Send(ctx context.Context, msg *Message) (*Message, error) {
	if t.closed.Load() {
		return nil, errors.New("transport closed")
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal message: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.baseURL.String(), strings.NewReader(string(data)))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
	}
	var result Message
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

func (t *HTTPTransport) Close() error {
	if t.closed.CompareAndSwap(false, true) {
		close(t.done)
	}
	return nil
}

func (t *HTTPTransport) IsClosed() bool { return t.closed.Load() }

// ServerConfig describes how to connect to an MCP server. It mirrors the
// configuration shape used by the CLI so the transport layer can build a
// Transport from user-supplied settings without depending on the config package.
type ServerConfig struct {
	Name    string
	URI     string
	Stdio   bool
	Command string
	Args    []string
}

// NewTransport builds a Transport for the given server configuration.
// HTTP servers use NewHTTPTransport; stdio servers use NewChildStdioTransport
// which spawns the configured command and communicates over its stdin/stdout.
func NewTransport(cfg ServerConfig) (Transport, error) {
	if cfg.Stdio {
		return NewChildStdioTransport(cfg.Command, cfg.Args)
	}
	if cfg.URI == "" {
		return nil, fmt.Errorf("MCP server %q: either stdio+command or uri must be set", cfg.Name)
	}
	return NewHTTPTransport(cfg.URI)
}

// Client provides a high-level interface over a Transport, handling request
// ID assignment and response matching.
type Client struct {
	transport Transport
	nextID    atomic.Int64
	mu        sync.Mutex
	pending   map[int64]chan *Message
}

// NewClient wraps a Transport with request/response correlation.
func NewClient(transport Transport) *Client {
	return &Client{
		transport: transport,
		pending:   make(map[int64]chan *Message),
	}
}

// Call sends a JSON-RPC request and waits for the matching response.
func (c *Client) Call(ctx context.Context, method string, params any) (*Message, error) {
	id := c.nextID.Add(1)
	msg := &Message{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  method,
	}
	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("marshal params: %w", err)
		}
		msg.Params = data
	}
	respCh := make(chan *Message, 1)
	c.mu.Lock()
	c.pending[id] = respCh
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()
	resp, err := c.transport.Send(ctx, msg)
	if err != nil {
		return nil, err
	}
	// If the transport returned a response synchronously, use it.
	if resp != nil {
		if resp.Error != nil {
			return nil, resp.Error
		}
		return resp, nil
	}
	// Otherwise, wait for the response via the pending channel.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-respCh:
		if resp.Error != nil {
			return nil, resp.Error
		}
		return resp, nil
	}
}

// Initialize sends the MCP initialize request and returns server capabilities.
func (c *Client) Initialize(ctx context.Context, protocolVersion string, clientInfo ClientInfo) (*InitializeResult, error) {
	params := map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities":    clientCapabilities{},
		"clientInfo":      clientInfo,
	}
	resp, err := c.Call(ctx, "initialize", params)
	if err != nil {
		return nil, err
	}
	var result InitializeResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("decode initialize result: %w", err)
	}
	_, _ = c.Call(ctx, "notifications/initialized", nil)
	return &result, nil
}

// ClientInfo describes the MCP client.
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type clientCapabilities struct{}

// InitializeResult is the response to an initialize request.
type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ServerInfo         `json:"serverInfo"`
}

// ServerCapabilities describes what the server supports.
type ServerCapabilities struct {
	Tools     *ToolCapabilities     `json:"tools,omitempty"`
	Resources *ResourceCapabilities `json:"resources,omitempty"`
	Prompts   *PromptCapabilities   `json:"prompts,omitempty"`
}

type ToolCapabilities struct {
	ListChanged bool `json:"listChanged,omitempty"`
}
type ResourceCapabilities struct {
	ListChanged bool `json:"listChanged,omitempty"`
}
type PromptCapabilities struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ListToolsResult is the response to tools/list.
type ListToolsResult struct {
	Tools      []ToolDefinition `json:"tools"`
	NextCursor string           `json:"nextCursor,omitempty"`
}

// ToolDefinition describes a tool the server can call.
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// CallToolResult is the response to tools/call.
type CallToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock is a piece of content returned by a tool call.
type ContentBlock struct {
	Type  string        `json:"type"`
	Text  string        `json:"text,omitempty"`
	Image *ImageContent `json:"image,omitempty"`
}

type ImageContent struct {
	Data     string `json:"data"`
	MIMEType string `json:"mimeType"`
}

// ListResourcesResult is the response to resources/list.
type ListResourcesResult struct {
	Resources  []ResourceDefinition `json:"resources"`
	NextCursor string               `json:"nextCursor,omitempty"`
}

// ResourceDefinition describes a resource the server can read.
type ResourceDefinition struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MIMEType    string `json:"mimeType,omitempty"`
}

// ReadResourceResult is the response to resources/read.
type ReadResourceResult struct {
	Contents []ResourceContent `json:"contents"`
}

type ResourceContent struct {
	URI      string `json:"uri"`
	MIMEType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
}

// ListPromptsResult is the response to prompts/list.
type ListPromptsResult struct {
	Prompts    []PromptDefinition `json:"prompts"`
	NextCursor string             `json:"nextCursor,omitempty"`
}

type PromptDefinition struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
}

type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// Close sends the MCP close notification.
func (c *Client) Close(ctx context.Context) error {
	_, err := c.Call(ctx, "notifications/exit", nil)
	if cerr := c.transport.Close(); cerr != nil && err == nil {
		err = cerr
	}
	return err
}
