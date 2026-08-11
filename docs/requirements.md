# Lana — Local Agents and Knowledge Store Requirements

## Version: 2.0
## Date: 2026-08-10
## Scope: local agents and local knowledge store

Lana supports two local, single-user capabilities: a durable structured-agent queue and a local knowledge store. This document describes the implemented product boundary only.

## Product boundary

### Supported local capabilities

- `lana agents` manages a fixed registry of local roles and durable, structured work items.
- `lana knowledge` ingests, searches, lists, and removes locally stored textual knowledge.

Both capabilities operate only on local files selected by the user or the configured workspace. Neither capability launches a shell, contacts a provider, discovers remote agents, sends telemetry, nor treats stored data as executable instructions.

### Exclusions

- Remote agent dispatch, collaboration, fleet management, background workers, and cloud control planes.
- Shell evaluation or provider execution derived from an agent task's input.
- Network retrieval, embeddings, telemetry, or credential handling in the knowledge store.
- Product-management, approval, retention-policy, and release workflows.

## Functional requirements

### Local agents

**FR-AGT-001 — Structured local tasks.** Lana MUST accept only registered local roles and valid JSON task input. Task input, result, and metadata are data records, never shell text, provider prompts, credentials, or an instruction to launch a process.

**FR-AGT-002 — Explicit execution boundary.** The standalone `lana agents` command MUST only record, inspect, and cancel tasks. It MUST fail clearly when `work` is requested without an embedding application supplying an explicit local `Executor`.

**FR-AGT-003 — Concurrent durable claims.** The durable task store MUST serialize mutations across independently started local processes. At most one valid worker may claim a queued task at a time; only that worker may renew or complete its lease.

**FR-AGT-004 — Cancellation and recovery.** Cancelling a queued task MUST make it terminal. Cancelling a running task MUST persist a cancellation request, signal a local owner when available, and take precedence over a later successful completion. After a stopped worker's lease expires, recovery MUST either requeue an un-cancelled task or make a cancellation request terminal.

### Local knowledge store

**FR-KNO-001 — Local, bounded ingestion.** Knowledge ingestion MUST register an explicit local regular file or directory, accept only supported text-file types, bound document count and bytes, and persist a local index. The store has no network, provider, embedding, or credential dependency.

**FR-KNO-002 — Safe source and store access.** The store MUST reject symlinked source files, index files, and store-directory path components rather than following them. Mutating operations MUST lock the full read-modify-write transaction, atomically replace the index, and synchronize the containing directory before reporting success.

**FR-KNO-003 — Deterministic retrieval and removal.** Search MUST use deterministic local matching and stable result ordering. `remove` MUST require an explicit `--force` confirmation and distinguish removal of one document from removal of a whole source.

**FR-KNO-004 — Terminal-safe human output.** Human-readable knowledge output MUST render control and format characters visibly, so stored content cannot emit terminal control sequences or disguise direction. JSON output may preserve the exact stored value for machine consumers.

## Safety constraints

- Agent task records are durable local metadata under the workspace. A caller that supplies an executor is responsible for that executor's effects and idempotence after lease recovery.
- Knowledge data is local user-selected content. The store does not grant it authority, execute it, or send it anywhere.
- Concurrent correctness depends on cooperating local Lana processes using the store's advisory locks; external non-cooperating modification is outside this interface.
- The supported implementation targets Linux and macOS, where descriptor-relative no-follow access and advisory locking are available.

## Acceptance expectations

Automated acceptance coverage MUST demonstrate: concurrent task claim and durable update behavior; durable cancellation and restart recovery; no-follow rejection for a knowledge index; and terminal-safe rendering of indexed control or format characters. Passing tests demonstrate only the corresponding local behavior.
