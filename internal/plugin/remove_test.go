package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemove(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(string) error
		wantError bool
		errMsg    string
	}{
		{
			name: "successful remove",
			setup: func(dir string) error {
				pluginDir := filepath.Join(dir, "testplugin")
				if err := os.Mkdir(pluginDir, 0755); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(pluginDir, "manifest.yaml"), []byte("name: testplugin\n"), 0644)
			},
			wantError: false,
		},
		{
			name: "plugin not found",
			setup: func(dir string) error {
				return nil // Don't create plugin
			},
			wantError: true,
			errMsg:    "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pluginsDir := t.TempDir()
			if err := tt.setup(pluginsDir); err != nil {
				t.Fatalf("setup failed: %v", err)
			}

			err := Remove(pluginsDir, "testplugin")
			if (err != nil) != tt.wantError {
				t.Errorf("Remove() error = %v, wantError %v", err, tt.wantError)
			}
			if tt.wantError && tt.errMsg != "" && err != nil {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("Remove() error = %v, want to contain %q", err, tt.errMsg)
				}
			}

			// Verify removal
			if !tt.wantError {
				pluginPath := filepath.Join(pluginsDir, "testplugin")
				if _, err := os.Stat(pluginPath); err == nil {
					t.Errorf("Remove() did not delete plugin directory")
				}
			}
		})
	}
}
