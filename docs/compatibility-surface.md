# Lana — Local Capability Surface

## Version: 2.0
## Date: 2026-08-10

Lana's supported product surface is deliberately small and local. This inventory describes observable command behavior and its safety constraints.

| Surface | Supported behavior | Safety boundary |
|---|---|---|
| `lana agents roles`, `describe` | Lists and describes the fixed local role registry. | Registry data is descriptive only; it does not select a remote model or launch a worker. |
| `lana agents submit`, `list`, `show`, `cancel` | Persists, inspects, and cancels structured local tasks. | Input must be valid JSON and a registered role. Task data is never interpreted as shell text or a provider request. |
| `lana agents work` | Runs only when an embedding application supplies an explicit local executor. | The standalone CLI rejects work before execution. Claims are owner-bound and lease-bound; cancellation is durable and wins over completion. |
| `lana knowledge ingest` | Registers a user-selected local text file or directory and updates the local index. | Bounded regular files only; source and store paths use no-follow access. No network, provider, embeddings, or credential processing. |
| `lana knowledge search`, `list`, `sources` | Reads the local index with deterministic ordering and citations. | Human-readable output escapes terminal control and format characters. JSON retains machine-readable stored values. |
| `lana knowledge remove` | Removes a selected document or all documents for a selected source. | It requires `--force`; source-wide removal requires the explicit `--source` selector. Store mutations are locked, atomic, and directory-synchronized. |

## Non-product surfaces

Remote dispatch, remote Git or PR management, HTTP/remote MCP, extensions, plugins, skills, shell execution, provider chat, telemetry, and product-management commands are not part of this agents-and-knowledge product contract. Their presence in source history or an embedding program does not extend the supported local surface above.

## Regression evidence

- Deterministic acceptance scenarios in [acceptance-scenarios.feature](acceptance-scenarios.feature) cover task claims/updates, cancellation/recovery, no-follow rejection, and terminal-safe output.
- Package tests exercise process-level store coordination and bounded local knowledge ingestion.
