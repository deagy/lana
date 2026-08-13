package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManifestValidation(t *testing.T) {
	tests := []struct {
		name      string
		manifest  *Manifest
		setup     func(dir string) error
		wantError bool
		errMsg    string
	}{
		{
			name: "valid manifest",
			manifest: &Manifest{
				Name:        "myplugin",
				Version:     "1.0.0",
				Description: "Test plugin",
				Entrypoint:  "plugin.sh",
			},
			setup: func(dir string) error {
				// Create executable entrypoint
				return os.WriteFile(filepath.Join(dir, "plugin.sh"), []byte("#!/bin/bash\necho hello"), 0755)
			},
			wantError: false,
		},
		{
			name: "missing name",
			manifest: &Manifest{
				Name:       "",
				Entrypoint: "plugin.sh",
			},
			setup: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "plugin.sh"), []byte("#!/bin/bash\n"), 0755)
			},
			wantError: true,
			errMsg:    "name is required",
		},
		{
			name: "invalid name - starts with number",
			manifest: &Manifest{
				Name:       "9plugin",
				Entrypoint: "plugin.sh",
			},
			setup: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "plugin.sh"), []byte("#!/bin/bash\n"), 0755)
			},
			wantError: true,
			errMsg:    "invalid name",
		},
		{
			name: "invalid name - uppercase",
			manifest: &Manifest{
				Name:       "MyPlugin",
				Entrypoint: "plugin.sh",
			},
			setup: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "plugin.sh"), []byte("#!/bin/bash\n"), 0755)
			},
			wantError: true,
			errMsg:    "invalid name",
		},
		{
			name: "valid name with hyphens",
			manifest: &Manifest{
				Name:       "my-cool-plugin",
				Entrypoint: "plugin.sh",
			},
			setup: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "plugin.sh"), []byte("#!/bin/bash\n"), 0755)
			},
			wantError: false,
		},
		{
			name: "missing entrypoint",
			manifest: &Manifest{
				Name:       "plugin",
				Entrypoint: "",
			},
			setup:     func(dir string) error { return nil },
			wantError: true,
			errMsg:    "entrypoint is required",
		},
		{
			name: "entrypoint not found",
			manifest: &Manifest{
				Name:       "plugin",
				Entrypoint: "missing.sh",
			},
			setup:     func(dir string) error { return nil },
			wantError: true,
			errMsg:    "entrypoint not found",
		},
		{
			name: "entrypoint not executable",
			manifest: &Manifest{
				Name:       "plugin",
				Entrypoint: "plugin.sh",
			},
			setup: func(dir string) error {
				// Create non-executable file
				return os.WriteFile(filepath.Join(dir, "plugin.sh"), []byte("#!/bin/bash\n"), 0644)
			},
			wantError: true,
			errMsg:    "not executable",
		},
		{
			name: "entrypoint is directory",
			manifest: &Manifest{
				Name:       "plugin",
				Entrypoint: "subdir",
			},
			setup: func(dir string) error {
				// Create directory instead of file
				return os.Mkdir(filepath.Join(dir, "subdir"), 0755)
			},
			wantError: true,
			errMsg:    "is a directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			if tt.setup != nil {
				if err := tt.setup(tmpDir); err != nil {
					t.Fatalf("setup failed: %v", err)
				}
			}

			err := tt.manifest.Validate(tmpDir)
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
			if tt.wantError && tt.errMsg != "" && err != nil {
				if len(err.Error()) == 0 || !contains(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %v, want to contain %q", err, tt.errMsg)
				}
			}
		})
	}
}

func TestEntrypointPath(t *testing.T) {
	m := &Manifest{
		Name:       "test",
		Entrypoint: "bin/run.sh",
	}
	expected := filepath.Join("/tmp/plugins/test", "bin/run.sh")
	got := m.EntrypointPath("/tmp/plugins/test")
	if got != expected {
		t.Errorf("EntrypointPath() = %q, want %q", got, expected)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr) != -1
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
