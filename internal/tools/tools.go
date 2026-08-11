// Package tools provides portable tool contracts and typed built-in tools.
package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/deagy/lana/internal/provider"
)

const ResultSchemaVersion = 1

type Definition struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}

func (d Definition) ProviderDefinition() provider.ToolDefinition {
	return provider.ToolDefinition{Name: d.Name, Description: d.Description, InputSchema: d.InputSchema}
}

type Call struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func (c Call) Validate() error {
	if c.ID == "" {
		return errors.New("tool call id is required")
	}
	if c.Name == "" {
		return errors.New("tool call name is required")
	}
	if len(c.Arguments) == 0 {
		return errors.New("tool call arguments are required")
	}
	if !json.Valid(c.Arguments) {
		return errors.New("tool call arguments are not valid JSON")
	}
	return nil
}

type Result struct {
	SchemaVersion int             `json:"schema_version"`
	CallID        string          `json:"call_id"`
	Name          string          `json:"name"`
	Content       json.RawMessage `json:"content,omitempty"`
	IsError       bool            `json:"is_error,omitempty"`
	ErrorCode     string          `json:"error_code,omitempty"`
	At            time.Time       `json:"at"`
}

func JSONResult(call Call, value any) (Result, error) {
	return JSONResultAt(time.Now().UTC(), call, value)
}

// JSONResultAt creates a result with an explicit timestamp. Tool adapters
// that already have an operation timestamp and deterministic tests should use
// this constructor.
func JSONResultAt(at time.Time, call Call, value any) (Result, error) {
	if at.IsZero() {
		return Result{}, errors.New("tool result timestamp is required")
	}
	content, err := json.Marshal(value)
	if err != nil {
		return Result{}, fmt.Errorf("marshal tool result: %w", err)
	}
	return Result{SchemaVersion: ResultSchemaVersion, CallID: call.ID, Name: call.Name, Content: provider.RedactJSON(content), At: at.UTC()}, nil
}

func ErrorResult(call Call, code string, err error) Result {
	return ErrorResultAt(time.Now().UTC(), call, code, err)
}

// ErrorResultAt creates a typed non-secret tool error with an explicit time.
func ErrorResultAt(at time.Time, call Call, code string, err error) Result {
	result := Result{SchemaVersion: ResultSchemaVersion, CallID: call.ID, Name: call.Name, IsError: true, ErrorCode: stableErrorCode(code), Content: json.RawMessage(fmt.Sprintf("%q", safeError(err))), At: at}
	if !at.IsZero() {
		result.At = at.UTC()
	}
	return result
}

// Validate ensures persisted and provider-visible results have unambiguous
// success/error semantics.
func (r Result) Validate() error {
	if r.SchemaVersion != ResultSchemaVersion {
		return fmt.Errorf("unsupported tool result schema version %d", r.SchemaVersion)
	}
	if r.CallID == "" || r.Name == "" {
		return errors.New("tool result call id and name are required")
	}
	if r.At.IsZero() {
		return errors.New("tool result timestamp is required")
	}
	if len(r.Content) > 0 && !json.Valid(r.Content) {
		return errors.New("tool result content is not valid JSON")
	}
	if r.IsError && r.ErrorCode == "" {
		return errors.New("tool error result code is required")
	}
	if !r.IsError && r.ErrorCode != "" {
		return errors.New("successful tool result must not have an error code")
	}
	return nil
}

// SanitizeResult redacts generic structured payloads and canonicalizes the
// timestamp before a result is rendered, persisted, or replayed.
func SanitizeResult(result Result) Result {
	result.Content = provider.RedactJSON(result.Content)
	if !result.At.IsZero() {
		result.At = result.At.UTC()
	}
	if result.IsError {
		result.ErrorCode = stableErrorCode(result.ErrorCode)
	}
	return result
}

func safeError(err error) string {
	if err == nil {
		return "tool failed"
	}
	return provider.Redact(err.Error())
}

func stableErrorCode(code string) string {
	code = strings.ToLower(strings.TrimSpace(code))
	if code == "" || len(code) > 64 {
		return "tool_failed"
	}
	for _, char := range code {
		if !(char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '_' || char == '-') {
			return "tool_failed"
		}
	}
	return code
}

// Authorizer is intentionally separate from Executor. A UI, policy engine, or
// test fake can decide whether a concrete call is permitted without changing
// tool implementations.
type Authorizer interface {
	Authorize(context.Context, Call) error
}

type Executor interface {
	Execute(context.Context, Call) (Result, error)
}

type AuthorizerFunc func(context.Context, Call) error

func (f AuthorizerFunc) Authorize(ctx context.Context, call Call) error { return f(ctx, call) }

type ExecutorFunc func(context.Context, Call) (Result, error)

func (f ExecutorFunc) Execute(ctx context.Context, call Call) (Result, error) { return f(ctx, call) }

// AllowAll is a useful default for non-interactive callers. Interactive
// applications should inject an explicit authorization implementation.
type AllowAll struct{}

func (AllowAll) Authorize(context.Context, Call) error { return nil }

type Registry struct{ executors map[string]Executor }

func NewRegistry(entries map[string]Executor) *Registry {
	copy := make(map[string]Executor, len(entries))
	for name, executor := range entries {
		copy[name] = executor
	}
	return &Registry{executors: copy}
}

func (r *Registry) Execute(ctx context.Context, call Call) (Result, error) {
	if err := call.Validate(); err != nil {
		return Result{}, err
	}
	executor := r.executors[call.Name]
	if executor == nil {
		return Result{}, fmt.Errorf("unknown tool %q", call.Name)
	}
	result, err := executor.Execute(ctx, call)
	if err != nil {
		return Result{}, err
	}
	if result.SchemaVersion == 0 {
		result.SchemaVersion = ResultSchemaVersion
	}
	if result.CallID == "" {
		result.CallID = call.ID
	}
	if result.Name == "" {
		result.Name = call.Name
	}
	result = SanitizeResult(result)
	if err := result.Validate(); err != nil {
		return Result{}, fmt.Errorf("invalid result from tool %q: %w", call.Name, err)
	}
	return result, nil
}

// Built-in tool argument/result shapes. The shell and file work is delegated
// to adapters; these types ensure calls and results remain serializable and
// provider-neutral.
type ReadFileInput struct {
	Path   string `json:"path"`
	Offset int64  `json:"offset,omitempty"`
	Limit  int64  `json:"limit,omitempty"`
}
type ReadFileOutput struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated,omitempty"`
}
type WriteFileInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}
type WriteFileOutput struct {
	Path  string `json:"path"`
	Bytes int    `json:"bytes"`
}
type ExecInput struct {
	Command string `json:"command"`
	Workdir string `json:"workdir,omitempty"`
}
type ExecOutput struct {
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
	ExitCode int    `json:"exit_code"`
}
type SearchInput struct {
	Query string `json:"query"`
	Path  string `json:"path,omitempty"`
	Limit int    `json:"limit,omitempty"`
}
type SearchMatch struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Text string `json:"text"`
}
type SearchOutput struct {
	Matches []SearchMatch `json:"matches"`
}

var Builtins = []Definition{
	{Name: "read_file", Description: "Read a workspace file", InputSchema: json.RawMessage(`{"type":"object","required":["path"],"properties":{"path":{"type":"string"},"offset":{"type":"integer"},"limit":{"type":"integer"}}}`)},
	{Name: "write_file", Description: "Write a workspace file", InputSchema: json.RawMessage(`{"type":"object","required":["path","content"],"properties":{"path":{"type":"string"},"content":{"type":"string"}}}`)},
	{Name: "exec", Description: "Run a command", InputSchema: json.RawMessage(`{"type":"object","required":["command"],"properties":{"command":{"type":"string"},"workdir":{"type":"string"}}}`)},
	{Name: "search", Description: "Search workspace text", InputSchema: json.RawMessage(`{"type":"object","required":["query"],"properties":{"query":{"type":"string"},"path":{"type":"string"},"limit":{"type":"integer"}}}`)},
}
