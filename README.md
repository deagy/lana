# Lana — Phase 1: Foundation

A terminal-first coding agent CLI written in Go. Phase 1 focuses on scaffolding the architecture and core abstractions.

## Phase 1 Status

**Goal:** Build the foundation with CLI structure, configuration, session persistence, interfaces, and a mock provider.

**Completed:**
- ✓ Repository scaffolding (Go module, directory structure)
- ✓ Provider interface (`internal/provider/provider.go`)
- ✓ Tool interface (`internal/tools/tool.go`)
- ✓ Approval policy interface (`internal/approval/policy.go`)
- ✓ Session store interface (`internal/session/store.go`)
- ✓ In-memory session store (`internal/session/memory.go`)
- ✓ Configuration layer (`internal/config/config.go`)
- ✓ Mock provider for testing (`internal/provider/mock.go`)
- ✓ Cobra CLI skeleton with root command and basic subcommands

**CLI Commands (Phase 1):**
- `lana version` — Show version info
- `lana config show|get|set|path` — Manage configuration
- `lana providers list` — List available providers
- `lana models list` — List models for current provider
- `lana sessions list|delete` — Manage sessions
- `lana doctor` — Check system health

## Building and Testing

```bash
# Build the binary
make build
./lana version

# Run all tests
make test

# Format and lint
make fmt
make lint

# Clean build artifacts
make clean
```

## Architecture Overview

### Layer 1: Provider Abstraction
The `internal/provider` package defines a clean interface for AI provider interactions:
- `Client`: versioned interface for chat completions
- `Stream`/`Reader`: event-driven streaming responses
- `Request`/`Message`/`Event`: provider-agnostic data structures
- `MockProvider`: deterministic testing provider

**Design:** Providers hide API-specific details (credentials, URLs, response formats). Business logic never touches provider internals.

### Layer 2: Tools and Approval
- `internal/tools`: Tool definitions, executors, and risk levels
- `internal/approval`: Pluggable approval policies (ask, auto-edit, full-auto)
- Design: Tools are self-contained; approval is decoupled from execution

### Layer 3: Session Persistence
- `internal/session`: Session store interface with in-memory implementation
- Append-only JSONL-style transcripts with schema versioning
- Supports session resumption and forking

### Layer 4: Configuration
- `internal/config`: Layered config (global, project, environment)
- Uses Viper for flexible configuration sources
- Sensible defaults for development

### Layer 5: CLI
- `cmd/lana`: Entry point
- `internal/cmd`: Cobra command definitions
- Commands split by concern (config, providers, sessions, etc.)

## Next Phase: Provider Implementations (Phase 2)

Phase 2 will implement the first real providers:
1. **OpenAI-compatible provider** — Configurable base URL, API key, model
2. **Ollama provider** — Local endpoint discovery and chat

Phase 2 will also add:
- Streaming chat completion to CLI and TUI
- Provider/model selection and diagnostics
- Generic OpenAI-compatible endpoint presets

## Project Structure

```
.
├── cmd/lana/
│   └── main.go                   # Entry point
├── internal/
│   ├── cmd/                      # Cobra commands
│   ├── config/                   # Configuration layer
│   ├── provider/                 # Provider abstraction
│   ├── tools/                    # Tool definitions
│   ├── approval/                 # Approval policies
│   └── session/                  # Session persistence
├── pkg/                          # Reusable public packages (future)
├── go.mod
├── Makefile
└── README.md
```

## Design Principles

1. **Provider Neutrality**: Business logic never depends on specific providers
2. **Clear Abstractions**: Provider, Tool, ApprovalPolicy, Store are separate concerns
3. **Testability**: Interfaces enable mocking and isolated testing
4. **Layered Config**: Global, project-level, and environment overrides
5. **No Embedded Credentials**: Credentials live in config or environment

## Next Steps

1. Run the basic CLI to verify the scaffold:
   ```bash
   make build
   ./lana config show
   ./lana doctor
   ```

2. Review the interfaces in `internal/provider`, `internal/tools`, `internal/approval`, and `internal/session`

3. Proceed to Phase 2: Implement the OpenAI-compatible and Ollama providers

## License

MIT
