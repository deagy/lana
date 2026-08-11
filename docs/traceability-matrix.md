# Lana — Local Agents and Knowledge Traceability

## Version: 2.0
## Date: 2026-08-10
## Requirement baseline: local agents and knowledge store

This matrix maps the supported local product behavior to implementation areas and deterministic evidence. It intentionally has no release or authority records.

| Requirement | Primary implementation | Deterministic evidence | Acceptance scenario |
|---|---|---|---|
| FR-AGT-001 | `internal/agents/registry.go`, `internal/agents/queue.go`, `internal/cmd/agents` | Registry and queue tests reject unknown roles and invalid/non-JSON task input. | — |
| FR-AGT-002 | `internal/cmd/agents/agents.go` | Command tests verify standalone `work` fails before an executor is invoked. | — |
| FR-AGT-003 | `internal/agents/store.go` | Agent-store tests run concurrent workers and separate helper processes. | SCN-AGT-001 |
| FR-AGT-004 | `internal/agents/queue.go`, `internal/agents/store.go` | Queue/store tests cover cancellation, cancellation-precedence, expired leases, and recovery. | SCN-AGT-002 |
| FR-KNO-001 | `internal/knowledge/store.go`, `internal/cmd/knowledge` | Knowledge-store tests cover local ingestion, file/document bounds, and source registration. | — |
| FR-KNO-002 | `internal/knowledge/store.go` | Knowledge-store tests cover concurrent process updates and source/index symlink rejection. | SCN-KNO-001 |
| FR-KNO-003 | `internal/knowledge/store.go`, `internal/cmd/knowledge` | Store and command tests cover stable search, document/source removal, and explicit force confirmation. | — |
| FR-KNO-004 | `internal/cmd/knowledge/knowledge.go` | Command tests verify control characters are escaped for humans while JSON remains machine-readable. | SCN-KNO-002 |

## Acceptance scenario inventory

| ID | Behavior |
|---|---|
| SCN-AGT-001 | Concurrent local workers claim once and preserve a durable terminal update. |
| SCN-AGT-002 | A running task's cancellation survives restart recovery and overrides a successful completion. |
| SCN-KNO-001 | A no-follow knowledge-index read rejects a symlink rather than following it. |
| SCN-KNO-002 | Human-readable knowledge output renders terminal control and format characters visibly. |
