package policy

import (
	"path/filepath"
	"testing"
)

func TestResolveWorkspacePath(t *testing.T) {
	tests := []struct {
		name      string
		workspace string
		relPath   string
		wantErr   bool
		wantPath  string
	}{
		{
			name:      "simple file",
			workspace: "/tmp/workspace",
			relPath:   "file.txt",
			wantErr:   false,
			wantPath:  "/tmp/workspace/file.txt",
		},
		{
			name:      "nested path",
			workspace: "/tmp/workspace",
			relPath:   "subdir/file.txt",
			wantErr:   false,
			wantPath:  "/tmp/workspace/subdir/file.txt",
		},
		{
			name:      "directory traversal attempt",
			workspace: "/tmp/workspace",
			relPath:   "../../../etc/passwd",
			wantErr:   true,
		},
		{
			name:      "empty relative path",
			workspace: "/tmp/workspace",
			relPath:   "",
			wantErr:   false,
			wantPath:  "/tmp/workspace",
		},
		{
			name:      "dot path",
			workspace: "/tmp/workspace",
			relPath:   ".",
			wantErr:   false,
			wantPath:  "/tmp/workspace",
		},
		{
			name:      "current dir file",
			workspace: "/tmp/workspace",
			relPath:   "./file.txt",
			wantErr:   false,
			wantPath:  "/tmp/workspace/file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveWorkspacePath(tt.workspace, tt.relPath)

			if (err != nil) != tt.wantErr {
				t.Errorf("error mismatch: got %v, want error=%v", err, tt.wantErr)
			}

			if !tt.wantErr && filepath.Clean(got) != filepath.Clean(tt.wantPath) {
				t.Errorf("path mismatch: got %q, want %q", got, tt.wantPath)
			}
		})
	}
}

func TestIsHighRisk(t *testing.T) {
	tests := []struct {
		cmd  string
		want bool
	}{
		{"echo hello", false},
		{"rm -rf /", true},
		{"rm -fr /home", true},
		{"git reset --hard", true},
		{"git push --force", true},
		{"git push -f", true},
		{"sudo ls", true},
		{"su root", true},
		{"chmod 777 /etc", true},
		{":(){ :|:& };:", true},
		{"chown root /", true},
		{"git commit -m test", false},
		{"grep -r pattern", false},
		{"ls -la", false},
		{"cat file.txt", false},
		{"git diff", false},
	}

	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			got := IsHighRisk(tt.cmd)
			if got != tt.want {
				t.Errorf("IsHighRisk(%q) = %v, want %v", tt.cmd, got, tt.want)
			}
		})
	}
}

func TestContainsSensitivePattern(t *testing.T) {
	tests := []struct {
		text string
		want bool
	}{
		{"normal text", false},
		{"api_key: sk-12345", true},
		{"password: secret123", true},
		{"Bearer eyJhbGc", true},
		{"-----BEGIN PRIVATE KEY-----", true},
		{"aws_access_key=AKIA12345", true},
		{"ssh-rsa AAAA...", true},
		{"user@example.com", false},
		{"github.com/user/repo", false},
		{"https://api.example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			got := ContainsSensitivePattern(tt.text)
			if got != tt.want {
				t.Errorf("ContainsSensitivePattern(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

func TestResolveWorkspacePathEmptyWorkspace(t *testing.T) {
	_, err := ResolveWorkspacePath("", "file.txt")
	if err == nil {
		t.Error("expected error for empty workspace")
	}
}
