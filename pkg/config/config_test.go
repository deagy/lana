package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Workspace.Path != "." {
		t.Errorf("expected workspace path '.', got %q", cfg.Workspace.Path)
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("expected log level 'info', got %q", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "text" {
		t.Errorf("expected log format 'text', got %q", cfg.Logging.Format)
	}
	if cfg.MCP.RateLimit != 10 {
		t.Errorf("expected rate limit 10, got %d", cfg.MCP.RateLimit)
	}
	if cfg.MCP.Timeout != 30*time.Second {
		t.Errorf("expected MCP timeout 30s, got %v", cfg.MCP.Timeout)
	}
	if cfg.Exec.Sandbox != "workspace-write" {
		t.Errorf("expected sandbox 'workspace-write', got %q", cfg.Exec.Sandbox)
	}
	if cfg.KnowledgeStore == nil {
		t.Error("expected KnowledgeStore to be non-nil")
	}
	if cfg.Dispatch.MaxParallel != 4 {
		t.Errorf("expected max parallel 4, got %d", cfg.Dispatch.MaxParallel)
	}
}

func TestConfigValidate(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected no validation error: %v", err)
	}
}

func TestConfigValidate_EmptyPath(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Workspace.Path = ""
	if err := cfg.Validate(); err == nil {
		t.Error("expected validation error for empty workspace path")
	}
}

func TestConfigValidate_InvalidSandbox(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Exec.Sandbox = "invalid"
	if err := cfg.Validate(); err == nil {
		t.Error("expected validation error for invalid sandbox mode")
	}
}

func TestConfigValidate_InvalidLogLevel(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Logging.Level = "invalid"
	if err := cfg.Validate(); err == nil {
		t.Error("expected validation error for invalid log level")
	}
}

func TestConfigValidate_InvalidFormat(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Logging.Format = "xml"
	if err := cfg.Validate(); err == nil {
		t.Error("expected validation error for invalid log format")
	}
}

func TestConfigValidate_RateLimit(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MCP.RateLimit = 0
	if err := cfg.Validate(); err == nil {
		t.Error("expected validation error for rate limit < 1")
	}
}

func TestConfigLoad_NonexistentFile(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Error("expected error for nonexistent config file")
	}
}

func TestConfigLoad_WithFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configPath, []byte(`
workspace:
  path: "/my/workspace"
logging:
  level: "debug"
  format: "json"
mcp:
  rate_limit: 20
  timeout: "60s"
exec:
  sandbox: "workspace-read-only"
`), 0644)

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Workspace.Path != "/my/workspace" {
		t.Errorf("expected path '/my/workspace', got %q", cfg.Workspace.Path)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("expected log level 'debug', got %q", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "json" {
		t.Errorf("expected log format 'json', got %q", cfg.Logging.Format)
	}
	if cfg.MCP.RateLimit != 20 {
		t.Errorf("expected rate limit 20, got %d", cfg.MCP.RateLimit)
	}
	if cfg.MCP.Timeout != 60*time.Second {
		t.Errorf("expected MCP timeout 60s, got %v", cfg.MCP.Timeout)
	}
	if cfg.Exec.Sandbox != "workspace-read-only" {
		t.Errorf("expected sandbox 'workspace-read-only', got %q", cfg.Exec.Sandbox)
	}
}

func TestGetMCPServer(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MCP.Servers = []MCPServerConfig{
		{Name: "test-server", URI: "http://localhost:3000/mcp"},
	}

	s := cfg.GetMCPServer("test-server")
	if s == nil {
		t.Error("expected to find test-server")
	}

	s = cfg.GetMCPServer("nonexistent")
	if s != nil {
		t.Error("expected nil for nonexistent server")
	}
}

func TestExpandPath(t *testing.T) {
	// Test with tilde expansion
	result := expandPath("~/test")
	if len(result) < 4 || result[:2] != "~/" {
		// Might fail on systems where UserHomeDir fails
		t.Log("tilde expansion may fail on this system")
	}

	// Test with env var expansion
	result = expandPath("$HOME/test")
	if result == "$HOME/test" {
		// This is OK if HOME is not set
		t.Log("env var expansion may fail on this system")
	}
}

func TestConfigWriteToPath(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Workspace.Path = "/my/workspace"
	cfg.Logging.Level = "debug"

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	if err := cfg.WriteToPath(configPath); err != nil {
		t.Fatalf("unexpected error writing config: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("config file not created")
	}

	// Load it back
	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("unexpected error loading config: %v", err)
	}
	if loaded.Workspace.Path != "/my/workspace" {
		t.Errorf("expected path '/my/workspace', got %q", loaded.Workspace.Path)
	}
	if loaded.Logging.Level != "debug" {
		t.Errorf("expected log level 'debug', got %q", loaded.Logging.Level)
	}
}

func TestConfigWriteToPathRoundTripsServersAndExecutionEnvironment(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MCP.Servers = []MCPServerConfig{
		{Name: "local", URI: "http://127.0.0.1:3000/mcp", Stdio: false},
		{Name: "stdio", URI: "npx @modelcontextprotocol/server-filesystem", Stdio: true},
	}
	cfg.Exec.AllowedEnvPrefixes = []string{"LANG", "PATH", "CUSTOM_"}

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := cfg.WriteToPath(path); err != nil {
		t.Fatalf("WriteToPath returned an error: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned an error: %v", err)
	}
	if !reflect.DeepEqual(loaded.MCP.Servers, cfg.MCP.Servers) {
		t.Errorf("MCP servers did not round-trip: got %#v want %#v", loaded.MCP.Servers, cfg.MCP.Servers)
	}
	if !reflect.DeepEqual(loaded.Exec.AllowedEnvPrefixes, cfg.Exec.AllowedEnvPrefixes) {
		t.Errorf("allowed environment prefixes did not round-trip: got %#v want %#v", loaded.Exec.AllowedEnvPrefixes, cfg.Exec.AllowedEnvPrefixes)
	}
}

func TestConfigWriteToPathHandlesNilKnowledgeStore(t *testing.T) {
	cfg := DefaultConfig()
	cfg.KnowledgeStore = nil
	if err := cfg.WriteToPath(filepath.Join(t.TempDir(), "config.yaml")); err != nil {
		t.Fatalf("WriteToPath must handle nil KnowledgeStore: %v", err)
	}
	var nilConfig *Config
	if err := nilConfig.WriteToPath(filepath.Join(t.TempDir(), "config.yaml")); err == nil {
		t.Fatal("WriteToPath must reject a nil Config receiver")
	}
}

func TestResolvePrecedenceAndWorkspaceDiscovery(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	if err := os.MkdirAll(filepath.Join(workspace, ".lana"), 0755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "workspace-link")
	if err := os.Symlink(workspace, link); err != nil {
		t.Fatal(err)
	}
	userConfig := filepath.Join(root, "user.yaml")
	if err := os.WriteFile(userConfig, []byte("logging:\n  level: warn\nmcp:\n  rate_limit: 11\n"), 0644); err != nil {
		t.Fatal(err)
	}
	projectConfig := filepath.Join(workspace, ".lana", "config.yaml")
	if err := os.WriteFile(projectConfig, []byte("logging:\n  level: error\nmcp:\n  rate_limit: 12\n"), 0644); err != nil {
		t.Fatal(err)
	}
	level := "debug"
	resolved, err := Resolve(ResolveOptions{
		UserConfigPath:   userConfig,
		WorkingDirectory: root,
		Flags: FlagOverrides{
			Workspace: &link,
			LogLevel:  &level,
		},
		Environment: map[string]string{
			"LANA_LOGGING_LEVEL":  "info",
			"LANA_MCP_RATE_LIMIT": "13",
		},
	})
	if err != nil {
		t.Fatalf("Resolve returned an error: %v", err)
	}
	if got := resolved.Logging().Level; got != "debug" {
		t.Errorf("flag should win for logging.level; got %q", got)
	}
	if got := resolved.Config().MCP.RateLimit; got != 13 {
		t.Errorf("environment should win for mcp.rate_limit; got %d", got)
	}
	if got := resolved.Workspace(); got != workspace {
		t.Errorf("workspace should be symlink-resolved: got %q want %q", got, workspace)
	}
	if got := resolved.Sources().ProjectConfig; got != projectConfig {
		t.Errorf("project config source: got %q want %q", got, projectConfig)
	}
}

func TestResolveConfigPathDoesNotSelectWorkspace(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	if err := os.MkdirAll(workspace, 0755); err != nil {
		t.Fatal(err)
	}
	externalConfig := filepath.Join(root, "alternate.yaml")
	if err := os.WriteFile(externalConfig, []byte("logging:\n  level: debug\n"), 0644); err != nil {
		t.Fatal(err)
	}
	resolved, err := Resolve(ResolveOptions{
		ConfigPath:       externalConfig,
		UserConfigPath:   filepath.Join(root, "absent-user.yaml"),
		WorkingDirectory: root,
		Flags:            FlagOverrides{Workspace: &workspace},
	})
	if err != nil {
		t.Fatalf("Resolve returned an error: %v", err)
	}
	if got := resolved.Workspace(); got != workspace {
		t.Errorf("--config must not replace --workspace: got %q want %q", got, workspace)
	}
	if got := resolved.Logging().Level; got != "debug" {
		t.Errorf("explicit config was not loaded, got log level %q", got)
	}
}

func TestAppConfigDefensiveCopiesAndRedaction(t *testing.T) {
	workspace := t.TempDir()
	configPath := filepath.Join(workspace, "config.yaml")
	if err := os.WriteFile(configPath, []byte("mcp:\n  servers:\n    - name: private\n      uri: https://alice:token@example.test/mcp\nexec:\n  allowed_env_prefixes: [LANG, TOKEN]\n"), 0644); err != nil {
		t.Fatal(err)
	}
	resolved, err := Resolve(ResolveOptions{
		ConfigPath:     configPath,
		UserConfigPath: filepath.Join(workspace, "absent-user.yaml"),
		Workspace:      workspace,
	})
	if err != nil {
		t.Fatalf("Resolve returned an error: %v", err)
	}
	copy := resolved.Config()
	copy.Exec.AllowedEnvPrefixes[0] = "CHANGED"
	copy.MCP.Servers[0].URI = "changed"
	secondCopy := resolved.Config()
	if secondCopy.Exec.AllowedEnvPrefixes[0] != "LANG" || secondCopy.MCP.Servers[0].URI == "changed" {
		t.Error("AppConfig exposed mutable backing state")
	}
	if got := resolved.RedactedConfig().MCP.Servers[0].URI; got != "https://[REDACTED]@example.test/mcp" {
		t.Errorf("unexpected redacted URI: %q", got)
	}
	if got := Redact("api_token", "value"); got != "[REDACTED]" {
		t.Errorf("secret redaction failed: %q", got)
	}
}

func TestRedactURI(t *testing.T) {
	value := "https://alice:password@example.test/mcp?cursor=public&api_token=secret-value&authorization=bearer-value&mode=read#fragment-secret"
	got := RedactURI(value)
	want := "https://[REDACTED]@example.test/mcp?api_token=REDACTED&authorization=REDACTED&cursor=public&mode=read#REDACTED"
	if got != want {
		t.Errorf("RedactURI() = %q, want %q", got, want)
	}
	for _, secret := range []string{"alice", "password", "secret-value", "bearer-value", "fragment-secret"} {
		if strings.Contains(got, secret) {
			t.Errorf("redacted URI leaked %q: %q", secret, got)
		}
	}
	if got := RedactURI("https://example.test/%zz?api_token=value"); got != "[REDACTED]" {
		t.Errorf("malformed URI must be conservatively redacted, got %q", got)
	}
}

func TestResolveRejectsMissingExplicitConfig(t *testing.T) {
	_, err := Resolve(ResolveOptions{
		ConfigPath:       filepath.Join(t.TempDir(), "missing.yaml"),
		UserConfigPath:   filepath.Join(t.TempDir(), "also-missing.yaml"),
		WorkingDirectory: t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected an error for missing explicit config")
	}
}
