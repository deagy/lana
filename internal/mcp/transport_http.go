package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// HTTPTransport implements an HTTP/SSE-based transport.
type HTTPTransport struct {
	url        string
	headers    map[string]string
	httpClient *http.Client
	buffer     *bytes.Buffer
	scanner    *bufio.Scanner
	closed     bool
}

// NewHTTPTransport creates a new HTTP transport that connects to the given URL.
func NewHTTPTransport(url string, headers map[string]string) (*HTTPTransport, error) {
	if headers == nil {
		headers = make(map[string]string)
	}

	// Create HTTP client with timeout
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Pre-test the connection by doing a simple request to verify the server is up
	// (This is optional, but helps catch configuration errors early)
	req, err := http.NewRequest("POST", url, bytes.NewReader([]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)))
	if err != nil {
		return nil, fmt.Errorf("create test request: %w", err)
	}

	// Set headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("test connection: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, string(body))
	}

	// Read response body to pre-populate the buffer
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	buffer := bytes.NewBuffer(bodyBytes)
	scanner := bufio.NewScanner(buffer)

	return &HTTPTransport{
		url:        url,
		headers:    headers,
		httpClient: httpClient,
		buffer:     buffer,
		scanner:    scanner,
		closed:     false,
	}, nil
}

// Write sends a request via HTTP POST.
func (t *HTTPTransport) Write(p []byte) (n int, err error) {
	if t.closed {
		return 0, io.EOF
	}

	req, err := http.NewRequest("POST", t.url, bytes.NewReader(p))
	if err != nil {
		return 0, fmt.Errorf("create request: %w", err)
	}

	// Set headers
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("http %d: %s", resp.StatusCode, string(body))
	}

	// Read and buffer the response
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("read response: %w", err)
	}

	// Reset the scanner with new data
	t.buffer = bytes.NewBuffer(bodyBytes)
	t.scanner = bufio.NewScanner(t.buffer)

	return len(p), nil
}

// Read reads responses or SSE events.
// For this implementation, we'll return one complete JSON object per read,
// or one SSE event if the response is SSE-formatted.
func (t *HTTPTransport) Read(p []byte) (n int, err error) {
	if t.closed {
		return 0, io.EOF
	}

	// Try to read a line from the buffer
	if !t.scanner.Scan() {
		if err := t.scanner.Err(); err != nil {
			return 0, err
		}
		// No more data in buffer
		return 0, io.EOF
	}

	line := t.scanner.Text()
	if line == "" {
		// Empty line, might be part of SSE framing, skip and try next
		return t.Read(p)
	}

	// Parse SSE format if line starts with "data:"
	if strings.HasPrefix(line, "data:") {
		line = strings.TrimPrefix(line, "data:")
		line = strings.TrimSpace(line)
	}

	// Try to parse as JSON to verify it's valid
	var obj interface{}
	if err := json.Unmarshal([]byte(line), &obj); err != nil {
		// Invalid JSON, skip and try next line
		return t.Read(p)
	}

	// Copy the line into the provided buffer
	lineBytes := []byte(line + "\n")
	if len(lineBytes) > len(p) {
		copy(p, lineBytes[:len(p)])
		return len(p), nil
	}

	copy(p, lineBytes)
	return len(lineBytes), nil
}

// Close closes the transport.
func (t *HTTPTransport) Close() error {
	t.closed = true
	return nil
}
