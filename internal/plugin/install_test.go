package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstall(t *testing.T) {
	tests := []struct {
		name         string
		setupSource  func(dir string) error
		reservedName bool
		preExisting  bool
		wantError    bool
		errMsg       string
	}{
		{
			name: "successful install",
			setupSource: func(dir string) error {
				if err := os.WriteFile(filepath.Join(dir, "manifest.yaml"), []byte(`
name: testplugin
version: 1.0.0
description: Test plugin
entrypoint: run.sh
`), 0644); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(dir, "run.sh"), []byte("#!/bin/bash\necho test"), 0755)
			},
			wantError: false,
		},
		{
			name: "reserved name collision",
			setupSource: func(dir string) error {
				if err := os.WriteFile(filepath.Join(dir, "manifest.yaml"), []byte(`
name: chat
version: 1.0.0
entrypoint: run.sh
`), 0644); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(dir, "run.sh"), []byte("#!/bin/bash\n"), 0755)
			},
			reservedName: true,
			wantError:    true,
			errMsg:       "conflicts with a built-in",
		},
		{
			name: "already installed",
			setupSource: func(dir string) error {
				if err := os.WriteFile(filepath.Join(dir, "manifest.yaml"), []byte(`
name: duplicate
version: 1.0.0
entrypoint: run.sh
`), 0644); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(dir, "run.sh"), []byte("#!/bin/bash\n"), 0755)
			},
			preExisting: true,
			wantError:   true,
			errMsg:      "already installed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup source plugin
			sourceDir := t.TempDir()
			if err := tt.setupSource(sourceDir); err != nil {
				t.Fatalf("setup failed: %v", err)
			}

			// Setup plugins directory
			pluginsDir := t.TempDir()

			// Setup reserved names
			reserved := map[string]bool{
				"chat": true, "run": true, "version": true,
				"config": true, "providers": true, "models": true,
				"sessions": true, "doctor": true, "mcp": true,
			}

			// Pre-create if needed
			if tt.preExisting {
				dupDir := filepath.Join(pluginsDir, "duplicate")
				if err := os.Mkdir(dupDir, 0755); err != nil {
					t.Fatalf("pre-existing setup failed: %v", err)
				}
			}

			// Test reserved name if needed
			if tt.reservedName {
				reserved["chat"] = true
			}

			manifest, err := Install(sourceDir, pluginsDir, reserved)
			if (err != nil) != tt.wantError {
				t.Errorf("Install() error = %v, wantError %v", err, tt.wantError)
			}
			if tt.wantError && tt.errMsg != "" && err != nil {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("Install() error = %v, want to contain %q", err, tt.errMsg)
				}
			}
			if !tt.wantError && manifest == nil {
				t.Errorf("Install() returned nil manifest on success")
			}
			if !tt.wantError && manifest != nil {
				// Verify installation
				installed := filepath.Join(pluginsDir, manifest.Name, "run.sh")
				info, err := os.Stat(installed)
				if err != nil {
					t.Errorf("installed plugin not found: %v", err)
				}
				if (info.Mode() & 0111) == 0 {
					t.Errorf("installed entrypoint not executable")
				}
			}
		})
	}
}
