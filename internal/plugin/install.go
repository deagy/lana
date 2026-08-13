package plugin

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// Install installs a plugin from the given source path into the plugins directory.
// It validates the manifest, checks for name collisions, and copies the entire
// plugin directory tree. The entrypoint is made executable if it exists.
func Install(sourcePath, pluginsDir string, reservedNames map[string]bool) (*Manifest, error) {
	// Load and validate manifest at source
	manifest, err := LoadManifest(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("load manifest: %w", err)
	}

	if err := manifest.Validate(sourcePath); err != nil {
		return nil, fmt.Errorf("invalid manifest: %w", err)
	}

	// Check for collision with reserved names
	if reservedNames[manifest.Name] {
		return nil, fmt.Errorf("plugin name %q conflicts with a built-in Lana command", manifest.Name)
	}

	// Check for collision with existing plugins
	pluginInstallDir := filepath.Join(pluginsDir, manifest.Name)
	if _, err := os.Stat(pluginInstallDir); err == nil {
		return nil, fmt.Errorf("plugin %q is already installed at %s", manifest.Name, pluginInstallDir)
	}

	// Create plugins directory if needed
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		return nil, fmt.Errorf("create plugins dir: %w", err)
	}

	// Copy the entire plugin directory
	if err := copyDir(sourcePath, pluginInstallDir); err != nil {
		// Clean up on failure
		_ = os.RemoveAll(pluginInstallDir)
		return nil, fmt.Errorf("copy plugin: %w", err)
	}

	// Make entrypoint executable
	entrypointPath := manifest.EntrypointPath(pluginInstallDir)
	if err := os.Chmod(entrypointPath, 0755); err != nil {
		_ = os.RemoveAll(pluginInstallDir)
		return nil, fmt.Errorf("make entrypoint executable: %w", err)
	}

	return manifest, nil
}

// copyDir recursively copies a directory tree from src to dst.
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Calculate destination path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)

		if d.IsDir() {
			return os.MkdirAll(dstPath, 0755)
		}

		// Copy file
		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		dstFile, err := os.Create(dstPath)
		if err != nil {
			return err
		}
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		return err
	})
}
