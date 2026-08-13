# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**Lana** is a local coding-agent CLI that provides interactive terminal conversations, noninteractive prompt execution, and structured agent orchestration. It integrates with the MCP (Model Context Protocol) ecosystem, maintains a knowledge store with semantic search, and provides extensibility through a plugin system and agent dispatch framework.

Lana is designed as a **provider-neutral platform**: it abstracts provider credentials, API contracts, and model selection behind clean interfaces (`internal/provider.Client`, `internal/cli.Kernel`, `internal/tools`), making it possible to swap or layer multiple AI providers without touching business logic.

## Build, Test, and Development Commands

### Building

```sh
# Build the binary
go build -o lana ./cmd/lana

# Build with version info (from release script)
./scripts/release.sh

# Build specific platform targets
GOOS=darwin GOARCH=arm64 go build -o lana-darwin-arm64 ./cmd/lana
GOOS=linux GOARCH=amd64 go build -o lana-linux-amd64 ./cmd/lana
```

### Testing

```sh
# Run all tests
go test ./...

# Run tests in a single package with verbose output
go test -v ./internal/knowledge

# Run a specific test
go test -v -run TestStore ./internal/knowledge

# Run tests with coverage
go test -cover ./...

# Run integration tests (if separate)
cd internal/agents && go test -tags=integration ./...

# Run with race detector
go test -race ./...
```

### Linting and Code Quality

```sh
# Format code
go fmt ./...

# Check for common mistakes
go vet ./...

# Import organization
go tool goimports -w .

# Full linting suite (if golangci-lint installed)
golangci-lint run ./...
```

### Development Workflow

```sh
# Live build and test during development
go build -o lana ./cmd/lana && ./lana exec "test command"

# Debug a specific command
./lana -v agents list  # verbose flag if implemented

# Inspect configuration paths
./lana system dirs
./lana system config
```

## Architecture and Key Concepts

### High-Level Design

Lana follows a **layered, provider-neutral architecture**:

```
┌─────────────────────────────────────────┐
│   TUI / CLI Output Layer                │ (root.go, tui.go)
│   ↓ (interactive vs. noninteractive)    │
├─────────────────────────────────────────┤
│   CLI Runtime (Kernel, Runtime, Turn)   │ (internal/cli, internal/agent)
│   ↓ (session state, approval broker)    │
├─────────────────────────────────────────┤
│   Provider Abstraction                  │ (internal/provider)
│   ↓ (models, streaming, credentials)    │
├─────────────────────────────────────────┤
│   Tool System                           │ (internal/tools)
│   ↓ (Definitions, Authorizer, calls)    │
├─────────────────────────────────────────┤
│   Knowledge Store, Plugins, Policy      │ (internal/knowledge, plugin, policy)
│   ↓ (semantic search, GitHub discovery) │
└─────────────────────────────────────────┘
```

### Critical Abstractions

**1. Provider Contract** (`internal/provider/provider.go`)
   - `Client`: versioned, sanitized AI provider abstraction
   - `Stream`: iterates over events (message.start, message.delta, tool.call, message.end, error)
   - `Request`: model, messages, tools, max_tokens
   - `Message`: role (user/assistant/system), content
   - Implementations hide API-specific details; Lana code never touches credentials or API URLs

**2. CLI Runtime** (`internal/cli/`)
   - `Kernel`: TurnExecutor for one turn of agent conversation
   - `Runtime`: session state, model, permissions, approval broker, provider binding
   - `PlainRenderer`: human-readable output
   - `JSONLRenderer`: structured output for pipelines
   - `ApprovalBroker`: forwards tool calls for interactive approval before execution

**3. Agent Turn** (`internal/agent/turn.go`)
   - `TurnRunner`: orchestrates message → tool call → tool result → next message loop
   - Respects max rounds, cancellation via context
   - Yields tool calls for approval/execution

**4. Tool System** (`internal/tools/`)
   - `Definition`: name, description, input schema, authorization rules
   - `Registry`: discovers/validates/authorizes tool calls
   - `Authorizer`: pluggable, workspace-aware policy enforcement
   - Builtins: `read_file`, `write_file`, `exec`, `search`
   - Tool calls are validated before execution; results are sanitized

**5. Session and State** (`internal/session/store.go`)
   - Append-only JSONL log per session
   - Schema-versioned records enable forward/backward compatibility
   - Recovery and fork support

### Folder Organization

- **`cmd/lana/`**: Entry point (root.go detects TTY and routes to TUI or exec)
- **`internal/cli/`**: Kernel, Runtime, Renderers, Approval
- **`internal/agent/`**: Turn orchestration
- **`internal/provider/`**: Provider abstraction (Client, Stream, Request, Message)
- **`internal/tools/`**: Tool definitions, registry, authorizer
- **`internal/knowledge/`**: Semantic search store (character n-gram embeddings)
- **`internal/plugin/`**: Plugin discovery, installation (local + GitHub)
- **`internal/mcp/`**: MCP client integration (JSON-RPC transport)
- **`internal/github/`, `internal/gitlab/`**: PR/MR creation
- **`internal/policy/`**: Workspace containment, risk levels (unrestricted, workspace-write, workspace-read-only)
- **`pkg/config/`**: Configuration management (viper integration)
- **`pkg/output/`**: Output formatting (colors, spinners, etc.)
- **`pkg/recovery/`**: Error recovery (panic handlers, retry logic)

## Important Design Invariants

**1. No Credentials in Lana Code**
   - Provider credentials live in `pkg/config` or environment
   - `internal/provider.Client` is injected; Lana never reads secrets directly
   - Configuration and secrets are managed by config layer

**2. Approval is Always Explicit**
   - `ApprovalBroker` routes tool calls for interactive approval
   - Noninteractive mode (`lana exec`) can defer approval or auto-approve if policy permits
   - Tools that modify files/execute commands always ask unless explicitly trusted

**3. Workspace Containment**
   - `internal/policy/policy.go` enforces workspace boundary
   - File operations are validated against workspace root
   - `exec` tool respects working-directory restrictions

**4. Turn Isolation**
   - Each agent conversation is one or more turns
   - Each turn produces a transcript entry (message + tool calls + results)
   - Sessions are immutable logs; new turns append, never overwrite

**5. Provider Neutrality**
   - Business logic never depends on provider-specific features
   - Model selection is configuration, not code
   - Tool schemas are provider-agnostic (JSON Schema)

## When Modifying Key Systems

### Adding a New Command

1. Define the command in `cmd/lana/` (use Cobra)
2. Implement logic in `internal/cmd/`
3. Add a handler to `internal/cli/runtime.go` if it needs session/provider/tools
4. Write tests in `_test.go` files adjacent to implementation
5. Update README.md and AGENTS.md with usage examples

### Adding a New Tool

1. Define in `internal/tools/` (name, schema, description, authorizer)
2. Implement executor in the tool's handler
3. Register with `Registry` in `internal/tools/tools.go`
4. Write unit tests for happy path and error cases
5. Test end-to-end: `lana exec "use the tool"` with appropriate flags

### Extending the Provider Contract

1. Changes to `internal/provider/` are **breaking** — affects all implementations
2. If adding a new event type or field, increment the contract version
3. Implement in at least one provider (e.g., mock) before adding to real providers
4. Update session schema version if turn format changes
5. Test serialization/deserialization in session recovery

### Modifying Policy or Workspace Logic

1. Update `internal/policy/policy.go` for rules
2. Update `internal/tools/authorizer.go` for tool-level enforcement
3. Test with `pkg/recovery/` to ensure panics are graceful
4. Workspace containment must remain a hard invariant — no exceptions for convenience

## Integration with Cadre Agent System

Lana can dispatch work to Cadre agents and integrate with Agentic SDLC gates:

```bash
# Initialize SDLC for a project
cadre sdlc init --profile default --project my-project

# Create a plan with Cadre agents
lana sdlc plan --task-id my-task --task "Implement feature"

# Dispatch agents
lana agents dispatch --task-id <task-id>

# Track SDLC status
lana sdlc status --task-id <task-id>

# Record gate decisions
lana sdlc decide --task-id <task-id> --gate G1 --role reviewer --decision approve
```

The `internal/cmd/sdlc/` module provides read-only inspection of SDLC run records. Agent dispatch integrates with Cadre's role catalog when available.

## Common Patterns

### Adding Structured Output

Use `pkg/output/` and `JSONLRenderer`:

```go
// Plain text
fmt.Printf("Tool: %s\n", tool.Name)

// Structured (JSONL for piping)
renderer.Result(map[string]interface{}{
    "tool": tool.Name,
    "status": "completed",
    "result": result,
})
```

### Handling Cancellation

Context is always available in agent turns:

```go
select {
case <-ctx.Done():
    return fmt.Errorf("cancelled: %w", ctx.Err())
case result := <-toolResultChan:
    // process
}
```

### File Operations with Workspace Safety

```go
// Always validate against workspace boundary
path, err := policy.ResolveWorkspacePath(requestedPath)
if err != nil {
    return fmt.Errorf("outside workspace: %w", err)
}
// Only then proceed with I/O
```

## Testing Strategy

- **Unit tests** are isolated (mock provider, mock tools, in-memory session)
- **Integration tests** use Docker Compose or local services (see `proving-ground` pattern in `/home/deagy/sdk/pkg`)
- **End-to-end tests** run the binary with real flags and inspect output/exit codes
- All tests in `*_test.go` files adjacent to implementation; use `-run` to focus

## Notes for Future Development

- **TUI Evolution**: The Codex-style feature plan (docs/codex-tui-feature-plan.md) outlines split-pane, file-diff, and progress-indicator enhancements; these are **presentation-only** changes and don't affect provider or tool logic
- **Plugin System**: GitHub plugin discovery is implemented; local plugin loading is the next step
- **Knowledge Store**: Currently uses character n-gram embeddings; semantic search queries are embedded the same way for ranking
- **MCP Transport**: JSON-RPC client is ready; server discovery and auto-start remain future work
