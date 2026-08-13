package policy

import (
	"fmt"
	"path/filepath"
)

// ResolveWorkspacePath resolves a relative path within a workspace.
// Returns an error if the path tries to escape the workspace.
func ResolveWorkspacePath(workspace, relPath string) (string, error) {
	if workspace == "" {
		return "", fmt.Errorf("workspace not set")
	}

	// Normalize paths
	absWorkspace, err := filepath.Abs(workspace)
	if err != nil {
		return "", fmt.Errorf("workspace path: %w", err)
	}

	// Join the paths
	requestedPath := filepath.Join(absWorkspace, relPath)

	// Resolve symlinks and normalize
	resolvedPath, err := filepath.EvalSymlinks(requestedPath)
	if err != nil {
		// File might not exist yet, just normalize
		resolvedPath = filepath.Clean(requestedPath)
	}

	// Check if the resolved path is within the workspace
	rel, err := filepath.Rel(absWorkspace, resolvedPath)
	if err != nil || (len(rel) >= 2 && rel[0:2] == "..") {
		return "", fmt.Errorf("path escapes workspace: %s", relPath)
	}

	return resolvedPath, nil
}

// IsHighRisk checks if a command is high-risk.
func IsHighRisk(command string) bool {
	highRiskPatterns := []string{
		"rm -rf",
		"rm -fr",
		"git reset --hard",
		"git push --force",
		"git push -f",
		"sudo ",
		"su ",
		"chmod 777",
		"chown",
		":(){ :|:& };:", // Fork bomb
	}

	for _, pattern := range highRiskPatterns {
		if contains(command, pattern) {
			return true
		}
	}

	return false
}

// ContainsSensitivePattern checks if content contains likely secrets.
func ContainsSensitivePattern(content string) bool {
	patterns := []string{
		"sk-", // OpenAI keys
		"api_key",
		"password",
		"secret",
		"token",
		"Bearer ",
		"Authorization:",
		"-----BEGIN PRIVATE KEY-----",
		"aws_access_key",
		"ssh-rsa",
	}

	for _, pattern := range patterns {
		if contains(content, pattern) {
			return true
		}
	}

	return false
}

func contains(s, substr string) bool {
	// Simple contains implementation
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
