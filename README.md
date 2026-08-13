# Lana — Coding Agent CLI

A terminal-first AI coding agent written in Go. Execute tasks interactively or automate workflows with structured output.

**Version:** 0.2.0  
**Status:** Phase 7 Complete (95% of v1.0 feature set)

## Quick Start

### Installation

```bash
# Build from source
git clone https://github.com/deagy/lana
cd lana
make build
./lana version
```

### Interactive Chat

```bash
# Start a conversation
lana chat

# Ask about code
lana chat "Explain this function"

# Continue a previous session
lana chat --resume <session-id>
```

### Non-Interactive (Automation)

```bash
# Run a single prompt
lana run "Find all TODO comments"

# Get JSON output for pipelines
lana run "analyze code" --output json | jq '.tool_output'

# Use in scripts
lana run "run tests" --approve full-auto || exit $?
```

## Key Features

✅ **Interactive & Non-Interactive**
- Chat mode for exploration
- Run mode for automation

✅ **9 Production Tools**
- File operations (read, write, list)
- Git management (status, diff, commit, branch)
- Shell execution
- Codebase search

✅ **Multiple Providers**
- OpenAI-compatible (OpenAI, Ollama, LM Studio, etc.)
- Ollama (local)

✅ **Safety Framework**
- Workspace containment
- Risk-based approval
- Secret detection
- Path traversal prevention

✅ **Structured Output**
- JSON for automation
- JSONL for streaming
- Plain text for terminals

✅ **Smart Sessions**
- Persistent conversation history
- Resume previous work
- Export and analysis

## Commands

### Chat — Interactive Conversations
```bash
lana chat [prompt]
  --provider, -p     Provider name
  --model, -m        Model name  
  --resume           Continue a session
```

### Run — Non-Interactive Execution
```bash
lana run <prompt>
  --provider, -p     Provider name
  --model, -m        Model name
  --output, -o       Format: plain, json, jsonl
  --approve          Mode: ask, auto-edit, full-auto
  --timeout          Timeout in seconds
  --save-session     Keep session after run
  --max-turns        Maximum agent loops
```

### Configuration
```bash
lana config show        # Show current config
lana config get KEY     # Get a value
lana config set KEY VAL # Set a value
lana config path        # Show config file location
```

### Management
```bash
lana sessions list      # List all sessions
lana sessions delete ID # Delete a session
lana providers list     # Show available providers
lana models list        # List available models
lana version            # Show version
lana doctor             # Run diagnostics
```

## Configuration

Config file: `~/.lana/config.yaml`

```yaml
provider:
  name: openai
  model: gpt-4
  endpoint: https://api.openai.com/v1
  api_key: sk-...

approval:
  mode: ask  # ask, auto-edit, full-auto

session:
  store_path: ~/.lana/sessions
```

Environment variables override config file:
```bash
export LANA_PROVIDER=openai
export LANA_MODEL=gpt-4
export LANA_API_KEY=sk-...
```

## Tools

### File Operations
- `read_file` — Read file contents (Low risk)
- `write_file` — Write/create files (Medium risk)
- `list_files` — List directories (Low risk)

### Git Operations
- `git_status` — Show status (Low risk)
- `git_diff` — Display changes (Low risk)
- `git_commit` — Create commits (Medium risk)
- `git_branch` — Manage branches (Medium risk)

### System
- `exec` — Execute shell commands (High risk)
- `search` — Search files with ripgrep/grep (Low risk)

## Output Formats

### Plain Text (default)
```bash
lana run "task" --output plain
```

### JSON
```bash
lana run "task" --output json
{
  "status": "message",
  "message": "...",
  "timestamp": 1691234567
}
```

### JSONL (streaming)
```bash
lana run "task" --output jsonl | jq '.status'
```

## Approval Modes

```bash
# ask (default): Prompt for each operation
lana run "edit code" --approve ask

# auto-edit: Auto-approve edits, ask for dangerous ops
lana run "refactor" --approve auto-edit

# full-auto: Auto-approve everything
lana run "run tests" --approve full-auto
```

## Exit Codes

Use in scripts for error handling:

```
0   Success
1   General error
2   Config error
3   Provider error
4   Approval denied
5   Tool error
6   Policy violation
7   Context cancelled
8   Timeout
9   Session error
10  Invalid input
```

## Documentation

- [USER_GUIDE.md](docs/USER_GUIDE.md) — Complete usage guide
- [API.md](docs/API.md) — Developer API reference
- [RELEASE_CHECKLIST.md](docs/RELEASE_CHECKLIST.md) — Release procedures
- [CHANGELOG.md](CHANGELOG.md) — Version history
- [CLAUDE.md](CLAUDE.md) — Project guidelines

## Development

### Building
```bash
# Build binary
make build

# Build release for all platforms
make release-all

# Install to $GOPATH/bin
make install
```

### Testing
```bash
# Run all tests
make test

# Run with coverage
make cover

# Run specific tests
go test ./internal/output/... -v
```

### Code Quality
```bash
# Format code
make fmt

# Lint code
make lint

# Clean artifacts
make clean
```

## Architecture

### Layered Design
```
CLI Commands
  ↓
Kernel (Runtime, Sessions)
  ↓
Provider Abstraction (Client, Events)
  ↓
Tool System (Registry, Executor)
  ↓
Safety & Policy (Workspace, Approval)
```

### Key Packages

- `internal/provider/` — Provider abstraction (Client, Reader, Events)
- `internal/tools/` — Tool definitions and registry
- `internal/execution/` — Tool executor with approval
- `internal/policy/` — Safety and workspace containment
- `internal/session/` — Session persistence
- `internal/storage/` — File-based session store
- `internal/runner/` — Non-interactive execution
- `internal/output/` — Output formatting
- `internal/cmd/` — CLI commands
- `internal/tui/` — TUI components

## Production Ready

✅ Core functionality tested (30 tests)  
✅ All 9 tools implemented and working  
✅ Safety framework with approval policies  
✅ Session management with persistence  
✅ Structured output for automation  
✅ Exit codes for error handling  

## Limitations

- TUI integration in progress (Part 3)
- Single-agent execution (multi-agent in roadmap)
- Limited plugin system (planned for v0.2)

## Roadmap

### v0.2.0 (Q3 2025)
- MCP protocol integration
- Plugin system
- Advanced output modes (CSV, Markdown)

### v0.3.0 (Q4 2025)
- Web UI dashboard
- Multi-agent orchestration

### v1.0.0 (2026)
- Stable API
- Production deployment guide

## Contributing

Contributions welcome! See [CONTRIBUTING.md](docs/CONTRIBUTING.md) for guidelines.

## Support

- **Issues:** https://github.com/deagy/lana/issues
- **Discussions:** https://github.com/deagy/lana/discussions
- **Email:** support@example.com

## License

MIT License. See [LICENSE](LICENSE) for details.

---

**Built with:**
- Go 1.23
- Cobra (CLI)
- Viper (Config)
- Bubble Tea (TUI)

**Status:** Actively maintained

Last updated: 2025-08-12
