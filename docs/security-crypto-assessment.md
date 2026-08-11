# Lana v1 security and cryptographic assurance record

## Record status

| Field | Value |
|---|---|
| Record ID | `LANA-G5-PREP-2026-08-10-R1` |
| Prepared at | 2026-08-11T05:56:44Z |
| Prepared by | Cryptographic Assurance Engineer role |
| Classification | Internal |
| Lifecycle use | G5 preparation evidence only |
| Requirements baseline | `LANA-G2-BASELINE-2026-08-10-R1` |
| Architecture | `docs/architecture.md` version 1.1 revised draft |
| Run | `.agentic-sdlc/runs/lana-cli-v2/run-record.json` |
| Source revision | Unavailable: this repository has no Git commit (`HEAD`) |
| Assessed snapshot | `sha256:05e31b6a98a06a6597038c22e0608668aee4d4e5ab928c0c95bd99541c7bc34a` |
| Knowledge retrieval | Unavailable in the authoritative run record; no retrieved claims are used here |

This record does not approve G5, accept residual risk, grant an exception,
authorize release, or choose key custody or data-retention policy. It uses no
live credential, key, certificate, or signing operation. All test material
inspected was synthetic repository material.

The snapshot digest binds the files beneath `cmd/`, `internal/`, `pkg/`,
`scripts/`, `.github/`, and `docs/` (excluding this record), plus `AGENTS.md`,
`go.mod`, `go.sum`, `Makefile`, `.agentic-sdlc/project.json`,
`.agentic-sdlc/impact-profile.json`, `.agentic-sdlc/authorities.json`, and the
named run record. It was calculated from a sorted list of per-file SHA-256
digests. A Git commit, immutable CI run, and artifact digest remain necessary
release bindings; this worktree digest is assessment evidence, not release
provenance.

## Authority and separation of duties

The project authority map assigns **deagy** as the Security Lead and Human Key
Owner for G5. Those assignments establish accountable human roles only. They
do not record approval or risk acceptance.

The Cryptographic Assurance Engineer role prepared this artifact. An
independent security reviewer who did not prepare or materially correct it must
assess the exact bound revision before any human gate decision. The Security
Lead may make the human G5 decision only through the lifecycle process. The
Human Key Owner must decide and own applicable signing-key or certificate
lifecycle choices, but this record does not authorize deagy or any agent to
create, import, export, rotate, revoke, escrow, or destroy live material.

## Current boundary and assurance posture

Lana is currently a local Go CLI. The root command exposes conversation,
noninteractive conversation, file operations, read-only Agentic SDLC
inspection, system information, and completion generation. Provider, OAuth,
local MCP, and extension integrations are target capabilities rather than
working v1 features. The ordinary binary fails conversation readiness because
no provider, authorizer, or executor is configured.

Existing controls are useful but incomplete:

- provider events, provider diagnostics, tool results, approval previews,
  displayed configuration, and session payloads pass through redaction code;
- the turn kernel requires separate provider, authorizer, and executor
  dependencies and validates tool calls before execution;
- the TUI verifies that the displayed approval broker is the runtime's actual
  broker when a broker is used;
- session files are created with mode `0600`, the session directory is created
  with mode `0700`, appends are synchronized, and torn final records have a
  bounded recovery path;
- non-unrestricted process execution and `workspace-write` mutation fail as
  unenforceable; canonical path checks reject known workspace escapes;
- remote Git, dispatch, shell, MCP, plugin, and skill commands are not wired
  into the current root command;
- local release tooling builds deterministic archives and verifies SHA-256
  checksums, but explicitly does not sign, attest, publish, or notarize them.

These controls do not establish a completed G5 posture. In particular, there
is no provider HTTP client, OAuth implementation, approved credential store,
stdio MCP manager, extension trust/capability mediator, OS process sandbox,
race-safe filesystem mutation primitive, mandatory redacting log handler,
release signature, provenance attestation, or key/certificate lifecycle.

## Threat and control inventory

| ID | Asset / trust boundary | Threat and failure mode | Existing control | Required control and evidence | Trace |
|---|---|---|---|---|---|
| TH-01 | API keys, OAuth codes/tokens, credential references | Secret enters CLI arguments, configuration, logs, events, sessions, tool results, or errors | Structured/event/session redaction; credentials absent from portable provider types; runtime has no working credential path | Select an OS-backed credential store; prevent echo and command-history capture; zero avoidable in-memory copies; secret-safe error taxonomy; scan stdout, stderr, logs, sessions, config displays, crash paths, and child environments with synthetic canaries | FR-002–004, FR-013, SEC-001, SEC-003; `TEST-PROV-002`, `TEST-AUTH-001–002`, `TEST-CFG-002`, `TEST-SEC-001` |
| TH-02 | Provider and OAuth network boundary | Endpoint substitution, cleartext transport, invalid certificate acceptance, hostname confusion, TLS downgrade, or insecure fallback | No provider transport exists, so the ordinary binary cannot make this connection | Explicit HTTPS authority allowlist; Go TLS verification with hostname and trusted-root validation; no `InsecureSkipVerify`; minimum approved TLS version; no cleartext or silent fallback; bounded redirect policy; tests for invalid, expired, wrong-host, untrusted-root, and downgraded connections | FR-003–004, NFR-001, SEC-003; `TEST-SEC-003`, `TEST-NET-003` |
| TH-03 | OAuth authorization and token lifecycle | CSRF, authorization-code interception, device-code phishing, redirect hijack, refresh replay, stale token, or logout that leaves usable credentials | OAuth is not implemented | Security Lead selects redirect-loopback or device flow; validate issuer/authority, state and redirect binding; require PKCE `S256` where the selected flow supports authorization code + PKCE; bound scopes and token lifetime; atomic refresh; fail closed on reuse/expiry; define revocation and logout behavior; test denial, cancellation, replay, malformed callback/device response, refresh failure, and revocation | FR-003–004, SEC-003; `TEST-AUTH-002`, `SCN-PROV-001` |
| TH-04 | Provider-to-tool boundary | Untrusted model output invokes a tool directly or supplies deceptive/secret-bearing arguments | Separate `Authorizer` and `Executor`; call/result schema validation; bounded/redacted approval preview; typed denial result | Every release composition must prove an authorizer and policy mediate every tool. Approval must show action, scope, and redacted arguments. Unknown tools, malformed arguments, cancellation, and missing policy must deny before side effect | FR-007–008, SEC-001, SEC-004; `TEST-TOOL-001–002`, `SCN-AUTH-001` |
| TH-05 | Noninteractive conversation | An embedding injects `tools.AllowAll` or another permissive authorizer and gives model output implicit authority | Ordinary binary has no ready provider/tool composition; `AllowAll` is not wired as its default | Treat `AllowAll` as test-only or require an explicitly approved, auditable noninteractive authorization policy. No release path may silently select it. Negative composition and black-box tests must prove missing/implicit authority fails readiness | FR-007, FR-009; `TEST-TOOL-001`, `TEST-POL-002` |
| TH-06 | Workspace files | Traversal, symlink escape, time-of-check/time-of-use swap, destructive write, or read outside scope | Canonical workspace/path checks; known symlink escapes rejected; contained mutation fails as unenforceable | Implement descriptor-relative, no-follow mutation on supported platforms and test races, or preserve fail-closed behavior. Process/file adapters must consume the evaluated canonical target without reopening a less-safe lexical path | FR-008–009, SEC-002; `TEST-POL-001–002`, `TEST-SEC-002`, `SCN-POL-001` |
| TH-07 | Local process boundary and environment | Shell injection, escape from a label-only policy, inherited credentials, unrestricted filesystem/network access, unbounded output, or orphan process | Non-unrestricted execution is rejected; legacy shell/dispatch commands are absent from root; dormant legacy executor has timeout/output limits | Use argument-vector execution where possible; define OS containment per Linux/macOS; minimal environment allowlist with no credential variables; process-group cancellation; CPU/memory/file/output/time limits; default network denial unless explicitly authorized; abuse tests. If unavailable, continue failing closed | FR-007–010, NFR-001, SEC-002, SEC-004; `TEST-TOOL-002`, `TEST-POL-002`, `TEST-NET-003` |
| TH-08 | Sessions and local logs | Prompt, source, credential, or tool data leaks through plaintext files, permissive modes, diagnostic attributes, backups, or recovery | Session create `0600`, directory create `0700`, structural redaction, bounded recovery; app avoids logging full config | Enforce/verify owner-only modes on existing directories/files, not only creation. Add handler-wide structured-log redaction and create log files owner-only. Define session/log owner, retention, deletion, backup, and local-at-rest expectations through authorized governance; scan recovered/forked files and logs | FR-005–006, FR-013, SEC-001; `TEST-SES-001–004`, `TEST-CFG-002`, `TEST-SEC-001` |
| TH-09 | Local stdio MCP child | Malicious executable substitution, inherited secrets, capability abuse, protocol confusion, oversized messages, hangs, output injection, or remote transport smuggling | MCP command is absent from root; no child is launched | Require an absolute executable and working directory, immutable identity/provenance, minimal environment, local stdio only, schema validation, message/resource limits, timeout/cancellation, process containment, authorization for tools, and remote descriptor rejection before launch | FR-010, NFR-001, SEC-004; `TEST-MCP-001–002`, `TEST-SEC-004`, `SCN-MCP-001` |
| TH-10 | Extension source, manifest, trust, and grants | Source substitution, Git ref movement, manifest deception, dependency malware, silent update, capability escalation, or activation without trust | Plugin/skill commands are absent from root; target design says disabled until trust | Pin Git sources to immutable object identity plus reviewed content digest; validate manifest/schema; record provenance and explicit trust; deny every undeclared/unimplemented capability; isolate execution; forbid background update and marketplace discovery; re-prompt after provenance/capability change; test malicious archives, symlinks, manifests, and grant changes | FR-011–012, SEC-004; `TEST-EXT-001–003`, `TEST-SEC-004`, `SCN-EXT-001` |
| TH-11 | Dependencies and CI execution | Compromised Go module, mutable CI action, missing vulnerability/license evidence, or unexpected networked build input | Exact Go module versions and `go.sum`; CI has read-only repository permission | Pin CI actions by immutable commit; make release-candidate vulnerability, license, and SBOM tools present and failure-enforcing; preserve dependency review evidence; isolate untrusted builds; review transitive dependencies and licenses | FR-014–015, SEC-005; `TEST-NET-001`, release evidence |
| TH-12 | Release artifacts and metadata | Artifact substitution, checksum replacement, source-revision spoofing, signer confusion, provenance forgery, signing-key compromise, or rollback to an unsigned artifact | Deterministic archives and SHA-256 checksum verification; source metadata fields exist | Bind committed source, builder identity, artifact digest, signature, provenance, and release metadata; sign once in an isolated build; verify before publication/promotion/install; prohibit unsigned fallback; rehearse compromise, expiry/revocation, recovery, rollback, and independent verification. Resolve macOS signing/notarization separately from generic artifact provenance | FR-015, NFR-004, SEC-005; `TEST-BLD-001`, `TEST-REL-001–002`, `TEST-SEC-005`, `SCN-REL-001` |
| TH-13 | Egress and privacy boundary | Hidden telemetry, crash upload, background update, dependency callback, or provider/Git use without explicit action | No telemetry client/queue is designed; excluded remote commands are absent from root | Dependency/endpoint inventory plus offline and network-observation tests. Only configured provider/OAuth and explicit trusted-extension Git fetch may egress; every other attempted connection must fail and produce local, secret-safe evidence | FR-014, NFR-001, SEC-001; `TEST-NET-001–003`, `SCN-NET-001` |
| TH-14 | Dormant compatibility source and command wiring | A later refactor re-exposes remote Git, dispatch, shell, mutable SDLC, MCP URI, plugin, skill, goal, plan, or knowledge paths | Root allowlist and tests assert current public command tree; `sdlc` exposes read-only commands only | Keep negative command-tree/help tests release-blocking; delete or isolate dormant capability where practical; require scope and G4/G5 re-entry before wiring any capability | FR-009–014, SEC-002, SEC-004; compatibility-surface release evidence |
| TH-15 | Local record integrity | Another local process or compromised account tampers with sessions, trust decisions, or audit evidence | OS file permissions on new sessions; append sequence validation | Define the local attacker/account-compromise boundary. If tamper evidence is required, select a key and custody design before adding MAC/signature claims; otherwise document that same-account modification is detectable only through structural validation, not cryptographic authentication | FR-005–006, FR-011–012, SEC-004; `TEST-SES-001–004`, `TEST-EXT-001–003` |

## Credential, authentication, TLS, and transport boundaries

| Flow | Current state | Required v1 boundary | Lifecycle owner |
|---|---|---|---|
| OpenAI API key | Not implemented; no credential field exists in portable request/event/session types | Acquire without echo or CLI argument, store through an approved OS-backed reference, inject only inside the adapter, scope minimally, never forward to tools/MCP/extensions, and support replacement/revocation without plaintext persistence | Security Lead selects control; credential user/provider owns the external secret |
| OpenAI OAuth | Not implemented | Explicit approved authority and flow, narrow scopes, validated callback/device exchange, safe refresh concurrency, expiry and revocation behavior, logout deletion, and no token diagnostics or persistence outside the credential store | Security Lead; deagy is assigned for G5 |
| Provider API | No concrete network backend | HTTPS only; validated hostname and trust roots; approved minimum TLS version and algorithm policy; no cleartext, certificate-skip, downgrade, or silent endpoint fallback | Security Lead |
| Local MCP | No current root command or child manager | Stdio child process only. No TLS is applicable to the Lana-to-child pipe; executable identity, provenance, process containment, environment minimization, protocol limits, and authorization replace network-channel assumptions | Security Lead with system owner |
| Explicit extension Git fetch | Target only | User-initiated fetch from an explicit source, secure Git transport, no credential leakage to child logs/config, immutable revision and content digest before trust, no submodule or hook execution unless separately contained and approved | Security Lead and supply-chain owner |
| Release signing/provenance | Not implemented | Identity/certificate/key mechanism must bind the committed source and artifact digest. The effective team policy mentions keyless cosign/Sigstore OIDC, while Lana design text still leaves Linux identity/custody and provenance format open; the Security Lead and Human Key Owner must reconcile applicability before implementation | deagy as Security Lead and Human Key Owner; independent verifier reviews evidence |

No certificate-validation or revocation claim is currently possible because no
provider/OAuth transport or release-signing implementation exists. The future
transport must use normal hostname and chain validation and must not add an
insecure fallback. Trust-root override, private CA use, certificate pinning,
OCSP/CRL behavior, proxy interception, minimum TLS version, and provider
endpoint allowlisting remain explicit design decisions and test inputs rather
than assumed defaults.

## Tool approval and containment requirements

The implemented authorization seam is necessary but does not itself prove that
a released composition uses the right authorizer. Interactive approval must be
bound to the same broker used by the executor, show a redacted and bounded
preview, require allow/deny for the concrete operation, and return denial or
cancellation before side effect. A model response never supplies authority.

Noninteractive behavior remains unresolved. The exported `tools.AllowAll` type
can be injected by an embedding application, although it is not the ordinary
binary default. Release composition tests must prove that `AllowAll` cannot be
selected accidentally and that absent policy, absent user authority, unknown
tools, and malformed arguments fail readiness or return a typed denial.

The three policy labels are not an OS sandbox. `workspace-write` mutation and
non-unrestricted process execution currently fail closed because race-safe
filesystem and process isolation are not implemented. `unrestricted` means the
user intentionally authorizes the actual local operation; it must never be a
model-selected fallback or a remedy for an unenforceable boundary.

## Session, diagnostic, and redaction requirements

Session contents are plaintext local conversation records, not encrypted or
cryptographically authenticated records. Mode `0600` and directory mode `0700`
are implemented on creation, but existing modes are not repaired. Application
encryption at rest is not selected; platform full-disk encryption is outside
Lana's current evidence. Retention, deletion, backups, record ownership, and
whether same-account tamper evidence is required remain governance decisions.

Redaction is defense in depth, not permission to pass credentials through a
boundary. Current structural redaction recognizes common secret keys and
credential-shaped strings, and tests cover representative API-key, bearer,
cookie, URI-userinfo, and refresh-token cases. The logger itself uses standard
`slog` handlers without a redacting wrapper, and optional log files are opened
with mode `0644`. Before G5 evidence can be complete, logging must enforce
redaction centrally, use owner-only files, handle nested groups and arbitrary
error values, and be tested with synthetic canaries across text and JSON modes.

## MCP and extension supply-chain requirements

The safe current property is non-availability: the root command does not expose
the legacy MCP, plugin, or skill command groups. The source tree still contains
legacy implementations, and configuration still accepts URI-shaped MCP entries.
These are not v1 trust controls and must remain unreachable or be rejected
before action.

A v1 MCP descriptor must be complete and local-stdio-only before process launch.
It must bind an absolute executable identity, arguments, working directory,
minimal environment, timeout, and resource/message limits. Tool invocation must
reuse the main authorization and policy path. Remote URI, HTTP, SSE, WebSocket,
missing trust, oversized output, invalid framing, timeout, and cancellation
must all fail closed.

An extension must remain disabled until Lana records immutable source
provenance, validated manifest/capabilities, explicit user trust, and enforceable
grants. A branch or tag name is not immutable provenance by itself. Activation
must fail on a changed revision, digest, manifest, requested capability, or
missing enforcement implementation. Silent update, marketplace discovery,
implicit network, Git hooks, and unreviewed submodule execution are prohibited.

## Release signing, provenance, and dependency integrity

Current release tooling produces deterministic Linux and macOS archives and a
SHA-256 checksum manifest. A checksum detects accidental or unauthenticated
content change only when the checksum itself arrives through a trusted path; it
does not authenticate a publisher. There is no release signature, attestation,
notarization, publication step, or provenance verifier. A source tree without a
commit is deliberately labeled `unknown`, which is not acceptable release
provenance.

Before a release candidate, the project must:

1. bind all evidence to a real immutable source commit and protected CI run;
2. pin build dependencies and CI actions to reviewed immutable identities;
3. build once in an isolated trusted build context and preserve the artifact
   digests;
4. make vulnerability, license, and SBOM evidence mandatory rather than
   silently `SKIPPED` for release jobs;
5. implement the approved signing/attestation mechanism with short-lived,
   least-privilege identity where possible and no unsigned fallback;
6. define the verification procedure, trusted identity, expiry/revocation or
   compromise response, recovery, and rollback behavior;
7. resolve macOS code-signing/notarization identity and certificate lifecycle;
8. independently verify artifact, checksum, signature/attestation, provenance,
   source revision, platform metadata, and installation behavior.

No signing key or certificate lifecycle is selected here. In particular, this
record does not infer that a long-lived Linux key exists merely because the
requirements mention key custody, and it does not infer that keyless signing
removes the need to govern OIDC issuer, subject, certificate identity, and
verification policy.

## Cryptographic inventory and posture

| ID | Use / asset | Algorithm, protocol, or trust dependency | Current / planned | Owner and lifecycle state | Posture and verification requirement |
|---|---|---|---|---|---|
| CR-01 | Release archive integrity | SHA-256 checksum (`sha256sum` or `shasum -a 256`) | Implemented locally | Release engineer; no key involved | Approved for digest integrity. It is not a signature. Verify checksum manifest and archive together through an authenticated release path |
| CR-02 | Go dependency content integrity | Go module versions plus `go.sum` hashes and Go toolchain trust services/configuration | Implemented | Engineering/release owner; dependency lifecycle active | Exact modules are recorded. Preserve checksum verification, dependency review, offline/reproducibility evidence, and toolchain/proxy/sumdb configuration used for release |
| CR-03 | Provider API transport | HTTPS/TLS with public or explicitly approved trust roots | Planned; backend absent | Security Lead; algorithm policy and trust roots unresolved | Require approved minimum TLS version, hostname/chain validation, secure negotiation, and no downgrade or insecure fallback. Test invalid chains, wrong host, expiry, minimum-version rejection, and redirect behavior |
| CR-04 | OAuth authorization/token exchange | OAuth 2.x flow; PKCE `S256` where applicable; TLS transport; state/redirect or device-flow binding | Planned; flow absent | Security Lead; flow, authority, scopes, cryptoperiod, rotation, revocation, and recovery unresolved | Threat-model the selected flow and verify replay, CSRF/code substitution, expiry, refresh concurrency, logout, revocation, and secret non-persistence |
| CR-05 | Provider API credential | User/API token stored behind a credential reference | Planned; no store | Security Lead; credential user/provider owns external issuance | Not a Lana cryptographic key, but a high-value bearer secret. Define acquisition, storage, scope, rotation/replacement, revocation, backup prohibition, and failure audit without values |
| CR-06 | Release artifact authenticity and provenance | Signature/attestation algorithm, certificate/identity, transparency evidence, and verifier policy | Not implemented; mechanism unresolved | deagy as Human Key Owner for applicable G5 decision; release implementer operates only after approval | Reconcile keyless Sigstore/cosign policy text with Lana-specific Linux/macOS requirements. Bind signer identity, source, build, subject digest, expiry/revocation/compromise and recovery; rehearse independent verification |
| CR-07 | macOS binary identity | Apple code-signing/notarization certificate and platform trust services | Planned; applicability/detail unresolved | Human Key Owner and release owner; issuance, custody, renewal, revocation, recovery unknown | Define required platforms/artifacts, certificate identity, timestamp/notarization evidence, hardened-runtime expectations if applicable, expiry/renewal and revocation response |
| CR-08 | Git extension provenance | Git object identity, reviewed source digest, and secure Git transport | Planned; extension path absent | Supply-chain owner and Security Lead | A branch/tag alone is insufficient. Record immutable object and content digest; decide whether signed commit/tag verification is required and define trust roots, revocation and failure behavior before claiming verified provenance |
| CR-09 | Session/trust-record confidentiality and integrity | No application encryption, MAC, or signature selected | Current: plaintext sessions; trust records planned | Data/Control Owner and Security Lead; retention/tamper-evidence decisions unresolved | Do not claim encryption or cryptographic tamper evidence. If either becomes required, add a separate key/certificate inventory, custody, recovery, migration, and destructive-lifecycle review before implementation |

No implemented negotiation, fallback, or cryptographic-agility layer exists in
Lana because provider/OAuth and signing backends are absent. Future adapters
must keep algorithms and trust configuration at narrow boundaries, reject
unsupported or weakened configurations, expose nonsecret algorithm/version
metadata for audit, and support migration without accepting silent fallback.
Algorithm deprecation, provider trust-root change, signer compromise, and
certificate expiry must have failure and recovery tests before the related
capability is released.

## Specialized capability and BOM applicability

| Capability / evidence category | Assessment for Lana v1 | State |
|---|---|---|
| QKD | No QKD link, service, protocol, or requirement appears in the local CLI scope | Not applicable to this v1 design |
| QKMS | No quantum key-management service or specialized key-distribution dependency appears in scope | Not applicable to this v1 design; this does not make ordinary OAuth/signing lifecycle N/A |
| QRNG | Lana does not provide or select a specialized random-number source | Not applicable to this v1 design; runtime randomness remains a platform/Go dependency when implemented |
| PQC | Lana implements no custom cryptography and has no stated post-quantum requirement | No Lana-specific PQC capability is applicable now; no claim of PQC resistance is made for provider TLS, Git, or signing ecosystems |
| HSM / PKCS #11 | No mechanism is selected | Unknown until release signing and macOS certificate custody are resolved; no HSM applicability claim is made |
| CBOM | The impact profile declares CBOM not applicable without semantics or rationale, while this record identifies ordinary TLS, OAuth, digest, and release-signing dependencies | Applicability/semantics require Security Lead reconciliation; no CBOM conformance claim |
| QBOM and other named specialized BOMs | Impact profile declares them not applicable and no specialized dependency is present in the v1 architecture | Not applicable on present scope; undefined semantics are not treated as conformance |

The impact profile's `qkms-crypto-impact` rationale says Lana has no
cryptographic key material. That is true of the currently implemented runtime
and of this assessment activity, but it is too broad for the target design:
OAuth credentials, TLS trust dependencies, release signing identity, and
possible macOS certificates still require ordinary lifecycle assurance. The
profile and its CBOM declaration must be reconciled by authorized lifecycle
work; this artifact deliberately does not edit the profile or gate record.

## Findings

| ID | Severity | Finding | Required disposition / owner |
|---|---|---|---|
| G5-CRIT-01 | High, release-blocking | No immutable Git source revision exists. The worktree snapshot cannot provide source-to-artifact provenance | Create and review a committed revision, bind later evidence and artifacts to it, and invalidate/re-run stale review evidence as required. Engineering/release owner |
| G5-CRIT-02 | High, capability-blocking | API-key/OAuth acquisition, approved credential storage, provider HTTPS/TLS policy, token lifecycle, revocation, and recovery are not implemented or decided | Security Lead selects the design; secrets/identity implementer supplies synthetic tests; independent security reviewer verifies the exact revision |
| G5-CRIT-03 | High, capability-blocking | Race-safe mutation and OS process containment are absent. Current fail-closed behavior is safe only while it remains enforced | Implement and verify the platform control, or keep the related modes/capabilities unavailable. Security Lead and containment implementer |
| G5-CRIT-04 | High, capability-blocking | Local MCP and extension identity, provenance, capability mediation, isolation, and trust records are not implemented; legacy source/config remains present | Keep surfaces unreachable/rejecting until implementation and malicious-input tests pass. Security Lead plus MCP/extension and supply-chain owners |
| G5-CRIT-05 | High, release-blocking | Release artifacts have SHA-256 checksums but no authenticated signature, provenance attestation, trusted signer policy, macOS signing/notarization evidence, or compromise/recovery plan | Security Lead and Human Key Owner reconcile and approve the lifecycle design; release implementer rehearses; independent reviewer verifies. No live keys handled by this record |
| G5-MED-01 | Medium | Standard `slog` handlers do not centrally redact attributes/errors, and file logs are created with mode `0644` | Add a recursively redacting handler, owner-only log creation, permission tests, and synthetic canary scans. Secrets/identity implementer |
| G5-MED-02 | Medium | Session owner-only permissions are set only at creation; existing lax modes are not detected or repaired. Retention, deletion, backup, and record ownership are undecided | Add permission validation/fail-closed behavior; route lifecycle decisions to authorized governance/data owner without inventing them here |
| G5-MED-03 | Medium | `tools.AllowAll` is exported for noninteractive embeddings and release composition has no guard proving it is test-only | Select explicit noninteractive semantics and add composition/black-box tests that reject implicit allow. Security Lead |
| G5-MED-04 | Medium, release-blocking | CI actions use mutable major tags, while vulnerability, license, and SBOM tools may be silently skipped in `make ci` | Pin actions/tool versions and make required release evidence fail when absent. Pipeline/supply-chain owners |
| G5-MED-05 | Medium | Configuration still accepts remote URI-shaped MCP entries; safety currently relies on the MCP command not being wired | Reject URI/non-stdio descriptors during v1 configuration validation and retain command-tree negative tests. MCP implementer/security reviewer |
| G5-MED-06 | Medium, gate-information blocker | The impact-profile no-key-material rationale and CBOM N/A entry do not account for planned TLS/OAuth/signing dependencies or define specialized BOM semantics | Security Lead reconciles applicability and records an owned rationale. Undefined applicable semantics must not be treated as conformance |

All High findings block the affected G5 capability or v1 release claim until
remediated and independently verified. This is an assurance conclusion, not a
gate decision or risk acceptance.

## Required implementation and verification evidence

| Evidence ID | Minimum evidence for the bound implementation revision | Independent review focus |
|---|---|---|
| G5-EV-01 | Provider/OAuth threat model, endpoint/redirect inventory, approved credential-store design, synthetic lifecycle tests, and captured secret-canary scan | No credential crosses config/event/session/log/tool/MCP/extension boundaries; transport downgrade and replay fail closed |
| G5-EV-02 | Composition tests for interactive and noninteractive authorization; broker identity; allow/deny/cancel; malformed calls; unknown tools; permissive-authorizer rejection | Provider output cannot grant authority; the preview is meaningful, bounded, and redacted |
| G5-EV-03 | Linux/macOS filesystem race and process-containment tests, resource limits, environment inventory, process-group cancellation, and network-egress observation | Claimed mode matches enforceable OS behavior; unsupported control fails closed |
| G5-EV-04 | Session/log mode tests, torn-record/fork scans, recursive redaction canaries, and written handling decision references | Permissions apply to existing and new files; no secret survives a durable or diagnostic boundary; no retention decision is inferred from tests |
| G5-EV-05 | Malicious local-stdio MCP fixtures, executable identity evidence, environment/resource limits, remote-descriptor rejection, and authorization traces | No remote transport or child/tool capability bypass; cancellation and oversize behavior are bounded |
| G5-EV-06 | Malicious extension fixtures, immutable Git/content provenance, manifest validation, explicit trust/grants, capability matrix, change/revocation behavior, and dependency review | Activation remains disabled on every missing/mismatched control and after provenance/capability change |
| G5-EV-07 | Committed source revision, protected build identity, immutable CI dependencies, mandatory scans/SBOM, artifact digests, signature/attestation, release metadata, macOS evidence, and offline verifier transcript | Artifact, source, builder, signer identity, signature/certificate policy, and provenance bind without unsigned or downgrade fallback |
| G5-EV-08 | Crypto inventory updated with exact implemented protocols/algorithms, trust roots, identities, cryptoperiods, rotation/revocation/recovery, algorithm migration, and failure tests | No unknown applicable semantics, expired identity, weak fallback, shared-key misuse, or ownerless lifecycle remains |

The existing `TEST-*`, `SCN-*`, and `AE-*` identifiers in
`docs/traceability-matrix.md` remain the planned functional evidence map. The
`G5-EV-*` items above add the security/cryptographic evidence bundle expected
for independent G5 review; they do not claim that any procedure has run.

## G5 preparation conclusion

This assessment is complete for the currently documented Lana v1 scope and the
bound uncommitted snapshot. It identifies working fail-closed and redaction
controls, separates them from target controls, records ordinary and specialized
cryptographic applicability, and assigns unresolved lifecycle decisions to the
existing accountable human roles.

G5 is not ready for approval on this evidence. High findings remain for source
revision binding, provider/OAuth credentials and TLS, filesystem/process
containment, MCP/extension trust, and release authenticity/provenance. The
Security Lead and Human Key Owner are assigned, but independent verification,
implementation evidence, exact-revision bindings, and human lifecycle decisions
are absent. No gate status or authority metadata was changed.
