---
name: beads-batch-ops
description: 'Batch beads task operations to minimize tool calls. Use when: (1) creating ≥3 tasks, (2) bulk status updates, (3) generating reports/queries, (4) creating task hierarchies (epics with children), (5) batch closing/starting tasks, (6) processing tasks with jq filters. Creates single scripts that run multiple bd commands instead of separate tool calls.'
---

# Beads Batch Operations

Execute multiple beads commands in a single script to minimize tool calls and improve efficiency.

## Core Principle

**One script, many operations** - Create a single Bash script that executes multiple `bd` commands in sequence, rather than making separate tool calls for each operation.

## When to Use This Skill

Load this skill when you need to:

- **Create ≥3 related tasks** - Batch task creation instead of individual tool calls
- **Bulk status updates** - Move multiple tasks through workflow states
- **Generate reports** - Query and format task data with jq
- **Create hierarchies** - Build epics with child tasks in one script
- **Batch close/start** - Transition multiple tasks at once
- **Complex queries** - Filter, sort, and transform task data

## Quick Start

### Pattern 1: Batch Task Creation

```bash
#!/usr/bin/env bash
set -euo pipefail

npx bd create --title "Task 1" --description "Description 1"
npx bd create --title "Task 2" --description "Description 2"
npx bd create --title "Task 3" --description "Description 3"

echo "✅ Created 3 tasks"
```

### Pattern 2: Create Epic with Children

```bash
#!/usr/bin/env bash
set -euo pipefail

# Create epic, capture ID
epic_id=$(npx bd create --title "User Authentication" \
  --description "Complete auth system" \
  --format json | jq -r '.id')

# Create children under epic
npx bd create --title "Implement JWT" --parent "$epic_id" --description "Token generation"
npx bd create --title "Build login endpoint" --parent "$epic_id" --description "POST /api/auth/login"

echo "✅ Created epic with 2 children"
```

### Pattern 3: Bulk Status Updates

```bash
#!/usr/bin/env bash
set -euo pipefail

# Start all ready tasks
ready_tasks=$(npx bd ready --format json | jq -r '.[].id')

for task_id in $ready_tasks; do
  npx bd start "$task_id"
  echo "Started: $task_id"
done
```

### Pattern 4: Generate Status Report

```bash
#!/usr/bin/env bash
set -euo pipefail

echo "=== Task Status Report ==="
echo
echo "📊 Counts by Status:"
npx bd list --format json | jq -r '
  group_by(.status) |
  map({status: .[0].status, count: length}) |
  .[] |
  "  \(.status): \(.count)"
'

echo
echo "📋 Ready Tasks:"
npx bd ready --format json | jq -r '.[] | "  - [\(.id)] \(.title)"'
```

## Essential jq Patterns

### Filter Tasks

```bash
# High priority tasks
npx bd list --format json | jq '.[] | select(.priority == "high")'

# Tasks in specific phase
npx bd list --format json | jq '.[] | select(.phase == "implementation")'

# Tasks with specific tag
npx bd list --format json | jq '.[] | select(.tags[]? == "bug")'
```

### Extract Data

```bash
# Get all task IDs
npx bd list --format json | jq -r '.[].id'

# Get titles and statuses
npx bd list --format json | jq -r '.[] | "\(.title) - \(.status)"'
```

### Count and Aggregate

```bash
# Count by status
npx bd list --format json | jq -r '
  group_by(.status) |
  map({status: .[0].status, count: length}) |
  .[]
'
```

## Best Practices

1. **Always use `set -euo pipefail`** - Fail fast on errors
2. **Capture IDs when needed** - Use `--format json | jq -r '.id'` for dependent operations
3. **Validate before operating** - Check state before bulk updates
4. **Provide progress feedback** - Echo status messages for long-running scripts
5. **Use arrays for static data** - Define multiple similar tasks in Bash arrays
6. **Add summary output** - End with summary of operations performed

## Script Template

```bash
#!/usr/bin/env bash
set -euo pipefail

# Your batch operations here

echo "✅ Summary of operations"
```

## Detailed References

For comprehensive patterns and examples, see:

| Need                          | Reference                                                    |
| ----------------------------- | ------------------------------------------------------------ |
| Full batch operation patterns | [references/batch-patterns.md](references/batch-patterns.md) |
| jq filtering and processing   | [references/jq-cookbook.md](references/jq-cookbook.md)       |
| Beads JSON schema details     | [references/beads-schema.md](references/beads-schema.md)     |

## Common Operations Quick Links

- **Bulk creation** → [batch-patterns.md § Pattern 1](references/batch-patterns.md#pattern-1-bulk-task-creation)
- **Task hierarchies** → [batch-patterns.md § Pattern 1](references/batch-patterns.md#creating-task-hierarchies-epic--children)
- **Status updates** → [batch-patterns.md § Pattern 2](references/batch-patterns.md#pattern-2-bulk-status-updates)
- **Reporting** → [batch-patterns.md § Pattern 3](references/batch-patterns.md#pattern-3-batch-queries-and-reporting)
- **jq filtering** → [jq-cookbook.md § Filtering](references/jq-cookbook.md#filtering-patterns)
- **JSON schema** → [beads-schema.md](references/beads-schema.md)

## Integration with Other Workflows

This skill complements:

- **Spec workflow** (`/sp:*` skills) - Batch operations for phase tasks
- **Task management** (`bd` CLI) - Efficient multi-operation scripts
- **Reporting** - Generate status summaries and dashboards
