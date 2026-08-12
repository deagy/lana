package config

import (
	"path/filepath"
	"testing"
)

func TestGetNestedValue(t *testing.T) {
	cfg := DefaultConfig()
	tests := []struct {
		key      string
		expected string
	}{
		{"workspace.path", cfg.Workspace.Path},
		{"workspace.max_file_size", cfg.Workspace.MaxFileSize},
		{"logging.level", cfg.Logging.Level},
		{"logging.format", cfg.Logging.Format},
		{"mcp.rate_limit", "10"},
		{"exec.sandbox", "workspace-write"},
		{"dispatch.max_parallel", "4"},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			val, err := GetNestedValue(cfg, tt.key)
			if err != nil {
				t.Fatalf("GetNestedValue(%q) error: %v", tt.key, err)
			}
			if val != tt.expected {
				t.Errorf("GetNestedValue(%q) = %q, want %q", tt.key, val, tt.expected)
			}
		})
	}
}

func TestGetNestedValueUnknown(t *testing.T) {
	cfg := DefaultConfig()
	_, err := GetNestedValue(cfg, "unknown.key")
	if err == nil {
		t.Fatal("expected error for unknown key")
	}
}

func TestSetNestedValue(t *testing.T) {
	cfg := DefaultConfig()
	tests := []struct {
		key   string
		value string
	}{
		{"logging.level", "debug"},
		{"logging.format", "json"},
		{"exec.sandbox", "unrestricted"},
		{"mcp.rate_limit", "20"},
		{"dispatch.max_parallel", "8"},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if err := SetNestedValue(cfg, tt.key, tt.value); err != nil {
				t.Fatalf("SetNestedValue(%q, %q) error: %v", tt.key, tt.value, err)
			}
			val, err := GetNestedValue(cfg, tt.key)
			if err != nil {
				t.Fatalf("GetNestedValue(%q) error: %v", tt.key, err)
			}
			if val != tt.value {
				t.Errorf("SetNestedValue then GetNestedValue(%q) = %q, want %q", tt.key, val, tt.value)
			}
		})
	}
}

func TestSetNestedValueUnknown(t *testing.T) {
	cfg := DefaultConfig()
	err := SetNestedValue(cfg, "unknown.key", "value")
	if err == nil {
		t.Fatal("expected error for unknown key")
	}
}

func TestSetNestedValueInvalidRateLimit(t *testing.T) {
	cfg := DefaultConfig()
	err := SetNestedValue(cfg, "mcp.rate_limit", "not-a-number")
	if err == nil {
		t.Fatal("expected error for invalid rate limit")
	}
}

func TestSetNestedValueKnowledgeStore(t *testing.T) {
	cfg := DefaultConfig()
	if err := SetNestedValue(cfg, "knowledge_store.path", "/tmp/knowledge"); err != nil {
		t.Fatalf("SetNestedValue error: %v", err)
	}
	val, err := GetNestedValue(cfg, "knowledge_store.path")
	if err != nil {
		t.Fatalf("GetNestedValue error: %v", err)
	}
	if val != "/tmp/knowledge" {
		t.Errorf("expected /tmp/knowledge, got %s", val)
	}
}

func TestConfigWriteAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := DefaultConfig()
	cfg.Logging.Level = "debug"
	cfg.Workspace.Path = "/tmp/workspace"

	if err := cfg.WriteToPath(path); err != nil {
		t.Fatalf("WriteToPath error: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if loaded.Logging.Level != "debug" {
		t.Errorf("expected debug, got %s", loaded.Logging.Level)
	}
	if loaded.Workspace.Path != "/tmp/workspace" {
		t.Errorf("expected /tmp/workspace, got %s", loaded.Workspace.Path)
	}
}

// TestDefaultConfig, TestExpandPath, and TestGetMCPServer are defined in config_test.go.
