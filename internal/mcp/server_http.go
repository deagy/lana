package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
)

// HTTPServer runs an MCP server over HTTP.
type HTTPServer struct {
	server   *Server
	port     int
	token    string // Optional bearer token for auth
	httpSvr  *http.Server
	mu       sync.Mutex
}

// NewHTTPServer creates a new HTTP-based MCP server.
func NewHTTPServer(server *Server, port int, token string) *HTTPServer {
	hs := &HTTPServer{
		server: server,
		port:   port,
		token:  token,
	}

	// Set up callbacks for sending responses via HTTP
	server.SetCallbacks(
		func(resp *Response) error {
			// HTTP responses are sent immediately, no queue needed
			return nil
		},
		func(notif *Notification) error {
			// Notifications could be queued for future SSE implementation
			return nil
		},
	)

	return hs
}

// Run starts the HTTP server and blocks until the context is cancelled or an error occurs.
func (hs *HTTPServer) Run(ctx context.Context) error {
	mux := http.NewServeMux()

	// MCP endpoint
	mux.HandleFunc("/mcp", hs.handleMCP)

	// Health check endpoint
	mux.HandleFunc("/health", hs.handleHealth)

	addr := fmt.Sprintf(":%d", hs.port)
	hs.httpSvr = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- hs.httpSvr.ListenAndServe()
	}()

	fmt.Fprintf(io.Discard, "HTTP MCP server listening on %s\n", addr)

	// Wait for context cancellation or server error
	select {
	case <-ctx.Done():
		hs.httpSvr.Shutdown(context.Background())
		return ctx.Err()
	case err := <-errChan:
		if err != http.ErrServerClosed {
			return err
		}
		return nil
	}
}

// handleMCP processes incoming JSON-RPC requests via HTTP.
func (hs *HTTPServer) handleMCP(w http.ResponseWriter, r *http.Request) {
	// Check authentication if token is configured
	if hs.token != "" {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer "+hs.token {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	// Only accept POST
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read request body
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Parse JSON-RPC request
	var req Request
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Handle request
	resp, err := hs.server.HandleRequest(context.Background(), &req)
	if err != nil {
		errorResp := &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &ResponseError{
				Code:    -32603,
				Message: "Internal error",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(errorResp)
		return
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		fmt.Fprintf(io.Discard, "Error encoding response: %v\n", err)
	}
}

// handleHealth provides a health check endpoint.
func (hs *HTTPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

// StartHTTPServer is a helper that creates and starts an HTTP server.
func StartHTTPServer(ctx context.Context, server *Server, port int, token string) error {
	httpServer := NewHTTPServer(server, port, token)
	return httpServer.Run(ctx)
}
