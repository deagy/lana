# Lana — Architecture Baseline

## Version: 1.1 (revised draft)
## Date: 2026-08-10
## Classification: internal

> This architecture records a proposed local-only design. It does not approve G3, G4, G5, release, or residual risk.

## 1. Architecture overview

Lana is a local Go process. The terminal interface, conversation runtime, durable session store, authorization broker, policy evaluator, local MCP client, and extension manager run on the developer's machine. A provider adapter is the narrow boundary for a user-selected model provider. No Lana-operated service participates in normal operation.

```
                              explicit user configuration
                                           |
┌──────────────────── local Lana process ─────────────────────────────┐
│ cmd/lana + cobra                                                     │
│   ├─ terminal TUI / plain renderer                                   │
│   ├─ app/config/logger (redacted diagnostics only)                   │
│   └─ cli.Runtime                                                     │
│       ├─ session.Store → <workspace>/.lana/sessions/*.jsonl          │
│       ├─ provider.Client → provider adapter ───────────────┐         │
│       ├─ approval broker → user allow/deny                 │         │
│       └─ agent turn kernel                                 │         │
│           └─ tool registry → policy → local file/process   │         │
│                                                             │         │
│ local MCP manager → configured child process over stdio ────┼────┐    │
│ extension manager → local path / explicit Git source        │    │    │
└────────────────────────────────────────────────────────────┼────┼────┘
                                                             │    │
                                              configured model API    │
                                              or OpenAI OAuth         │
                                                             │    │
                                                     model provider  local MCP
```

The only intended outbound flows are: a configured provider request, an explicit OpenAI OAuth flow, and an explicitly requested Git fetch of a trusted extension source. Local MCP uses child-process stdio; it does not authorize remote MCP transport. There is no telemetry, analytics, crash reporting, remote queue, or Lana control-plane connection.

## 2. Decisions, traceability, and alternatives

| ID | Selected decision and boundary | Meaningful alternative considered | Requirement trace |
|---|---|---|---|
| AD-001 | Cobra command wiring plus Bubble Tea for the interactive TUI; a non-TTY invocation uses the plain renderer. | A TUI-only surface would exclude automation; a separate remote/web UI would exceed the local-first boundary. | FR-001, NFR-001, NFR-002 |
| AD-002 | `provider.Client` exposes portable requests, versioned events, and streams, not provider SDK types or credentials. | Letting the turn kernel call an SDK directly would couple sessions, tools, and error behavior to a provider. This remains selected; concrete provider support is deferred. | FR-002, FR-004, NFR-002, SEC-001, SEC-003 |
| AD-003 | Authentication is adapter-private and separate from runtime and persistence. Credentials never cross the event/config/session/log boundary. | Plaintext config/session credentials are prohibited. OAuth callback or device-flow selection and credential-store implementation are G5-unresolved. | FR-003, FR-004, FR-013, SEC-001, SEC-003 |
| AD-004 | Use schema-versioned append-only JSONL sessions, valid-prefix recovery, and immutable parent references for forks. | A mutable transcript or remote/shared history would weaken local recovery and exceed scope. Information-handling lifecycle is not decided here. | FR-005, FR-006, NFR-003, SEC-001 |
| AD-005 | Separate provider, authorizer, and executor; the interactive presenter collects the allow/deny decision before a tool executes. | Provider-directed execution or an implicit noninteractive allow-all route is not selected. Noninteractive authority semantics remain G5-unresolved. | FR-007, FR-008, SEC-004 |
| AD-006 | Treat policy modes as policy, not an OS sandbox. Enforce known path checks and fail closed for unenforceable contained mutation/execution. | Claiming canonicalization alone provides containment is rejected. Descriptor-relative no-follow writes and process containment are deferred to G5. | FR-009, NFR-002, SEC-002 |
| AD-007 | Target MCP is a configured local child process over stdio, mediated by the tool boundary. | HTTP/SSE/WebSocket/URI transports and anonymous launch are excluded. Exact child identity, environment, capability, and limit enforcement are G5-unresolved. | FR-010, SEC-004, NFR-001 |
| AD-008 | Target extensions start from an explicit local path or Git source, record provenance, then require an explicit trust record and enforceable grants before activation. | Marketplace discovery, silent updates, and manifest-only trust are rejected. Git identity/integrity and capability mediation remain G4/G5-unresolved. | FR-011, FR-012, SEC-004 |
| AD-009 | No Lana telemetry architecture: no collector, queue, crash reporter, or background scheduler; logs are local and redacted. | An opt-in telemetry path is not a v1 alternative. Negative dependency/endpoint and runtime evidence remains required. | FR-014, NFR-001, SEC-001 |
| AD-010 | Target Linux/macOS release artifacts bind version, digest, signature, and source revision. | An unsigned artifact or a checksum unbound to source is insufficient. Signing identity, key custody, notarization, and provenance format remain G5-unresolved. | FR-015, NFR-004, SEC-005 |

## 3. Component responsibilities

| Component | Responsibility | Must not own |
|---|---|---|
| `cmd/lana`, `cmd/lana/root` | Command wiring, configuration resolution, renderer selection | Provider credentials, direct tool authority |
| `internal/tui` | Interactive transcript, slash commands, cancellation, approval prompt | Authorization policy or secret storage |
| `internal/cli` | Turn lifecycle, portable request construction, session continuation | Provider SDK configuration or platform sandbox claims |
| `internal/agent` | Provider stream/tool-call loop with bounded rounds | Credential storage and UI decisions |
| `internal/provider` | Provider-neutral contract, adapter error redaction | Secrets in events/logs; persistent configuration ownership |
| `internal/session` | Ordered append-only records, load/recover/list/fork behavior | Network sync, shared history, telemetry export, or information-lifecycle policy |
| `internal/tools` | Portable tool schemas, authorizer/executor separation | Allow-by-default policy in interactive use |
| `internal/policy` | Path canonicalization, risk categorization, fail-closed decisions | OS sandbox guarantee |
| MCP module (planned) | Local stdio server lifecycle and request mediation | Remote transports or implicit server trust |
| extension module (planned) | Local/Git provenance, manifest validation, trust/capability record | Marketplace discovery, silent updates, unmediated privilege |
| release pipeline (planned) | Build, checksums, signatures, provenance evidence | Gate approval or signing-key authority |

## 4. Core flows

### 4.1 Conversation and session flow

1. The root command resolves a canonical workspace and immutable configuration, exposing only a redacted configuration representation to diagnostics.
2. The runtime validates that a provider, authorizer, and executor are configured before opening the TUI or creating a session.
3. On the first prompt it creates an append-only session record. It appends the user message, provider events, tool results, and final assistant message in order.
4. The provider adapter streams normalized events. Adapter errors are redacted before entering the runtime.
5. A tool-call event goes to the authorization broker. The UI allows or denies it; only then may the executor invoke the local operation.
6. Cancellation propagates through the stream and tool context. A resume reconstructs portable user/assistant history; a torn final JSONL record is recovered before fork/resume can proceed.

### 4.2 Tool/policy flow

```
provider tool.call → validate schema → user authorization → policy.Evaluate
                                                        ├─ allow → executor
                                                        ├─ deny/approval → typed error result
                                                        └─ unenforceable → typed error result
```

`workspace-write` is deliberately not treated as contained while mutation is subject to symlink races. `workspace-read-only` and `workspace-write` reject execution because the current policy cannot enforce a process boundary. `unrestricted` is an explicit authority mode, not a safe default.

### 4.3 MCP and extension flow

1. A configuration entry names a local stdio MCP command and its constrained launch settings.
2. The MCP manager validates the entry, starts the child only after the required trust/approval decision, and mediates resource/tool calls through the tool authorization path.
3. An extension install starts from a user-selected local path or Git source. It records source and revision, validates the manifest, and remains disabled pending an explicit recorded trust decision.
4. Capability requests are evaluated individually. Missing enforcement capability is a denial, not an implicit grant.

## 5. Data and trust boundaries

| Data | Location | Trust rule |
|---|---|---|
| Provider credentials | OS credential facility or G5-approved secret mechanism | Never session/config/log data; not passed to plugins or MCP by default |
| Conversation transcript | `.lana/sessions/*.jsonl` within the workspace | Owner-only permissions where supported; append-only, versioned, recoverable |
| Provider events/tool results | Session records | Redacted at inbound error/diagnostic boundaries; size limits required for G5 design |
| MCP launch descriptor | Local configuration | User supplied; command/environment/capabilities require validation and mediation |
| Extension source/trust record | Local extension state | Source/revision immutable; activation requires an explicit trust record |
| Logs | Local stderr or configured local file | Structured, redacted, no remote exporter |
| Release artifacts | Release distribution | Digest/signature/source revision must agree |

## 6. G4/G5 design dependencies

The following are intentionally unresolved and block the corresponding operational claims: provider/OAuth privacy and information-handling terms; secret-store and OAuth callback/device-flow design; noninteractive tool authority; race-safe filesystem mutation and OS-level process containment; MCP child-process and extension capability enforcement; Git-source integrity/licensing policy; signing identity/key custody/macOS notarization; and evidence proving no telemetry dependencies or endpoints. These are not accepted risks. This architecture assigns no retention duration, record owner, or gate outcome.

## 7. Dependency and failure-mode inventory

This is a design inventory, not a claim that every dependency is implemented or that its control is approved.

| Boundary/dependency | Failure or misuse mode | Required current/target behavior | Trace |
|---|---|---|---|
| Terminal, stdin/stdout, renderer | No prompt, cancellation, or a renderer write failure | Reject an empty prompt; propagate cancellation; keep plain stdout to assistant text and tool activity on stderr. | FR-001, NFR-002 |
| Provider adapter and selected endpoint | Missing configuration, rejected/expired credentials, unavailable stream, malformed event, or diagnostic containing a secret | Fail before session creation when not ready where possible; redact adapter diagnostics; validate/sanitize events before render or persistence. Concrete endpoint/OAuth behavior is G4/G5-unresolved. | FR-002–004, NFR-002, SEC-001, SEC-003 |
| Session filesystem | Directory/file creation error, append/sync error, duplicate/invalid sequence, or torn final record | Do not report a successful turn after persistence failure; retain only the valid prefix during recovery; require recovery before fork/resume. | FR-005, FR-006, NFR-003 |
| Authorization and tool executor | Denial, cancellation, schema error, missing executor, or a policy refusal | Return a typed, redacted tool error to the model and persist that outcome without secrets; never bypass the authorizer. | FR-007, FR-008, SEC-001 |
| Workspace policy and local filesystem/processes | Path escape, symlink race, disallowed mutation, or execution without enforceable containment | Reject outside paths; return `policy_unenforceable` for contained mutation/execution that cannot be safely enforced; `unrestricted` remains an explicit authority mode. | FR-009, SEC-002 |
| MCP child process (target) | Remote descriptor, incomplete launch descriptor, untrusted identity, launch/timeout/cancellation failure, oversized message/resource | Reject before launch or invocation; bound and redact mediated results. Identity, environment, capability mediation, and limits are G5-unresolved. | FR-010, SEC-004 |
| Extension source and activation (target) | Unsupported source, missing immutable provenance, malformed manifest, no trust record, unenforceable grant, or Git-fetch failure | Leave disabled and reject activation. Source licensing/integrity and capability enforcement remain G4/G5-unresolved. | FR-011, FR-012, SEC-004 |
| Release tooling (target) | Missing/mismatched artifact, digest, signature, or source revision | Verification must fail. Key/certificate custody, signing identity, macOS notarization, and attestation form are G5-unresolved. | FR-015, NFR-004, SEC-005 |

## 8. Trust-flow decisions

1. A user supplies configuration and initiates a conversation. The root command resolves the workspace/configuration and exposes only a redacted diagnostic view (FR-001, FR-013, SEC-001).
2. The adapter alone uses provider credentials; it produces portable, sanitized events for the turn kernel (FR-002–004, SEC-001, SEC-003). Provider selection, terms, and OAuth/credential mechanics remain G4/G5 dependencies.
3. A provider-requested tool call crosses from untrusted model output into schema validation, interactive authorization, and policy evaluation before an executor can act (FR-007–009, SEC-002, SEC-004).
4. Target MCP and extension inputs are lower-trust local/Git-sourced inputs. They do not become active merely because they are configured or parseable: launch/activation needs the respective validated descriptor/provenance, trust decision, and enforceable mediation (FR-010–012, SEC-004). The unresolved controls remain G4/G5 work.
5. Session records and local logs are persistence boundaries, not credential stores. Release verification is a separate consumer boundary that must bind the artifact to its source revision (FR-005, FR-015, NFR-003, SEC-001, SEC-005).
