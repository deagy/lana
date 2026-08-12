# Lana CLI

A local coding-agent CLI for interactive terminal conversations, noninteractive prompts, and agent orchestration.

## Features

- **Interactive Conversations**: Natural language interface for coding tasks
- **Noninteractive Prompts**: Scriptable agent execution
- **Plugin System**: Extensible with local and GitHub plugins
- **Knowledge Store**: Local knowledge management with semantic search
- **GitHub/GitLab Integration**: PR/MR creation and management
- **SDLC Tracking**: Agentic SDLC lifecycle governance
- **Agent Dispatch**: Structured work items with role-based execution

## Installation

```bash
# Build from source
go build -o lana ./cmd/lana

# Or use the release script
./scripts/release.sh
```

## Quick Start

```bash
# Interactive conversation
lana "Explain this repository"

# Noninteractive prompt
lana exec "Summarize the test failure" --jsonl

# File operations
lana file read src/main.go
lana file write src/main.go "// Your code here"

# Create a goal
lana goal create --objective "Implement MCP client" --with-budget --token-budget 500

# Create a plan
lana plan create --step "Design MCP protocol" --step "Implement JSON-RPC" --step "Add transport"
```

## Commands

### Core Commands

| Command | Description |
|---------|-------------|
| `lana` | Start interactive conversation |
| `lana exec` | Run noninteractive prompt |
| `lana file` | File operations (read, write, delete, copy, move, search) |
| `lana knowledge` | Manage local knowledge store |
| `lana agents` | Manage agent roles and work items |
| `lana sdlc` | Inspect Agentic SDLC run records |

### Plugin Commands

| Command | Description |
|---------|-------------|
| `lana plugin list` | List installed plugins |
| `lana plugin install` | Install a plugin |
| `lana plugin remove` | Remove a plugin |
| `lana plugin github-search` | Search GitHub for plugins |
| `lana plugin github-install` | Install plugin from GitHub |

### SDLC Commands

| Command | Description |
|---------|-------------|
| `lana sdlc status` | Show lifecycle gate status |
| `lana sdlc plan` | Create dispatch plan |
| `lana sdlc decide` | Record gate decision |
| `lana sdlc approve-from-gitlab` | Approve from GitLab MR |
| `lana sdlc approve-from-github` | Approve from GitHub PR |

### System Commands

| Command | Description |
|---------|-------------|
| `lana system version` | Show version information |
| `lana system health` | Show health status |
| `lana system config` | Show configuration |
| `lana system dirs` | Show directory paths |
| `lana system env` | Show environment variables |

### Configuration

| Command | Description |
|---------|-------------|
| `lana config show` | Show current configuration |
| `lana config get` | Get a specific value |
| `lana config set` | Set a value |
| `lana config path` | Show config file path |

### Completion

| Command | Description |
|---------|-------------|
| `lana completion bash` | Generate bash completion script |
| `lana completion zsh` | Generate zsh completion script |
| `lana completion fish` | Generate fish completion script |

## Configuration

Configuration is stored in `~/.config/lana/config.yaml`.

```yaml
# Example configuration
logging:
  level: info
  json: false

provider:
  type: openai
  model: gpt-4
  api-key: your-api-key

tools:
  authorizer: default
  executor: default

knowledge:
  store: local
  path: ~/.local/share/lana/knowledge
```

## Plugins

Lana supports a plugin system for extending functionality.

### Installing Plugins

```bash
# Search for plugins
lana plugin github-search "your search term"

# Install from GitHub
lana plugin github-install owner/repo

# List installed plugins
lana plugin list

# Remove a plugin
lana plugin remove <plugin-name>
```

### Creating Plugins

Plugins are Go modules that implement the Lana plugin interface.

```go
package main

import (
    "github.com/deagy/lana/pkg/plugin"
)

type MyPlugin struct {
    name string
}

func (p *MyPlugin) Name() string {
    return p.name
}

func (p *MyPlugin) Execute(ctx context.Context, args []string) error {
    // Your plugin logic
    return nil
}

func main() {
    plugin.Register(&MyPlugin{name: "my-plugin"})
}
```

## SDLC Integration

Lana supports Agentic SDLC lifecycle governance.

### Initialize SDLC

```bash
# Initialize SDLC tracking
cadre sdlc init --profile default --project my-project --classification internal
```

### Track Lifecycle Gates

```bash
# Check status
lana sdlc status --task-id my-task

# Create plan
lana sdlc plan --task-id my-task --task "Implement feature X"

# Record decision
lana sdlc decide --task-id my-task --gate G1 --role reviewer --decision approve --actor user1 --evidence-uri https://github.com/repo/pull/1
```

## Agent Dispatch

Lana supports structured agent dispatch with roles.

### Create Work Items

```bash
# Create a goal
lana goal create --objective "Implement new feature" --with-budget --token-budget 1000

# Create a plan
lana plan create --step "Design architecture" --step "Implement core logic" --step "Add tests"

# Dispatch agents
lana agents dispatch --task-id my-task
```

## Knowledge Store

Lana includes a local knowledge store with semantic search.

### Manage Knowledge

```bash
# Add knowledge
lana knowledge add --title "API Design" --content "Best practices for REST APIs"

# Search knowledge
lana knowledge search "API design patterns"

# List all records
lana knowledge list

# Delete a record
lana knowledge delete <id>
```

## Development

### Building

```bash
go build -o lana ./cmd/lana
```

### Testing

```bash
go test ./...
```

### Linting

```bash
golangci-lint run ./...
```

### Coverage

```bash
go test -cover ./...
```

## Architecture

```
lana/
├── cmd/lana/           # Main CLI entry point
├── internal/           # Internal packages
│   ├── cmd/           # Command implementations
│   ├── github/        # GitHub client
│   ├── gitlab/        # GitLab client
│   ├── knowledge/     # Knowledge store
│   ├── mcp/           # MCP client
│   ├── plugin/        # Plugin system
│   ├── policy/        # Policy enforcement
│   ├── provider/      # Provider abstraction
│   ├── session/       # Session management
│   ├── tools/         # Tool implementations
│   └── tui/           # Terminal UI
├── pkg/               # Public packages
│   ├── config/        # Configuration
│   ├── logger/        # Logging
│   ├── output/        # Rich output
│   ├── recovery/      # Error recovery
│   └── sandbox/       # Sandbox utilities
└── scripts/           # Build and release scripts
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests (`go test ./...`)
5. Submit a pull request

## License

MIT License - see LICENSE file for details.

## Support

- Documentation: See inline help (`lana --help`)
- Issues: GitHub Issues
- Discussions: GitHub Discussions

## Acknowledgments

Built with Go and inspired by modern CLI tools for developer productivity.
