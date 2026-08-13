package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config represents the merged configuration from all sources.
type Config struct {
	Provider ProviderConfig
	Approval ApprovalConfig
	Session  SessionConfig
	MCP      MCPConfig
	Extra    map[string]interface{}
}

// ProviderConfig configures the AI provider.
type ProviderConfig struct {
	Name   string // "openai", "ollama"
	Model  string
	APIKey string
	Endpoint string
	CustomHeaders map[string]string
}

// ApprovalConfig configures the approval policy.
type ApprovalConfig struct {
	Mode string // "ask", "auto-edit", "full-auto"
}

// SessionConfig configures session persistence.
type SessionConfig struct {
	StorePath string // Directory for session storage
}

// MCPConfig configures MCP server integration.
type MCPConfig struct {
	Servers []MCPServerConfig
}

// MCPServerConfig represents a single MCP server configuration.
type MCPServerConfig struct {
	Name                string
	Transport           string            // "stdio" (default) or "http"
	Command             string            // stdio: command to run
	Args                []string          // stdio: command arguments
	Env                 map[string]string // stdio: environment variables
	URL                 string            // http: server URL
	Headers             map[string]string // http: custom headers
	Disabled            bool
	RiskLevel           string // "low", "medium" (default), "high"
	StartTimeoutSeconds int    // default 10
	CallTimeoutSeconds  int    // default 60
}

// Loader handles loading configuration from multiple sources.
type Loader struct {
	v *viper.Viper
}

// NewLoader creates a new configuration loader.
func NewLoader() *Loader {
	v := viper.New()
	v.SetConfigType("yaml")

	// Set defaults
	v.SetDefault("provider.name", "openai-compat")
	v.SetDefault("provider.model", "gpt-4")
	v.SetDefault("approval.mode", "ask")
	v.SetDefault("session.store_path", defaultSessionPath())
	v.SetDefault("mcp.servers", []interface{}{})

	return &Loader{v: v}
}

// Load loads configuration from files and environment.
func (l *Loader) Load(globalConfigPath string) (*Config, error) {
	// Load global config if it exists
	if globalConfigPath != "" && fileExists(globalConfigPath) {
		l.v.SetConfigFile(globalConfigPath)
		if err := l.v.ReadInConfig(); err != nil {
			return nil, err
		}
	}

	// Load project config if it exists
	if projectConfigPath := ".lana/config.yaml"; fileExists(projectConfigPath) {
		l.v.SetConfigFile(projectConfigPath)
		if err := l.v.MergeInConfig(); err != nil {
			return nil, err
		}
	}

	// Bind environment variables
	l.v.BindEnv("provider.name", "LANA_PROVIDER")
	l.v.BindEnv("provider.model", "LANA_MODEL")
	l.v.BindEnv("provider.api_key", "LANA_API_KEY")
	l.v.BindEnv("provider.endpoint", "LANA_ENDPOINT")
	l.v.BindEnv("approval.mode", "LANA_APPROVAL_MODE")

	return l.toConfig()
}

// Set sets a configuration value.
func (l *Loader) Set(key string, value interface{}) {
	l.v.Set(key, value)
}

// Get retrieves a configuration value.
func (l *Loader) Get(key string) interface{} {
	return l.v.Get(key)
}

// GetString retrieves a string configuration value.
func (l *Loader) GetString(key string) string {
	return l.v.GetString(key)
}

// WriteGlobal writes the configuration to the global config file.
func (l *Loader) WriteGlobal(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	return l.v.WriteConfigAs(path)
}

func (l *Loader) toConfig() (*Config, error) {
	// Unmarshal MCP servers configuration
	var mcpServers []MCPServerConfig
	if err := l.v.UnmarshalKey("mcp.servers", &mcpServers); err != nil {
		// If there's an error, just use empty list
		mcpServers = []MCPServerConfig{}
	}

	return &Config{
		Provider: ProviderConfig{
			Name:     l.v.GetString("provider.name"),
			Model:    l.v.GetString("provider.model"),
			APIKey:   l.v.GetString("provider.api_key"),
			Endpoint: l.v.GetString("provider.endpoint"),
		},
		Approval: ApprovalConfig{
			Mode: l.v.GetString("approval.mode"),
		},
		Session: SessionConfig{
			StorePath: l.v.GetString("session.store_path"),
		},
		MCP: MCPConfig{
			Servers: mcpServers,
		},
		Extra: l.v.AllSettings(),
	}, nil
}

func defaultSessionPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/.lana/sessions"
	}
	return filepath.Join(home, ".lana", "sessions")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
