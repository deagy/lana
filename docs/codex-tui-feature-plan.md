# Codex-Style TUI Agent Interface — Feature Plan

**Status:** proposal — design only, not approved for implementation.
**Owner for scope decisions:** Product Owner; provider and security decisions require the designated security/architecture authority.
**Review date:** before Phase 1 implementation begins.
**Related documents:**
- [architecture.md](architecture.md) — current architecture baseline
- [codex-like-agent-cli-remaining-plan.md](codex-like-agent-cli-remaining-plan.md) — phased delivery plan (Phases 0–6)
- [requirements.md](requirements.md) — current product boundary
- [compatibility-surface.md](compatibility-surface.md) — public command surface

---

## 1. Executive Summary

This plan proposes evolving Lana's existing Bubble Tea TUI (`internal/tui/tui.go`) into a **Codex-style interactive agent interface** with split-pane layouts, rich tool-call panels, file-diff rendering, command execution views, progress indicators, and comprehensive keyboard navigation. The design preserves Lana's provider-neutral contracts (`internal/provider.Client`, `internal/cli.Kernel`, `internal/tools`), its approval-broker authorization path, and its workspace-policy enforcement — it changes only the **presentation layer** and adds a small set of supporting rendering modules.


---

## 2. Current State Analysis

### 2.1 What Exists Today

| Component | Location | Capabilities |
|---|---|---|
| **TUI framework** | `internal/tui/tui.go` | Bubble Tea `Model`/`Update`/`View` loop; `textarea.Model` composer; transcript as `[]line`; slash commands (`/help`, `/model`, `/permissions`, `/quit`, `/session`); history navigation (↑/↓); Ctrl+C cancellation; approval surface (y/n); status bar; color + no-color modes |
| **Provider contract** | `internal/provider/provider.go` | `Client`, `Stream`, `Request`, `Message`, `Event` (versioned, sanitized, redacted); event types: `message.start`, `message.delta`, `tool.call`, `message.end`, `error` |
| **CLI runtime** | `internal/cli/runtime.go` | `Kernel` (TurnExecutor), `Runtime` (session, model, permissions, approval broker), `PlainRenderer`, `JSONLRenderer`, `ApprovalBroker` |
| **Agent turn** | `internal/agent/turn.go` | `TurnRunner` with tool-call loop, max rounds, cancellation-aware |
| **Tool system** | `internal/tools/tools.go` | `Definition`, `Call`, `Result`, `Registry`, `Authorizer`, `AllowAll`, builtins: `read_file`, `write_file`, `exec`, `search` |
| **Session store** | `internal/session/store.go` | Append-only JSONL, recovery, fork, schema-versioned records |
| **Agent queue** | `internal/agents/queue.go` | Durable task queue with dependency tracking, leasing, cancellation |
| **Policy** | `internal/policy/policy.go` | Workspace containment, risk levels, `unrestricted`/`workspace-write`/`workspace-read-only` modes |
| **Root wiring** | `cmd/lana/root/root.go` | Detects TTY → launches `tui.Run()`; otherwise falls through to `exec` |

### 2.2 Current TUI Limitations (Gaps vs. Codex)

| Gap | Current behavior | Codex target |
|---|---|---|
| **Layout** | Single-column transcript + single textarea at bottom | Split-pane: transcript (left/top) + tool output (right/bottom) |
| **Tool rendering** | Single line: `tool: approval/tool activity: read_file` | Card/panel per tool call with status, name, arguments preview, running indicator, output, and result |
| **File diffs** | Not rendered; write_file shows as generic tool line | Unified diff view with +/- highlighting, file header, line numbers |
| **Command output** | Not rendered; exec shows as generic tool line | Dedicated panel with stdout (monospace) and stderr (red) |
| **Progress** | No visual indicator during provider streaming | Animated spinner / progress bar during `message.delta` streaming |
| **Markdown** | Plain text only | Rendered markdown with code blocks, headers, lists |
| **Syntax highlighting** | None | Language-specific highlighting for code blocks |
| **Keyboard shortcuts** | Minimal (Ctrl+C, y/n, ↑/↓, /commands) | Comprehensive: Tab switching, panel focus, scroll, inline commands |
| **Session management** | `/session` slash command | Visual session list, switch, fork from TUI |
| **Multi-turn context** | Implicit in transcript | Thread view, conversation summary, context window indicator |
| **Narrow terminal** | Truncates with `Width(width-2)` | Adaptive: stack panes vertically below width threshold |
| **Copy/paste** | Not implemented | OSC 52 clipboard integration (already a transitive dep: `go-osc52`) |
The TUI remains a **presentation-only** concern: it asks the runtime and approval broker for data and never implements authorization, executes tools, or holds provider credentials. All business logic stays in `internal/cli`, `internal/agent`, `internal/tools`, and `internal/session`.

---

## 3. Target State Design

### 3.1 Layout Specification

```
┌─────────────────────────────────────────────────────────────────┐
│ Lana v0.5.0  session:abc123  model:gpt-4o  permissions:ask     │ ← Header bar
├──────────────────────────────────┬──────────────────────────────┤
│                                  │                              │
│  👤 You:                         │  🔧 write_file (running…)    │
│  Refactor the auth module        │                              │
│                                  │  ── src/auth/handler.go ──   │
│  🤖 Lana:                        │                              │
│                                  │  - fmt.Println("hello")     │
│  I'll refactor the auth module   │  + fmt.Println("hi")        │
│  for you. Let me read the file   │                              │
│  first.                          │  [3 changes, 2 deletions]    │
│                                  │                              │
│  🔧 read_file (completed)        │  stdout:                     │
│  ── README.md ──                 │  $ go test ./...             │
│  # Project                       │  PASS                        │
│  Some docs…                      │  ok   myproject 0.523s       │
│                                  │                              │
│  👤 You:                         │                              │
│  Looks good, commit it           │                              │
│                                  │                              │
├──────────────────────────────────┴──────────────────────────────┤
│  ❯ _                            │ [1:transcript] [2:tool] [3:] │ ← Input bar
└─────────────────────────────────────────────────────────────────┘
```

**Width-dependent behavior:**
- **Wide** (≥100 cols): Side-by-side split at ~60/40 ratio
- **Medium** (60–99 cols): Side-by-side at ~70/30 ratio
- **Narrow** (<60 cols): Stacked vertically, transcript on top

### 3.2 Component Inventory

#### 3.2.1 Transcript Pane (`internal/tui/pane/transcript.go`)
- Renders user messages, assistant messages, and system messages
- Supports markdown rendering (headers, bold, italic, code spans)
- Code blocks with syntax highlighting
- Scrollable with mouse wheel or `j`/`k` / `Ctrl+D`/`Ctrl+U`
- Timestamps optional (configurable)

#### 3.2.2 Tool Panel (`internal/tui/pane/tool.go`)
- One card per tool call showing:
  - **Status indicator**: pending, running (spinning), completed, failed, denied
  - **Tool name** and icon
  - **Arguments preview** (redacted, bounded)
  - **Running indicator**: animated spinner during execution
  - **Output view**:
    - For `write_file`: unified diff with +/- highlighting
    - For `exec`: stdout (monospace, green tint) + stderr (red tint)
    - For `read_file`: file content preview (truncated)
    - For `search`: match list with path:line:snippet
  - **Collapse/expand** toggle

#### 3.2.3 Input Bar (`internal/tui/pane/input.go`)
- Multi-line `textarea.Model` (expandable to 5+ lines)
- `Ctrl+Enter` to send, `Shift+Enter` for newline
- `/` to trigger command palette
- Live model/permissions indicator
- Send button or key hint

#### 3.2.4 Header Bar (`internal/tui/pane/header.go`)
- Version, session ID, model, permissions
- Connection status indicator
- Quick-access buttons: session list, settings, help

#### 3.2.5 Status Bar (`internal/tui/pane/status.go`)
- Tab/panel selector: `[1:transcript] [2:tool] [3:input]`
- Tool call count, active turn indicator
- Keyboard shortcut hints

#### 3.2.6 Approval Surface (enhanced)
- Modal overlay for approval requests
- Shows full redacted preview
- y/n/ctrl+c with clear visual affordance
- Auto-focus on approval when pending

### 3.3 Keyboard Shortcut Map

| Key | Action |
|---|---|
| `Ctrl+Enter` | Send message |
| `Shift+Enter` | Newline in input |
| `Tab` | Switch focus: transcript ↔ tool panel ↔ input |
| `1` / `2` / `3` | Jump to transcript / tool panel / input |
| `j` / `k` | Scroll transcript down/up (when focused) |
| `Ctrl+D` / `Ctrl+U` | Page down/up in transcript |
| `g` / `G` | Scroll transcript to top/bottom |
| `Ctrl+C` | Cancel active turn or approval |
| `y` / `n` | Approve/deny pending tool call |
| `/` | Open command palette |
| `?` | Show keyboard shortcuts help |
| `q` | Quit (with confirmation if dirty) |
| `Ctrl+L` | Clear transcript |
| `Ctrl+S` | Save session |
| `Ctrl+F` | Fork session |
| `Ctrl+O` | Open session list |
| `Esc` | Clear input / dismiss modal |
| `↑` / `↓` | History navigation (when input focused) |
| `Ctrl+R` | Retry last turn |
| `Ctrl+Z` | Undo last user message |

### 3.4 Command Palette

Invoked with `/` or `Ctrl+Shift+P`:
- `/model <name>` — Switch model
- `/permissions <mode>` — Change permissions
- `/session new` — Start new session
- `/session list` — List sessions
- `/session switch <id>` — Switch session
- `/session fork` — Fork current session
- `/clear` — Clear transcript
- `/help` — Show help
- `/shortcuts` — Show keyboard shortcuts
- `/export` — Export session transcript
- `/quit` — Exit

### 3.5 Session Management

- **Session list**: `Ctrl+O` opens a dropdown/modal listing recent sessions with:
  - Session ID, created time, first message preview
  - Search/filter
  - Click to switch, `d` to delete, `f` to fork
- **Fork**: `Ctrl+F` creates a fork of the current session (parent reference preserved in session store)

---

## 4. Architecture Changes

### 4.1 New Modules

```
internal/tui/
├── tui.go                  # Existing: Model, Init, Update, View, Run (refactor to compose panes)
├── pane/
│   ├── transcript.go       # New: transcript pane with markdown rendering
│   ├── tool.go             # New: tool call card/panel rendering
│   ├── input.go            # New: input bar with multi-line textarea
│   ├── header.go           # New: header bar
│   ├── status.go           # New: status bar with tab selector
│   ├── approval.go         # New: enhanced approval modal
│   ├── session_list.go     # New: session list modal
│   └── command_palette.go  # New: command palette
├── render/
│   ├── markdown.go         # New: markdown to lipgloss/string renderer
│   ├── diff.go             # New: unified diff renderer
│   ├── syntax.go           # New: syntax highlighting (chroma adapter)
│   └── spinner.go          # New: animated spinner frames
├── keymap.go               # New: centralized keyboard shortcut definitions
├── layout.go               # New: responsive layout calculator
└── tui_test.go             # Existing tests (extend)
```

### 4.2 Refactors to Existing Modules

#### `internal/tui/tui.go`
- **Current**: Monolithic `Model` with `transcript []line`, `composer textarea.Model`, single `View()` method
- **Target**: `Model` becomes a **layout orchestrator** that composes multiple `Pane` interfaces. Each pane has its own `Update()`, `View()`, and `Focus()`/`Blur()`. The `Model` handles focus routing, global key events, and window resize distribution.

#### `internal/cli/runtime.go`
- **No structural changes needed.** The TUI continues to use `*cli.Runtime` and `*cli.ApprovalBroker` as dependencies.
- **Optional enhancement**: Add `Runtime.Transcript()` accessor to allow the TUI to render the full conversation history on session resume.

#### `internal/provider/provider.go`
- **No changes needed.** The TUI consumes `provider.Event` through the existing `EventSink` mechanism.
- **Optional enhancement**: Add a `EventGroup` type that batches events for batched rendering (e.g., group all `message.delta` events into one transcript update per tick).

#### `internal/tools/tools.go`
- **No changes needed.** The TUI renders tool results through the event stream.
- **Optional enhancement**: Add a `ResultRenderer` interface that tool result types can implement for specialized rendering (diff for write_file, monospace for exec output). This is low-priority; the TUI can dispatch on `call.Name` instead.

### 4.3 Data Flow

```
Provider Stream
    │
    ▼
provider.Event ──→ EventSink (cli.Kernel) ──→ tui.eventMsg
                                                    │
                                                    ▼
                                             Model.addEvent()
                                                    │
                                    ┌───────────────┼───────────────┐
                                    ▼               ▼               ▼
                              transcript lines   tool cards     status update
                                    │               │               │
                                    └───────────────┴───────────────┘
                                                    │
                                                    ▼
                                             Model.View()
                                                    │
                                    ┌───────────────┼───────────────┐
                                    ▼               ▼               ▼
                              transcript.Pane  tool.Pane       input.Pane
                                                    │
                                             (composed by layout)
                                                    │
                                                    ▼
                                             tea.Program.Render()
```

### 4.4 Pane Interface

```go
// Pane is the contract for all TUI sub-components.
type Pane interface {
    Init() tea.Cmd
    Update(tea.Msg) (Pane, tea.Cmd)
    View() string
    Focus() tea.Cmd
    Blur()
    Width() int
    Height() int
    SetSize(width, height int)
}
```

### 4.5 Layout Engine

```go
// Layout calculates pane positions based on terminal size.
type Layout struct {
    Wide    bool   // >=100 cols: side-by-side
    Medium  bool   // 60-99 cols: narrow side-by-side
    Narrow  bool   // <60 cols: stacked
}

func (l *Layout) Calculate(width, height int) LayoutConfig
```

---

## 5. Implementation Phases

### Phase T1 — Foundation: Pane Architecture (2–3 days)

**Goal:** Refactor `internal/tui/tui.go` from monolithic Model to composable Pane architecture.

| Task | File | Description |
|---|---|---|
| Define `Pane` interface | `internal/tui/pane/pane.go` | `Init`, `Update`, `View`, `Focus`, `Blur`, `Width`, `Height`, `SetSize` |
| Extract transcript pane | `internal/tui/pane/transcript.go` | Move `[]line` transcript + rendering logic from old `Model` into `TranscriptPane` |
| Extract input pane | `internal/tui/pane/input.go` | Move `composer textarea.Model` into `InputPane` with enhanced multi-line support |
| Extract status pane | `internal/tui/pane/status.go` | Move status bar into `StatusPane` with tab selector |
| Create layout orchestrator | `internal/tui/layout.go` | `Layout` type that composes panes, handles resize, focuses correct pane |
| Rewrite `Model` | `internal/tui/tui.go` | Thin orchestrator delegating to panes; preserve `Run()`, `New()`, `Options` |
| Update tests | `internal/tui/tui_test.go` | Refactor existing tests to work with new pane architecture |

**Acceptance criteria:**
- `go test ./internal/tui/...` passes
- `go build ./cmd/lana` succeeds
- Existing TUI behavior preserved: slash commands, history, Ctrl+C, approval (y/n), status bar, color/no-color
- No regression in `root_test.go`

### Phase T2 — Tool Panel (2–3 days)

**Goal:** Replace single-line tool messages with rich tool call cards.

| Task | File | Description |
|---|---|---|
| Define tool event model | `internal/tui/pane/tool.go` | `ToolCard` struct: status, name, arguments, output, result |
| Tool status tracking | `internal/tui/pane/tool.go` | Track tool calls by ID; transition: pending → running → completed/failed/denied |
| Running spinner | `internal/tui/render/spinner.go` | Frame-based spinner characters |
| Diff rendering | `internal/tui/render/diff.go` | Unified diff: `+` green, `-` red, ` ` dim, file header, line numbers |
| Exec output rendering | `internal/tui/pane/tool.go` | Monospace stdout, red stderr, exit code badge |
| Event routing | `internal/tui/tui.go` | Route `EventToolCall` to tool pane; `EventTextDelta` to transcript; `EventError` to transcript |
| Approval integration | `internal/tui/pane/approval.go` | Move approval into modal overlay on top of tool panel |

**Acceptance criteria:**
- Tool calls render as cards with status icons
- `write_file` shows unified diff with syntax coloring
- `exec` shows stdout/stderr in monospace
- Running tool calls show animated spinner
- Approval requests show as modal overlay

### Phase T3 — Markdown & Syntax (2–3 days)

**Goal:** Add markdown rendering and syntax highlighting to transcript.

| Task | File | Description |
|---|---|---|
| Markdown renderer | `internal/tui/render/markdown.go` | Convert markdown to lipgloss-styled strings (headers, bold, italic, code, lists) |
| Syntax highlighter | `internal/tui/render/syntax.go` | Adapter for chroma (or built-in) to highlight code blocks by language |
| Code block detection | `internal/tui/render/markdown.go` | Detect fenced code blocks in streaming text; render inline as plain until block closes |
| Transcript markdown | `internal/tui/pane/transcript.go` | Render assistant messages as markdown; keep user messages plain |
| Streaming rendering | `internal/tui/pane/transcript.go` | Handle partial markdown during streaming (don't close unclosed tags) |

**Dependencies to add** (evaluate):
- `github.com/alecthomas/chroma/v2` — syntax highlighting
- `github.com/charmbracelet/glamour` — markdown rendering (bigger, more features)
- OR: lightweight custom renderer using lipgloss (smaller, fewer features)

**Recommendation:** Start with a lightweight custom markdown renderer using lipgloss for headers, bold, italic, inline code, and fenced code blocks. Add glamour later if needed.

**Acceptance criteria:**
- Assistant messages render with markdown formatting
- Code blocks are syntax-highlighted
- Streaming text renders incrementally without layout thrash
- Narrow terminal degrades gracefully (no overflow)

### Phase T4 — Layout & Navigation (2 days)

**Goal:** Split-pane layout, keyboard navigation, responsive design.

| Task | File | Description |
|---|---|---|
| Responsive layout | `internal/tui/layout.go` | Wide/medium/narrow layout calculation |
| Focus management | `internal/tui/tui.go` | Route keys to focused pane; global keys bypass focus |
| Keyboard shortcuts | `internal/tui/keymap.go` | Centralized key binding definitions and handlers |
| Scroll implementation | `internal/tui/pane/transcript.go` | Scroll offset, page up/down, home/end |
| Mouse support | `internal/tui/tui.go` | Enable mouse capture for scroll wheel and click-to-focus |
| Tab switching | `internal/tui/tui.go` | Tab/1/2/3 to switch between transcript, tool, input |
| Command palette | `internal/tui/pane/command_palette.go` | Filterable command list, keyboard navigation |

**Acceptance criteria:**
- Side-by-side layout at wide terminals
- Stacked layout at narrow terminals
- Tab key cycles focus between transcript, tool panel, input
- Scroll works with mouse wheel and keyboard
- Command palette filters and executes commands
- All keyboard shortcuts from section 3.3 functional

### Phase T5 — Session Management (1–2 days)

**Goal:** Visual session list, fork, and switch from within TUI.

| Task | File | Description |
|---|---|---|
| Session list modal | `internal/tui/pane/session_list.go` | Browse, search, select sessions |
| Session API | `internal/tui/tui.go` | Integrate with `cli.Runtime.SessionID` and session store |
| Fork action | `internal/tui/tui.go` | Call session store fork; create new session reference |
| Export transcript | `internal/tui/tui.go` | Export current session as markdown/text |
| Context indicator | `internal/tui/pane/header.go` | Show session info, token estimate if available |

**Acceptance criteria:**
- `Ctrl+O` opens session list
- Can switch between sessions
- `Ctrl+F` forks current session
- Export produces valid markdown file

### Phase T6 — Polish & Hardening (1–2 days)

**Goal:** Copy/paste, animations, edge cases, accessibility.

| Task | File | Description |
|---|---|---|
| OSC 52 clipboard | `internal/tui/pane/input.go` | Copy selected text to system clipboard |
| Alt/option keys | `internal/tui/keymap.go` | Support meta-modifier for power-user shortcuts |
| Terminal escape cleanup | `internal/tui/tui.go` | Ensure terminal state restored on all exit paths |
| UTF-8 / wide chars | `internal/tui/layout.go` | Proper handling of CJK characters in narrow mode |
| Loading states | `internal/tui/pane/*` | Skeleton screens for initial load |
| Error boundaries | `internal/tui/tui.go` | Graceful degradation if a pane fails to render |
| No-color mode | All panes | Verify all panes work without ANSI colors |

**Acceptance criteria:**
- Copy/paste works in supported terminals
- No terminal corruption on exit
- CJK characters render correctly
- All panes degrade to plain text without color
- Error in one pane doesn't crash the entire TUI

---

## 6. Module/Seam Boundaries

```
+------------------------------------------------------------------+
|  cmd/lana/root/root.go                                           |
|    |                                                             |
|    +---> tui.Run(opts)                                          |
|             |                                                    |
|             v                                                    |
|  +----------------------------------------------------------+    |
|  |  internal/tui/                         (presentation)     |    |
|  |  ├── tui.go              Model orchestrator               |    |
|  |  ├── layout.go           Responsive layout                |    |
|  |  ├── keymap.go           Keyboard shortcuts               |    |
|  |  ├── pane/                                                |    |
|  |  |   ├── transcript.go   Chat transcript                  |    |
|  |  |   ├── tool.go         Tool call cards                  |    |
|  |  |   ├── input.go        Multi-line input                 |    |
|  |  |   ├── header.go       Header bar                       |    |
|  |  |   ├── status.go       Status bar                       |    |
|  |  |   ├── approval.go     Approval modal                   |    |
|  |  |   ├── session_list.go Session list                     |    |
|  |  |   +-- command_palette.go Command palette               |    |
|  |  +-- render/                                               |    |
|  |      ├── markdown.go     Markdown to lipgloss             |    |
|  |      ├── diff.go         Unified diff renderer            |    |
|  |      ├── syntax.go       Syntax highlighting              |    |
|  |      +-- spinner.go      Animated spinner                 |    |
|  +----------------------------------------------------------+    |
|             |            |            |                          |
|             v            v            v                          |
|  cli.Runtime      cli.ApprovalBroker   provider.Event            |
|  (business logic) (authorization)      (streaming data)          |
|             |            |            |                          |
|             v            v            v                          |
|  internal/cli         internal/tools    internal/provider        |
|  internal/session     internal/agent    internal/policy          |
+------------------------------------------------------------------+
```

**Hard boundaries:**
- `internal/tui/*` imports **only** `internal/cli`, `internal/provider`, `internal/tools`, `internal/session`, `internal/policy`, and Bubble Tea deps
- `internal/tui/*` does **not** import `internal/agents`, `internal/cmd/*`, `internal/github`, `internal/gitlab`, `internal/mcp`, `internal/plugin`, or any provider SDK
- `internal/tui/render/*` does **not** import any Lana internal packages — only lipgloss, chroma (if used), and Go stdlib
- The `Pane` interface is the only contract between `tui.go` and `pane/*`
- No business logic (authorization, tool execution, provider calls) lives in the TUI

---

## 7. Testing Strategy

### 7.1 Unit Tests

| Target | Approach |
|---|---|
| **Pane interfaces** | Mock panes; verify `Model.Update` routes messages correctly; verify focus changes |
| **Layout engine** | Test wide/medium/narrow calculations for various terminal sizes |
| **Keymap** | Verify each shortcut maps to correct action; verify conflicts |
| **Markdown renderer** | Golden tests: markdown input to expected lipgloss output |
| **Diff renderer** | Golden tests: unified diff input to colored output; verify no terminal escape injection |
| **Syntax highlighter** | Golden tests: code snippets to highlighted output for each supported language |
| **Spinner** | Verify frame rotation over time |
| **Command palette** | Filter logic; selection; execution |
| **Session list** | Data model; search; actions |

### 7.2 Integration Tests

| Target | Approach |
|---|---|
| **Event routing** | Feed mock `provider.Event`s; verify correct pane updates |
| **Approval flow** | Mock `ApprovalBroker`; verify modal display, y/n response, cancel |
| **Session resume** | Load session from store; verify transcript renders correctly |
| **Tool call lifecycle** | Simulate full tool call: call to running to result to render |

### 7.3 PTY / Black-Box Tests

| Target | Approach |
|---|---|
| **Full TUI session** | Run `lana` in a PTY; send input; capture output; verify rendering |
| **Resize handling** | Send `SIGWINCH`; verify layout adapts |
| **Ctrl+C cancellation** | Verify clean exit and terminal restoration |
| **Narrow terminal** | Run at 40 cols; verify stacked layout |
| **No-color mode** | Run with `TERM=dumb`; verify no ANSI codes |
| **UTF-8 / CJK** | Send CJK input; verify rendering |
| **Control character injection** | Send terminal escape sequences; verify they are escaped in output |

### 7.4 Golden Test Format

```go
func TestDiffRenderer(t *testing.T) {
    tests := []struct {
        name  string
        input string // unified diff
        want  string // expected lipgloss output (ANSI)
    }{
        {
            name:  "simple add",
            input: "--- a/foo.txt\n+++ b/foo.txt\n@@ -1 +1,2 @@\n-hello\n+hello\n+world\n",
            want:  "+hello\n+world\n", // with lipgloss green styling
        },
        // ...
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := renderDiff(tt.input)
            if got != tt.want {
                t.Fatalf("diff = %q, want %q", got, tt.want)
            }
        })
    }
}
```

---

## 8. Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| **Bubble Tea complexity** — Pane composition adds significant complexity to the Update loop | Medium | High | Start with 2 panes (transcript + input); add tool panel in Phase T2 only after foundation is solid |
| **Markdown rendering performance** — Real-time markdown during streaming can cause lag | Medium | Medium | Use incremental rendering; debounce; fall back to plain text during fast streaming |
| **Syntax highlighting dependencies** — chroma adds ~5MB binary size | Low | Low | Use lightweight custom renderer first; add chroma only if needed |
| **Terminal escape injection** — Tool output or provider text could contain escape sequences | Medium | High | All rendering goes through lipgloss; never `fmt.Print` raw text; sanitize all input |
| **Narrow terminal usability** — Stacked layout may be cramped | Medium | Medium | Careful width calculations; ensure minimum readable widths; test at 40 cols |
| **Regression of existing TUI** — Refactoring could break existing tests | High | Medium | Keep existing tests as regression baseline; refactor incrementally with tests passing at each step |
| **Provider unavailability** — TUI may hang if provider is unreachable | Low | High | Respect context cancellation; show error in transcript; allow user to retry |

---

## 9. Dependencies and Blockers

### 9.1 Technical Dependencies

| Dependency | Status | Notes |
|---|---|---|
| Bubble Tea v1.3.10 | Available | Already in go.mod |
| Bubbles v1.0.0 | Available | Already in go.mod (textarea, list, spinner) |
| Lipgloss v1.1.0 | Available | Already in go.mod |
| go-osc52 (clipboard) | Transitive dep | Available via `github.com/aymanbagabas/go-osc52/v2` |
| Chroma (syntax highlighting) | Optional | Not in go.mod; add only if custom renderer insufficient |
| Glamour (markdown) | Optional | Not in go.mod; consider if custom renderer insufficient |

### 9.2 Product Dependencies

| Dependency | Status | Impact |
|---|---|---|
| Phase 0 approval (provider, auth, security) | Not approved | TUI can be built and tested with mock provider; real integration requires Phase 0 |
| Provider adapter implementation | Not implemented | TUI works with any `provider.Client`; testing uses stubs |
| Tool executor implementation | Partial (builtins exist) | `read_file`, `write_file`, `exec`, `search` builtins exist; more tools expand rendering needs |
| Session store integration | Implemented | Already available via `internal/session` |
| Approval broker | Implemented | Already available via `internal/cli.ApprovalBroker` |

### 9.3 Blockers

1. **No provider configured** — Cannot do end-to-end testing with real model; must use mock/stub provider
2. **Phase 0 not approved** — Security decisions (sandbox, credential handling) affect what tools the TUI can display
3. **Existing TUI tests are limited** — Only 6 test functions; refactoring requires expanding test coverage first

---

## 10. Acceptance Criteria

### 10.1 Phase T1 (Foundation)
- [ ] `internal/tui` refactored to composable Pane architecture
- [ ] All existing TUI tests pass after refactor
- [ ] `go build ./cmd/lana` succeeds
- [ ] Basic conversation loop works: user input to assistant response to display
- [ ] Slash commands still functional
- [ ] History navigation works
- [ ] Ctrl+C cancellation works
- [ ] Approval y/n works

### 10.2 Phase T2 (Tool Panel)
- [ ] Tool calls render as cards with status icons
- [ ] `write_file` shows unified diff with +/- coloring
- [ ] `exec` shows stdout (monospace) and stderr (red)
- [ ] Running tool calls show animated spinner
- [ ] Approval requests show as modal overlay
- [ ] Tool cards can be collapsed/expanded

### 10.3 Phase T3 (Markdown & Syntax)
- [ ] Assistant messages render with markdown formatting
- [ ] Code blocks are syntax-highlighted
- [ ] Streaming text renders incrementally
- [ ] Narrow terminal degrades gracefully

### 10.4 Phase T4 (Layout & Navigation)
- [ ] Side-by-side layout at wide terminals (>=100 cols)
- [ ] Stacked layout at narrow terminals (<60 cols)
- [ ] Tab key cycles focus between transcript, tool, input
- [ ] Scroll works with mouse and keyboard
- [ ] Command palette filters and executes
- [ ] All keyboard shortcuts from section 3.3 functional

### 10.5 Phase T5 (Session Management)
- [ ] Session list accessible via Ctrl+O
- [ ] Can switch between sessions
- [ ] Session fork via Ctrl+F
- [ ] Export session as markdown

### 10.6 Phase T6 (Polish)
- [ ] Copy/paste works in supported terminals
- [ ] No terminal corruption on any exit path
- [ ] CJK characters render correctly
- [ ] No-color mode works for all panes
- [ ] Error in one pane doesn't crash TUI

### 10.7 Cross-Phase Criteria
- [ ] `go test ./internal/tui/...` passes (unit + integration)
- [ ] PTY black-box tests pass for: TTY, piped, resize, UTF-8, no-color, Ctrl+C
- [ ] Golden tests for diff renderer, markdown renderer, syntax highlighter
- [ ] No regression in non-TUI commands (`lana exec`, `lana agents`, `lana file`, etc.)
- [ ] Binary size increase <= 2MB (excluding optional chroma/glamour)

---

## Appendix A: Comparison with Existing Codex CLI

| Feature | OpenAI Codex CLI | Lana Target |
|---|---|---|
| Layout | Split pane (transcript + tool) | Split pane (same) |
| Markdown | Rendered | Rendered (custom lightweight) |
| Syntax highlighting | Chroma-based | Custom or Chroma |
| File diffs | Unified diff view | Unified diff with lipgloss |
| Command output | Monospace stdout/stderr | Monospace stdout/stderr |
| Tool status | Icons + spinner | Icons + spinner |
| Keyboard shortcuts | Extensive | Comprehensive (target section 3.3) |
| Session management | List, switch, fork | List, switch, fork |
| Copy/paste | OSC 52 | OSC 52 (already transitive dep) |
| Provider | OpenAI only | Provider-neutral (any) |
| Sandbox | Built-in | Policy-based (existing) |
| Approval | Interactive | Interactive (existing broker) |

## Appendix B: Recommended Dependency Additions

Only add if the lightweight custom approach is insufficient:

```go
// Option 1: Chroma for syntax highlighting (recommended if needed)
github.com/alecthomas/chroma/v2 v2.14.0

// Option 2: Glamour for full markdown rendering (heavier)
github.com/charmbracelet/glamour v0.8.0
```

**Recommendation:** Implement custom lightweight markdown + syntax renderer first (Phase T3). Evaluate adding chroma/glamour only if the custom renderer doesn't meet quality expectations.

## Appendix C: File Change Summary

| File | Action | Description |
|---|---|---|
| `internal/tui/tui.go` | Refactor | Thin orchestrator; delegate to panes |
| `internal/tui/tui_test.go` | Refactor + extend | Update existing tests; add pane/layout tests |
| `internal/tui/pane/pane.go` | **New** | Pane interface definition |
| `internal/tui/pane/transcript.go` | **New** | Transcript pane |
| `internal/tui/pane/tool.go` | **New** | Tool call card pane |
| `internal/tui/pane/input.go` | **New** | Input bar pane |
| `internal/tui/pane/header.go` | **New** | Header bar pane |
| `internal/tui/pane/status.go` | **New** | Status bar pane |
| `internal/tui/pane/approval.go` | **New** | Approval modal |
| `internal/tui/pane/session_list.go` | **New** | Session list modal |
| `internal/tui/pane/command_palette.go` | **New** | Command palette |
| `internal/tui/render/markdown.go` | **New** | Markdown renderer |
| `internal/tui/render/diff.go` | **New** | Diff renderer |
| `internal/tui/render/syntax.go` | **New** | Syntax highlighter |
| `internal/tui/render/spinner.go` | **New** | Spinner frames |
| `internal/tui/keymap.go` | **New** | Keyboard shortcuts |
| `internal/tui/layout.go` | **New** | Responsive layout |
| `docs/codex-tui-feature-plan.md` | **New** | This document |

**Total: 15 new files, 3 refactored files.**
- **Context indicator**: Shows approximate token/context usage if provider supports it