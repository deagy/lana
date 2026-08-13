# Lana API Reference

This document describes the Lana APIs for tool developers and integrators.

## Tool Development

### Implementing a Tool

Tools must implement the `Tool` interface in `internal/tools/tool.go`:

```go
type Tool interface {
    Name() string                                    // Unique identifier
    Description() string                             // Human-readable description
    InputSchema() json.RawMessage                   // JSON Schema for inputs
    Execute(ctx context.Context, input json.RawMessage) (string, error)  // Execution
}
```

### Example: Simple Tool

```go
package impl

import (
    "context"
    "encoding/json"
    "fmt"
    "github.com/deagy/lana/internal/tools"
)

type MyTool struct {
    workspace string
}

func NewMyTool(workspace string) *MyTool {
    return &MyTool{workspace: workspace}
}

func (mt *MyTool) MyOperation() tools.Tool {
    schema := json.RawMessage(`{
        "type": "object",
        "properties": {
            "input": {
                "type": "string",
                "description": "Input text"
            }
        },
        "required": ["input"]
    }`)

    return &tools.Definition{
        NameVal:        "my_tool",
        DescriptionVal: "Does something useful",
        SchemaVal:      schema,
        RiskLevel:      tools.RiskLevelLow,
        ExecutorVal:    &myExecutor{workspace: mt.workspace},
    }
}

type myExecutor struct {
    workspace string
}

func (e *myExecutor) Execute(_ interface{}, input json.RawMessage) (string, error) {
    var req struct {
        Input string `json:"input"`
    }
    if err := json.Unmarshal(input, &req); err != nil {
        return "", fmt.Errorf("parse input: %w", err)
    }

    // Implement your logic
    result := fmt.Sprintf("Processed: %s", req.Input)
    return result, nil
}
```

### Registering a Tool

Register tools in `internal/tools/impl/registry_init.go`:

```go
func InitializeRegistry(workspace string) (*tools.Registry, error) {
    registry := tools.NewRegistry()

    // Your tool
    myTool := NewMyTool(workspace)
    registry.Register(myTool.MyOperation())

    return registry, nil
}
```

### Tool Risk Levels

```go
const (
    RiskLevelLow    = "low"     // Auto-approve
    RiskLevelMedium = "medium"  // Ask for approval
    RiskLevelHigh   = "high"    // Always ask
)
```

Choose based on impact:
- **Low:** Read-only, non-destructive
- **Medium:** Modifies files, state changes
- **High:** Executes commands, system changes

### Best Practices

1. **Validate Input** — Check all user inputs
2. **Sanitize Output** — Clean sensitive data from results
3. **Handle Errors** — Return meaningful error messages
4. **Set Timeouts** — Prevent hanging operations
5. **Use Context** — Respect cancellation via `ctx`

## Provider Integration

### Implementing a Provider

Providers implement `internal/provider.Client`:

```go
type Client interface {
    Chat(ctx context.Context, req *Request) (Reader, error)
    Name() string
    Model() string
    SupportedModels(ctx context.Context) ([]ModelInfo, error)
}
```

### Provider Events

A `Reader` streams events:

```go
type Event interface {
    Type() string  // "message.start", "message.delta", "tool.call", etc.
}

// Concrete events
&MessageStartEvent{Role: "assistant"}
&MessageDeltaEvent{Content: "text"}
&ToolCallEvent{ID: "...", Name: "tool", Input: json.RawMessage}
&MessageEndEvent{StopReason: "stop"}
&ErrorEvent{Err: error}
```

### Example: Custom Provider

```go
type CustomProvider struct {
    model    string
    endpoint string
}

func (p *CustomProvider) Chat(ctx context.Context, req *Request) (Reader, error) {
    // Make API call
    reader := &customReader{}
    go reader.streamEvents(ctx, req)
    return reader, nil
}

func (p *CustomProvider) Name() string { return "custom" }
func (p *CustomProvider) Model() string { return p.model }
func (p *CustomProvider) SupportedModels(ctx context.Context) ([]ModelInfo, error) {
    return []ModelInfo{{Name: "model1"}}, nil
}
```

## Runtime APIs

### Session Store

```go
type Store interface {
    Create(ctx context.Context, opts CreateOpts) (string, error)
    Get(ctx context.Context, id string) (*Session, error)
    List(ctx context.Context) ([]SessionMetadata, error)
    AppendMessage(ctx context.Context, sessionID string, msg *Message) error
    Save(ctx context.Context, sessionID string, state *Session) error
    Delete(ctx context.Context, id string) error
    Close() error
}
```

### Session Structure

```go
type Session struct {
    ID           string
    CreatedAt    time.Time
    UpdatedAt    time.Time
    Model        string
    Provider     string
    Title        string
    Transcript   []Message
}

type Message struct {
    Role      string
    Content   string
    Timestamp time.Time
    ToolCalls []ToolCall
}

type ToolCall struct {
    ID       string
    Name     string
    Input    json.RawMessage
    Status   string  // "pending", "approved", "completed", "failed"
    Result   string
    Error    string
}
```

### Tool Executor

```go
type Executor struct {
    // Private fields
}

func NewExecutor(registry *Registry, policy Policy, broker Broker) *Executor {
    return &Executor{...}
}

func (e *Executor) Execute(ctx context.Context, id, toolName string, 
    input json.RawMessage) (*Result, error) {
    // Validates, checks approval, executes tool
    return &Result{...}, nil
}
```

## Output APIs

### Formatter

```go
type Formatter interface {
    FormatResult(r Result) (string, error)
}

type Result struct {
    Status     string
    Message    string
    ToolName   string
    ToolInput  map[string]interface{}
    ToolOutput string
    Error      string
    Timestamp  int64
}
```

### Usage

```go
formatter := output.NewFormatter("json")  // "json", "jsonl", "plain"
result := output.Result{
    Status:  "message",
    Message: "Hello",
    Timestamp: time.Now().Unix(),
}
text, _ := formatter.FormatResult(result)
fmt.Println(text)  // {"status":"message",...}
```

## Safety APIs

### Workspace Policy

```go
func ResolveWorkspacePath(workspace, relPath string) (string, error) {
    // Validates path is within workspace
    // Prevents directory traversal
}

func IsHighRisk(command string) bool {
    // Detects dangerous commands
}

func ContainsSensitivePattern(content string) bool {
    // Detects API keys, passwords, etc.
}
```

### Approval Policy

```go
type Policy interface {
    ShouldApprove(ctx context.Context, toolName string, level RiskLevel) bool
}

type Broker interface {
    Request(ctx context.Context, toolName, description string) (bool, error)
}
```

## Configuration APIs

```go
type Config struct {
    Provider Provider
    Approval Approval
    Session  Session
}

type Provider struct {
    Name     string
    Model    string
    Endpoint string
    APIKey   string
}

// Load config
loader := config.NewLoader()
cfg, _ := loader.Load(path)
```

## Testing APIs

### Mock Provider

```go
type MockProvider struct {
    Events []Event
}

func (m *MockProvider) Chat(ctx context.Context, req *Request) (Reader, error) {
    return &MockReader{events: m.Events}, nil
}
```

### Test Session Store

```go
store := &MemoryStore{}
sessionID, _ := store.Create(ctx, session.CreateOpts{
    Model: "test", Provider: "mock",
})
sess, _ := store.Get(ctx, sessionID)
```

## Integration Example

```go
package main

import (
    "context"
    "github.com/deagy/lana/internal/providers"
    "github.com/deagy/lana/internal/tools/impl"
    "github.com/deagy/lana/internal/approval"
)

func main() {
    ctx := context.Background()

    // Create provider
    factory := providers.NewFactory("openai", "gpt-4", "", "sk-...")
    client, _ := factory.Create()

    // Initialize tools
    registry, _ := impl.InitializeRegistry("/home/user/project")

    // Create policy
    policy := approval.NewStaticPolicy(approval.FullAutoMode)

    // Execute tool
    executor := execution.NewExecutor(registry, policy, nil)
    result, _ := executor.Execute(ctx, "session1", "search",
        json.RawMessage(`{"pattern": "TODO"}`))

    println(result.Output)
}
```

## Error Handling

All APIs return errors that can be checked:

```go
result, err := executor.Execute(ctx, id, tool, input)
if err != nil {
    switch err.(type) {
    case *tools.ToolNotFoundError:
        // Handle missing tool
    default:
        // Handle general error
    }
}
```

## Performance Considerations

1. **Buffering** — EventPipeline buffers 100 events
2. **Concurrency** — Thread-safe registry and stores
3. **Cancellation** — Respect context.Done() for cleanup
4. **Timeouts** — Set timeouts on long operations

## Versioning

Lana follows semantic versioning:

- **Major** — Breaking API changes
- **Minor** — New features, backward compatible
- **Patch** — Bug fixes

Current version available via:
```bash
lana version
```

## Documentation Standards

When adding new APIs:
1. Document with godoc comments
2. Include usage examples
3. Document error cases
4. Note thread safety
5. Include performance notes

## Feedback

Have questions about the API? Open an issue at:
https://github.com/deagy/lana/issues
