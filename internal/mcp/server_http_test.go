package mcp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deagy/lana/internal/tools/impl"
)

func TestHTTPServerMCPEndpoint(t *testing.T) {
	registry, err := impl.InitializeRegistry(".")
	if err != nil {
		t.Fatalf("Initialize registry: %v", err)
	}

	server := NewServer(registry)
	httpServer := NewHTTPServer(server, 0, "")

	// Create test HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", httpServer.handleMCP)
	mux.HandleFunc("/health", httpServer.handleHealth)
	testSrv := httptest.NewServer(mux)
	defer testSrv.Close()

	// Test initialize
	reqBody := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`
	resp, err := http.Post(testSrv.URL+"/mcp", "application/json", bytes.NewBufferString(reqBody))
	if err != nil {
		t.Fatalf("POST /mcp: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status = %d, want 200", resp.StatusCode)
	}

	var result Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Decode response: %v", err)
	}

	if result.Error != nil {
		t.Errorf("Response error = %v, want nil", result.Error)
	}
}

func TestHTTPServerListTools(t *testing.T) {
	registry, err := impl.InitializeRegistry(".")
	if err != nil {
		t.Fatalf("Initialize registry: %v", err)
	}

	server := NewServer(registry)
	httpServer := NewHTTPServer(server, 0, "")

	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", httpServer.handleMCP)
	testSrv := httptest.NewServer(mux)
	defer testSrv.Close()

	reqBody := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`
	resp, err := http.Post(testSrv.URL+"/mcp", "application/json", bytes.NewBufferString(reqBody))
	if err != nil {
		t.Fatalf("POST /mcp: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status = %d, want 200", resp.StatusCode)
	}

	var result Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Decode response: %v", err)
	}

	var listResult ListToolsResult
	if err := json.Unmarshal(result.Result, &listResult); err != nil {
		t.Fatalf("Unmarshal result: %v", err)
	}

	if len(listResult.Tools) == 0 {
		t.Errorf("No tools returned")
	}
}

func TestHTTPServerHealth(t *testing.T) {
	registry, err := impl.InitializeRegistry(".")
	if err != nil {
		t.Fatalf("Initialize registry: %v", err)
	}

	server := NewServer(registry)
	httpServer := NewHTTPServer(server, 0, "")

	mux := http.NewServeMux()
	mux.HandleFunc("/health", httpServer.handleHealth)
	testSrv := httptest.NewServer(mux)
	defer testSrv.Close()

	resp, err := http.Get(testSrv.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status = %d, want 200", resp.StatusCode)
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Decode response: %v", err)
	}

	if result["status"] != "healthy" {
		t.Errorf("Status = %q, want healthy", result["status"])
	}
}

func TestHTTPServerMethodNotAllowed(t *testing.T) {
	registry, err := impl.InitializeRegistry(".")
	if err != nil {
		t.Fatalf("Initialize registry: %v", err)
	}

	server := NewServer(registry)
	httpServer := NewHTTPServer(server, 0, "")

	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", httpServer.handleMCP)
	testSrv := httptest.NewServer(mux)
	defer testSrv.Close()

	resp, err := http.Get(testSrv.URL + "/mcp")
	if err != nil {
		t.Fatalf("GET /mcp: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusMethodNotAllowed)
	}
}

func TestHTTPServerInvalidJSON(t *testing.T) {
	registry, err := impl.InitializeRegistry(".")
	if err != nil {
		t.Fatalf("Initialize registry: %v", err)
	}

	server := NewServer(registry)
	httpServer := NewHTTPServer(server, 0, "")

	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", httpServer.handleMCP)
	testSrv := httptest.NewServer(mux)
	defer testSrv.Close()

	resp, err := http.Post(testSrv.URL+"/mcp", "application/json", bytes.NewBufferString("invalid json"))
	if err != nil {
		t.Fatalf("POST /mcp: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestHTTPServerCORSHeaders(t *testing.T) {
	registry, err := impl.InitializeRegistry(".")
	if err != nil {
		t.Fatalf("Initialize registry: %v", err)
	}

	server := NewServer(registry)
	httpServer := NewHTTPServer(server, 0, "")

	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", httpServer.handleMCP)
	testSrv := httptest.NewServer(mux)
	defer testSrv.Close()

	reqBody := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`
	resp, err := http.Post(testSrv.URL+"/mcp", "application/json", bytes.NewBufferString(reqBody))
	if err != nil {
		t.Fatalf("POST /mcp: %v", err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("Access-Control-Allow-Origin") == "" {
		t.Errorf("CORS header missing")
	}

	if resp.Header.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", resp.Header.Get("Content-Type"))
	}
}
