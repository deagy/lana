# Lana — Phase 2: First Provider Vertical

A terminal-first coding agent CLI written in Go. Phase 2 adds working provider implementations and streaming chat.

## Current Status

**Phase:** 2 of 6 (Foundation + First Provider Vertical)

### Phase 1 ✅ Complete
- Repository scaffolding with clean architecture
- Core provider, tool, approval policy, and session store interfaces
- Configuration layer with layered overrides
- Mock provider for testing

### Phase 2 ✅ Complete
- **OpenAI-compatible provider** — Configurable URL, API key, model
- **Ollama provider** — Local endpoint with model discovery
- **Streaming chat** — Single-turn and interactive modes
- **Provider/model selection** — Dynamic configuration
- **Provider diagnostics** — Connection checks
- **23 tests passing** (Phase 1 + Phase 2)

## CLI Commands

### Chat
- `lana chat [prompt]` — Start or continue a session
  - `--model <model>` — Override default model
  - `--provider <provider>` — Override default provider
  - `--resume <id>` — Resume previous session

### Configuration
- `lana config show|get|set|path` — Manage configuration
- `lana providers list` — Show available providers
- `lana providers status` — Check provider connectivity
- `lana models list` — List models for current provider
- `lana sessions list|delete` — Manage sessions
- `lana doctor` — System health check
- `lana version` — Version info

## Quick Start

### 1. Build the binary
```bash
make build
./lana version
```

### 2. Configure a provider

**OpenAI API:**
```bash
lana config set provider.name openai-compat
lana config set provider.endpoint https://api.openai.com/v1
lana config set provider.api_key sk-your-key-here
lana config set provider.model gpt-4
```

**Ollama (local):**
```bash
lana config set provider.name ollama
lana config set provider.model llama2
# Make sure Ollama is running: ollama serve
```

**LM Studio (local):**
```bash
lana config set provider.name openai-compat
lana config set provider.endpoint http://localhost:1234/v1
lana config set provider.api_key not-needed
lana config set provider.model local-model
```

### 3. Start a chat
```bash
# Single message
lana chat "Hello, what can you do?"

# Interactive chat (type 'exit' to quit)
lana chat
```

### 4. Check provider status
```bash
lana doctor                  # Full system check
lana providers list          # List available providers
lana providers status        # Check current provider connectivity
lana models list             # List available models
```

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
