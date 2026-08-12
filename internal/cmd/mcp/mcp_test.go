package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/deagy/lana/internal/mcp"
)

// captureStdout runs fn with stdout redirected to a buffer and returns the
// captured output along with any error from fn.
func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	previousStdout := os.Stdout
	previousStderr := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = stderrWriter
	err = fn()
	_ = writer.Close()
	os.Stdout = previousStdout
	_ = stderrWriter.Close()
	os.Stderr = previousStderr
	var output bytes.Buffer
	if _, err := output.ReadFrom(reader); err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	var stderrBuf bytes.Buffer
	if _, err := stderrBuf.ReadFrom(stderrReader); err != nil {
		t.Fatal(err)
	}
	if err := stderrReader.Close(); err != nil {
		t.Fatal(err)
	}
	// Combine stdout and stderr for inspection.
	return output.String() + stderrBuf.String(), err
}

// mockMCPServer returns an httptest.Server that implements a minimal MCP
// server: it responds to initialize, tools/list, resources/list, prompts/list,
// resources/read, and tools/call with canned responses.
func mockMCPServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var req mcp.Message
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		switch req.Method {
		case "initialize":
			json.NewEncoder(w).Encode(mcp.Message{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{"tools":{},"resources":{},"prompts":{}},"serverInfo":{"name":"test-server","version":"1.0.0"}}`),
			})
		case "notifications/initialized":
			w.WriteHeader(http.StatusOK)
		case "notifications/exit":
			json.NewEncoder(w).Encode(mcp.Message{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(`{}`),
			})
		case "tools/list":
			json.NewEncoder(w).Encode(mcp.Message{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(`{"tools":[{"name":"echo","description":"Echo input","inputSchema":{"type":"object"}}]}`),
			})
		case "resources/list":
			json.NewEncoder(w).Encode(mcp.Message{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(`{"resources":[{"uri":"file:///test.txt","name":"test","description":"A test file"}]}`),
			})
		case "prompts/list":
			json.NewEncoder(w).Encode(mcp.Message{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(`{"prompts":[{"name":"greet","description":"Greet someone","arguments":[{"name":"name","description":"Name","required":true}]}]}`),
			})
		case "resources/read":
			json.NewEncoder(w).Encode(mcp.Message{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(`{"contents":[{"uri":"file:///test.txt","text":"hello world"}]}`),
			})
		case "tools/call":
			json.NewEncoder(w).Encode(mcp.Message{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(`{"content":[{"type":"text","text":"echoed"}],"isError":false}`),
			})
		default:
			json.NewEncoder(w).Encode(mcp.Message{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &mcp.RPCError{Code: mcp.CodeMethodNotFound, Message: "unknown method: " + req.Method},
			})
		}
	}))
}

func TestListResourcesConnectsAndDisplaysResources(t *testing.T) {
	server := mockMCPServer()
	defer server.Close()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	contents := "mcp:\n  servers:\n    - name: test\n      uri: " + server.URL + "\n"
	if err := os.WriteFile(configPath, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}

	output, err := captureStdout(t, func() error {
		cmd := NewCommand()
		cmd.SetArgs([]string{"list-resources", "--config", configPath})
		return cmd.Execute()
	})
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(output, "test") || !strings.Contains(output, "file:///test.txt") {
		t.Fatalf("expected resource output, got: %q", output)
	}
}

func TestListToolsConnectsAndDisplaysTools(t *testing.T) {
	server := mockMCPServer()
	defer server.Close()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	contents := "mcp:\n  servers:\n    - name: test\n      uri: " + server.URL + "\n"
	if err := os.WriteFile(configPath, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}

	output, err := captureStdout(t, func() error {
		cmd := NewCommand()
		cmd.SetArgs([]string{"list-tools", "--config", configPath})
		return cmd.Execute()
	})
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(output, "echo") || !strings.Contains(output, "Echo input") {
		t.Fatalf("expected tool output, got: %q", output)
	}
}

func TestReadResourceConnectsAndReturnsContent(t *testing.T) {
	server := mockMCPServer()
	defer server.Close()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	contents := "mcp:\n  servers:\n    - name: test\n      uri: " + server.URL + "\n"
	if err := os.WriteFile(configPath, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}

	output, err := captureStdout(t, func() error {
		cmd := NewCommand()
		cmd.SetArgs([]string{"read-resource", "--config", configPath, "--server", "test", "--uri", "file:///test.txt"})
		return cmd.Execute()
	})
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(output, "hello world") {
		t.Fatalf("expected resource content, got: %q", output)
	}
}

func TestListTemplatesConnectsAndDisplaysPrompts(t *testing.T) {
	server := mockMCPServer()
	defer server.Close()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	contents := "mcp:\n  servers:\n    - name: test\n      uri: " + server.URL + "\n"
	if err := os.WriteFile(configPath, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}

	output, err := captureStdout(t, func() error {
		cmd := NewCommand()
		cmd.SetArgs([]string{"list-templates", "--config", configPath})
		return cmd.Execute()
	})
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(output, "greet") || !strings.Contains(output, "required") {
		t.Fatalf("expected prompt output, got: %q", output)
	}
}

func TestCallToolConnectsAndReturnsResult(t *testing.T) {
	server := mockMCPServer()
	defer server.Close()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	contents := "mcp:\n  servers:\n    - name: test\n      uri: " + server.URL + "\n"
	if err := os.WriteFile(configPath, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}

	output, err := captureStdout(t, func() error {
		cmd := NewCommand()
		cmd.SetArgs([]string{"call-tool", "--config", configPath, "--server", "test", "--tool", "echo", "--args", `{}`})
		return cmd.Execute()
	})
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(output, "echoed") {
		t.Fatalf("expected tool result, got: %q", output)
	}
}

func TestServerInfoConnectsAndDisplaysCapabilities(t *testing.T) {
	server := mockMCPServer()
	defer server.Close()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	contents := "mcp:\n  servers:\n    - name: test\n      uri: " + server.URL + "\n"
	if err := os.WriteFile(configPath, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}

	output, err := captureStdout(t, func() error {
		cmd := NewCommand()
		cmd.SetArgs([]string{"server-info", "--config", configPath})
		return cmd.Execute()
	})
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(output, "test-server") || !strings.Contains(output, "tools: yes") {
		t.Fatalf("expected server info, got: %q", output)
	}
}

func TestJSONOutputFormat(t *testing.T) {
	server := mockMCPServer()
	defer server.Close()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	contents := "mcp:\n  servers:\n    - name: test\n      uri: " + server.URL + "\n"
	if err := os.WriteFile(configPath, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}

	output, err := captureStdout(t, func() error {
		cmd := NewCommand()
		cmd.SetArgs([]string{"list-tools", "--config", configPath, "--json"})
		return cmd.Execute()
	})
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	// Verify it's valid JSON
	var result []any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("expected valid JSON array, got: %q, err: %v", output, err)
	}
}

func TestRedactsMCPServerCredentialsInOutput(t *testing.T) {
	const secret = "mcp-credential-must-not-appear"
	server := mockMCPServer()
	defer server.Close()

	// Use a URI with embedded credentials pointing at the mock server.
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	contents := "mcp:\n  servers:\n    - name: private\n      uri: http://alice:" + secret + "@" + strings.TrimPrefix(server.URL, "http://") + "/mcp?api_token=" + secret + "\n"
	if err := os.WriteFile(configPath, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{
		{"list-resources", "--config", configPath},
		{"list-resources", "--config", configPath, "--json"},
		{"list-tools", "--config", configPath},
		{"server-info", "--config", configPath},
	} {
		t.Run(args[0], func(t *testing.T) {
			output, err := captureStdout(t, func() error {
				cmd := NewCommand()
				cmd.SetArgs(args)
				return cmd.Execute()
			})
			if err != nil {
				t.Fatalf("command failed: %v", err)
			}
			if strings.Contains(output, secret) {
				t.Fatalf("output leaked MCP credentials: %q", output)
			}
		})
	}
}

func TestReadResourceDoesNotEchoSensitiveURI(t *testing.T) {
	const secret = "mcp-input-secret-must-not-appear"
	server := mockMCPServer()
	defer server.Close()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("mcp:\n  servers:\n    - name: private\n      uri: "+server.URL+"\n"), 0600); err != nil {
		t.Fatal(err)
	}

	output, err := captureStdout(t, func() error {
		cmd := NewCommand()
		cmd.SetArgs([]string{"read-resource", "--config", configPath, "--server", "private", "--uri", "file:///test?token=" + secret})
		return cmd.Execute()
	})
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if strings.Contains(output, secret) {
		t.Fatalf("output leaked sensitive URI: %q", output)
	}
}

func TestCallToolDoesNotEchoSensitiveArgs(t *testing.T) {
	const secret = "mcp-tool-arg-secret-must-not-appear"
	server := mockMCPServer()
	defer server.Close()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("mcp:\n  servers:\n    - name: private\n      uri: "+server.URL+"\n"), 0600); err != nil {
		t.Fatal(err)
	}

	output, err := captureStdout(t, func() error {
		cmd := NewCommand()
		cmd.SetArgs([]string{"call-tool", "--config", configPath, "--server", "private", "--tool", "echo", "--args", `{"api_token":"` + secret + `"}`})
		return cmd.Execute()
	})
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if strings.Contains(output, secret) {
		t.Fatalf("output leaked sensitive args: %q", output)
	}
}

func TestConnectsToStdioServer(t *testing.T) {
	// Verify that the ServerManager can build a stdio transport config
	// without actually spawning a process (we only test the config path).
	mgr := mcp.NewServerManager(0)
	defer mgr.Close(context.Background())

	// A stdio server with a non-existent command should fail to connect
	// but should not panic.
	_, err := mgr.Connect(context.Background(), "bad",
		mcp.ServerConfig{Name: "bad", Stdio: true, Command: "/nonexistent/binary"},
		false)
	if err == nil {
		t.Fatal("expected error connecting to non-existent stdio server")
	}
}

func TestHTTPServerConfig(t *testing.T) {
	server := mockMCPServer()
	defer server.Close()

	mgr := mcp.NewServerManager(0)
	defer mgr.Close(context.Background())

	client, err := mgr.Connect(context.Background(), "http-test",
		mcp.ServerConfig{Name: "http-test", URI: server.URL},
		false)
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	if !client.IsConnected() {
		t.Fatal("expected client to be connected")
	}
}

func TestCachesConnections(t *testing.T) {
	server := mockMCPServer()
	defer server.Close()

	mgr := mcp.NewServerManager(0)
	defer mgr.Close(context.Background())

	cfg := mcp.ServerConfig{Name: "cached", URI: server.URL}
	first, err := mgr.Connect(context.Background(), "cached", cfg, false)
	if err != nil {
		t.Fatalf("first connect: %v", err)
	}
	second, err := mgr.Connect(context.Background(), "cached", cfg, false)
	if err != nil {
		t.Fatalf("second connect: %v", err)
	}
	if first != second {
		t.Fatal("expected cached connection to be the same instance")
	}
}

func TestForceReconnect(t *testing.T) {
	server := mockMCPServer()
	defer server.Close()

	mgr := mcp.NewServerManager(0)
	defer mgr.Close(context.Background())

	cfg := mcp.ServerConfig{Name: "reconnect", URI: server.URL}
	first, err := mgr.Connect(context.Background(), "reconnect", cfg, false)
	if err != nil {
		t.Fatalf("first connect: %v", err)
	}
	second, err := mgr.Connect(context.Background(), "reconnect", cfg, true)
	if err != nil {
		t.Fatalf("reconnect: %v", err)
	}
	if first == second {
		t.Fatal("expected force-reconnect to return a different instance")
	}
}

func TestCommandDoesNotFailWithNoServers(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("mcp:\n  servers: []\n"), 0600); err != nil {
		t.Fatal(err)
	}

	output, err := captureStdout(t, func() error {
		cmd := NewCommand()
		cmd.SetArgs([]string{"list-resources", "--config", configPath})
		return cmd.Execute()
	})
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(output, "No MCP servers configured") {
		t.Fatalf("expected no-servers message, got: %q", output)
	}
}

// Ensure cobra is imported for NewCommand usage in tests.
var _ = cobra.Command{}
