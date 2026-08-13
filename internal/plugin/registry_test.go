package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstalledPlugins(t *testing.T) {
	pluginsDir := t.TempDir()

	// Create one valid plugin
	validDir := filepath.Join(pluginsDir, "valid")
	if err := os.Mkdir(validDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(validDir, "manifest.yaml"), []byte(`
name: validplugin
version: 1.0.0
description: A valid plugin
entrypoint: run.sh
`), 0644); err != nil {
		t.Fatalf("write manifest failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(validDir, "run.sh"), []byte("#!/bin/bash\n"), 0755); err != nil {
		t.Fatalf("write entrypoint failed: %v", err)
	}

	// Create one broken plugin (missing entrypoint)
	brokenDir := filepath.Join(pluginsDir, "broken")
	if err := os.Mkdir(brokenDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(brokenDir, "manifest.yaml"), []byte(`
name: brokenplugin
version: 1.0.0
entrypoint: missing.sh
`), 0644); err != nil {
		t.Fatalf("write manifest failed: %v", err)
	}

	// Test registry scan (should return only valid plugin, skip broken)
	plugins, err := InstalledPlugins(pluginsDir)
	if err != nil {
		t.Errorf("InstalledPlugins() error = %v", err)
	}

	if len(plugins) != 1 {
		t.Errorf("InstalledPlugins() returned %d plugins, want 1", len(plugins))
	}
	if len(plugins) > 0 && plugins[0].Name != "validplugin" {
		t.Errorf("InstalledPlugins() returned plugin %q, want validplugin", plugins[0].Name)
	}
}

func TestInstalledPluginsCreateDir(t *testing.T) {
	pluginsDir := filepath.Join(t.TempDir(), "nonexistent", "plugins")

	plugins, err := InstalledPlugins(pluginsDir)
	if err != nil {
		t.Errorf("InstalledPlugins() error = %v, want nil", err)
	}
	if len(plugins) != 0 {
		t.Errorf("InstalledPlugins() returned %d plugins, want 0", len(plugins))
	}

	// Verify directory was created
	if _, err := os.Stat(pluginsDir); err != nil {
		t.Errorf("plugins dir not created: %v", err)
	}
}

func TestFindPlugin(t *testing.T) {
	pluginsDir := t.TempDir()

	// Create a plugin
	pluginDir := filepath.Join(pluginsDir, "myplugin")
	if err := os.Mkdir(pluginDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "manifest.yaml"), []byte(`
name: myplugin
version: 1.0.0
entrypoint: run.sh
`), 0644); err != nil {
		t.Fatalf("write manifest failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "run.sh"), []byte("#!/bin/bash\n"), 0755); err != nil {
		t.Fatalf("write entrypoint failed: %v", err)
	}

	// Test finding existing plugin
	plugin, err := FindPlugin(pluginsDir, "myplugin")
	if err != nil {
		t.Errorf("FindPlugin() error = %v", err)
	}
	if plugin == nil || plugin.Name != "myplugin" {
		t.Errorf("FindPlugin() returned %v, want myplugin manifest", plugin)
	}

	// Test finding non-existent plugin
	_, err = FindPlugin(pluginsDir, "nonexistent")
	if err == nil {
		t.Errorf("FindPlugin() error = nil, want not found error")
	}
}
