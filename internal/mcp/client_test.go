package mcp

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"
)

// TestClientInitialize tests the initialization handshake.
func TestClientInitialize(t *testing.T) {
	// Create a fake server that responds to initialize
	serverIn, clientOut := io.Pipe()
	clientIn, serverOut := io.Pipe()

	// Start a fake server goroutine
	go fakeInitializeServer(t, serverIn, serverOut)

	// Create client with the pipe transport
	transport := &pipeTransport{
		in:  clientIn,
		out: clientOut,
	}
	client := NewClient(transport)

	// Initialize
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Initialize(ctx, "test", "1.0")
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	client.Close()
}

// TestClientListTools tests tool listing.
func TestClientListTools(t *testing.T) {
	serverIn, clientOut := io.Pipe()
	clientIn, serverOut := io.Pipe()

	go func() {
		// Read initialize request
		buf := make([]byte, 4096)
		serverIn.Read(buf)

		// Send initialize response
		initResp := Response{
			JSONRPC: "2.0",
			ID:      1,
			Result: json.RawMessage(`{
				"protocolVersion": "2024-11-05",
				"capabilities": {},
				"serverInfo": {"name": "test", "version": "1.0"}
			}`),
		}
		respBytes, _ := json.Marshal(initResp)
		serverOut.Write(append(respBytes, '\n'))

		// Read list tools request
		buf = make([]byte, 4096)
		serverIn.Read(buf)

		// Send list tools response
		toolsResp := Response{
			JSONRPC: "2.0",
			ID:      2,
			Result: json.RawMessage(`{
				"tools": [
					{
						"name": "echo",
						"description": "Echo tool",
						"inputSchema": {"type": "object", "properties": {"text": {"type": "string"}}}
					}
				]
			}`),
		}
		respBytes, _ = json.Marshal(toolsResp)
		serverOut.Write(append(respBytes, '\n'))

		serverIn.Close()
		serverOut.Close()
	}()

	transport := &pipeTransport{
		in:  clientIn,
		out: clientOut,
	}
	client := NewClient(transport)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Initialize first
	_ = client.Initialize(ctx, "test", "1.0")

	// List tools
	tools, err := client.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}

	if tools[0].Name != "echo" {
		t.Fatalf("expected tool name 'echo', got %s", tools[0].Name)
	}

	client.Close()
}

// TestClientCallTool tests tool invocation.
func TestClientCallTool(t *testing.T) {
	serverIn, clientOut := io.Pipe()
	clientIn, serverOut := io.Pipe()

	go func() {
		// Read initialize request
		buf := make([]byte, 4096)
		serverIn.Read(buf)

		// Send initialize response
		initResp := Response{
			JSONRPC: "2.0",
			ID:      1,
			Result: json.RawMessage(`{
				"protocolVersion": "2024-11-05",
				"capabilities": {},
				"serverInfo": {"name": "test", "version": "1.0"}
			}`),
		}
		respBytes, _ := json.Marshal(initResp)
		serverOut.Write(append(respBytes, '\n'))

		// Read call tool request
		buf = make([]byte, 4096)
		serverIn.Read(buf)

		// Send call tool response
		callResp := Response{
			JSONRPC: "2.0",
			ID:      2,
			Result: json.RawMessage(`{
				"content": [{"type": "text", "text": "Hello, world!"}],
				"isError": false
			}`),
		}
		respBytes, _ = json.Marshal(callResp)
		serverOut.Write(append(respBytes, '\n'))

		serverIn.Close()
		serverOut.Close()
	}()

	transport := &pipeTransport{
		in:  clientIn,
		out: clientOut,
	}
	client := NewClient(transport)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Initialize first
	_ = client.Initialize(ctx, "test", "1.0")

	// Call tool
	args, _ := json.Marshal(map[string]string{"text": "hello"})
	output, isError, err := client.CallTool(ctx, "echo", json.RawMessage(args))
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}

	if isError {
		t.Fatalf("tool returned error, expected success")
	}

	if output != "Hello, world!" {
		t.Fatalf("expected 'Hello, world!', got %s", output)
	}

	client.Close()
}

// pipeTransport is a simple transport for testing using io.Pipe.
type pipeTransport struct {
	in  io.Reader
	out io.Writer
}

func (p *pipeTransport) Read(b []byte) (int, error) {
	return p.in.Read(b)
}

func (p *pipeTransport) Write(b []byte) (int, error) {
	return p.out.Write(b)
}

func (p *pipeTransport) Close() error {
	return nil
}

// fakeInitializeServer is a fake server that handles initialize requests.
func fakeInitializeServer(t *testing.T, in io.Reader, out io.Writer) {
	scanner := &scannerBuf{r: in}

	// Read request
	line := scanner.Scan()
	if !strings.Contains(string(line), "initialize") {
		t.Logf("expected initialize request, got: %s", string(line))
	}

	// Send response
	resp := Response{
		JSONRPC: "2.0",
		ID:      1,
		Result: json.RawMessage(`{
			"protocolVersion": "2024-11-05",
			"capabilities": {},
			"serverInfo": {"name": "fake", "version": "1.0"}
		}`),
	}
	respBytes, _ := json.Marshal(resp)
	out.Write(append(respBytes, '\n'))
}

// scannerBuf provides line-scanning for testing
type scannerBuf struct {
	r   io.Reader
	buf []byte
}

func (s *scannerBuf) Scan() []byte {
	var result []byte
	buf := make([]byte, 1)
	for {
		n, _ := s.r.Read(buf)
		if n == 0 {
			break
		}
		result = append(result, buf[0])
		if buf[0] == '\n' {
			break
		}
	}
	return result
}
