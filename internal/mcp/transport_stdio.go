package mcp

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"
)

// StdioTransport implements a stdio-based transport (spawns a subprocess).
type StdioTransport struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
	done   chan error
}

// NewStdioTransport creates a new stdio transport, spawning the given command.
func NewStdioTransport(command string, args []string, env map[string]string) (*StdioTransport, error) {
	cmd := exec.Command(command, args...)

	// Preserve parent environment and add/override with provided env
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Set up pipes
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("create stderr pipe: %w", err)
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start command: %w", err)
	}

	// Drain stderr in background (non-blocking, just consume it)
	go drainStderr(stderr)

	// Monitor process completion
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	return &StdioTransport{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
		done:   done,
	}, nil
}

// Write implements io.Writer.
func (t *StdioTransport) Write(p []byte) (n int, err error) {
	return t.stdin.Write(p)
}

// Read implements io.Reader.
func (t *StdioTransport) Read(p []byte) (n int, err error) {
	return t.stdout.Read(p)
}

// Close closes the transport and terminates the subprocess.
func (t *StdioTransport) Close() error {
	// Close stdin to signal EOF
	_ = t.stdin.Close()

	// Wait for process to exit gracefully (up to 5 seconds)
	select {
	case err := <-t.done:
		return err
	case <-time.After(5 * time.Second):
		// Force kill if it doesn't exit
		_ = t.cmd.Process.Kill()
		return <-t.done
	}
}

// drainStderr reads and discards stderr output (non-blocking).
// In a real implementation, this could log or redirect the output.
func drainStderr(stderr io.ReadCloser) {
	_, _ = io.ReadAll(stderr)
	_ = stderr.Close()
}
