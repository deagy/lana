# Lana Implementation Plan

**Task:** Implement Lana as a Codex CLI clone
**Task ID:** `lana-implementation-plan`
**Generated:** 2026-08-10T22:54Z
**Classification:** internal
**Project:** /home/deagy/sdk/lana

## 1. Overview

Lana is a local CLI tool that clones the core Codex coding agent experience. Unlike the full cloud-hosted Codex platform, Lana runs entirely on the developer's local machine, providing interactive AI-assisted coding through a terminal-based interface.

**Key distinction:** Lana is a *local CLI application*, not a cloud service. This simplifies architecture (no deployment, no database, no API server) but requires careful design for the local execution model, model integration, and session management.

## 2. Architecture

### 2.1 Component Model

```
+---------------------------------------------------+
|                     lana                           |
|  +-----------+  +------------+  +---------------+ |
|  |   CLI     |->|  Session   |->|  Model        | |
|  |  Layer    |  |  Manager   |  |  Provider     | |
|  | (cobra)   |  |            |  |  (OpenAI)     | |
|  +-----------+  +------------+  +---------------+ |
|       |              |                             |
|  +-----------+  +------------+                    |
|  |  File     |  |  Prompt    |                    |
|  |  System   |  |  Builder   |                    |
|  +-----------+  +------------+                    |
|       |              |                            |
|  +-----------+  +------------+                    |
|  |  Tool     |  |  Context   |                    |
|  |  Runner   |  |  Store     |                    |
|  +-----------+  +------------+                    |
+---------------------------------------------------+
```

### 2.2 Core Components

| Component | Responsibility | Module Path |
|-----------|---------------|-------------|
| CLI Layer | Command parsing, flags, subcommands, TTY handling | `cmd/lana/` |
| Session Manager | Conversation state, turn history, tool execution loop | `internal/session/` |
| Model Provider | Abstraction over AI model API calls | `internal/provider/` |
| Prompt Builder | System prompt assembly, context injection | `internal/prompt/` |
| File System | Read/write files in the workspace | `internal/fs/` |
| Tool Runner | Execute shell commands, read files, write files | `internal/tool/` |
| Context Store | Manage workspace context (git status, file tree) | `internal/context/` |

### 2.3 Trust Boundaries

| Boundary | Description |
|----------|-------------|
| User input -> CLI | User commands are trusted; validated for safe subcommand routing |
| CLI -> Model Provider | API key passed as environment variable (never hardcoded); HTTPS only |
| Model Provider -> Tools | Model output parsed against strict schema; tools sandboxed to workspace |
| Tools -> File System | All file operations scoped to the target workspace directory |
| Tools -> Shell | Shell commands executed with bounded timeouts and resource limits |

### 2.4 Data Flows

1. **Initialization:** `lana <workspace>` -> reads git repo metadata, builds initial context -> sends to model
2. **Interactive loop:** User input -> session manager -> prompt builder -> model -> tools -> session state update -> response
3. **File operations:** model requests file read/write -> tool runner validates scope -> executes operation -> returns result

## 3. Scope

### 3.1 In Scope (MVP)

- Interactive CLI with continuous conversation mode
- File system read/write tools (scoped to workspace)
- Shell command execution (bounded, time-limited)
- Git-aware context (file tree, changed files, branch info)
- Multi-turn conversation with full history
- OpenAI-compatible API integration (configurable endpoint)
- Configuration via environment variables and config file
- Basic error handling and recovery

### 3.2 Out of Scope (Future)

- Multi-model support (beyond OpenAI-compatible)
- File system diff/review mode
- Agent-to-agent communication
- Persistent conversation storage
- Web UI / non-interactive mode
- Plugin/tool ecosystem
- Code review / PR integration
- Multi-repository/workspace switching
- Knowledge store integration
- Security scanning / code analysis tools

## 4. Module Structure

```
lana/
+-- cmd/
|   +-- lana/
|       +-- root.go          # Root command (cobra)
|       +-- run.go           # Main interactive command
|       +-- config.go        # Config subcommand
|       +-- main.go          # Entry point
+-- internal/
|   +-- cli/
|   |   +-- spinner.go       # TTY spinner for loading states
|   |   +-- input.go         # User input handling (readline)
|   |   +-- output.go        # Formatted output rendering
|   +-- config/
|   |   +-- config.go        # Config struct and loading
|   |   +-- model.go         # Config file schema
|   +-- context/
|   |   +-- context.go       # Workspace context collection
|   |   +-- git.go           # Git-aware context (status, diff)
|   |   +-- tree.go          # File tree collection
|   +-- fs/
|   |   +-- reader.go        # Safe file reading
|   |   +-- writer.go        # Safe file writing
|   +-- prompt/
|   |   +-- builder.go       # System prompt assembly
|   |   +-- system.go        # Fixed system prompt template
|   |   +-- user.go          # User message formatting
|   +-- provider/
|   |   +-- provider.go      # Provider interface
|   |   +-- openai.go        # OpenAI-compatible implementation
|   +-- session/
|   |   +-- session.go       # Conversation state management
|   |   +-- history.go       # Message history with limits
|   +-- tool/
|       +-- runner.go        # Tool execution orchestration
|       +-- shell.go         # Bounded shell command execution
|       +-- file_read.go     # File read tool
|       +-- file_write.go    # File write tool
+-- pkg/
|   +-- errors/
|       +-- errors.go        # Custom error types
+-- go.mod
+-- go.sum
+-- README.md
```

## 5. API / Contract Design

### 5.1 CLI Interface

```
lana [flags] <command>

Commands:
  run        Start interactive coding session (default)
  config     View/edit configuration

Flags:
  -w, --workspace string   Target workspace directory (default: cwd)
  -k, --api-key string     API key (overrides env var)
  -m, --model string       Model name (overrides config)
  -e, --endpoint string    API endpoint (overrides config)
  -c, --config string      Config file path
  -v, --verbose            Enable verbose output
  --version                Show version
```

### 5.2 Model Provider Interface

```go
// internal/provider/provider.go
type Provider interface {
    Complete(ctx context.Context, messages []Message) (*Response, error)
}

type Message struct {
    Role    string  // "system", "user", "assistant", "tool"
    Content string
    ToolCallID   string  // for tool result messages
    ToolCalls    []ToolCall // for assistant messages requesting tool use
}

type ToolCall struct {
    ID       string
    Function FunctionCall
}

type FunctionCall struct {
    Name      string
    Arguments string  // JSON string
}

type Response struct {
    Message    Message
    Usage      Usage
    StopReason string // "stop", "tool_calls", "length"
}

type Usage struct {
    PromptTokens     int
    CompletionTokens int
    TotalTokens      int
}
```

### 5.3 Tool Contracts

All tools follow a uniform interface:

```go
type Tool interface {
    Name() string
    Description() string
    Schema() string
    Execute(ctx context.Context, argsJSON string) (*ToolResult, error)
}

type ToolResult struct {
    Content string
    IsError bool
}
```

### 5.4 Tool Invocation Schema

Tools are declared with JSON schemas matching the OpenAI tool calling format. Example:

```json
{
  "type": "function",
  "function": {
    "name": "read_file",
    "description": "Read the contents of a file from the workspace",
    "parameters": {
      "type": "object",
      "properties": {
        "path": { "type": "string", "description": "Relative path from workspace root" }
      },
      "required": ["path"]
    }
  }
}
```

## 6. Development Phases

### Phase 0: Project Bootstrap (Estimated: 1-2 days)

| Task | Owner | Deliverable |
|------|-------|-------------|
| Initialize Go module with proper import path | go-service-implementer | `go.mod`, basic project structure |
| Set up `cmd/lana/main.go` entry point | go-service-implementer | Compilable binary (`lana version` works) |
| Configure `goimports`, `gofmt`, `go vet` | go-service-implementer | CI-ready formatting |
| Write `README.md` with build instructions | technical-writer | Project README |

**Gate:** Binary builds locally, `go test ./...` passes.

### Phase 1: Core CLI & Configuration (Estimated: 2-3 days)

| Task | Owner | Deliverable |
|------|-------|-------------|
| Implement cobra root command with flags | go-service-implementer | `cmd/lana/root.go`, `run.go` |
| Implement config file loading (YAML/TOML) | backend-engineer | `internal/config/` |
| Implement environment variable overrides | backend-engineer | Config precedence: CLI > env > config file |
| Write unit tests for config loading | go-service-implementer | Test coverage for config module |

**Gate:** `lana run --workspace ./test` shows help; config loads from file and env vars correctly.

### Phase 2: Model Provider Integration (Estimated: 3-4 days)

| Task | Owner | Deliverable |
|------|-------|-------------|
| Define `Provider` interface | api-contract-engineer | `internal/provider/provider.go` |
| Implement OpenAI-compatible provider | go-service-implementer | `internal/provider/openai.go` |
| Implement streaming response support | go-service-implementer | SSE streaming to terminal |
| Write integration test with mock API | go-service-implementer | Mock-based provider tests |

**Gate:** Can send a message to a mock API and receive a response.

### Phase 3: Session & Conversation Management (Estimated: 3-4 days)

| Task | Owner | Deliverable |
|------|-------|-------------|
| Implement session state machine | backend-engineer | `internal/session/session.go` |
| Implement message history with token limits | backend-engineer | Truncation policy for long conversations |
| Implement system prompt builder | backend-engineer | `internal/prompt/builder.go` |
| Define fixed system prompt template | api-contract-engineer | System prompt as documented template |

**Gate:** Session tracks full conversation history; system prompt injects correctly.

### Phase 4: File System & Tools (Estimated: 4-5 days)

| Task | Owner | Deliverable |
|------|-------|-------------|
| Implement safe file reader | go-service-implementer | Path validation, size limits |
| Implement safe file writer | go-service-implementer | Atomic writes, workspace scoping |
| Implement bounded shell executor | go-service-implementer | Timeouts, resource limits, workspace scoping |
| Implement tool runner orchestration | backend-engineer | Execute tool calls from model responses |
| Write Gherkin scenarios for tool behavior | test-engineer | Tool execution acceptance tests |

**Gate:** Model can successfully request file read/write and shell execution.

### Phase 5: Interactive CLI & Output (Estimated: 3-4 days)

| Task | Owner | Deliverable |
|------|-------|-------------|
| Implement TTY spinner for loading | go-service-implementer | `internal/cli/spinner.go` |
| Implement user input with readline | go-service-implementer | History, autocomplete hints |
| Implement formatted output (markdown rendering) | go-service-implementer | Code blocks, formatting |
| Implement streaming output to terminal | go-service-implementer | Incremental display of model responses |

**Gate:** Full interactive loop works end-to-end with real API.

### Phase 6: Context & Git Integration (Estimated: 2-3 days)

| Task | Owner | Deliverable |
|------|-------|-------------|
| Implement workspace context collector | backend-engineer | File tree, changed files |
| Implement git-aware context | go-service-implementer | `git status`, `git diff` for changed files |
| Add context injection to prompt builder | backend-engineer | Context included in system prompt |

**Gate:** Model receives workspace context in system message.

### Phase 7: Testing, Polish & Documentation (Estimated: 3-4 days)

| Task | Owner | Deliverable |
|------|-------|-------------|
| Integration tests for full loop | test-engineer | E2E tests with mock API |
| Code review of all modules | code-reviewer | Review comments, approval |
| Write comprehensive README | technical-writer | Usage examples, config reference |
| Error handling review | code-reviewer | Graceful error paths documented |
| Final quality gate review | test-engineer | All gates passed, ready for release |

## 7. Testing Strategy

### 7.1 Unit Tests (per module)

- **Config:** Load, validate, precedence (CLI > env > file)
- **Provider:** Mock API response handling, error cases
- **Session:** History management, truncation logic
- **Prompt Builder:** Correct message assembly
- **Tools:** Input validation, error handling, workspace scoping
- **File System:** Path traversal prevention, size limits

### 7.2 Integration Tests

- Full interactive loop with mock model API
- Tool execution with real file system (temp directory)
- Shell command execution with timeout enforcement
- Configuration loading from file and env vars

### 7.3 Gherkin Acceptance Tests

```gherkin
Feature: Lana interactive coding session

  Scenario: User starts a session and receives a response
    Given I am in a git repository at "/tmp/test-workspace"
    When I run "lana run" with model "gpt-5.6"
    And I type "hello, what can you do?"
    Then I should see a model response within 30 seconds
    And the response should mention my capabilities

  Scenario: Model requests to read a file
    Given I am in a session with a git repository
    And I have a file "test.go" with content "package main"
    When the model requests to read "test.go"
    Then the file contents should be returned to the model

  Scenario: Model requests to write a file
    Given I am in a session with a git repository
    When the model requests to write "hello.go" with content "package main"
    Then the file should be created in the workspace

  Scenario: Shell command times out
    Given I am in a session
    When the model runs a shell command that exceeds the timeout
    Then the command should be terminated and an error returned
```

## 8. Security Considerations

| Concern | Mitigation |
|---------|-----------|
| API key exposure | Passed via env var or CLI flag only; never logged or stored in config file by default |
| File system access | All operations scoped to target workspace; path traversal checks |
| Shell command safety | Bounded timeouts; workspace-scoped execution; no root/privileged access |
| Model output injection | Strict tool schema validation; tool calls parsed from structured JSON, not free text |
| Token limits | Conversation history truncated when approaching model context window |
| Resource limits | Shell commands time-bound; file read size-limited |

## 9. Dependencies

### Core Dependencies

```
github.com/spf13/cobra          # CLI framework
github.com/spf13/viper          # Configuration management
github.com/sashabaranov/go-openai # OpenAI Go client (or custom HTTP client)
gopkg.in/yaml.v3                # Config file parsing
github.com/mattn/go-runewidth   # Terminal width for formatting
```

### Development Dependencies

```
github.com/golangci/golangci-lint  # Linting
github.com/cucumber/godog          # Gherkin test framework
github.com/stretchr/testify        # Testing assertions
```

## 10. Implementation Conventions

### 10.1 Go Style (from go-service-implementer role)

- Run `gofmt`, `goimports`, `go vet` on every change
- Use contexts for all I/O and API operations
- Safe error handling with `fmt.Errorf("wrapped: %w", err)`
- No secret logging; all sensitive data masked
- Interface segregation for testability

### 10.2 Testing Style (from test-engineer role)

- Unit tests alongside production code (same package)
- Table-driven tests for multiple input/output combinations
- Mock interfaces for all external dependencies (API, file system)
- Gherkin scenarios for acceptance-critical paths

### 10.3 Code Review (from code-reviewer role)

- Every change reviewed before merging
- Focus on: error handling, resource cleanup, security boundaries
- Negative path testing required for all tools

### 10.4 Architecture (from cloud-architect role)

- Single binary, no external services required
- All state held in memory during session
- Graceful degradation on API failure

## 11. Quality Gates

| Gate | Phase | Requirement |
|------|-------|-------------|
| G1 (Intent) | Phase 0 | Task scope approved; project bootstrapped |
| G2 (Requirements) | Phase 1 | Feature list, scope boundaries, and non-requirements documented |
| G3 (Architecture) | Phase 2 | Component model, interfaces, trust boundaries defined and reviewed |
| G4 (Governance) | Phase 3 | Configuration model validated; no sensitive data in defaults |
| G5 (Security) | Phase 4 | Tool sandboxing, file scoping, shell limits verified |
| G6 (Verification) | Phase 6-7 | All tests passing; E2E integration tests passing; code review complete |
| G7 (Evidence) | Phase 7 | Build passes, tests pass, documentation complete, ready for release |

## 12. Risk Register

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|-----------|
| OpenAI API rate limits | Medium | High | Implement retry with backoff; respect rate limit headers |
| Shell command hangs | Medium | Medium | Hard timeout on all shell execution |
| Model generates unsafe tool calls | Low | High | Strict JSON schema validation; tool names pre-registered |
| Workspace path traversal | Low | High | All file paths validated against workspace root |
| Large file reads exhaust memory | Medium | Low | Configurable size limits on file reads |
| Token limit exceeded in long sessions | High | Medium | Automatic history truncation with configurable policy |

## 13. Deliverables Summary

| Artifact | Owner | Location |
|----------|-------|----------|
| Source code | go-service-implementer | `lana/` |
| Config schema | backend-engineer | `internal/config/model.go` |
| Provider interface | api-contract-engineer | `internal/provider/provider.go` |
| Tool interfaces | api-contract-engineer | `internal/tool/runner.go` |
| Unit tests | go-service-implementer | `*_test.go` alongside code |
| Integration tests | test-engineer | `internal/*/internal_test.go` |
| Gherkin scenarios | test-engineer | `features/*.feature` |
| System prompt | api-contract-engineer | `internal/prompt/system.go` |
| README | technical-writer | `README.md` |
| Architecture decision | cloud-architect | This document |

## 14. Dispatch Attribution

This plan was produced by the orchestrator acting on behalf of the dispatched planning team:

| Agent | Role | Contribution |
|-------|------|-------------|
| api-contract-engineer | API/schema contract design | Provider interface, tool contracts, CLI interface, system prompt template |
| backend-engineer | Go service implementation | Session management, config loading, prompt building, context collection |
| cloud-architect | System architecture | Component model, trust boundaries, data flows, architecture principles |
| threat-modeler | Threat analysis | Security considerations, trust boundary definitions |
| test-engineer | Testing strategy | Unit/integration/acceptance test strategy, Gherkin scenarios |
| code-reviewer | Code quality standards | Review criteria, error handling conventions, negative path testing |
| frontend-engineer | N/A (CLI-only) | Not applicable - no frontend component in this project |

