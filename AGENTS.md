# Agent System

Lana supports structured agent orchestration with roles, work items, and lifecycle tracking.

## Agent Roles

Agents are organized into roles that define their responsibilities:

- **Primary**: Main agent executing the task
- **Reviewer**: Agent reviewing the work
- **Support**: Advisory agents providing guidance

## Work Items

### Goals

Goals define high-level objectives:

```bash
# Create a goal with budget
lana goal create \
  --objective "Implement MCP client" \
  --with-budget \
  --token-budget 500
```

### Plans

Plans break goals into actionable steps:

```bash
# Create a plan with steps
lana plan create \
  --step "Design MCP protocol" \
  --step "Implement JSON-RPC" \
  --step "Add transport" \
  --goal-id <goal-id>
```

### Tasks

Tasks are individual units of work:

```bash
# List tasks
lana agents list

# Get task details
lana agents get <task-id>

# Complete a task
lana agents complete <task-id> --summary "Done"
```

## Agent Dispatch

Agents can be dispatched to execute tasks:

```bash
# Dispatch agents for a task
lana agents dispatch --task-id <task-id>

# Check dispatch status
lana agents status --task-id <task-id>
```

## Lifecycle Integration

Agent work items integrate with Agentic SDLC:

```bash
# Initialize SDLC for a project
cadre sdlc init --profile default --project my-project

# Plan a task
lana sdlc plan --task-id my-task --task "Implement feature"

# Check status
lana sdlc status --task-id my-task

# Record decisions
lana sdlc decide --task-id my-task --gate G1 --role reviewer --decision approve
```

## Custom Agents

Create custom agent roles by implementing the agent interface:

```go
type Agent interface {
    Name() string
    Execute(ctx context.Context, task Task) (Result, error)
}
```

## Best Practices

1. **Define clear objectives**: Goals should be specific and measurable
2. **Break into steps**: Plans should have actionable, testable steps
3. **Track progress**: Use SDLC gates to track lifecycle stages
4. **Review work**: Assign reviewers for quality assurance
5. **Document decisions**: Record rationale for important choices
