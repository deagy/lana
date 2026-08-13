# Lana User Guide

**Lana** is a terminal-first coding agent that helps you with real-time code tasks. It supports interactive chat and non-interactive automation.

## Quick Start

### Installation

```bash
# Build from source
git clone https://github.com/deagy/lana
cd lana
go build -o lana ./cmd/lana
./lana --version
```

### Basic Usage

#### Interactive Chat
```bash
# Start a conversation
lana chat

# Chat with a specific model
lana chat --provider openai --model gpt-4

# Continue a previous session
lana chat --resume <session-id>
```

#### Non-Interactive Execution
```bash
# Run a single prompt
lana run "Find all TODO comments"

# With structured output
lana run "Analyze errors" --output json

# Save the session
lana run "Generate report" --save-session
```

## Configuration

### Config File Location
```bash
~/.lana/config.yaml
```

### Example Configuration
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

### Environment Variables
```bash
export LANA_PROVIDER=openai
export LANA_MODEL=gpt-4
export LANA_API_KEY=sk-...
```

## Commands

### chat — Interactive Conversations
```bash
lana chat [prompt]
```

**Options:**
- `--provider, -p` — Provider name
- `--model, -m` — Model name
- `--resume` — Continue a session

**Examples:**
```bash
# Start interactive chat
lana chat

# Chat with initial prompt
lana chat "Explain this code"

# Use specific model
lana chat --provider openai --model gpt-4-turbo

# Resume previous session
lana chat --resume abc123def456
```

### run — Non-Interactive Execution
```bash
lana run <prompt> [flags]
```

**Options:**
- `--provider, -p` — Provider name
- `--model, -m` — Model name
- `--output, -o` — Format: plain, json, jsonl (default: plain)
- `--approve` — Approval mode: ask, auto-edit, full-auto (default: ask)
- `--timeout` — Timeout in seconds (0 = no limit)
- `--save-session` — Save session after execution
- `--max-turns` — Maximum agent loops (default: 10)

**Examples:**
```bash
# Basic execution
lana run "analyze this code"

# JSON output for pipelines
lana run "find issues" --output json

# JSONL streaming
lana run "search codebase" --output jsonl | jq '.status'

# Auto-approve (for CI/CD)
lana run "run tests" --approve full-auto || exit $?

# With timeout
lana run "long operation" --timeout 300
```

### sessions — Manage Sessions
```bash
lana sessions list      # List all sessions
lana sessions delete ID # Delete a session
```

### providers — Manage Providers
```bash
lana providers list     # List available providers
lana providers status   # Check provider health
```

### models — List Available Models
```bash
lana models list        # List all available models
lana models list --provider openai  # Models for a provider
```

### config — Configuration Management
```bash
lana config show        # Display current config
lana config get KEY     # Get a config value
lana config set KEY VAL # Set a config value
lana config path        # Show config file location
```

### doctor — Diagnostics
```bash
lana doctor             # Run diagnostic checks
```

### version — Version Info
```bash
lana version            # Show version and build info
```

### plugin — Manage Plugins
```bash
lana plugin list                    # List installed plugins
lana plugin install <path>          # Install plugin from directory
lana plugin remove <name>           # Uninstall a plugin
lana plugin info <name>             # Show plugin details

# Run an installed plugin
lana <plugin-name> [args...]
```

**Examples:**
```bash
# List all plugins
lana plugin list

# Install a plugin from a directory
lana plugin install ~/my-lana-plugin

# Get details about a plugin
lana plugin info my-lana-plugin

# Run the installed plugin
lana my-lana-plugin --help
lana my-lana-plugin analyze file.go
```

## Writing a Plugin

Plugins are executable programs that extend Lana with new commands. Any language that can produce an executable works: Bash, Go, Python, Node.js, etc.

### Plugin Structure

A plugin is a directory with this layout:

```
my-plugin/
├── manifest.yaml      # Plugin metadata (required)
└── run.sh            # Entrypoint script (executable)
```

### Manifest Format

The `manifest.yaml` defines your plugin:

```yaml
name: my-plugin              # Lowercase with hyphens: ^[a-z][a-z0-9-]*$
version: 1.0.0              # Semantic version
description: What it does   # One-line description
entrypoint: run.sh          # Path to executable (relative to plugin root)

# Optional: MCP servers the plugin provides
# mcpServers:
#   - name: my-server
#     transport: stdio
#     command: ./mcp-server
#     args:
#       - --config
#       - config.json
```

### Entrypoint Script

The entrypoint is executed as `<entrypoint> <args...>` in your current working directory:

```bash
#!/bin/bash
# Receives all arguments from command line
echo "Plugin received: $@"

# Your plugin logic here
if [[ $1 == "--help" ]]; then
  echo "Usage: lana my-plugin [options]"
  exit 0
fi

# Do work and exit with status code
exit 0
```

### Installation

Install your plugin (from a local directory):

```bash
lana plugin install ~/my-plugin
```

This:
1. Validates the manifest and entrypoint
2. Checks the name doesn't collide with built-in commands
3. Copies the entire plugin directory to `~/.lana/plugins/<name>/`
4. Makes the entrypoint executable

### Usage

Once installed, your plugin becomes a Lana subcommand:

```bash
# The command name is the plugin name
lana my-plugin --help
lana my-plugin analyze file.go
```

Plugins run in your **current working directory** (not the plugin install directory), so they naturally work with relative paths like any normal CLI tool.

### MCP Server Integration (Optional)

If your plugin provides an MCP server, declare it in `manifest.yaml`:

```yaml
name: my-plugin
version: 1.0.0
entrypoint: run.sh

mcpServers:
  - name: my-mcp-server
    transport: stdio
    command: ./mcp-server-binary
    args:
      - --config
      - ./config.json
```

When installed, Lana automatically:
1. Registers the MCP server in the global config
2. Rewrites the command path to an absolute path inside your plugin directory
3. Makes the MCP server's tools available to agents

Verify with:
```bash
lana mcp list     # Should show your MCP server
lana mcp tools my-mcp-server  # List its tools
```

**Note:** If you later uninstall the plugin, remove its MCP servers manually:
```bash
lana mcp remove my-mcp-server
```

### Best Practices

1. **Shebang** — Always use a shebang (`#!/bin/bash`, `#!/usr/bin/env python3`, etc.)
2. **Permissions** — Lana automatically makes the entrypoint executable, but make sure it's marked as executable in your source
3. **Exit Codes** — Use meaningful exit codes (0 = success, non-zero = failure)
4. **Help Text** — Support `--help` so users can discover options
5. **Working Directory** — Don't assume the plugin's install directory; always work with absolute paths or the caller's directory
6. **Errors to Stderr** — Write errors and warnings to stderr, output to stdout
7. **No Sandboxing** — Plugins run with your user's permissions (same as running any CLI tool)

### Example Plugin (Bash)

```bash
#!/bin/bash
# ~/my-awesome-plugin/run.sh

set -e

show_help() {
  echo "Usage: lana my-awesome-plugin [command] [args...]"
  echo ""
  echo "Commands:"
  echo "  analyze <file>   Analyze a file"
  echo "  report           Generate a report"
}

if [[ $# -eq 0 ]] || [[ $1 == "--help" ]] || [[ $1 == "-h" ]]; then
  show_help
  exit 0
fi

case "$1" in
  analyze)
    file="$2"
    echo "Analyzing $file..."
    # Your logic here
    ;;
  report)
    echo "Generating report..."
    # Your logic here
    ;;
  *)
    echo "Unknown command: $1" >&2
    show_help >&2
    exit 1
    ;;
esac
```

Manifest:
```yaml
name: my-awesome-plugin
version: 1.0.0
description: Analyze and report on your codebase
entrypoint: run.sh
```

Install and use:
```bash
lana plugin install ~/my-awesome-plugin
lana my-awesome-plugin analyze main.go
lana my-awesome-plugin report
```

## Approval System

The approval system protects against dangerous operations.

### Risk Levels

**Low Risk (Auto-approve)**
- File reads
- Directory listings
- Search operations

**Medium Risk (Ask by default)**
- File writes
- Git commits
- Git branches

**High Risk (Always ask)**
- Shell execution
- Dangerous git commands (reset --hard, push --force)

### Approval Modes

#### ask (default)
```bash
lana run "edit code" --approve ask
# Prompts for each medium/high risk operation
```

#### auto-edit
```bash
lana run "refactor" --approve auto-edit
# Auto-approves edits, asks for dangerous ops
```

#### full-auto
```bash
lana run "run tests" --approve full-auto
# Auto-approves everything (use with caution!)
```

## Output Formats

### Plain Text (default)
```bash
lana run "search" --output plain
# Human-readable output
# [tool_name] result...
```

### JSON
```bash
lana run "search" --output json
# Single JSON object with complete result
{
  "status": "message",
  "content": "...",
  "timestamp": 1691234567
}
```

### JSONL
```bash
lana run "search" --output jsonl
# One JSON object per line (streaming)
{"status": "message_start", "timestamp": 1691234567}
{"status": "message_delta", "message": "...", "timestamp": 1691234567}
{"status": "tool_call", "tool": "search", "timestamp": 1691234567}
```

## Exit Codes

For automation and scripting:

```
0   Success
1   General error
2   Configuration error
3   Provider error (auth, API)
4   Approval denied
5   Tool execution error
6   Policy violation (path escape, etc.)
7   Operation cancelled
8   Timeout
9   Session error
10  Invalid input
```

**Example:**
```bash
lana run "task" --approve ask
if [ $? -eq 4 ]; then
  echo "User denied operation"
fi
```

## Workflows

### Code Review
```bash
# Read file
lana run "Review this file" --approve ask

# Get suggestions
lana run "Suggest improvements"

# Apply changes
lana run "Apply your suggestions" --approve ask
```

### Debugging
```bash
# Search for errors
lana run "Find ERROR in logs" --output json | jq '.tool_output'

# Analyze patterns
lana run "What's causing these errors?"

# Generate fix
lana run "Generate a fix for this"
```

### Codebase Analysis
```bash
# Find TODOs
lana run "Find all TODO comments" --output jsonl

# Count functions
lana run "How many functions in this repo?"

# Architecture insights
lana run "What's the architecture of this codebase?"
```

### CI/CD Integration
```bash
#!/bin/bash
set -e

# Run tests with auto-approval
lana run "Run all tests" --approve full-auto

# Generate report
lana run "Generate test report" --output json > report.json

# Check for issues
lana run "Are there critical issues?" --timeout 60 || exit $?

echo "Pipeline complete!"
```

### Git Workflow
```bash
# Show changes
lana run "Show my git changes"

# Commit with message
lana run "Commit these changes with a descriptive message" --approve ask

# Create PR description
lana run "Generate a PR description"

# Review changes
lana run "Review this diff for issues"
```

## Tools Available

### File Operations
- **read_file** — Read file contents
- **write_file** — Write or update files
- **list_files** — List directory contents

### Git Operations
- **git_status** — Show working directory status
- **git_diff** — Display file changes
- **git_commit** — Create commits
- **git_branch** — Manage branches

### System Operations
- **exec** — Execute shell commands
- **search** — Search files (ripgrep/grep)

## Common Patterns

### Interactive Development
```bash
# Start session
lana chat

# Ask questions and iterate
# (Type messages, see real-time responses)
# Tools execute with approval prompts
# Press Ctrl+C to exit
```

### Batch Processing
```bash
# Run non-interactively
for file in *.go; do
  lana run "Analyze $file for issues" --output json >> results.json
done
```

### Pipeline Integration
```bash
# Pipe results to other tools
lana run "find todos" --output jsonl | \
  jq '.tool_output' | \
  sort | uniq -c
```

### Error Handling
```bash
# Check for specific errors
lana run "task" --output json | jq '.error' || true

# Retry on timeout
for i in 1 2 3; do
  lana run "task" --timeout 30 && break
  sleep $((i * 5))
done
```

## Troubleshooting

### Provider Connection Issues
```bash
# Check provider health
lana providers status

# Verify credentials
lana doctor

# Test with explicit config
lana run "test" --provider openai
```

### Tool Execution Errors
```bash
# Check tool availability
lana models list

# Use verbose output
lana run "task" --output json | jq '.error'

# Check workspace permissions
ls -la $(pwd)
```

### Session Issues
```bash
# List sessions
lana sessions list

# Delete old sessions
lana sessions delete SESSION_ID

# Start fresh
lana chat  # Creates new session
```

## Tips & Tricks

### Time-Saving Commands
```bash
# Create aliases
alias lrun='lana run'
alias lchat='lana chat'

# Use short prompts
lrun "find TODOs"

# Chain operations
lrun "analyze" | jq '.tool_output' | grep -i error
```

### Approval Workflow
```bash
# Review what changes will be made
lana run "show proposed changes" --approve ask

# Apply changes
lana run "apply the changes" --approve ask
```

### Session Management
```bash
# Continue important sessions
lana chat --resume SESSION_ID

# Save work for later
lana run "long task" --save-session
```

## Limitations & Workarounds

### Large Files
- Files > 10MB may timeout
- Workaround: Split into chunks or use search

### Rate Limiting
- API providers have rate limits
- Workaround: Use `--timeout` and exponential backoff

### Long Running Tasks
- Default timeout is unlimited, but set `--timeout` for CI/CD
- Workaround: Break into smaller tasks or use `--max-turns`

## Best Practices

1. **Start Interactive** — Use `lana chat` to explore and test
2. **Then Automate** — Use `lana run` once you know what you want
3. **Check Approval** — Use `--approve ask` for first runs
4. **Save Sessions** — Use `--save-session` for important work
5. **Handle Errors** — Check exit codes in scripts
6. **Output Structured** — Use `--output json` for processing

## Getting Help

```bash
# Show help for a command
lana chat --help
lana run --help

# Check version
lana version

# Run diagnostics
lana doctor
```

## Feedback & Issues

Report issues at: https://github.com/deagy/lana/issues

Suggest features at: https://github.com/deagy/lana/discussions
