# Remaining plan: safe coding-agent CLI and terminal TUI

**Status:** proposal — no coding-agent runtime is approved or released.  
**Owner for scope decisions:** Product Owner; provider and security decisions require the designated security/architecture authority.  
**Review date:** before any Phase 1 implementation begins.

## Boundary and evidence

The currently supported product is the local, single-user `lana agents` queue and `lana knowledge` store. Those are implemented foundations and remain out of scope for this plan; they neither provide a model provider nor authorize code execution. There is no immutable Git revision/public release to label it as released. The authoritative baseline is [requirements.md](requirements.md) and the public-command boundary is [compatibility-surface.md](compatibility-surface.md). Both exclude provider chat, shell execution, MCP, plugins, and extensions.

Some older design material describes a Codex-like CLI. It is draft and conflicts with that baseline; it is input for this proposal, not approval to wire its commands. Today the root composition supplies no concrete provider, authentication, tool authorizer, or tool executor. `lana`/`lana exec` therefore reaches `cli.UnavailableExecutor`. Provider, session, tool-contract, and basic Bubble Tea TUI packages are reusable foundations, not an end-user coding agent.

## Phase 0 — decide whether to re-enter coding-agent scope

Human approval must select a named provider, pinned model identifiers and SDK, authentication method and credential store, data classification/residency, retention, cost/latency limits, supported OSes, and the initial tool set. It must also approve the threat model, approval modes, sandbox technology, deletion and recovery semantics, operator ownership, and degraded behavior. Model output, retrieved content, and tool output remain untrusted; a model never authorizes an action.

Before capability work, establish the CI/test substrate: pinned workflows; Linux race tests; deterministic integration and acceptance mapping; fuzz boundaries; and `gofmt`, `goimports`, `go vet`, and lint. Future behavioral specifications should use Godog executed through `go test`, with Testify `require` and `assert` for Go assertions. They are desired future stack components only: the current code uses a custom acceptance parser and has no direct Godog/Testify dependencies. Decide whether credentialed provider/signing jobs run on the team-standard GitLab CI protected runner tiers, or document and approve a bounded exception for the current GitHub Actions workflow; neither is implicit.

**Blockers:** the team profile leaves model provider and eval framework `not_yet_selected`; OAuth/API credential handling, key custody, release authenticity, safe contained writes, and OS process sandboxing are unresolved. The current product scope is local agents/knowledge only, and the repository has no Git `HEAD`/revision to bind release evidence. The initial release must not revive hidden MCP/plugin commands or execute through `sh -c`.

**Exit criteria:** a reviewed requirements update, provider/auth and security decisions, a bounded first-release tool list, testable acceptance scenarios, the CI/test substrate, and a recorded protected-runner/CI exception decision are approved. Until then, retain deterministic rejection of future commands. Each later phase must land with its corresponding tests and evidence, not defer verification to release preparation.

## Delivery phases

| Phase | Scope and dependencies | Measurable acceptance criteria |
| --- | --- | --- |
| 1. Runtime provider and authentication | Implement a provider adapter behind `internal/provider.Client`; compose it in the root only after Phase 0. Add a credential-reference interface and selected flow (not plaintext flags, config, session, logs, or tool context). Pin model/SDK; define timeout, retry, truncation, refusal, malformed-event, and unavailable-provider behavior. | A real configured adapter streams a turn; no configuration reports the existing safe unavailable error. Integration tests cover cancellation and every degraded path. Secret-scanning/redaction tests prove credentials are absent from output and persisted JSONL. |
| 2. Safe tools and sandbox | Build typed, schema-validated `internal/tools` adapters plus a deny-by-default authorizer and approval broker. Start with explicitly approved read-only tools. Add descriptor-relative, no-follow workspace access and race tests before contained writes. If a process tool is approved, execute an argument vector through the selected OS sandbox with resource/time/output limits and no inherited secret environment; never invoke a shell string. | Unknown, malformed, denied, cancelled, out-of-workspace, symlink-swap, and limit-exceeded calls fail before side effect. Every mutation/process call presents redacted action, scope, and arguments and requires the configured approval. Tests prove no `sh -c` path and exercise sandbox escape attempts. |
| 3. Sessions and context | Use `internal/session.Store` as the versioned append-only source of truth. Define context-window selection/summarization, prompt-version artifacts, persistence/redaction, session listing, resume, recovery, and fork semantics. Do not silently replay unfinished side effects. | Restart/recovery/fork tests preserve valid history and parent immutability; corrupt suffix recovery is explicit. Context selection stays within the configured budget, marks truncation, and an eval baseline measures turn quality before provider/prompt changes. |
| 4. MCP and extensions | Treat these as a new local-first trust boundary, not restored legacy commands. Specify manifest/signature/provenance, version pinning, permission declaration, installation location, process isolation, transport allowlist, lifecycle/updates/removal, audit records, and per-call authorization. Remote transports require a separate approved scope. | Disabled by default; unsigned, unpinned, malformed, over-permissioned, or network-capable extensions are rejected before launch. Contract/integration tests prove declared permissions and redacted audit records. |
| 5. TUI and command execution UX | Build on `internal/tui`: viewport/scrollback, streaming renderer, accessible keyboard composer, slash-command menu, session discovery/resume/fork, tool timeline, approval and diff views, cancellation, and narrow terminal-safe rendering. Keep a noninteractive `exec` renderer with explicit JSONL contract. | Keyboard-only tests cover compose, cancel, approval deny/allow, session navigation, and resize. Golden tests cover streaming, long output, error states, diff redaction, and no terminal escape injection. CLI tests distinguish human output from machine JSONL. |
| 6. Release-evidence aggregation and rehearsal | Aggregate the tests/evidence delivered with prior phases. Rehearse the PTY black-box suite on Linux and macOS for TTY/piped input, resize, UTF-8/no-color, Ctrl-C, terminal restoration, and control/secret canaries; rerun capability-specific adversarial provider/auth/tool/sandbox/MCP/extension tests, compatibility migration/JSONL-flag tests, and durable-store crash/stress tests. Establish final eval, latency, and cost baselines; produce rollback, credential-revocation, and support procedures. | CI proves the pinned toolchain and deterministic suites; platform PTY tests prove a safe terminal after every exit path. Adversarial tests cover every approved capability. Release evidence binds an immutable revision/tag, SBOM, mandatory license/vulnerability scans, signing/provenance verification, and an explicit macOS notarization decision. A clean-host smoke test proves installation, unavailable-provider failure, configured conversation, approval enforcement, and rollback. |

## Proposed module/API seams

Keep provider-neutral contracts and inject implementations at composition time:

- `internal/provider`: `Client`, request/message/event contracts, selected adapter, credential-reference resolver, event validation/redaction.
- `internal/cli`: `Kernel`/`TurnExecutor`, `Runtime`, renderers, provider unavailability behavior, and no provider-specific credential leakage.
- `internal/tools`: tool definitions/calls/results, registry, authorizer, approval broker, workspace capability, and sandboxed process executor. The evaluated descriptor/capability—not a re-parsed path—crosses from policy to side effect.
- `internal/session`: append-only records, recovery/fork, context assembler, and explicit transcript schema migration policy.
- `internal/tui`: presentation only; it asks the runtime/approval broker and does not implement authorization or execute tools itself.
- `internal/extensions` (only after Phase 4 approval): manifest/verification, constrained host, lifecycle manager, and MCP transport adapter.

The stable external surface should be limited to approved root commands and versioned JSONL/event schemas. Provider, auth, sandbox, and extension interfaces remain internal until integration and compatibility commitments are separately approved.

## Sequencing, risks, and release controls

Phases 1 and 2 depend on Phase 0; Phase 3 depends on a working runtime; Phase 4 depends on safe process isolation and supply-chain decisions; Phase 5 depends on the runtime, session, and approval seams; Phase 6 gates every release. No phase may widen the current local-agents/knowledge contract by accident. If safe descriptor-relative mutation or selected sandbox guarantees cannot be delivered on a supported OS, retain read-only tools and reject the affected capability.

Rollback is a version rollback plus disabling provider/tool/extension composition; it does not claim deletion of provider-side data. Credential compromise handling must revoke/replace the external credential and invalidate local references without logging their values.

## Source references and assumptions

This plan is based on the current local product baseline and command boundary, the reusable contracts in `internal/provider`, `internal/cli`, `internal/tools`, `internal/session`, and `internal/tui`, plus the outstanding controls recorded in [security-crypto-assessment.md](security-crypto-assessment.md). Assumption: a future coding-agent release remains a local single-user developer tool unless a later approved scope says otherwise. No provider, authentication, sandbox, MCP, extension, or remote-data behavior is asserted as implemented here.
