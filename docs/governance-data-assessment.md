# Lana v1 governance and data assessment

## Record status and revision binding

| Field | Value |
|---|---|
| Record ID | `LANA-G4-DATA-2026-08-10-R1` |
| Prepared | 2026-08-10 America/Phoenix (`2026-08-11T05:56:48Z` evidence snapshot) |
| Scope | Lana v1 local provider-connected coding CLI |
| Accountable governance/data owner | deagy |
| Artifact classification | internal |
| Decision state | G4 preparation; human decisions in section 8 remain pending |

This is a revision-bound governance/data assessment, not a G4 decision,
approval, exception, risk acceptance, release authorization, legal conclusion,
or retention schedule. The repository has no Git commit at the evidence time,
so `HEAD` cannot bind this record. The binding is the current working-tree
evidence set and SHA-256 values below. A change to a bound artifact requires a
new assessment revision or an explicit diff assessment.

| Evidence | SHA-256 |
|---|---|
| `docs/requirements.md` | `a976a3f54af7a7362e4d525ddd4adfbe81f2bfeff2f67a13dce06c57bfb56c0d` |
| `docs/architecture.md` | `5390ea62cefe697b76610c73a557e183f56e7d78f939faeea481fc1eddef9b44` |
| `docs/api-contracts.md` | `3a2981a85a886200c9ac5d4bfe25df537d2cbb453fae5af513d9e57531a0be6d` |
| `docs/compatibility-surface.md` | `1170b76df87c10f9fd8be1469f4e1e3d5866e340499d2e51d1ed64e7e8610c9e` |
| `cmd/lana/root/root.go` | `272e1dc9b3b2aec5d245e9f4e3f80753b8abb954e638d0c029525cc964090ebe` |
| `internal/cli/runtime.go` | `ef9b80d052113898971cfee17c6542454f878900cedbfc7bcb382b9deb793d62` |
| `internal/provider/provider.go` + `internal/provider/adapters.go` | `697391252cab697e6b06df8507b5b1435cacc97fb38c325f735ca5e1e99735b6` + `755aee674b4e3bf0e8adef15292d668dd56e8f974fac0e5204c28ae989a58793` |
| `internal/session/store.go` | `c50ded605279d3e07faae089e004f28e47cca89c27304c4d74d43f4bfde7ffde` |
| `internal/tools/tools.go` + `internal/cmd/file/file.go` | `cfe64e818802f41303d8c246010a87128050c8685013a45df4027ffd3c786d85` + `3448b7241d973b5b3f9a5c7e068f66486b998c67301ba4023458d28d859de90c` |
| `pkg/config/config.go` + `pkg/logger/logger.go` | `57b0c1d0f9d133d7b0a33e46c79f3519d958b8841bd0093d0043ff27da5cdc55` + `f73583a6e172f177075c57d935824881158d0821d0d1db0779d2aabcbe88fd01` |
| `go.mod` + `Makefile` | `f80f3fe6818eb5c19f26887839991a10d6c32c3de316f8fb3cfbf112f970375f` + `d6844b3f74c580fd0c26751f8323ec793d67c35df226f858aa9a7710d33cef28` |
| `scripts/release.sh` + `scripts/releasepack/main.go` | `92a8496b426e88d743ce237cf0771e3a4ba5bc78d3f24a77ae62bd9f6df44aec` + `3579b92f74ad47a9fe5ff689245cdf469d11cb0d808e6d40bd67650d2932e562` |
| `.github/workflows/ci.yml` | `3e2319e488d6619bc8ef756e7d0602e210b17c09d5fd23fa28bd34f03a899a96` |
| `.agentic-sdlc/runs/lana-implementation-plan/dispatch-plan.json` | `665422c64bee551e130b49f9be0b4b94f899d214e47cd2532697901e416fdab1` |

Evidence anchors include the current root allowlist and legacy exclusions
(`cmd/lana/root/root.go:144-155`), session-store location and composition
(`cmd/lana/root/root.go:186-210`), persisted turn lineage
(`internal/cli/runtime.go:544-619`), append/recovery/fork behavior
(`internal/session/store.go:64-148,165-206,244-297`), configuration data shape
and redaction (`pkg/config/config.go:18-78,134-163`), local logging destinations
and modes (`pkg/logger/logger.go:43-52`), CI execution
(`.github/workflows/ci.yml:1-29`), and local-only release behavior
(`docs/build-release.md:1-40`). Knowledge retrieval was unavailable/empty for
this scope: the bound lifecycle dispatch plan records `knowledge_context` as
`unavailable`, and the authorized local store reported zero messages, chunks,
or sources. Current repository evidence therefore remains authoritative.

## 1. Applicability and classification rules

Data governance is applicable. Lana persists prompt/session/tool content,
transforms it into provider requests and derived outputs, reads and changes
workspace data, can write local logs, is intended to cross a provider boundary,
and is built on an externally hosted CI runner. This applicability statement
does not decide G4.

The requirements baseline labels the product artifact `internal`, but no
organization-wide classification taxonomy or handling matrix is specified.
Until deagy decides that taxonomy:

- Source, prompt, session, provider-event, MCP, extension, tool, and log data
  inherit the highest classification and handling restrictions of their source;
  transformation or redaction does not automatically lower classification.
- API keys, OAuth codes/tokens, cookies, private keys, and equivalent
  authentication material are **credential secrets** for technical handling and
  are prohibited from configuration dumps, prompts, provider events, sessions,
  tool results, logs, extension state, and release artifacts. This technical
  label is not a newly declared enterprise classification tier.
- Untrusted provider output, tool output, MCP content, and extension content are
  not authoritative merely because Lana stores or renders them.
- Any data whose applicable classification cannot be established is `unknown`
  with deagy accountable for resolution; it must not cross a provider, Git,
  CI/release, MCP, or extension boundary until resolved.

## 2. Inventory and end-to-end lineage

All entries have accountable owner **deagy**. `Existing` means observable in
the bound source; `planned` means a target contract and is not an availability
claim.

| ID / state | Data and collection | Processing and derived outputs | Stores, transfer, sharing, and jurisdiction | Lifecycle and recovery |
|---|---|---|---|---|
| DG-DATA-01 / existing | Workspace paths, file content, metadata, search patterns, write input, and command/tool arguments supplied by the user or provider. Classification inherits the workspace/source; formal label is otherwise unknown. | Canonical path/policy results; bounded file output; search matches; copied, moved, written, deleted, or executed content; typed tool results. Tool results are redacted structurally but can still contain source content. | Local workspace and stdout/stderr; conversational tool results also go to the provider request and session. Host jurisdiction is unknown. No Lana backup service exists. | Source-file retention, backup, deletion, and recovery inherit the user's filesystem/VCS/backup environment and are not specified. Direct `lana file` operations are existing rooted commands and are not session-recorded. Backup copies created by `file write --backup` persist independently. |
| DG-DATA-02 / existing | User prompts, model selection, permission mode, session identifiers, timestamps, portable provider events, tool results, assistant text, outcomes, and parent-session links. Classification inherits all included source/prompt/tool data. | Prompt plus resumed history becomes a provider request; streamed deltas are assembled into assistant text; provider events and tool results are sanitized/redacted; fork copies the complete parent history and adds provenance. | Memory and `<workspace>/.lana/sessions/<id>.jsonl`; directory `0700`, new file `0600`, append + sync. Planned provider transfer is external; concrete destination/jurisdiction is unknown. No remote Lana sync exists. | No retention duration or session-delete API exists. Files persist until an external/manual deletion. A fork is a second full copy requiring separate deletion. Recovery rewrites only the valid prefix of a torn final record; it is not backup restore. Host backups may preserve deleted records; behavior is unknown. |
| DG-DATA-03 / existing seam, planned provider | API key/OAuth material, credential reference, provider/model/endpoint selection, portable request messages/tool schemas, returned events/errors, and provider-side service metadata. Credentials are credential secrets; request/response content inherits session/source classification. | A planned backend injects credentials adapter-privately. Existing adapters sanitize events and redact credential-shaped diagnostics. There is no concrete production network backend, OAuth flow, credential store, or configured provider in the rooted binary. Adapter kinds in source do not constitute approved providers. | Planned transfer from the developer host to a selected model/OAuth provider over an external network. Provider identity, endpoint, service terms, training/abuse use, subprocessors, retention, deletion, residency, and jurisdictions are unknown. Provider-side copies are outside Lana's local deletion control. | Local credential retention is not implemented. Provider-side lifecycle and deletion are pending provider selection and a human terms decision. Token refresh/revocation and local logout deletion are planned G5 work. |
| DG-DATA-04 / existing | Configuration from defaults, `~/.config/lana/config.yaml`, `<workspace>/.lana/config.yaml`, `LANA_*` environment variables, flags, and legacy MCP/knowledge/plugin/skill/dispatch fields. Configuration is internal unless inherited data makes it higher; secrets are prohibited. | Precedence merge, path expansion, validation, and redacted display copy. Current redaction covers secret-shaped keys and URI userinfo/query components, but current validation still accepts legacy MCP URI fields. | Local files, process environment, and memory. No intended transfer. Host jurisdiction and backups are unknown. Current config-write helpers create parent directories `0755`; resulting file-mode guarantees are not established. | No retention or deletion mechanism exists beyond manual file/environment management. Recovery is from user-managed copies. Legacy fields must be rejected/removed before v1 rather than treated as approved data contracts. |
| DG-DATA-05 / existing | Diagnostic messages, error text, timestamps, levels, structured attributes, and optional application-provided values. Classification inherits the highest source value reaching the log. Credential material is prohibited. | Text or JSON formatting; provider/config/tool redaction exists at named boundaries, but the generic logger does not enforce field redaction itself. | stdout by default; optional local file plus stdout. The logger creates file directories `0755` and files `0644`. There is no remote sink in the bound code. Terminal capture, shell history, or CI capture can create additional copies outside Lana. | No rotation, retention, deletion, or backup behavior is implemented. File logging at `0644` is not an owner-only control and must not carry prompt/session/tool bodies. Human lifecycle choice is pending. |
| DG-DATA-06 / existing contracts | Tool definitions, calls, arguments, authorization previews/decisions, policy results, stdout/stderr, file content, search matches, exit codes, errors, and timestamps. Classification inherits the accessed source and arguments. | Schema validation, preview redaction, authorization, execution, result bounding by adapters, result redaction, renderer notice, and session persistence. The portable executor seam exists; production built-in executors are injection-dependent. | Local process/workspace; provider receives tool definitions, calls originate in provider output, tool results return to the provider and persist in sessions. Exact provider jurisdiction is unknown. | Results persist with their sessions with no schedule/delete API. Direct rooted file-command output is not durably recorded by Lana, but terminal capture may retain it. |
| DG-DATA-07 / planned MCP; legacy config existing | Local MCP descriptor, executable path, arguments, working directory, environment allowlist, trust identity, resource/template/tool requests, content, tool arguments/results, limits, and diagnostics. Descriptor data is internal; exchanged content inherits its source. | Target flow validates descriptor/trust, launches local child over stdio, bounds messages/resources, redacts diagnostics, and routes tools through authorization. No target MCP manager exists. Legacy URI config and source-visible commands exist but are absent from the root. | Intended transfer is only between Lana and a local child process over stdio on the developer host. The child is a separate trust boundary and could egress unless process/network controls prevent it; those G5 controls are not implemented. Remote MCP is prohibited. | Descriptor/content retention is unspecified. MCP results would inherit session retention when persisted. Child caches/logs/backups and delete behavior are unknown and must be declared per approved server. |
| DG-DATA-08 / planned extensions; legacy source existing | Local or explicit Git source, source URL/path, revision/digest, manifest, license/notice data, requested/granted capabilities, trust decision, installation/activation state, content, and errors. Source is untrusted; provenance/trust data is internal; handled content inherits its source. | Target flow fetches/copies only on explicit action, validates manifest and immutable provenance, records trust/grants, and keeps inactive on mismatch. Target manager does not exist. Legacy plugin/skill code can copy and enable without these controls but is absent from the root. | Local extension directories and planned explicit Git-source transfer. Git host, account, request metadata, residency, and jurisdictions are unknown. Runtime filesystem/process/MCP/credential/network access must be denied unless enforceable and granted. | Trust-record, source-copy, cache, uninstall, revocation, retention, backup, and deletion semantics are unspecified. Git hosting retains its own copies under unresolved terms. No marketplace or background update is allowed. |
| DG-DATA-09 / legacy source, excluded root | Knowledge inputs, chunks/records, search queries/results, goal/plan/lifecycle state, dispatch tasks/history, and Git remote metadata in source-visible legacy packages. Classification would inherit source. | Legacy packages can read/persist/remove or execute data, but the v1 root deliberately omits knowledge, goal, plan, dispatch, Git, plugin/skill, and MCP groups. Rooted `sdlc` is read-only inspection. | Legacy local `.lana` stores and possible Git/process egress if reintroduced. They are not v1 stores or approved transfers. | Keep absent or reject before action. Re-entry requires a separate inventory, owner-confirmed lifecycle, authority contract, and G4/G5 review; no existing legacy retention behavior is accepted here. |
| DG-DATA-10 / existing CI and local release | Source, dependency metadata/downloads, build metadata (`VERSION`, `COMMIT`, epoch), test fixtures/results, logs, binaries, completions, archives, checksums, and optional SBOM/vulnerability/license output. Source classification is internal; release distribution classification is pending. | GitHub Actions checks out source and Go, then `make ci` formats/vets/tests/builds four Linux/macOS targets, verifies reproducibility/checksums/install, and optionally scans/generates SBOM. Optional checks skip successfully when tools are absent. | GitHub-hosted runners and dependency/action providers are external processing boundaries with unresolved runner region/jurisdiction and service retention. `dist/` is local/runner storage; the workflow does not upload or publish it. Release scripts explicitly do not contact a publisher or signer. | Local `dist/` persists until manual cleanup. Runner/log/cache retention is controlled by unresolved GitHub/repository settings. No publication, signature, notarization, or downstream deletion exists. Source revision is `unknown` without Git unless explicitly supplied. |

The telemetry stage for every category DG-DATA-01 through DG-DATA-10 is
**none permitted**: no category or derived output may be collected, queued, or
exported as product telemetry. The transfers explicitly named in the table are
functional or build-system flows, not telemetry exceptions. Any backup or
export not named there is unknown and must be inventoried before use; any
deletion or recovery not named there is not implemented or guaranteed.

## 3. Credential and provider boundary

Existing controls are portable request/event types with no credential field,
adapter error redaction, structured-payload sanitization, configuration
redaction helpers, and readiness failure before a default unconfigured runtime
creates a session. These reduce disclosure risk but do not establish a provider
information-handling decision.

Before any provider or OAuth network backend is enabled, DG-CTRL-01 requires a
deagy-owned, revisioned provider register entry containing: provider/legal
entity and service; exact endpoints and model identifiers; account/API product;
terms and privacy-notice versions; every transmitted data category; purpose;
provider use for training, abuse monitoring, support, or human review;
retention/deletion behavior; subprocessors; residency and processing
jurisdictions; opt-out/settings; incident contact; and an explicit allow/deny
decision for each Lana classification. Unknown applicable semantics block that
provider. G5 separately owns OAuth topology, secret-store selection, token
lifecycle, transport, and revocation controls.

## 4. Residency, sharing, non-egress, and minimization

The intended product boundary is local-first. Lana must have no collector,
analytics SDK, crash uploader, remote queue, background scheduler, Lana
control-plane endpoint, or deferred upload store. The only proposed runtime
egress exceptions are a user-initiated approved provider request, approved
OAuth action, and approved explicit Git extension fetch. CI dependency/action
retrieval is a build-system transfer, not product telemetry, and must be
inventoried separately.

This is a design boundary, not completed negative evidence. A source scan found
no production Go import of `net/http` and no production HTTP endpoint literal
in the bound tree; endpoint literals found are test fixtures, JSON-schema
identifiers, and legacy MCP examples. That result does not prove transitive
dependencies, subprocesses, DNS, provider backends, future extensions, or CI
tools cannot egress.

Enforcement requirements:

- DG-CTRL-02: maintain an allowlist of the exact network-capable product paths;
  bind each to an explicit foreground user action, approved provider/source,
  purpose, transmitted fields, and cancellation. Startup, local file/session
  actions, completion, help, and read-only `sdlc` must be network-silent.
- DG-CTRL-03: exclude credentials and minimize provider context, tool output,
  MCP content, and Git request metadata to fields necessary for the requested
  action. Do not send whole environment/configuration/workspace data by default.
- DG-CTRL-04: treat an MCP child, tool subprocess, or extension as a potential
  egress path. Default environment and network capability to denied/empty and
  fail closed when G5 cannot enforce that boundary.
- DG-CTRL-05: record developer-host, provider, Git host, CI runner, artifact
  host, backup, and subprocess jurisdictions. `unknown` is acceptable in this
  preparation record but blocks enabling the affected cross-boundary flow.

## 5. Retention, deletion, backup, and recovery

No duration is selected by this assessment. Until the decisions in section 8
are made, Lana must not imply automatic expiry, guaranteed erasure, or provider
deletion. The following controls are required after those human choices:

- DG-CTRL-06: implement and document session list/inspect/delete semantics that
  cover the selected session, all forks/copies chosen by the user, indexes, and
  derived state. Confirm before deletion and emit only non-content evidence.
- DG-CTRL-07: document that workspace/VCS/OS/cloud backups and provider/Git/CI
  copies are separate deletion domains. Record which domains receive deletion
  requests, which expire by an approved schedule, and which cannot be recalled.
- DG-CTRL-08: define log destinations, owner-only permissions, rotation,
  retention, and deletion. Change local file creation from `0644` to `0600` (or
  a demonstrably equivalent owner-only control) before file logs can contain
  internal derived diagnostics; request/response/session/tool bodies remain
  prohibited by default.
- DG-CTRL-09: define extension source/cache/trust-record uninstall and deletion,
  MCP child-state lifecycle, configuration cleanup, and credential-reference
  removal. Secret deletion/revocation implementation remains G5 work.
- DG-CTRL-10: keep recovery distinct from retention: session valid-prefix
  recovery must not silently erase valid history; forks are independent copies;
  local release artifacts rebuild from bound source; backup restore must not
  reintroduce records beyond the approved lifecycle without detection.

Database backup/restore execution is not applicable to the bound v1 design:
there is no production database. Any later database introduction requires
database-reliability-engineer coordination plus a revised lineage assessment.

## 6. MCP and extension governance requirements

- DG-CTRL-11: remove or reject legacy `mcp.servers[].uri` and all remote
  transports before action. Each local server needs an owner-approved descriptor,
  executable identity/digest, purpose, data categories, classification ceiling,
  environment allowlist, capabilities, limits, child-state locations,
  retention/deletion statement, and evidence that unsupported data fails closed.
- DG-CTRL-12: require an extension inventory entry for provenance, immutable Git
  revision/digest, author/source, license and notices, requested/granted
  capabilities, classification ceiling, network destinations, local stores,
  update behavior, retention/deletion, and revocation. Missing license,
  provenance, trust, or enforceable grants keeps the extension inactive.
- DG-CTRL-13: trust decisions must be explicit, attributable, timestamped, bound
  to the exact descriptor/provenance and grant set, locally inspectable, and
  revocable. A manifest, local location, or prior version's trust is not enough.

## 7. CI, release, and evidence requirements

- DG-CTRL-14: pin CI actions/tools to reviewed immutable versions; inventory Go
  module/action/tool downloads and their destinations; prevent prompts,
  sessions, local config, credentials, `.lana` state, or logs from entering build
  context, cache, test output, SBOM, archive, provenance, or release notes.
- DG-CTRL-15: make required release checks fail closed. A vulnerability,
  license, SBOM, telemetry/endpoint, secret, or provenance check required for a
  release candidate cannot retain the current optional `SKIPPED` success
  behavior. Local developer convenience may remain explicitly non-release.
- DG-CTRL-16: package allowlist tests must prove each archive contains only the
  intended binary and completions. Secret/content scans must cover binaries,
  archives, checksums, SBOM, logs, and provenance. Publication is prohibited
  until distribution classification, artifact host, retention/removal, signing,
  and source-revision identity are decided.
- DG-CTRL-17: preserve evidence with artifact hashes, source binding, actor,
  command/tool versions, environment class, timestamps, results, and independent
  reviewer. Evidence retention duration is itself pending human decision.

Required verification obligations:

| Evidence ID | Procedure and pass condition |
|---|---|
| DG-TEST-01 | Seed unique credential canaries through provider errors, config, tool arguments/results, MCP/extension failures, cancellation, and logging; no canary appears in stdout/stderr, sessions, log files, CI logs, or release artifacts. |
| DG-TEST-02 | Seed classified prompt/workspace/tool content; demonstrate inheritance through request, event, tool result, session, fork, logs, MCP/extension mediation, and artifacts, with prohibited destinations blocked. |
| DG-TEST-03 | Observe DNS and network traffic for startup, help/completion, local file/session operations, read-only `sdlc`, tests, and idle time; pass only with zero product egress. Separately exercise each foreground exception and match destination/fields to DG-CTRL-02. |
| DG-TEST-04 | Dependency/source/binary scan finds no telemetry SDK, endpoint, queue, crash uploader, analytics identifier, deferred-upload store, or unexpected network client; every exception is reviewed. |
| DG-TEST-05 | Delete a session graph after the human deletion decision; prove selected session/forks/indexes/derived local state are gone and report backup/provider domains separately rather than claiming global erasure. |
| DG-TEST-06 | Verify session directory/file and log/config/trust-state permissions on Linux and macOS; fail when files are group/world readable or a secret is persisted. |
| DG-TEST-07 | Reject remote/incomplete/untrusted MCP descriptors before launch; prove environment/network deny defaults, message/resource bounds, cancellation, redaction, and session lifecycle behavior. |
| DG-TEST-08 | Reject an extension with missing/mismatched provenance, license decision, trust, classification ceiling, or enforceable grant; uninstall/revoke according to the human lifecycle decision. |
| DG-TEST-09 | Run release CI with required tools absent and prove the release candidate fails; inspect runner logs/caches/artifacts and verify the archive allowlist, source binding, SBOM, scans, and no publication side effect. |
| DG-TEST-10 | Restore/recover a torn session and any approved backup fixture; prove only a valid prefix is recovered and expired/deleted records are not silently resurrected. |

These extend planned `TEST-AUTH-*`, `TEST-SES-*`, `TEST-MCP-*`, `TEST-EXT-*`,
`TEST-NET-*`, `TEST-SEC-*`, and `TEST-REL-*` in the traceability matrix. They
are requirements, not claims of execution.

## 8. Pending human decisions assigned to deagy

Every item below is unresolved. The accountable owner assignment does not
answer it or confer gate approval.

| Decision ID | Required human choice | Why it blocks |
|---|---|---|
| DG-DEC-01 | Select the formal classification taxonomy/handling matrix and decide whether each v1 data category and derived output is allowed at each boundary. | Applicable classification semantics remain unknown. |
| DG-DEC-02 | Select the provider and OAuth service policy after reviewing the provider-register fields in section 3, including jurisdictions, retention/deletion, training/abuse use, and subprocessors. | Provider transfer cannot be authorized from interface code alone. |
| DG-DEC-03 | Set retention triggers/durations for sessions and forks, logs, config/trust/provenance records, MCP/extension state, CI logs/caches, local `dist/`, release evidence, and published artifacts if publication is later approved. | No current artifact or external policy supplies these durations. |
| DG-DEC-04 | Select deletion scope and user semantics for sessions/forks, logs, config, credential references, MCP/extension state, CI data, backups, provider/Git copies, and published artifacts; decide required evidence and exceptions. | Local file deletion cannot support a global-erasure claim. |
| DG-DEC-05 | Identify developer-host/backup, provider, Git, CI runner, artifact-host, and subprocess jurisdictions and select residency/non-egress constraints for each classification. | External/host jurisdictions and backup topology are unknown. |
| DG-DEC-06 | Select approved MCP servers and extension sources/licensing rules, classification ceilings, capability policy, trust-record lifecycle, and child/source data handling. | No server/extension is governable from a generic descriptor alone. |
| DG-DEC-07 | Decide whether release artifacts are internal or public, select the publication host, log/cache/artifact retention/removal policy, and reconcile the current GitHub Actions workflow with the project GitLab CI standard. | Current CI processes source externally but does not publish; the intended release boundary is incomplete. |
| DG-DEC-08 | Select G4 evidence-retention duration and the authoritative evidence store. | The shared baseline explicitly does not provide a project-specific evidence schedule. |

## 9. G4 preparation conclusion

Inventory, lineage, present/target distinction, accountable owner, enforcement
points, and verification obligations are recorded for the bound working tree.
G4 is **not decided by this artifact**. DG-DEC-01 through DG-DEC-08 and the
provider/MCP/extension platform semantics they control remain `unknown` and
must be resolved by deagy before an independent G4 review can conclude that all
applicable governance/data criteria are satisfied. G5 remains independently
responsible for security enforcement and secret, OAuth, sandbox, process,
signing, and capability controls; this record does not approve those controls.
