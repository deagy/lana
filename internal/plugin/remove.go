package plugin

import (
	"fmt"
	"os"
	"path/filepath"
)

// Remove uninstalls a plugin by deleting its directory.
func Remove(pluginsDir, name string) error {
	pluginPath := filepath.Join(pluginsDir, name)

	// Check if plugin exists
	if _, err := os.Stat(pluginPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("plugin %q not found", name)
		}
		return fmt.Errorf("stat plugin: %w", err)
	}

	// Remove the plugin directory
	if err := os.RemoveAll(pluginPath); err != nil {
		return fmt.Errorf("remove plugin: %w", err)
	}

	return nil
}
