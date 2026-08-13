package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"gopkg.in/yaml.v3"

	"github.com/deagy/lana/internal/config"
)

// Manifest represents a plugin's metadata and configuration.
type Manifest struct {
	Name        string                   `yaml:"name"`
	Version     string                   `yaml:"version"`
	Description string                   `yaml:"description"`
	Entrypoint  string                   `yaml:"entrypoint"`
	MCPServers  []config.MCPServerConfig `yaml:"mcp_servers,omitempty"`
}

// LoadManifest loads and parses a manifest.yaml from the given directory.
func LoadManifest(pluginDir string) (*Manifest, error) {
	manifestPath := filepath.Join(pluginDir, "manifest.yaml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}

	return &m, nil
}

// Validate checks the manifest for correctness and completeness.
// It verifies the plugin name matches the allowed pattern and the entrypoint is executable.
func (m *Manifest) Validate(pluginDir string) error {
	// Check name is provided
	if m.Name == "" {
		return fmt.Errorf("name is required")
	}

	// Check name matches pattern: lowercase alphanumeric with hyphens
	namePattern := regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	if !namePattern.MatchString(m.Name) {
		return fmt.Errorf("invalid name %q: must start with lowercase letter and contain only lowercase letters, numbers, and hyphens", m.Name)
	}

	// Check entrypoint is provided
	if m.Entrypoint == "" {
		return fmt.Errorf("entrypoint is required")
	}

	// Check entrypoint exists and is executable
	entrypointPath := filepath.Join(pluginDir, m.Entrypoint)
	info, err := os.Stat(entrypointPath)
	if err != nil {
		return fmt.Errorf("entrypoint not found at %s: %w", entrypointPath, err)
	}

	if info.IsDir() {
		return fmt.Errorf("entrypoint %s is a directory, not a file", entrypointPath)
	}

	// Check if executable
	if (info.Mode() & 0111) == 0 {
		return fmt.Errorf("entrypoint %s is not executable", entrypointPath)
	}

	return nil
}

// EntrypointPath returns the absolute path to the plugin's entrypoint executable.
func (m *Manifest) EntrypointPath(pluginDir string) string {
	return filepath.Join(pluginDir, m.Entrypoint)
}
