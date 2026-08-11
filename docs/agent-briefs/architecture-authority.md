# Agent Brief: Architecture Authority (Wave 1)

## Task ID
lana-implementation-plan

## Objective
Design the system architecture for the Lana CLI — a Codex CLI clone written in Go.

## Key Design Decisions Required
1. **CLI Framework**: cobra vs. cli (urfave) vs. built-in flag parsing
2. **Module Structure**: monorepo package layout, separation of concerns
3. **Agent Dispatch Model**: how subagents are spawned and coordinated
4. **MCP Protocol**: how to connect to and interact with MCP servers
5. **Persistence Layer**: file-based config, how to store goals/plans/records
6. **Plugin System**: how to discover, load, and execute plugins/skills
7. **Extension Points**: how users can extend Lana's capabilities
8. **Concurrency Model**: goroutine-based parallel dispatch

## Output Artifacts Required
1. docs/architecture.md - System architecture with:
   - Component diagram (text description)
   - Module/package layout
   - Data flow between components
   - Decision log with rationale
2. docs/module-design.md - Detailed module specifications
3. docs/api-contracts.md - Interface contracts for each module

## Constraints
- Go 1.24+ only
- Local CLI tool — no server component needed
- Must support the full Codex CLI feature surface listed in requirements
- Follow project technology-standards.md and team-profile.yaml
