package output

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestJSONFormatter(t *testing.T) {
	formatter := &JSONFormatter{}
	result := Result{
		Status:    "message",
		Message:   "Hello",
		Timestamp: 1000,
	}

	output, err := formatter.FormatResult(result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed Result
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("output not valid JSON: %v", err)
	}

	if parsed.Message != "Hello" {
		t.Errorf("message mismatch: got %s, want Hello", parsed.Message)
	}
}

func TestJSONFormatterToolResult(t *testing.T) {
	formatter := &JSONFormatter{}
	result := Result{
		Status:     "tool_result",
		ToolName:   "read_file",
		ToolOutput: "file contents",
		Approved:   true,
		Timestamp:  time.Now().Unix(),
	}

	output, err := formatter.FormatResult(result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed Result
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("output not valid JSON: %v", err)
	}

	if parsed.ToolName != "read_file" {
		t.Errorf("tool name mismatch: got %s, want read_file", parsed.ToolName)
	}

	if !parsed.Approved {
		t.Error("approved flag should be true")
	}
}

func TestPlainFormatterMessage(t *testing.T) {
	formatter := &PlainFormatter{}
	result := Result{
		Status:    "message",
		Message:   "Test message",
		Timestamp: 1000,
	}

	output, err := formatter.FormatResult(result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output != "Test message" {
		t.Errorf("output mismatch: got %q, want %q", output, "Test message")
	}
}

func TestPlainFormatterToolStart(t *testing.T) {
	formatter := &PlainFormatter{}
	result := Result{
		Status:   "tool_start",
		ToolName: "read_file",
		ToolInput: map[string]interface{}{
			"path": "main.go",
		},
		Timestamp: 1000,
	}

	output, err := formatter.FormatResult(result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, "read_file") {
		t.Errorf("output should contain tool name: %s", output)
	}

	if !strings.Contains(output, "Input") {
		t.Errorf("output should contain 'Input': %s", output)
	}
}

func TestPlainFormatterToolResult(t *testing.T) {
	formatter := &PlainFormatter{}
	result := Result{
		Status:     "tool_result",
		ToolName:   "search",
		ToolOutput: "match 1\nmatch 2",
		Timestamp:  1000,
	}

	output, err := formatter.FormatResult(result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, "search") {
		t.Errorf("output should contain tool name: %s", output)
	}

	if !strings.Contains(output, "Output") {
		t.Errorf("output should contain 'Output': %s", output)
	}
}

func TestNewFormatter(t *testing.T) {
	tests := []struct {
		name     string
		fmtType  string
		wantType string
	}{
		{"json", "json", "*output.JSONFormatter"},
		{"jsonl", "jsonl", "*output.JSONFormatter"},
		{"plain", "plain", "*output.PlainFormatter"},
		{"text", "text", "*output.PlainFormatter"},
		{"unknown", "unknown", "*output.PlainFormatter"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewFormatter(tt.fmtType)
			// Just check that it returns a non-nil formatter
			if f == nil {
				t.Error("NewFormatter returned nil")
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input   string
		maxLen  int
		want    string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello..."},
		{"a", 1, "a"},
		{"ab", 1, "a..."},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}
