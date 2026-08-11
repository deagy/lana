# Lana — Module Design Baseline

## Version: 1.1 (revised draft)
## Date: 2026-08-10

> Proposed module responsibilities for the revised G3 baseline. Planned modules are labelled; this document does not declare them implemented or approved.

## 1. Package map

```
github.com/deagy/lana
├── cmd/lana/                         # process entry point
├── cmd/lana/root/                    # cobra wiring and composition
├── internal/app/                     # immutable app/config/logger context
├── internal/cli/                     # portable conversation runtime/renderers
├── internal/tui/                     # Bubble Tea terminal interface
├── internal/agent/                   # bounded provider/tool turn loop
├── internal/provider/                # neutral stream contract and adapters
├── internal/session/                 # append-only JSONL sessions
├── internal/tools/                   # tool schemas, authorizer, executor seam
├── internal/policy/                  # local operation policy, not OS sandbox
├── internal/mcp/              (plan) # local stdio MCP lifecycle and mediation
├── internal/extension/        (plan) # local/Git source, trust, capabilities
├── internal/auth/             (plan) # OpenAI API-key/OAuth acquisition facade
├── internal/release/          (plan) # artifact metadata/verification helpers
├── internal/cmd/                      # current local CLI command groups
└── pkg/
    ├── config/                       # precedence, validation, redaction
    ├── logger/                       # local structured logging
    ├── sandbox/                      # policy facade, not OS containment
    └── version/                      # build metadata
```

The current root command wires only `exec`, `file`, `sdlc`, `system`, and `completion` in addition to the root conversation path. Other `internal/cmd/*` packages remain source-visible legacy capability and are not current root commands; they are nevertheless a release-compatibility concern if reintroduced, embedded, or presented as supported. The concrete v1 disposition, including `sdlc` read-only containment and the legacy `goal`/`plan` packages, is in [compatibility-surface.md](compatibility-surface.md).

## 2. Implemented-design seams

### 2.1 `internal/provider`

`provider.Client` exposes `Stream(context.Context, Request) (Stream, error)`. `Request` carries portable messages and tool definitions; `Event` is a schema-versioned envelope. An adapter wraps a concrete backend and redacts credential-shaped errors before returning them to the turn runtime.

Required extension: an OpenAI backend that maps the selected OpenAI API/key or OAuth credential into an adapter-private request. Authentication configuration must not become fields on `provider.Request`, `provider.Event`, or `session.Record`.

Failure contract: an absent adapter/backend fails readiness or stream creation; a backend error crosses the adapter through `provider.Redact`; a received event is sanitized and schema-validated before rendering or persistence. This is the interface decision that realizes AD-002/AD-003 and traces to FR-002–004, NFR-002, SEC-001, and SEC-003. Provider selection, endpoint policy, OAuth topology, credential store, rotation, and revocation are not module defaults; they remain G4/G5 unresolved.

### 2.2 `internal/cli`, `internal/agent`, and `internal/tui`

`cli.Runtime` owns in-memory history and session persistence. `agent.TurnRunner` owns the provider/tool loop; its injected `Authorizer` and `Executor` prevent a provider response from bypassing user authority. `tui.Model` renders streaming output and acts as the interactive approval presenter.

The runtime must continue to reject an unavailable provider before a session is created. For noninteractive commands, a G5-approved explicit authorization mode is required; `tools.AllowAll` is test/embedding support, not a release default.

The selected seam avoids an alternative in which provider events execute local operations directly. A tool call must flow through `tools.Call` validation, `Authorizer`, and policy before `Executor`; denial, cancellation, and unenforceable decisions become typed tool results. This traces AD-005/AD-006 to FR-007–009, NFR-002, SEC-002, and SEC-004.

### 2.3 `internal/session`

The session store writes one JSON object per line with `schema_version`, `session_id`, `sequence`, timestamp, kind, and data. Files use owner-only mode on creation, append with synchronization, and are recovered by retaining the valid prefix of a torn final record. `Create`, `Append`, `Load`, `List`, and `Fork` are the persistent-session contract.

Session payload types must stay portable and redacted. Provider credentials, OAuth tokens/codes, extension secrets, and raw environment values must never enter this store.

Failure handling is deliberately local: append/load/recovery failures stop the relevant operation; a torn final record is repaired only by explicit recovery; resume/fork require recovery. Remote synchronization and a retention/record-ownership scheme are not alternatives selected by this module. This traces AD-004 to FR-005–006, NFR-003, and SEC-001.

### 2.4 `internal/tools` and `internal/policy`

`tools.Call` validates a tool id, name, and JSON arguments. `tools.Authorizer` is distinct from `tools.Executor`; `tools.Result` is a versioned, serializable reply to the provider. Built-in definitions cover workspace read/write/search and command execution.

`policy.Policy` returns `allow`, `deny`, `require-approval`, or `unenforceable` with risk and a canonical path. It resolves symlinks but does not eliminate mutation races and does not restrict an executed process at the OS boundary. Therefore callers must preserve its fail-closed decision: a non-unrestricted execute and a `workspace-write` mutation are currently unenforceable, not permitted.

`pkg/sandbox` is a convenience facade over this policy result, not an isolation implementation. The alternative—calling it a sandbox or accepting a best-effort write check—is specifically not selected. Descriptor-relative/no-follow filesystem operations and OS-level process containment are G5 completion work (FR-009, SEC-002).

## 3. Planned modules and contracts

### 3.1 `internal/auth` — OpenAI credential facade

```go
type Method string
const (
    MethodAPIKey Method = "api_key"
    MethodOAuth  Method = "oauth"
)

type Credential struct {
    Provider string
    Method   Method
    // Secret material is deliberately unexported and never serializable.
}

type Store interface {
    Load(context.Context, string) (Credential, error)
    Save(context.Context, Credential) error
    Delete(context.Context, string) error
}

type OAuthFlow interface {
    Start(context.Context) (Authorization, error)
    Complete(context.Context, Callback) (Credential, error)
    Refresh(context.Context, Credential) (Credential, error)
}
```

The concrete store, OAuth flow (redirect or device), token rotation/revocation, and secure platform support are G5 dependencies. A file-backed plaintext implementation is prohibited.

### 3.2 `internal/mcp` — local stdio only

```go
type ServerDescriptor struct {
    Name              string
    Command           string
    Args              []string
    WorkingDirectory  string
    EnvironmentAllow  []string
    Timeout           time.Duration
    Limits            ResourceLimits
    Trusted           bool
}

type ResourceLimits struct {
    MaxMessageBytes  int64
    MaxResourceBytes int64
}

type Client interface {
    ListResources(context.Context, string) ([]Resource, error)
    ReadResource(context.Context, string, string) ([]Content, error)
    ListTemplates(context.Context, string) ([]ResourceTemplate, error)
    ListTools(context.Context, string) ([]Tool, error)
    CallTool(context.Context, string, string, json.RawMessage) (tools.Result, error)
    Close() error
}
```

Only a local child process with stdio is valid. The module must require and validate command, arguments, working directory, environment allowlist, timeout, and both resource limits; it must bound message/output sizes, propagate cancellation, redact diagnostics, and route calls through authorization/capability policy. HTTP/SSE/URI transports and anonymous/untrusted launch are excluded. This baseline intentionally sets no descriptor defaults.

The selected interface deliberately does not treat `Trusted bool` as sufficient evidence by itself: the implementing boundary must bind it to validated identity/trust data before launch. This interface must fail before launch/call on a remote or incomplete descriptor, missing trust/capability data, timeout, cancellation, or oversized input/output. Child identity, environment semantics, capability enforcement, and exact resource/message limits are G5-unresolved; they are not supplied by this baseline (AD-007; FR-010; SEC-004).

### 3.3 `internal/extension` — provenance, trust, and capabilities

```go
type SourceKind string
const (
    SourceLocal SourceKind = "local"
    SourceGit   SourceKind = "git"
)

type Provenance struct {
    Kind     SourceKind
    Location string
    Revision string // required for Git activation
    Digest   string
}

type Manifest struct {
    Name         string
    Version      string
    Capabilities []string
}

type TrustRecord struct {
    Provenance Provenance
    Granted    []string
    AcceptedAt time.Time
}
```

Install copies or fetches an explicit source, validates the manifest, and records immutable provenance. Activation requires a matching trust record. Capability enforcement, Git signature/identity rules, and source-license policy must be completed through G4/G5; absence of enforcement means deny activation.

Meaningful alternatives are intentionally deferred or rejected: a marketplace, background update, manifest-only activation, and a mutable unpinned Git source are not v1 activation paths. An unsupported source, fetch failure, malformed manifest, provenance mismatch, absent trust record, or unenforceable grant leaves the extension inactive (AD-008; FR-011–012; SEC-004). G4/G5 must resolve licensing/information-handling applicability, Git verification, and capability mediation.

### 3.4 `internal/release` and build automation

The release flow must produce versioned Linux/macOS binaries or archives, a checksum manifest, a signature, and source-revision metadata. Verification must fail if the artifact, digest, signature, and revision do not bind together. The module/automation must not hold signing authority; signing identity selection, key custody, macOS codesign/notarization, and provenance format remain G5-controlled.

The alternative of treating a successful local build, an archive, or a checksum alone as a release-integrity claim is not selected. Missing or mismatched binding data is a verification failure. This traces AD-010 to FR-015, NFR-004, and SEC-005; signing identity, key/certificate custody, notarization, and attestation remain G5-unresolved.

## 4. Configuration additions (planned)

Configuration keeps the existing precedence and redaction boundary. A future public shape may contain nonsecret references only:

```yaml
provider:
  kind: openai
  model: ""
  auth: oauth              # api_key | oauth; no secret value
  credential_ref: "openai/default"
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
extensions:
  directories: []
  enabled: []               # entries require a matching trust record
telemetry:
  enabled: false            # fixed false; not an opt-in product feature
```

The present configuration type is not yet this final contract. Every MCP field shown is required and the placeholders assert no default. Credentials must be supplied through the approved auth mechanism, never the YAML, environment dump, or CLI history. Configuration rendering uses `RedactedConfig` or the equivalent future redaction boundary.

## 5. Module invariants

| Invariant | Enforced by / evidence needed |
|---|---|
| Credentials never cross into durable/session/log contracts | Auth/provider boundary; redaction and persistence scans |
| A provider cannot execute a tool without an authority decision | Turn runner + authorizer integration test |
| Local policy cannot be presented as OS sandboxing | Policy result semantics and CLI/TUI wording tests |
| Remote MCP is impossible through the release surface | Descriptor/transport validation tests |
| Extensions are untrusted until provenance and trust match | Install/activation negative tests |
| No telemetry code path exists | Dependency, endpoint, and runtime-network negative evidence |
| Artifact signatures bind an auditable source revision | Release rehearsal and independent verification |

## 6. Unresolved control points

The interfaces above deliberately leave authority in the proper G4/G5 decisions: credential storage/OAuth topology; macOS and Linux signing identity; process/filesystem containment; noninteractive permission policy; MCP and extension capability mediation; Git-source verification/licensing; and no-telemetry evidence. No retention duration, record owner, or gate outcome is selected here. No module is permitted to substitute a convenience default for these decisions.

## 7. Interface-to-requirement map

| Interface/flow | Selected failure semantics | Trace |
|---|---|---|
| `provider.Client.Stream` and `provider.Event` | Fail readiness/stream setup without a configured adapter; redact and reject invalid events before presentation/persistence. | FR-002–004, NFR-002, SEC-001, SEC-003 |
| `cli.Runtime` and `session.Store` | Do not continue a turn on persistence failure; recover only the valid record prefix before resume/fork. | FR-001, FR-005–006, NFR-003 |
| `tools.Authorizer` → `policy.Policy` → `tools.Executor` | Deny or emit a typed unenforceable result rather than bypass the boundary. | FR-007–009, SEC-002, SEC-004 |
| planned `internal/mcp.Client` | Reject invalid/remote/untrusted/incomplete descriptors before launch or call; bound mediated traffic. | FR-010, NFR-001, SEC-004 |
| planned `internal/extension` | Keep an extension inactive without matching provenance, trust, and enforceable grants. | FR-011–012, SEC-004 |
| planned `internal/release` | Treat an unbound artifact/digest/signature/revision tuple as verification failure. | FR-015, NFR-004, SEC-005 |
