package output

import (
	"encoding/json"
	"fmt"
)

// Result represents a tool execution or turn result.
type Result struct {
	Status     string                 `json:"status"`
	Message    string                 `json:"message,omitempty"`
	ToolName   string                 `json:"tool_name,omitempty"`
	ToolInput  map[string]interface{} `json:"tool_input,omitempty"`
	ToolOutput string                 `json:"tool_output,omitempty"`
	Error      string                 `json:"error,omitempty"`
	Approved   bool                   `json:"approved,omitempty"`
	Timestamp  int64                  `json:"timestamp"`
}

// Formatter handles output formatting for different modes.
type Formatter interface {
	FormatResult(r Result) (string, error)
}

// JSONFormatter outputs one JSON object per line.
type JSONFormatter struct{}

func (f *JSONFormatter) FormatResult(r Result) (string, error) {
	data, err := json.Marshal(r)
	if err != nil {
		return "", fmt.Errorf("marshal result: %w", err)
	}
	return string(data), nil
}

// PlainFormatter outputs human-readable text.
type PlainFormatter struct{}

func (f *PlainFormatter) FormatResult(r Result) (string, error) {
	switch r.Status {
	case "message":
		return r.Message, nil
	case "tool_start":
		return fmt.Sprintf("🔧 %s\n  Input: %v", r.ToolName, r.ToolInput), nil
	case "tool_result":
		return fmt.Sprintf("✓ %s\n  Output: %s", r.ToolName, truncate(r.ToolOutput, 200)), nil
	case "tool_error":
		return fmt.Sprintf("✗ %s error: %s", r.ToolName, r.Error), nil
	case "error":
		return fmt.Sprintf("Error: %s", r.Error), nil
	default:
		return fmt.Sprintf("[%s] %s", r.Status, r.Message), nil
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// NewFormatter creates a formatter based on the type.
func NewFormatter(fmtType string) Formatter {
	switch fmtType {
	case "json", "jsonl":
		return &JSONFormatter{}
	case "plain", "text":
		return &PlainFormatter{}
	default:
		return &PlainFormatter{}
	}
}
