package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHTTPTransportPlainJSON tests HTTP transport with plain JSON response.
func TestHTTPTransportPlainJSON(t *testing.T) {
	// Create a fake server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := Response{
			JSONRPC: "2.0",
			ID:      1,
			Result:  json.RawMessage(`{"status": "ok"}`),
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create transport
	transport, err := NewHTTPTransport(server.URL, nil)
	if err != nil {
		t.Fatalf("NewHTTPTransport failed: %v", err)
	}
	defer transport.Close()

	// Send a request
	req := Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "test",
	}
	reqBytes, _ := json.Marshal(req)
	_, err = transport.Write(append(reqBytes, '\n'))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read response
	buf := make([]byte, 4096)
	n, err := transport.Read(buf)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	var resp Response
	if err := json.Unmarshal(buf[:n], &resp); err != nil {
		t.Fatalf("Parse response failed: %v", err)
	}

	if resp.ID != 1 {
		t.Fatalf("expected ID 1, got %d", resp.ID)
	}
}

// TestHTTPTransportSSE tests HTTP transport with SSE-formatted response.
func TestHTTPTransportSSE(t *testing.T) {
	// Create a fake server that returns SSE-formatted data
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		response := Response{
			JSONRPC: "2.0",
			ID:      1,
			Result:  json.RawMessage(`{"status": "ok"}`),
		}
		respBytes, _ := json.Marshal(response)

		// Write SSE format
		w.Write([]byte("data: "))
		w.Write(respBytes)
		w.Write([]byte("\n\n"))
	}))
	defer server.Close()

	// Create transport
	transport, err := NewHTTPTransport(server.URL, nil)
	if err != nil {
		t.Fatalf("NewHTTPTransport failed: %v", err)
	}
	defer transport.Close()

	// Send a request
	req := Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "test",
	}
	reqBytes, _ := json.Marshal(req)
	_, err = transport.Write(append(reqBytes, '\n'))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read response
	buf := make([]byte, 4096)
	n, err := transport.Read(buf)
	if err != nil && err.Error() != "EOF" {
		t.Fatalf("Read failed: %v", err)
	}

	if n > 0 {
		var resp Response
		if err := json.Unmarshal(buf[:n], &resp); err != nil {
			t.Fatalf("Parse response failed: %v", err)
		}

		if resp.ID != 1 {
			t.Fatalf("expected ID 1, got %d", resp.ID)
		}
	}
}
