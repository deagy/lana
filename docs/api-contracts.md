# Lana — API and Data Contracts Baseline

## Version: 1.1 (revised draft)
## Date: 2026-08-10

> Contracts labelled “planned” define the intended release surface. They do not claim an implementation, security approval, or release authorization.

## 1. CLI surface: current versus target contract

The current root command exposes this command tree (as verified from `cmd/lana/root`):

```text
lana [--workspace PATH] [--config PATH] [--verbose LEVEL] [--json] [--quiet] [PROMPT...]
lana exec [--model MODEL] [--session ID] [--jsonl] [PROMPT]
lana file {read|write|delete|copy|move|search|info} ...
lana sdlc {status|list-runs|show-run|read-plan|read-record} ...
lana system {version|health|schema|config|env|dirs} ...
lana completion {bash|zsh|fish|powershell}
```

`--workspace` selects the workspace; `--config` selects the project configuration source and does not change the workspace. `--json` selects JSON diagnostic logging; it is not conversation event output. `--jsonl` belongs only to `lana exec` and writes one sanitized, validated `provider.Event` JSON object per line to stdout, with no header, status line, ANSI escape, or final summary. Without `--jsonl`, plain stdout contains assistant text only; tool notices are written to stderr and a successful command appends one newline. Errors use the command error path rather than a synthetic JSONL error record.

The root conversation selects the interactive UI only when both standard input and output are terminals; otherwise it reads a positional prompt or standard input and uses the plain renderer. The root currently has no configured conversational provider unless an embedding application injects the runtime dependencies, so the ordinary binary fails readiness before creating a session. That current behavior must not be documented as a working provider integration.

The target v1 conversation surface is:

```text
lana [--workspace PATH] [--config PATH] [--verbose LEVEL] [--json] [PROMPT...]
lana exec [--model MODEL] [--session ID] [--jsonl] [PROMPT...]
```

When a G4/G5-resolved provider integration is available, the target behavior is the same terminal/nonterminal selection and renderer semantics above. `--session` is the published resume spelling; the source retains `--resume` only as a hidden compatibility alias. The target must fail secret-safely before session creation for an unavailable provider (FR-001–004, NFR-002, SEC-001, SEC-003).

Interactive slash commands are local UI commands:

```text
/help
/status
/model [name]
/permissions [mode]
/resume <session-id>
/new
/quit
```

`/permissions` displays or selects only a G5-approved policy mode. It must not claim OS containment or enable an implicit noninteractive allow-all authority.

The slash commands are target interactive behavior, not separately registered Cobra commands. `file` is the current direct CLI family and is policy-enforced, but its `workspace-write` mutation path is presently required to fail as unenforceable until G5 containment work exists. Direct file-command behavior does not replace the conversational FR-007 authorization flow (FR-007–009, SEC-002).

### 1.1 Planned authentication commands

```text
lana auth login --provider openai [--method oauth|api-key]
lana auth logout --provider openai
lana auth status --provider openai
```

`login --method api-key` obtains a key without echoing it and stores only a G5-approved credential reference. `login --method oauth` starts the approved OAuth flow and reports no authorization code, access token, or refresh token. `status` reports configuration state, never secret material. Exact OAuth redirect/device-flow flags remain pending G5.

These commands are not in the current root command tree. Provider choice, supported endpoints, OAuth flow shape, credential-store behavior, token lifecycle, and provider information-handling applicability remain G4/G5 unresolved. Their names therefore describe a target contract, not availability.

### 1.2 Planned local MCP commands

```text
lana mcp list-resources [--server NAME]
lana mcp read-resource --server NAME --uri URI
lana mcp list-templates [--server NAME]
lana mcp list-tools [--server NAME]
lana mcp call-tool --server NAME --tool NAME --arguments JSON
```

`NAME` identifies a configured **local stdio** server. The command rejects unknown servers, remote URI/transport descriptors, invalid resource URIs/arguments, incomplete or untrusted launch descriptors, oversized content, and denied/unenforceable actions. Tool calls use the same authorization outcome contract as built-in tools.

These exact target flags deliberately differ from the legacy `internal/cmd/mcp` package (`--args`, URI-bearing configuration and stub/list output), which is not registered on the current root command. The target must not claim MCP connectivity until its local-stdio manager, authorization path, and G5 controls are implemented. Child-process identity, environment/capability mediation, and concrete limits are G5-unresolved (FR-010, SEC-004).

### 1.3 Planned extension commands

```text
lana extension install --source PATH_OR_GIT [--revision REVISION]
lana extension list
lana extension trust <name> --provenance DIGEST --grant CAPABILITY[,CAPABILITY...]
lana extension enable <name>
lana extension disable <name>
```

`install` accepts an explicit local source or Git source only. A Git source must be pinned to a recorded revision before activation. `trust` is an explicit user decision and records source provenance plus grants; `enable` rejects an extension without a matching trust record or enforceable grants. Marketplace discovery and automatic update are not contracts.

These commands are target names, not current root commands. The legacy `plugin` and `skill` packages use different names and can write/enable local content without the target provenance/trust/capability contract; their v1 containment is specified in [compatibility-surface.md](compatibility-surface.md). Git-source integrity, licensing/information-handling applicability, and capability enforcement remain G4/G5 unresolved (FR-011–012, SEC-004).

### 1.4 Local tool result contract

Provider-visible tool results use a versioned envelope:

```json
{
  "schema_version": 1,
  "call_id": "call-123",
  "name": "read_file",
  "content": {"path":"main.go","content":"package main\\n"},
  "is_error": false,
  "error_code": "",
  "at": "2026-08-10T00:00:00Z"
}
```

On denial or an unenforceable boundary, `is_error` is `true` and `error_code` is a stable nonsecret identifier such as `authorization_denied`, `policy_denied`, or `policy_unenforceable`. Error content must not contain secrets or raw credentials.

## 2. Provider-neutral streaming contract

```go
type Request struct {
    Model    string
    Messages []Message
    Tools    []ToolDefinition
    Metadata map[string]string
}

type Event struct {
    SchemaVersion int
    ID            string
    TurnID        string
    Type          EventType // message.start, message.delta, tool.call, message.end, error
    At            time.Time
    Data          json.RawMessage
}

type Client interface {
    Stream(context.Context, Request) (Stream, error)
}
```

`Request`, `Event`, and `Message` contain no credential field. OpenAI API-key and OAuth credentials are adapter-private and injected only for the provider request. Provider errors cross into Lana only through a redacting adapter.

## 3. Session file contract

Session files reside at `<workspace>/.lana/sessions/<session-id>.jsonl`, with directory mode `0700` and file mode `0600` where supported. They are append-only JSON Lines, one schema-versioned record per line:

```json
{
  "schema_version": 1,
  "session_id": "session-123",
  "sequence": 2,
  "at": "2026-08-10T00:00:00Z",
  "kind": "message.user",
  "data": {"role":"user","content":"Explain this repository"}
}
```

Valid record kinds include `session.created`, `session.forked`, `message.user`, `provider.event`, `tool.result`, and `message.assistant`. `session.created` is sequence 1. Sequence values increase by one; the store assigns them. A torn final record is removed only during explicit recovery; a recovered session carries `recovered: true`. A forked session records its immutable `parent_id`.

Session data MUST NOT contain provider keys, OAuth material, extension secrets, raw unredacted configuration, or telemetry identifiers.

## 4. Planned configuration contract

Configuration precedence is: defaults < user config < workspace config < `LANA_*` environment < CLI flags. The current configuration has legacy MCP URI and compatibility fields; it cannot remain v1-compatible merely by being parsed. The target validator must reject those legacy remote/URI descriptors before connection or launch, as recorded in [compatibility-surface.md](compatibility-surface.md).

```yaml
workspace:
  path: "."
logging:
  level: info
  format: text
provider:
  kind: openai
  model: ""
  auth: oauth
  credential_ref: openai/default
mcp:
  servers:
    - name: local-tools
      transport: stdio
      command: <required-absolute-command-path>
      args: <required-array>
      working_directory: <required-absolute-directory>
      environment_allowlist: <required-array>
      timeout: <required-duration>
      resource_limits:
        max_message_bytes: <required-positive-integer>
        max_resource_bytes: <required-positive-integer>
exec:
  sandbox: workspace-write
  timeout: 60s
extensions:
  directories: []
telemetry:
  enabled: false
```

Each MCP descriptor field shown above is required for v1 validation; values are examples, not defaults. Secret values are invalid in this configuration contract. Configuration introspection and diagnostics must redact URI userinfo, tokens, keys, authorization values, and secret-shaped parameters. `telemetry.enabled` is fixed false; it is a boundary assertion, not an opt-in collection switch.

## 5. Planned MCP and extension data contracts

```json
{
  "name": "local-tools",
  "transport": "stdio",
  "command": "<required-absolute-command-path>",
  "args": "<required-array>",
  "working_directory": "<required-absolute-directory>",
  "environment_allowlist": "<required-array>",
  "timeout": "<required-duration>",
  "resource_limits": {
    "max_message_bytes": "<required-positive-integer>",
    "max_resource_bytes": "<required-positive-integer>"
  },
  "trusted": true
}
```

The target public configuration must not accept a transport URI or `http`, `https`, SSE, or WebSocket MCP transport. Values shown are examples only; no runtime defaults are asserted by this contract. The current legacy `mcp.servers[].uri` field is an observed compatibility surface, not evidence that this target validation exists.

```json
{
  "name": "example-extension",
  "version": "1.2.0",
  "provenance": {
    "kind": "git",
    "location": "https://example.invalid/owner/repo.git",
    "revision": "40-hex-or-immutable-revision",
    "digest": "sha256:..."
  },
  "capabilities": ["workspace.read"],
  "trusted": false
}
```

An installed extension is not active merely because it has a parseable manifest. Activation requires a matching explicit trust record and enforceable grants. The exact manifest schema, Git identity verification, capability enforcement, and information-handling lifecycle are pending G4/G5; this contract assigns no retention duration or record owner.

## 6. Release-artifact contract

Each Linux/macOS artifact must be accompanied by:

```text
lana_<version>_<os>_<arch>.{tar.gz,zip}
lana_<version>_<os>_<arch>.sha256
lana_<version>_<os>_<arch>.sig
release-metadata.json
```

`release-metadata.json` must identify the semantic version, source revision, build timestamp, platform/architecture, digest, signing identity/key fingerprint, signature format, and verification command. A release is invalid if verification cannot bind its artifact to its digest, signature, and source revision. macOS codesigning/notarization and Linux signing-key authority are explicitly pending G5.

## 7. Non-contractual/blocked surfaces

Published compatibility commands are subject to concrete v1 release blockers, not merely a scope disclaimer. [compatibility-surface.md](compatibility-surface.md) requires removal or deterministic pre-action rejection for remote Git, dispatch, legacy MCP URI configuration, plugin/skill, knowledge, legacy shell, and state-mutating lifecycle/goal/plan commands. The rooted `lana sdlc` commands are read-only inspection only and must remain so. Other future remote-control, cloud-session, marketplace, telemetry-upload, and lifecycle-gate-approval surfaces require explicit scope re-entry and, where applicable, a separate contract and gate review.

## 8. Contract failure and trace map

| Contract boundary | Required failure/output semantics | Trace |
|---|---|---|
| Root/`exec` conversation | Fail readiness before session creation when dependencies are unavailable; plain versus JSONL output stays protocol-specific as described above. | FR-001–004, NFR-002, SEC-001, SEC-003 |
| Session JSONL | Reject invalid writes; recover only a torn final record's valid prefix before resume/fork. | FR-005–006, NFR-003 |
| Tool result | Return a versioned result with `is_error: true` and a stable nonsecret error code for denial or unenforceable policy. | FR-007–009, SEC-002 |
| Target MCP/extension | Reject before action if descriptor/provenance/trust/grant data is invalid, remote, absent, or unenforceable; do not imply current availability. | FR-010–012, SEC-004 |
| Target release metadata | Verification fails if artifact, digest, signature, and source revision do not bind. Signing implementation remains G5-unresolved. | FR-015, NFR-004, SEC-005 |
