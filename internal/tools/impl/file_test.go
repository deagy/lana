package impl

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/deagy/lana/internal/tools"
)

func TestFileToolRead(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := "Hello, World!"
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Create tool
	ft := NewFileTool(tmpDir)
	tool := ft.Read()

	// Test read
	input := json.RawMessage(`{"path": "test.txt"}`)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != testContent {
		t.Errorf("content mismatch: got %q, want %q", result, testContent)
	}
}

func TestFileToolReadNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	ft := NewFileTool(tmpDir)
	tool := ft.Read()

	input := json.RawMessage(`{"path": "nonexistent.txt"}`)
	_, err := tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestFileToolReadEscapeAttempt(t *testing.T) {
	tmpDir := t.TempDir()
	ft := NewFileTool(tmpDir)
	tool := ft.Read()

	// Try to read outside workspace
	input := json.RawMessage(`{"path": "../../../etc/passwd"}`)
	_, err := tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for path traversal attempt")
	}
}

func TestFileToolWrite(t *testing.T) {
	tmpDir := t.TempDir()
	ft := NewFileTool(tmpDir)
	tool := ft.Write()

	// Write file
	content := "Test content"
	input := json.RawMessage(`{"path": "test.txt", "content": "Test content"}`)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify result
	if result == "" {
		t.Error("expected non-empty result")
	}

	// Verify file exists
	written, err := os.ReadFile(filepath.Join(tmpDir, "test.txt"))
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}

	if string(written) != content {
		t.Errorf("content mismatch: got %q, want %q", string(written), content)
	}
}

func TestFileToolWriteCreateDir(t *testing.T) {
	tmpDir := t.TempDir()
	ft := NewFileTool(tmpDir)
	tool := ft.Write()

	// Write to nested path
	input := json.RawMessage(`{"path": "subdir/nested/test.txt", "content": "nested"}`)
	_, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify nested file exists
	_, err = os.ReadFile(filepath.Join(tmpDir, "subdir", "nested", "test.txt"))
	if err != nil {
		t.Fatalf("nested file not found: %v", err)
	}
}

func TestFileToolList(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte(""), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte(""), 0644)
	os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "subdir", "file3.txt"), []byte(""), 0644)

	ft := NewFileTool(tmpDir)
	tool := ft.List()

	// List non-recursive
	input := json.RawMessage(`{"path": ".", "recursive": false}`)
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == "" {
		t.Error("expected non-empty listing")
	}

	// List recursive
	input = json.RawMessage(`{"path": ".", "recursive": true}`)
	result, err = tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == "" {
		t.Error("expected non-empty listing")
	}
}

func TestFileToolSchemas(t *testing.T) {
	tmpDir := t.TempDir()
	ft := NewFileTool(tmpDir)

	// Check read schema
	readTool := ft.Read()
	if readTool.Name() != "read_file" {
		t.Errorf("tool name mismatch: got %s, want read_file", readTool.Name())
	}

	if readTool.InputSchema() == nil || len(readTool.InputSchema()) == 0 {
		t.Error("tool schema is empty")
	}

	// Check write schema
	writeTool := ft.Write()
	if writeTool.Name() != "write_file" {
		t.Errorf("tool name mismatch: got %s, want write_file", writeTool.Name())
	}

	// Check list schema
	listTool := ft.List()
	if listTool.Name() != "list_files" {
		t.Errorf("tool name mismatch: got %s, want list_files", listTool.Name())
	}
}

func TestFileToolRiskLevels(t *testing.T) {
	tmpDir := t.TempDir()
	ft := NewFileTool(tmpDir)

	readDef := ft.Read().(*tools.Definition)
	if readDef.RiskLevel != tools.RiskLevelLow {
		t.Errorf("read_file risk should be Low, got %v", readDef.RiskLevel)
	}

	writeDef := ft.Write().(*tools.Definition)
	if writeDef.RiskLevel != tools.RiskLevelMedium {
		t.Errorf("write_file risk should be Medium, got %v", writeDef.RiskLevel)
	}

	listDef := ft.List().(*tools.Definition)
	if listDef.RiskLevel != tools.RiskLevelLow {
		t.Errorf("list_files risk should be Low, got %v", listDef.RiskLevel)
	}
}
