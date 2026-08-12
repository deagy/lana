// Package config provides configuration management subcommands.
package config

import (
	"fmt"
	"time"
)

// GetNestedValue retrieves a configuration value by dot-separated key path.
// Supported keys: workspace.path, workspace.max_file_size, logging.level,
// logging.format, logging.to_file, logging.file_path, mcp.rate_limit,
// mcp.timeout, exec.sandbox, exec.timeout, knowledge_store.path,
// plugin_dir, skill_dir, dispatch.max_parallel, dispatch.default_timeout.
func GetNestedValue(cfg *Config, key string) (string, error) {
	switch key {
	case "workspace.path":
		return cfg.Workspace.Path, nil
	case "workspace.max_file_size":
		return cfg.Workspace.MaxFileSize, nil
	case "logging.level":
		return cfg.Logging.Level, nil
	case "logging.format":
		return cfg.Logging.Format, nil
	case "logging.to_file":
		if cfg.Logging.ToFile {
			return "true", nil
		}
		return "false", nil
	case "logging.file_path":
		return cfg.Logging.FilePath, nil
	case "mcp.rate_limit":
		return fmt.Sprintf("%d", cfg.MCP.RateLimit), nil
	case "mcp.timeout":
		return cfg.MCP.Timeout.String(), nil
	case "exec.sandbox":
		return cfg.Exec.Sandbox, nil
	case "exec.timeout":
		return cfg.Exec.Timeout.String(), nil
	case "knowledge_store.path":
		if cfg.KnowledgeStore != nil {
			return cfg.KnowledgeStore.Path, nil
		}
		return "", nil
	case "plugin_dir":
		return cfg.PluginDir, nil
	case "skill_dir":
		return cfg.SkillDir, nil
	case "dispatch.max_parallel":
		return fmt.Sprintf("%d", cfg.Dispatch.MaxParallel), nil
	case "dispatch.default_timeout":
		return cfg.Dispatch.DefaultTimeout.String(), nil
	default:
		return "", fmt.Errorf("unknown config key: %s", key)
	}
}

// SetNestedValue sets a configuration value by dot-separated key path.
func SetNestedValue(cfg *Config, key, value string) error {
	switch key {
	case "workspace.path":
		cfg.Workspace.Path = value
	case "workspace.max_file_size":
		cfg.Workspace.MaxFileSize = value
	case "logging.level":
		cfg.Logging.Level = value
	case "logging.format":
		cfg.Logging.Format = value
	case "logging.to_file":
		cfg.Logging.ToFile = value == "true"
	case "logging.file_path":
		cfg.Logging.FilePath = value
	case "mcp.rate_limit":
		var n int
		if _, err := fmt.Sscanf(value, "%d", &n); err != nil {
			return fmt.Errorf("invalid rate limit value: %s", value)
		}
		cfg.MCP.RateLimit = n
	case "mcp.timeout":
		d, err := parseDuration(value)
		if err != nil {
			return err
		}
		cfg.MCP.Timeout = d
	case "exec.sandbox":
		cfg.Exec.Sandbox = value
	case "exec.timeout":
		d, err := parseDuration(value)
		if err != nil {
			return err
		}
		cfg.Exec.Timeout = d
	case "knowledge_store.path":
		if cfg.KnowledgeStore == nil {
			cfg.KnowledgeStore = &KnowledgeStoreConfig{}
		}
		cfg.KnowledgeStore.Path = value
	case "plugin_dir":
		cfg.PluginDir = value
	case "skill_dir":
		cfg.SkillDir = value
	case "dispatch.max_parallel":
		var n int
		if _, err := fmt.Sscanf(value, "%d", &n); err != nil {
			return fmt.Errorf("invalid max parallel value: %s", value)
		}
		cfg.Dispatch.MaxParallel = n
	case "dispatch.default_timeout":
		d, err := parseDuration(value)
		if err != nil {
			return err
		}
		cfg.Dispatch.DefaultTimeout = d
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}
	return nil
}

func parseDuration(s string) (time.Duration, error) {
	if len(s) == 0 {
		return 0, fmt.Errorf("empty duration")
	}
	multiplier := time.Second
	switch s[len(s)-1] {
	case 's':
		s = s[:len(s)-1]
	case 'm':
		s = s[:len(s)-1]
		multiplier = time.Minute
	case 'h':
		s = s[:len(s)-1]
		multiplier = time.Hour
	case 'd':
		s = s[:len(s)-1]
		multiplier = 24 * time.Hour
	}
	var n int64
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
		return 0, fmt.Errorf("invalid duration: %s", s+string(s[len(s)-1]))
	}
	return time.Duration(n) * multiplier, nil
}
