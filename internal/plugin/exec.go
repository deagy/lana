package plugin

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// Run executes a plugin as a subprocess, passing through stdin/stdout/stderr.
// The plugin receives the provided arguments and runs in the caller's working directory.
// Context cancellation is respected.
func Run(ctx context.Context, pluginDir string, manifest *Manifest, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	entrypointPath := manifest.EntrypointPath(pluginDir)

	// Get current working directory for the plugin
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	// Create command
	cmd := exec.CommandContext(ctx, entrypointPath, args...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Dir = cwd

	// Run and return the error (which includes exit code)
	return cmd.Run()
}
