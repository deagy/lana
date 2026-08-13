package plugin

import (
	"fmt"
	"os"
	"path/filepath"
)

// InstalledPlugins scans the plugins directory and returns all valid installed plugins.
// Invalid plugins (those that fail to load or validate) are skipped with a warning to stderr.
// This provides partial-failure tolerance: one broken plugin doesn't prevent others from loading.
func InstalledPlugins(pluginsDir string) ([]*Manifest, error) {
	// Create plugins directory if it doesn't exist
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		return nil, fmt.Errorf("create plugins dir: %w", err)
	}

	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return nil, fmt.Errorf("read plugins dir: %w", err)
	}

	var plugins []*Manifest

	for _, entry := range entries {
		if !entry.IsDir() {
			continue // Skip non-directory entries
		}

		pluginPath := filepath.Join(pluginsDir, entry.Name())
		manifest, err := LoadManifest(pluginPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to load plugin %q: %v\n", entry.Name(), err)
			continue
		}

		if err := manifest.Validate(pluginPath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Invalid plugin %q: %v\n", entry.Name(), err)
			continue
		}

		plugins = append(plugins, manifest)
	}

	return plugins, nil
}

// FindPlugin searches for an installed plugin by name.
func FindPlugin(pluginsDir, name string) (*Manifest, error) {
	plugins, err := InstalledPlugins(pluginsDir)
	if err != nil {
		return nil, err
	}

	for _, p := range plugins {
		if p.Name == name {
			return p, nil
		}
	}

	return nil, fmt.Errorf("plugin %q not found", name)
}
