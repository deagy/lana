package system

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestEnvCommandNeverPrintsSecretValues(t *testing.T) {
	const key = "LANA_TEST_API_TOKEN"
	const secret = "system-env-secret-must-not-appear"
	t.Setenv(key, secret)

	for _, args := range [][]string{{}, {"--secret"}} {
		cmd := envCommand()
		var output bytes.Buffer
		cmd.SetOut(&output)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("env command returned an error: %v", err)
		}
		if strings.Contains(output.String(), secret) {
			t.Fatalf("env output leaked a secret with args %v: %q", args, output.String())
		}
		if !strings.Contains(output.String(), key+"=***REDACTED***") {
			t.Fatalf("env output did not redact %s: %q", key, output.String())
		}
	}
}

func TestConfigJSONRedactsMCPURI(t *testing.T) {
	configPath := t.TempDir() + "/config.yaml"
	const secret = "system-config-secret-must-not-appear"
	contents := "mcp:\n  servers:\n    - name: private\n      uri: https://alice:" + secret + "@example.test/mcp?api_token=" + secret + "\n"
	if err := os.WriteFile(configPath, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}

	output := captureStdout(t, func() error {
		cmd := configCommand()
		cmd.SetArgs([]string{"--config", configPath, "--json"})
		return cmd.Execute()
	})
	if strings.Contains(output, secret) || strings.Contains(output, "alice") {
		t.Fatalf("config JSON leaked MCP credentials: %q", output)
	}
	if !strings.Contains(output, "[REDACTED]") {
		t.Fatalf("config JSON did not contain the redaction marker: %q", output)
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
