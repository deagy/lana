package plugin

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPluginListEmpty(t *testing.T) {
	dir := t.TempDir()
	originalPluginsDir := pluginsDir
	pluginsDir = dir
	defer func() { pluginsDir = originalPluginsDir }()

	cmd := NewCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"list"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "No plugins installed") {
		t.Fatalf("expected 'No plugins installed', got: %q", out.String())
	}
}

func TestPluginInstallAndList(t *testing.T) {
	dir := t.TempDir()
	originalPluginsDir := pluginsDir
	pluginsDir = dir
	defer func() { pluginsDir = originalPluginsDir }()

	// Create a source directory
	sourceDir := filepath.Join(t.TempDir(), "test-plugin")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "README.md"), []byte("test plugin"), 0644); err != nil {
		t.Fatal(err)
	}

	// Install the plugin
	cmd := NewCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"install", sourceDir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("install failed: %v (%s)", err, errOut.String())
	}

	// List plugins
	cmd = NewCommand()
	out = bytes.Buffer{}
	errOut = bytes.Buffer{}
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"list"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	output := out.String()
	if !strings.Contains(output, "test-plugin") {
		t.Fatalf("expected 'test-plugin' in output, got: %q", output)
	}
	if !strings.Contains(output, "v0.0.1") {
		t.Fatalf("expected 'v0.0.1' in output, got: %q", output)
	}
}

func TestPluginInstallJSON(t *testing.T) {
	dir := t.TempDir()
	originalPluginsDir := pluginsDir
	pluginsDir = dir
	defer func() { pluginsDir = originalPluginsDir }()

	sourceDir := filepath.Join(t.TempDir(), "json-plugin")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}

	cmd := NewCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"install", "--name", "my-plugin", "--version", "1.2.3", sourceDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	cmd = NewCommand()
	out = bytes.Buffer{}
	errOut = bytes.Buffer{}
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"list", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	var plugins []PluginManifest
	if err := json.Unmarshal([]byte(out.String()), &plugins); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}
	if plugins[0].Name != "my-plugin" {
		t.Fatalf("expected name 'my-plugin', got %q", plugins[0].Name)
	}
	if plugins[0].Version != "1.2.3" {
		t.Fatalf("expected version '1.2.3', got %q", plugins[0].Version)
	}
}

func TestPluginInfo(t *testing.T) {
	dir := t.TempDir()
	originalPluginsDir := pluginsDir
	pluginsDir = dir
	defer func() { pluginsDir = originalPluginsDir }()

	sourceDir := filepath.Join(t.TempDir(), "info-plugin")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}

	cmd := NewCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"install", sourceDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	// Get info
	cmd = NewCommand()
	out = bytes.Buffer{}
	errOut = bytes.Buffer{}
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"info", "info-plugin"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	output := out.String()
	if !strings.Contains(output, "Name:        info-plugin") {
		t.Fatalf("expected plugin name in output, got: %q", output)
	}
	if !strings.Contains(output, "Version:     0.0.1") {
		t.Fatalf("expected version in output, got: %q", output)
	}
}

func TestPluginInfoNotFound(t *testing.T) {
	dir := t.TempDir()
	originalPluginsDir := pluginsDir
	pluginsDir = dir
	defer func() { pluginsDir = originalPluginsDir }()

	cmd := NewCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"info", "nonexistent"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for nonexistent plugin")
	}
	if !strings.Contains(errOut.String(), "no plugins found") {
		t.Fatalf("expected 'no plugins found' error, got: %q", errOut.String())
	}
}

func TestPluginEnableDisable(t *testing.T) {
	dir := t.TempDir()
	originalPluginsDir := pluginsDir
	pluginsDir = dir
	defer func() { pluginsDir = originalPluginsDir }()

	sourceDir := filepath.Join(t.TempDir(), "toggle-plugin")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}

	cmd := NewCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"install", sourceDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	// Disable
	cmd = NewCommand()
	out = bytes.Buffer{}
	errOut = bytes.Buffer{}
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"disable", "toggle-plugin"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "disabled") {
		t.Fatalf("expected 'disabled' in output, got: %q", out.String())
	}

	// Enable
	cmd = NewCommand()
	out = bytes.Buffer{}
	errOut = bytes.Buffer{}
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"enable", "toggle-plugin"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "enabled") {
		t.Fatalf("expected 'enabled' in output, got: %q", out.String())
	}
}

func TestPluginRemove(t *testing.T) {
	dir := t.TempDir()
	originalPluginsDir := pluginsDir
	pluginsDir = dir
	defer func() { pluginsDir = originalPluginsDir }()

	sourceDir := filepath.Join(t.TempDir(), "remove-plugin")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}

	cmd := NewCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"install", sourceDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	// Remove with force
	cmd = NewCommand()
	out = bytes.Buffer{}
	errOut = bytes.Buffer{}
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"remove", "--force", "remove-plugin"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Plugin removed") {
		t.Fatalf("expected 'Plugin removed' in output, got: %q", out.String())
	}

	// Verify it's gone
	cmd = NewCommand()
	out = bytes.Buffer{}
	errOut = bytes.Buffer{}
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"list"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "remove-plugin") {
		t.Fatalf("plugin should be removed, but found in output: %q", out.String())
	}
}

func TestPluginRemoveWithoutForce(t *testing.T) {
	dir := t.TempDir()
	originalPluginsDir := pluginsDir
	pluginsDir = dir
	defer func() { pluginsDir = originalPluginsDir }()

	cmd := NewCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"remove", "some-plugin"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error without --force")
	}
	if !strings.Contains(errOut.String(), "no plugins found") {
		t.Fatalf("expected 'no plugins found' error, got: %q", errOut.String())
	}
}

func TestPluginInstallRequiresArg(t *testing.T) {
	dir := t.TempDir()
	originalPluginsDir := pluginsDir
	pluginsDir = dir
	defer func() { pluginsDir = originalPluginsDir }()

	cmd := NewCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"install"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for missing argument")
	}
}

func TestPluginInstallSourceMustBeDirectory(t *testing.T) {
	dir := t.TempDir()
	originalPluginsDir := pluginsDir
	pluginsDir = dir
	defer func() { pluginsDir = originalPluginsDir }()

	// Create a file, not a directory
	file := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(file, []byte("not a directory"), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := NewCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"install", file})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for file source")
	}
	if !strings.Contains(errOut.String(), "must be a directory") {
		t.Fatalf("expected 'must be a directory' error, got: %q", errOut.String())
	}
}
