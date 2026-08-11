package mcp

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListAndInfoOutputRedactsMCPServerCredentials(t *testing.T) {
	const secret = "mcp-credential-must-not-appear"
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	contents := "mcp:\n  servers:\n    - name: private\n      uri: https://alice:" + secret + "@example.test/mcp?api_token=" + secret + "&authorization=" + secret + "\n"
	if err := os.WriteFile(configPath, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		args []string
	}{
		{"list text", []string{"list-resources", "--config", configPath}},
		{"list JSON", []string{"list-resources", "--config", configPath, "--json"}},
		{"templates JSON", []string{"list-templates", "--config", configPath, "--json"}},
		{"tools", []string{"list-tools", "--config", configPath}},
		{"server info", []string{"server-info", "--config", configPath}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			output := captureStdout(t, func() error {
				cmd := NewCommand()
				cmd.SetArgs(test.args)
				return cmd.Execute()
			})
			if strings.Contains(output, secret) || strings.Contains(output, "alice") {
				t.Fatalf("output leaked MCP credentials: %q", output)
			}
			if !strings.Contains(output, "[REDACTED]") {
				t.Fatalf("output did not include a redaction marker: %q", output)
			}
		})
	}
}

func TestReadResourceAndCallToolDoNotEchoSensitiveInput(t *testing.T) {
	const secret = "mcp-input-secret-must-not-appear"
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("mcp:\n  servers:\n    - name: private\n      uri: https://example.test/mcp\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"read-resource", "--config", configPath, "--server", "private", "--uri", "https://example.test/resource?access_token=" + secret},
		{"call-tool", "--config", configPath, "--server", "private", "--tool", "lookup", "--args", `{"api_token":"` + secret + `"}`},
	} {
		output := captureStdout(t, func() error {
			cmd := NewCommand()
			cmd.SetArgs(args)
			return cmd.Execute()
		})
		if strings.Contains(output, secret) {
			t.Fatalf("output leaked sensitive input: %q", output)
		}
	}
}

func captureStdout(t *testing.T, run func() error) string {
	t.Helper()
	previous := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	err = run()
	_ = writer.Close()
	os.Stdout = previous
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if _, err := output.ReadFrom(reader); err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	return output.String()
}
