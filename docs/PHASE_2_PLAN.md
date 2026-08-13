# Phase 2: First Provider Vertical — Plan

**Goal:** Implement OpenAI-compatible and Ollama providers; add streaming chat.

## Phase 2 Deliverables

### 1. OpenAI-Compatible Provider (`internal/providers/openai.go`)
**Scope:**
- Implement `provider.Client` interface
- Support configurable base URL, API key, model
- Streaming chat completions
- Tool/function calling (basic)
- Error handling (rate limits, auth, timeouts)
- Usage metadata (tokens, cost where available)

**Test Coverage:**
- Mock server (httptest) for deterministic testing
- Rate limit handling
- Partial stream interruption
- Invalid response handling
- Tool call parsing

**Configuration Keys:**
- `provider.name: "openai-compat"`
- `provider.api_key: "sk-..."`
- `provider.endpoint: "https://api.openai.com/v1"` (or custom)
- `provider.model: "gpt-4"`

### 2. Ollama Provider (`internal/providers/ollama.go`)
**Scope:**
- Implement `provider.Client` interface
- Local endpoint discovery (default: http://localhost:11434)
- Model discovery from running Ollama instance
- Streaming chat completions
- Error handling (connection refused, model not found)

**Test Coverage:**
- Mock Ollama responses
- Connection error handling
- Model discovery

**Configuration Keys:**
- `provider.name: "ollama"`
- `provider.endpoint: "http://localhost:11434"`
- `provider.model: "llama2"` (or other available model)

### 3. Streaming Chat CLI (`internal/cmd/chat.go`)
**Scope:**
- `lana chat [prompt]` command
- Start new session or continue existing
- Stream provider responses line-by-line to terminal
- Show tool calls with approval prompts
- Support `--model` and `--provider` overrides

**Implementation:**
```go
cmd := &cobra.Command{
    Use: "chat [prompt]",
    RunE: func(cmd *cobra.Command, args []string) error {
        // Load provider from config
        // Create or resume session
        // Send message
        // Stream response
        // Handle tool calls with approval broker
    },
}
```

### 4. Streaming Chat TUI (placeholder)
- Placeholder for `internal/tui/tui.go` (Phase 3)
- Can reuse streaming infrastructure from CLI

### 5. Provider/Model Selection
**Commands:**
```bash
lana providers list              # List available provider implementations
lana models list                 # List models for configured provider
lana config set provider.name ollama
lana config set provider.model llama2
```

**Behavior:**
- Auto-detect Ollama if running locally
- Validate model exists before allowing selection
- `lana doctor` checks provider connectivity

### 6. Generic OpenAI-Compatible Endpoint Presets
**Scope:**
- LM Studio, LiteLLM, OpenRouter, vLLM, etc.
- Configuration examples in README
- User-friendly endpoint validation

**Configuration Examples:**
```yaml
# OpenRouter (OpenAI-compatible)
provider:
  name: openai-compat
  endpoint: "https://openrouter.io/api/v1"
  api_key: "sk-or-..."
  model: "gpt-4-turbo"

# LM Studio (local)
provider:
  name: openai-compat
  endpoint: "http://localhost:1234/v1"
  api_key: "not-needed"
  model: "local-model"

# Ollama
provider:
  name: ollama
  endpoint: "http://localhost:11434"
  model: "llama2"
```

## Implementation Roadmap

### Step 1: OpenAI-Compatible Provider (Day 1)
1. Create `internal/providers/openai.go`
2. Implement HTTP client with streaming
3. Parse SSE (Server-Sent Events) responses
4. Handle errors (401, 429, 500, timeout)
5. Implement tool call parsing
6. Write comprehensive tests with mock server

### Step 2: Ollama Provider (Day 1)
1. Create `internal/providers/ollama.go`
2. Implement model discovery via `/api/tags`
3. Implement streaming chat via `/api/chat`
4. Handle connection errors gracefully
5. Write tests

### Step 3: CLI Chat Command (Day 2)
1. Create `internal/cmd/chat.go`
2. Implement session creation/resumption logic
3. Add streaming output to console
4. Add tool call approval prompts
5. Handle cancellation (Ctrl+C)

### Step 4: Provider Selection & Diagnostics (Day 2)
1. Add `providers list` subcommand (list available implementations)
2. Add `models list` subcommand (list models for current provider)
3. Update `doctor` to check provider connectivity
4. Validate configuration before operations

### Step 5: Integration Tests (Day 3)
1. Mock OpenAI server (httptest)
2. Mock Ollama server (httptest)
3. Stream interruption scenarios
4. Configuration merging tests
5. End-to-end chat flow

## File Structure After Phase 2

```
internal/
├── providers/                    (NEW)
│   ├── openai.go                # OpenAI-compatible implementation
│   ├── openai_test.go           # Tests with mock server
│   ├── ollama.go                # Ollama implementation
│   └── ollama_test.go           # Tests with mock server
├── cmd/
│   ├── chat.go                  (NEW)
│   ├── chat_test.go             (NEW)
│   └── (existing)
├── provider/                    (Phase 1)
│   ├── provider.go              # Interface (unchanged)
│   ├── mock.go                  # Mock implementation
│   └── provider_test.go         # Tests
└── (other Phase 1 packages)
```

## Key Implementation Details

### OpenAI-Compatible Streaming Response
```
data: {"object":"chat.completion.chunk","choices":[{"delta":{"content":"Hello"}}]}
data: {"object":"chat.completion.chunk","choices":[{"delta":{"content":" world"}}]}
data: [DONE]
```

**Implementation:**
- Use `bufio.Scanner` to read SSE lines
- Parse `data:` prefix
- Unmarshal JSON payload
- Emit `MessageDelta` events to Reader

### Ollama Streaming Response
```
{"model":"llama2","created_at":"...","message":{"role":"assistant","content":"Hello"},"done":false}
{"model":"llama2","created_at":"...","done":true,"total_duration":...}
```

**Implementation:**
- Use `json.Decoder` to read JSONL
- Each line is a complete JSON object
- Accumulate `content` fields until `done: true`
- Emit `MessageDelta` events progressively

### Configuration Priority (for Phase 2)
1. Command-line flags (`--model`, `--provider`)
2. Environment variables (`LANA_MODEL`, `LANA_PROVIDER`)
3. Project config (`.lana/config.yaml`)
4. Global config (`~/.lana/config.yaml`)
5. Defaults

## Testing Strategy

### Unit Tests
- Provider request/response serialization
- Stream parsing (SSE for OpenAI, JSONL for Ollama)
- Configuration loading and merging
- Tool call extraction

### Integration Tests
- Mock HTTP server for OpenAI-compatible
- Mock HTTP server for Ollama
- Stream interruption scenarios
- Configuration validation

### End-to-End Tests
- Full chat flow with mock provider
- Session creation and resumption
- Tool call approval prompts

## Definition of Done (Phase 2)

- [ ] OpenAI-compatible provider implements `Client` interface
- [ ] Ollama provider implements `Client` interface
- [ ] Both providers support tool calling
- [ ] `lana chat` command works with both providers
- [ ] `lana config` supports provider/model selection
- [ ] `lana doctor` checks provider connectivity
- [ ] All tests pass
- [ ] README updated with configuration examples
- [ ] `go vet` and `go fmt` pass
- [ ] Commit message explains design decisions

## Success Criteria

A user can:
1. Configure OpenAI-compatible endpoint:
   ```bash
   lana config set provider.name openai-compat
   lana config set provider.endpoint https://api.openai.com/v1
   lana config set provider.api_key sk-...
   lana config set provider.model gpt-4
   ```

2. Start a chat:
   ```bash
   lana chat "Hello, what can you do?"
   ```

3. See streamed response with tool calls displayed

4. Configure and use Ollama locally:
   ```bash
   lana config set provider.name ollama
   lana chat "What is 2+2?"
   ```

5. Resume sessions across runs:
   ```bash
   lana chat                           # List recent sessions
   lana chat --resume <session-id>    # Continue conversation
   ```
