// Package config provides configuration loading and validation.
package config

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// Precedence documents the order used by Resolve. Each source overrides the
// source to its left: built-in defaults < user config < project config <
// LANA_* environment variables < command-line flags.
const Precedence = "defaults < user config < project config < environment < flags"

// Config holds all Lana configuration.
type Config struct {
	Workspace      WorkspaceConfig       `mapstructure:"workspace" yaml:"workspace" json:"workspace"`
	Logging        LoggingConfig         `mapstructure:"logging" yaml:"logging" json:"logging"`
	MCP            MCPConfig             `mapstructure:"mcp" yaml:"mcp" json:"mcp"`
	Exec           ExecConfig            `mapstructure:"exec" yaml:"exec" json:"exec"`
	KnowledgeStore *KnowledgeStoreConfig `mapstructure:"knowledge_store" yaml:"knowledge_store" json:"knowledge_store"`
	PluginDir      string                `mapstructure:"plugin_dir" yaml:"plugin_dir" json:"plugin_dir"`
	SkillDir       string                `mapstructure:"skill_dir" yaml:"skill_dir" json:"skill_dir"`
	Dispatch       DispatchConfig        `mapstructure:"dispatch" yaml:"dispatch" json:"dispatch"`
}

// WorkspaceConfig holds workspace settings.
type WorkspaceConfig struct {
	Path        string `mapstructure:"path" yaml:"path" json:"path"`
	MaxFileSize string `mapstructure:"max_file_size" yaml:"max_file_size" json:"max_file_size"`
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	Level    string `mapstructure:"level" yaml:"level" json:"level"`
	Format   string `mapstructure:"format" yaml:"format" json:"format"`
	ToFile   bool   `mapstructure:"to_file" yaml:"to_file" json:"to_file"`
	FilePath string `mapstructure:"file_path" yaml:"file_path" json:"file_path"`
}

// MCPConfig holds MCP client settings.
type MCPConfig struct {
	RateLimit int               `mapstructure:"rate_limit" yaml:"rate_limit" json:"rate_limit"`
	Timeout   time.Duration     `mapstructure:"timeout" yaml:"timeout" json:"timeout"`
	Servers   []MCPServerConfig `mapstructure:"servers" yaml:"servers" json:"servers"`
}

// MCPServerConfig holds configuration for a single MCP server.
type MCPServerConfig struct {
	Name    string   `mapstructure:"name" yaml:"name" json:"name"`
	URI     string   `mapstructure:"uri" yaml:"uri" json:"uri"`
	Stdio   bool     `mapstructure:"stdio" yaml:"stdio" json:"stdio"`
	Command string   `mapstructure:"command" yaml:"command" json:"command"`
	Args    []string `mapstructure:"args" yaml:"args" json:"args"`
}

// ExecConfig holds execution settings.
type ExecConfig struct {
	Sandbox            string        `mapstructure:"sandbox" yaml:"sandbox" json:"sandbox"`
	Timeout            time.Duration `mapstructure:"timeout" yaml:"timeout" json:"timeout"`
	AllowedEnvPrefixes []string      `mapstructure:"allowed_env_prefixes" yaml:"allowed_env_prefixes" json:"allowed_env_prefixes"`
}

// KnowledgeStoreConfig holds knowledge store settings.
type KnowledgeStoreConfig struct {
	Path string `mapstructure:"path" yaml:"path" json:"path"`
}

// DispatchConfig holds dispatch settings.
type DispatchConfig struct {
	MaxParallel    int           `mapstructure:"max_parallel" yaml:"max_parallel" json:"max_parallel"`
	DefaultTimeout time.Duration `mapstructure:"default_timeout" yaml:"default_timeout" json:"default_timeout"`
}

// FlagOverrides contains the small set of global CLI settings that may
// override file and environment configuration. A nil field means that its flag
// was not supplied, which is importantly different from an empty value.
type FlagOverrides struct {
	Workspace *string
	LogLevel  *string
	LogFormat *string
}

// ResolveOptions describes configuration discovery. ConfigPath is the path
// selected by --config; it replaces the automatically discovered *project*
// config and never changes the workspace. Workspace is the value selected by
// --workspace and is resolved independently of ConfigPath.
//
// Environment is intended for deterministic callers and tests. When nil, the
// process environment is used.
type ResolveOptions struct {
	ConfigPath       string
	UserConfigPath   string
	Workspace        string
	WorkingDirectory string
	Environment      map[string]string
	Flags            FlagOverrides
}

// Sources identifies the files that contributed to a resolved configuration.
// Empty fields mean that the corresponding optional source did not exist.
type Sources struct {
	UserConfig    string
	ProjectConfig string
}

// AppConfig is the immutable, fully resolved configuration used at runtime.
// Its accessors return copies so callers cannot mutate the process-wide
// configuration accidentally.
type AppConfig struct {
	config    Config
	workspace string
	sources   Sources
}

// Config returns a defensive copy of the fully resolved configuration.
func (c *AppConfig) Config() Config { return cloneConfig(c.config) }

// Workspace returns the canonical, symlink-resolved workspace path.
func (c *AppConfig) Workspace() string { return c.workspace }

// Sources returns a copy of the configuration source metadata.
func (c *AppConfig) Sources() Sources { return c.sources }

// Logging returns the resolved logging settings.
func (c *AppConfig) Logging() LoggingConfig { return c.config.Logging }

// RedactedConfig returns a safe-to-log copy of the configuration. Secret-like
// values and URI userinfo are removed at this boundary; runtime code should
// use this method when emitting configuration diagnostics.
func (c *AppConfig) RedactedConfig() Config {
	return c.config.Redacted()
}

// Redacted returns a safe-to-display copy of c. It is intended for command
// output and diagnostics; callers that need to connect to a configured server
// must continue using the original configuration.
func (c Config) Redacted() Config {
	result := cloneConfig(c)
	result.MCP.Servers = RedactMCPServers(result.MCP.Servers)
	return result
}

// RedactMCPServer returns a safe-to-display copy of server.
func RedactMCPServer(server MCPServerConfig) MCPServerConfig {
	server.URI = RedactURI(server.URI)
	return server
}

// RedactMCPServers returns safe-to-display copies of configured servers.
func RedactMCPServers(servers []MCPServerConfig) []MCPServerConfig {
	result := append([]MCPServerConfig(nil), servers...)
	for i := range result {
		result[i] = RedactMCPServer(result[i])
	}
	return result
}

// MCPServer returns a copy of a configured MCP server by name. It is a
// convenience for consumers of the resolved configuration and preserves the
// AppConfig immutability boundary.
func (c *AppConfig) MCPServer(name string) (MCPServerConfig, bool) {
	for _, server := range c.config.MCP.Servers {
		if strings.EqualFold(server.Name, name) {
			return server, true
		}
	}
	return MCPServerConfig{}, false
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Workspace: WorkspaceConfig{
			Path:        ".",
			MaxFileSize: "1mb",
		},
		Logging: LoggingConfig{
			Level:    "info",
			Format:   "text",
			ToFile:   false,
			FilePath: "",
		},
		MCP: MCPConfig{
			RateLimit: 10,
			Timeout:   30 * time.Second,
		},
		Exec: ExecConfig{
			Sandbox:            "workspace-write",
			Timeout:            60 * time.Second,
			AllowedEnvPrefixes: []string{"LANG", "PATH", "HOME"},
		},
		KnowledgeStore: &KnowledgeStoreConfig{
			Path: "~/.agents/knowledge-store",
		},
		PluginDir: "~/.local/share/lana/plugins",
		SkillDir:  "~/.local/share/lana/skills",
		Dispatch: DispatchConfig{
			MaxParallel:    4,
			DefaultTimeout: 10 * time.Minute,
		},
	}
}

// DefaultUserConfigPath returns Lana's per-user configuration path. It is a
// separate discovery concern from the workspace and is deliberately not
// affected by --workspace.
func DefaultUserConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate user config directory: %w", err)
	}
	return filepath.Join(home, ".config", "lana", "config.yaml"), nil
}

// DefaultProjectConfigPath returns the project configuration location for an
// already canonical workspace.
func DefaultProjectConfigPath(workspace string) string {
	return filepath.Join(workspace, ".lana", "config.yaml")
}

// ResolveWorkspace returns an absolute, symlink-resolved directory path. A
// single resolver avoids the common mistake of validating a lexical path but
// later operating through a symlink that points outside the intended tree.
func ResolveWorkspace(path string) (string, error) {
	return resolveWorkspace(path, "")
}

func resolveWorkspace(path, base string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("workspace path must not be empty")
	}
	path = expandPath(path)
	if !filepath.IsAbs(path) && base != "" {
		path = filepath.Join(base, path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("make workspace path absolute: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("resolve workspace %q: %w", path, err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("stat workspace %q: %w", resolved, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("workspace %q is not a directory", resolved)
	}
	return resolved, nil
}

// Resolve loads and freezes application configuration. The precedence is
// defaults < user config < project config < LANA_* environment < flags. The
// --config source selects a project configuration file only; --workspace is
// independently resolved and controls automatic project-file discovery.
func Resolve(opts ResolveOptions) (*AppConfig, error) {
	workingDir := opts.WorkingDirectory
	if workingDir == "" {
		var err error
		workingDir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
	}
	workingDir, err := resolveWorkspace(workingDir, "")
	if err != nil {
		return nil, fmt.Errorf("resolve working directory: %w", err)
	}

	values := defaultValues()
	sources := Sources{}
	userPath := opts.UserConfigPath
	if userPath == "" {
		userPath, err = DefaultUserConfigPath()
		if err != nil {
			return nil, err
		}
	}
	if loaded, found, loadErr := readOptionalConfig(userPath, workingDir); loadErr != nil {
		return nil, loadErr
	} else if found {
		mergeValues(values, loaded)
		sources.UserConfig = userPath
	}

	// Determine the project root before reading its configuration. This lets a
	// user config or --workspace select a project without allowing --config to
	// silently change where commands operate.
	bootstrapWorkspace := nestedString(values, "workspace", "path")
	if envValue, ok := lookupEnv(opts.Environment, "LANA_WORKSPACE_PATH"); ok {
		bootstrapWorkspace = envValue
	}
	if opts.Workspace != "" {
		bootstrapWorkspace = opts.Workspace
	}
	if opts.Flags.Workspace != nil {
		bootstrapWorkspace = *opts.Flags.Workspace
	}
	workspace, err := resolveWorkspace(bootstrapWorkspace, workingDir)
	if err != nil {
		return nil, err
	}

	projectPath := opts.ConfigPath
	if projectPath == "" {
		projectPath = DefaultProjectConfigPath(workspace)
		if loaded, found, loadErr := readOptionalConfig(projectPath, workingDir); loadErr != nil {
			return nil, loadErr
		} else if found {
			mergeValues(values, loaded)
			sources.ProjectConfig = projectPath
		}
	} else {
		projectPath = expandPath(projectPath)
		if !filepath.IsAbs(projectPath) {
			projectPath = filepath.Join(workingDir, projectPath)
		}
		loaded, found, loadErr := readOptionalConfig(projectPath, workingDir)
		if loadErr != nil {
			return nil, loadErr
		}
		if !found {
			return nil, fmt.Errorf("read explicit config %q: file does not exist", projectPath)
		}
		mergeValues(values, loaded)
		sources.ProjectConfig = projectPath
	}

	applyEnvironment(values, opts.Environment)
	if opts.Workspace != "" {
		setValue(values, "workspace.path", opts.Workspace)
	}
	applyFlags(values, opts.Flags)

	cfg, err := valuesConfig(values)
	if err != nil {
		return nil, err
	}
	finalWorkspace, err := resolveWorkspace(cfg.Workspace.Path, workingDir)
	if err != nil {
		return nil, err
	}
	cfg.Workspace.Path = finalWorkspace
	expandConfigPaths(&cfg)
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &AppConfig{config: cloneConfig(cfg), workspace: finalWorkspace, sources: sources}, nil
}

func defaultValues() map[string]any {
	return map[string]any{
		"workspace":       map[string]any{"path": ".", "max_file_size": "1mb"},
		"logging":         map[string]any{"level": "info", "format": "text", "to_file": false, "file_path": ""},
		"mcp":             map[string]any{"rate_limit": 10, "timeout": "30s", "servers": []any{}},
		"exec":            map[string]any{"sandbox": "workspace-write", "timeout": "60s", "allowed_env_prefixes": []any{"LANG", "PATH", "HOME"}},
		"knowledge_store": map[string]any{"path": "~/.agents/knowledge-store"},
		"plugin_dir":      "~/.local/share/lana/plugins",
		"skill_dir":       "~/.local/share/lana/skills",
		"dispatch":        map[string]any{"max_parallel": 4, "default_timeout": "10m"},
	}
}

func readOptionalConfig(path, base string) (map[string]any, bool, error) {
	path = expandPath(path)
	if !filepath.IsAbs(path) {
		path = filepath.Join(base, path)
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read config %q: %w", path, err)
	}
	values := map[string]any{}
	if err := yaml.Unmarshal(data, &values); err != nil {
		return nil, false, fmt.Errorf("parse config %q: %w", path, err)
	}
	return values, true, nil
}

func mergeValues(dst, src map[string]any) {
	for key, sourceValue := range src {
		if sourceMap, ok := sourceValue.(map[string]any); ok {
			if targetMap, ok := dst[key].(map[string]any); ok {
				mergeValues(targetMap, sourceMap)
				continue
			}
		}
		dst[key] = sourceValue
	}
}

func nestedString(values map[string]any, parent, child string) string {
	if nested, ok := values[parent].(map[string]any); ok {
		if value, ok := nested[child].(string); ok {
			return value
		}
	}
	return ""
}

func setValue(values map[string]any, dotted string, value any) {
	parts := strings.Split(dotted, ".")
	current := values
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]any)
		if !ok {
			next = map[string]any{}
			current[part] = next
		}
		current = next
	}
	current[parts[len(parts)-1]] = value
}

func lookupEnv(environment map[string]string, key string) (string, bool) {
	if environment != nil {
		value, ok := environment[key]
		return value, ok
	}
	return os.LookupEnv(key)
}

func applyEnvironment(values map[string]any, environment map[string]string) {
	for _, binding := range []struct {
		env string
		key string
	}{
		{"LANA_WORKSPACE_PATH", "workspace.path"},
		{"LANA_WORKSPACE_MAX_FILE_SIZE", "workspace.max_file_size"},
		{"LANA_LOGGING_LEVEL", "logging.level"},
		{"LANA_LOGGING_FORMAT", "logging.format"},
		{"LANA_LOGGING_TO_FILE", "logging.to_file"},
		{"LANA_LOGGING_FILE_PATH", "logging.file_path"},
		{"LANA_MCP_RATE_LIMIT", "mcp.rate_limit"},
		{"LANA_MCP_TIMEOUT", "mcp.timeout"},
		{"LANA_EXEC_SANDBOX", "exec.sandbox"},
		{"LANA_EXEC_TIMEOUT", "exec.timeout"},
		{"LANA_EXEC_ALLOWED_ENV_PREFIXES", "exec.allowed_env_prefixes"},
		{"LANA_KNOWLEDGE_STORE_PATH", "knowledge_store.path"},
		{"LANA_PLUGIN_DIR", "plugin_dir"},
		{"LANA_SKILL_DIR", "skill_dir"},
		{"LANA_DISPATCH_MAX_PARALLEL", "dispatch.max_parallel"},
		{"LANA_DISPATCH_DEFAULT_TIMEOUT", "dispatch.default_timeout"},
	} {
		if value, ok := lookupEnv(environment, binding.env); ok {
			setValue(values, binding.key, environmentValue(binding.key, value))
		}
	}
}

func environmentValue(key, value string) any {
	switch key {
	case "logging.to_file":
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	case "mcp.rate_limit", "dispatch.max_parallel":
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	case "exec.allowed_env_prefixes":
		if value == "" {
			return []any{}
		}
		parts := strings.Split(value, ",")
		result := make([]any, 0, len(parts))
		for _, part := range parts {
			result = append(result, strings.TrimSpace(part))
		}
		return result
	}
	return value
}

func applyFlags(values map[string]any, flags FlagOverrides) {
	if flags.Workspace != nil {
		setValue(values, "workspace.path", *flags.Workspace)
	}
	if flags.LogLevel != nil {
		setValue(values, "logging.level", *flags.LogLevel)
	}
	if flags.LogFormat != nil {
		setValue(values, "logging.format", *flags.LogFormat)
	}
}

func valuesConfig(values map[string]any) (Config, error) {
	data, err := yaml.Marshal(values)
	if err != nil {
		return Config{}, fmt.Errorf("serialize resolved config: %w", err)
	}
	var cfg Config
	if err := yaml.NewDecoder(bytes.NewReader(data)).Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse resolved config: %w", err)
	}
	return cfg, nil
}

func expandConfigPaths(cfg *Config) {
	if cfg.KnowledgeStore != nil {
		cfg.KnowledgeStore.Path = expandPath(cfg.KnowledgeStore.Path)
	}
	cfg.PluginDir = expandPath(cfg.PluginDir)
	cfg.SkillDir = expandPath(cfg.SkillDir)
	cfg.Logging.FilePath = expandPath(cfg.Logging.FilePath)
}

func cloneConfig(cfg Config) Config {
	result := cfg
	result.MCP.Servers = append([]MCPServerConfig(nil), cfg.MCP.Servers...)
	result.Exec.AllowedEnvPrefixes = append([]string(nil), cfg.Exec.AllowedEnvPrefixes...)
	if cfg.KnowledgeStore != nil {
		store := *cfg.KnowledgeStore
		result.KnowledgeStore = &store
	}
	return result
}

// IsSensitiveKey reports whether a configuration or structured-log key is
// likely to carry a secret. It intentionally follows the project security
// requirement's KEY, TOKEN, SECRET, and PASSWORD patterns.
func IsSensitiveKey(key string) bool {
	upper := strings.ToUpper(key)
	return strings.Contains(upper, "KEY") || strings.Contains(upper, "TOKEN") ||
		strings.Contains(upper, "SECRET") || strings.Contains(upper, "PASSWORD") ||
		strings.Contains(upper, "CREDENTIAL") || strings.Contains(upper, "AUTH") ||
		strings.Contains(upper, "BEARER") || strings.Contains(upper, "SESSION") ||
		strings.Contains(upper, "COOKIE") || strings.Contains(upper, "PRIVATE")
}

// Redact returns a safe representation of value when key is secret-like.
func Redact(key, value string) string {
	if IsSensitiveKey(key) {
		return "[REDACTED]"
	}
	return value
}

// RedactURI removes credentials and secret-bearing URL components. URI
// fragments are always removed because fragments frequently carry opaque
// credentials and cannot be safely classified by key.
func RedactURI(value string) string {
	parsed, err := url.Parse(value)
	if err != nil {
		// A malformed URI cannot be safely inspected for embedded credentials.
		return "[REDACTED]"
	}

	hadUserinfo := parsed.User != nil
	if hadUserinfo {
		// Use an unescaped marker first, then restore the familiar bracketed
		// rendering after URL serialization.
		parsed.User = url.User("REDACTED")
	}
	query := parsed.Query()
	for key := range query {
		if IsSensitiveKey(key) {
			query[key] = []string{"REDACTED"}
		}
	}
	parsed.RawQuery = query.Encode()
	if parsed.Fragment != "" || parsed.RawFragment != "" {
		parsed.Fragment = "REDACTED"
		parsed.RawFragment = ""
	}
	result := parsed.String()
	if hadUserinfo {
		result = strings.Replace(result, "REDACTED@", "[REDACTED]@", 1)
	}
	return result
}

// Load reads configuration from the specified file path.
func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	// Set defaults
	v.SetDefault("workspace.path", ".")
	v.SetDefault("workspace.max_file_size", "1mb")
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "text")
	v.SetDefault("logging.to_file", false)
	v.SetDefault("mcp.rate_limit", 10)
	v.SetDefault("mcp.timeout", "30s")
	v.SetDefault("exec.sandbox", "workspace-write")
	v.SetDefault("exec.timeout", "60s")
	v.SetDefault("knowledge_store.path", "~/.agents/knowledge-store")
	v.SetDefault("plugin_dir", "~/.local/share/lana/plugins")
	v.SetDefault("skill_dir", "~/.local/share/lana/skills")
	v.SetDefault("dispatch.max_parallel", 4)
	v.SetDefault("dispatch.default_timeout", "10m")

	cfg := DefaultConfig()

	if path != "" {
		if err := v.ReadInConfig(); err != nil {
			// Config file not found is not an error; use defaults
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				return nil, fmt.Errorf("read config file %q: %w", path, err)
			}
		}
	}

	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Expand paths
	cfg.Workspace.Path = expandPath(cfg.Workspace.Path)
	if cfg.KnowledgeStore != nil {
		cfg.KnowledgeStore.Path = expandPath(cfg.KnowledgeStore.Path)
	}
	cfg.PluginDir = expandPath(cfg.PluginDir)
	cfg.SkillDir = expandPath(cfg.SkillDir)

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// expandPath expands ~ and environment variables in a path.
func expandPath(p string) string {
	if len(p) == 0 {
		return p
	}
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			if p == "~" {
				p = home
			} else {
				p = filepath.Join(home, p[2:])
			}
		}
	}
	return os.ExpandEnv(p)
}

// Validate checks that configuration values are valid.
func (c *Config) Validate() error {
	if c.Workspace.Path == "" {
		return fmt.Errorf("workspace.path must not be empty")
	}
	if c.MCP.RateLimit < 1 {
		return fmt.Errorf("mcp.rate_limit must be >= 1")
	}
	switch c.Exec.Sandbox {
	case "unrestricted", "workspace-write", "workspace-read-only":
		// valid
	default:
		return fmt.Errorf("exec.sandbox: invalid value %q (must be unrestricted, workspace-write, or workspace-read-only)", c.Exec.Sandbox)
	}
	switch c.Logging.Format {
	case "text", "json":
		// valid
	default:
		return fmt.Errorf("logging.format: invalid value %q (must be text or json)", c.Logging.Format)
	}
	switch c.Logging.Level {
	case "debug", "info", "warn", "error":
		// valid
	default:
		return fmt.Errorf("logging.level: invalid value %q (must be debug, info, warn, or error)", c.Logging.Level)
	}
	return nil
}

// GetMCPServer returns the MCP server config by name.
func (c *Config) GetMCPServer(name string) *MCPServerConfig {
	for _, s := range c.MCP.Servers {
		if strings.EqualFold(s.Name, name) {
			return &s
		}
	}
	return nil
}

// WriteToPath writes the config to a YAML file at the given path.
func (c *Config) WriteToPath(path string) error {
	if c == nil {
		return fmt.Errorf("write config: config must not be nil")
	}
	// Expand path
	path = expandPath(path)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	v := viper.New()
	v.Set("workspace.path", c.Workspace.Path)
	v.Set("workspace.max_file_size", c.Workspace.MaxFileSize)
	v.Set("logging.level", c.Logging.Level)
	v.Set("logging.format", c.Logging.Format)
	v.Set("logging.to_file", c.Logging.ToFile)
	v.Set("logging.file_path", c.Logging.FilePath)
	v.Set("mcp.rate_limit", c.MCP.RateLimit)
	v.Set("mcp.timeout", c.MCP.Timeout.String())
	v.Set("mcp.servers", c.MCP.Servers)
	v.Set("exec.sandbox", c.Exec.Sandbox)
	v.Set("exec.timeout", c.Exec.Timeout.String())
	v.Set("exec.allowed_env_prefixes", c.Exec.AllowedEnvPrefixes)
	if c.KnowledgeStore != nil {
		v.Set("knowledge_store.path", c.KnowledgeStore.Path)
	}
	v.Set("plugin_dir", c.PluginDir)
	v.Set("skill_dir", c.SkillDir)
	v.Set("dispatch.max_parallel", c.Dispatch.MaxParallel)
	v.Set("dispatch.default_timeout", c.Dispatch.DefaultTimeout.String())

	if err := v.WriteConfigAs(path); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}
