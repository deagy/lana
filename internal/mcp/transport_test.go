package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mockTransport implements Transport for testing.
type mockTransport struct {
	response  *Message
	sendCount int
}

func (m *mockTransport) Send(_ context.Context, msg *Message) (*Message, error) {
	m.sendCount++
	return m.response, nil
}

func (m *mockTransport) Close() error   { return nil }
func (m *mockTransport) IsClosed() bool { return false }

// mockWriteCloser wraps bytes.Buffer to implement io.WriteCloser.
type mockWriteCloser struct {
	*bytes.Buffer
}

func (m *mockWriteCloser) Close() error { return nil }

func TestStdioTransportSend(t *testing.T) {
	// Test that stdio transport can be created and closed.
	var buf bytes.Buffer
	tr := NewStdioTransport(&mockWriteCloser{&buf}, &buf)
	if tr.IsClosed() {
		t.Fatal("expected transport not closed")
	}
	if err := tr.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
	if !tr.IsClosed() {
		t.Fatal("expected transport closed")
	}
}

func TestHTTPTransportNew(t *testing.T) {
	tr, err := NewHTTPTransport("http://localhost:3000/mcp")
	if err != nil {
		t.Fatalf("NewHTTPTransport failed: %v", err)
	}
	if tr.IsClosed() {
		t.Fatal("expected transport not closed")
	}
	if err := tr.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
}

func TestHTTPTransportInvalidURL(t *testing.T) {
	_, err := NewHTTPTransport("://invalid")
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestHTTPTransportSend(t *testing.T) {
	// Create a mock HTTP server that returns a JSON-RPC response.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"result":  map[string]any{"status": "ok"},
		})
	}))
	defer server.Close()

	tr, err := NewHTTPTransport(server.URL)
	if err != nil {
		t.Fatalf("NewHTTPTransport failed: %v", err)
	}
	defer tr.Close()

	client := NewClient(tr)
	ctx := context.Background()
	resp, err := client.Call(ctx, "test/method", nil)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	if resp.JSONRPC != "2.0" {
		t.Errorf("expected JSONRPC 2.0, got %s", resp.JSONRPC)
	}
}

func TestHTTPTransportServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer server.Close()

	tr, err := NewHTTPTransport(server.URL)
	if err != nil {
		t.Fatalf("NewHTTPTransport failed: %v", err)
	}
	defer tr.Close()

	client := NewClient(tr)
	ctx := context.Background()
	_, err = client.Call(ctx, "test/method", nil)
	if err == nil {
		t.Fatal("expected error for 500 status")
	}
}

func TestRPCError(t *testing.T) {
	err := &RPCError{Code: CodeInternalError, Message: "internal error"}
	if err.Error() != "MCP error -32603: internal error" {
		t.Errorf("unexpected error string: %s", err.Error())
	}
}

func int64Ptr(i int64) *int64 { return &i }
