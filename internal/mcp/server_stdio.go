package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
)

// StdioServer runs an MCP server over stdin/stdout.
type StdioServer struct {
	server *Server
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
	mu     sync.Mutex
}

// NewStdioServer creates a new stdio-based MCP server.
func NewStdioServer(server *Server, stdin io.Reader, stdout, stderr io.Writer) *StdioServer {
	ss := &StdioServer{
		server: server,
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
	}

	// Set up callbacks for sending responses
	server.SetCallbacks(
		func(resp *Response) error {
			return ss.sendResponse(resp)
		},
		func(notif *Notification) error {
			return ss.sendNotification(notif)
		},
	)

	return ss
}

// Run starts the stdio server and blocks until the context is cancelled or an error occurs.
func (ss *StdioServer) Run(ctx context.Context) error {
	return ss.server.ReadLoop(ctx, ss.stdin)
}

// sendResponse writes a JSON-RPC response to stdout.
func (ss *StdioServer) sendResponse(resp *Response) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	data, err := json.Marshal(resp)
	if err != nil {
		fmt.Fprintf(ss.stderr, "Marshal error: %v\n", err)
		return err
	}

	_, err = ss.stdout.Write(data)
	if err != nil {
		fmt.Fprintf(ss.stderr, "Write error: %v\n", err)
		return err
	}

	_, err = ss.stdout.Write([]byte("\n"))
	if err != nil {
		fmt.Fprintf(ss.stderr, "Write error: %v\n", err)
		return err
	}

	return nil
}

// sendNotification writes a JSON-RPC notification to stdout.
func (ss *StdioServer) sendNotification(notif *Notification) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	data, err := json.Marshal(notif)
	if err != nil {
		fmt.Fprintf(ss.stderr, "Marshal error: %v\n", err)
		return err
	}

	_, err = ss.stdout.Write(data)
	if err != nil {
		fmt.Fprintf(ss.stderr, "Write error: %v\n", err)
		return err
	}

	_, err = ss.stdout.Write([]byte("\n"))
	if err != nil {
		fmt.Fprintf(ss.stderr, "Write error: %v\n", err)
		return err
	}

	return nil
}

// StartStdioServer is a helper that sets up and starts a stdio server.
func StartStdioServer(ctx context.Context, server *Server) error {
	stdioServer := NewStdioServer(server, os.Stdin, os.Stdout, os.Stderr)
	return stdioServer.Run(ctx)
}
