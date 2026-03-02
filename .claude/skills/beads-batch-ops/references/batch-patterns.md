# Batch Operation Patterns

This reference covers efficient batching patterns for beads commands to minimize tool calls.

## Core Principle

**One script, many operations** - Create a single Bash script that executes multiple `bd` commands in sequence, rather than making separate tool calls for each operation.

## Pattern 1: Bulk Task Creation

### Creating Multiple Independent Tasks

```bash
#!/usr/bin/env bash
set -euo pipefail

# Create multiple tasks in one script
npx bd create --title "Implement user authentication" \
  --description "Add JWT-based auth system with login/logout endpoints"

npx bd create --title "Add password hashing" \
  --description "Use bcrypt for secure password storage"

npx bd create --title "Create auth middleware" \
  --description "Middleware to verify JWT tokens on protected routes"

npx bd create --title "Write auth integration tests" \
  --description "Test login, logout, and protected route access"

echo "✅ Created 4 authentication tasks"
```

### Creating Task Hierarchies (Epic + Children)

> **ralph.sh epic naming convention**: `ralph.sh` matches epics to branches by stripping
> the numeric prefix from the branch name and converting hyphens to spaces, then doing a
> substring match against epic titles. For branch `003-smoke-test-bugs`, the feature slug
> is `smoke-test-bugs` → `smoke test bugs`. The epic title **must contain this exact slug**
> (case-insensitive). Use `"NNN: <feature-slug-with-spaces>"` as the title format.
> Example: branch `003-smoke-test-bugs` → epic title `"003: smoke test bugs"`.

```bash
#!/usr/bin/env bash
set -euo pipefail

# Create epic first — use --silent to get just the ID (--format json is not supported)
# IMPORTANT: epic title must contain the branch feature slug (hyphens→spaces) for ralph.sh
# Branch 003-my-feature → title must contain "my feature"
epic_id=$(bd create --title "003: my feature" \
  --description "Complete auth system with JWT, password hashing, and middleware" \
  --type epic \
  --silent)

echo "Created epic: $epic_id"

# Create child tasks under the epic
npx bd create --title "Implement JWT generation" \
  --description "Create token signing and verification functions" \
  --parent "$epic_id"

npx bd create --title "Build login endpoint" \
  --description "POST /api/auth/login with email/password validation" \
  --parent "$epic_id"

npx bd create --title "Build logout endpoint" \
  --description "POST /api/auth/logout to invalidate tokens" \
  --parent "$epic_id"

npx bd create --title "Add auth middleware" \
  --description "Middleware to protect routes requiring authentication" \
  --parent "$epic_id"

echo "✅ Created epic with 4 child tasks"
```

## Pattern 2: Bulk Status Updates

### Moving Multiple Tasks Through Workflow

```bash
#!/usr/bin/env bash
set -euo pipefail

# Get all ready tasks and start them
ready_tasks=$(npx bd ready --format json | jq -r '.[].id')

for task_id in $ready_tasks; do
  npx bd start "$task_id"
  echo "Started: $task_id"
done

echo "✅ Started $(echo "$ready_tasks" | wc -l) tasks"
```

### Bulk Close Pattern

```bash
#!/usr/bin/env bash
set -euo pipefail

# Close multiple tasks by ID
task_ids=("task-001" "task-002" "task-003" "task-004")

for id in "${task_ids[@]}"; do
  npx bd close "$id"
  echo "Closed: $id"
done

echo "✅ Closed ${#task_ids[@]} tasks"
```

## Pattern 3: Batch Queries and Reporting

### Generate Status Report

```bash
#!/usr/bin/env bash
set -euo pipefail

echo "=== Beads Task Status Report ==="
echo

# Count by status using jq
echo "📊 Task Counts by Status:"
npx bd list --format json | jq -r '
  group_by(.status) |
  map({status: .[0].status, count: length}) |
  .[] |
  "  \(.status): \(.count)"
'

echo
echo "📋 Ready Tasks:"
npx bd ready --format json | jq -r '.[] | "  - [\(.id)] \(.title)"'

echo
echo "🚧 In Progress Tasks:"
npx bd list --format json | jq -r '
  .[] |
  select(.status == "in_progress") |
  "  - [\(.id)] \(.title)"
'

echo
echo "✅ Recently Closed (last 5):"
npx bd list --format json | jq -r '
  .[] |
  select(.status == "closed") |
  "\(.closedAt // .updatedAt) \(.id) \(.title)"
' | sort -r | head -5 | cut -d' ' -f2- | sed 's/^/  - [/' | sed 's/ /] /'
```

### Filter Tasks by Criteria

```bash
#!/usr/bin/env bash
set -euo pipefail

# Find high-priority tasks in specific phase
echo "🔥 High Priority Tasks in Development Phase:"
npx bd list --format json | jq -r '
  .[] |
  select(.phase == "development" and .priority == "high") |
  "  [\(.id)] \(.title) - \(.status)"
'

# Find tasks with specific tags
echo
echo "🏷️ Tasks Tagged 'security':"
npx bd list --format json | jq -r '
  .[] |
  select(.tags[]? == "security") |
  "  [\(.id)] \(.title)"
'
```

## Pattern 4: Conditional Batch Operations

### Process Tasks Based on State

```bash
#!/usr/bin/env bash
set -euo pipefail

# Start tasks only if no tasks currently in progress
in_progress_count=$(npx bd list --format json | jq '[.[] | select(.status == "in_progress")] | length')

if [ "$in_progress_count" -eq 0 ]; then
  echo "No tasks in progress. Starting next ready task..."
  next_task=$(npx bd ready --format json | jq -r '.[0].id')

  if [ "$next_task" != "null" ] && [ -n "$next_task" ]; then
    npx bd start "$next_task"
    echo "✅ Started: $next_task"
  else
    echo "No ready tasks available"
  fi
else
  echo "⚠️ $in_progress_count task(s) already in progress. Not starting new tasks."
fi
```

### Batch Create with Validation

```bash
#!/usr/bin/env bash
set -euo pipefail

# Read task definitions from array
tasks=(
  "Implement user login|Add JWT-based authentication with email/password"
  "Create user registration|New user signup with email verification"
  "Add password reset|Implement forgot password flow with email tokens"
)

created=0
failed=0

for task_def in "${tasks[@]}"; do
  IFS='|' read -r title description <<< "$task_def"

  # Check if task with similar title already exists
  existing=$(npx bd list --format json | jq -r --arg title "$title" '
    .[] | select(.title == $title) | .id
  ')

  if [ -z "$existing" ]; then
    npx bd create --title "$title" --description "$description"
    echo "✅ Created: $title"
    ((created++))
  else
    echo "⚠️ Skipped (exists): $title [$existing]"
    ((failed++))
  fi
done

echo
echo "Summary: Created $created, Skipped $failed"
```

## Pattern 5: Data Transformation Pipelines

### Extract and Reformat Data

```bash
#!/usr/bin/env bash
set -euo pipefail

# Generate markdown checklist from ready tasks
echo "## Ready Tasks Checklist"
echo
npx bd ready --format json | jq -r '.[] | "- [ ] \(.title) (`\(.id)`)"'
```

### Cross-Reference Tasks

```bash
#!/usr/bin/env bash
set -euo pipefail

# Find tasks blocked by in-progress tasks
echo "=== Task Dependencies ==="

# Get all tasks with blockedBy relationships
npx bd list --format json | jq -r '
  .[] |
  select(.blockedBy != null and (.blockedBy | length > 0)) |
  {id: .id, title: .title, blockedBy: .blockedBy, status: .status}
' | jq -s '
  .[] |
  "Task: \(.title) (\(.id))\n  Status: \(.status)\n  Blocked by: \(.blockedBy | join(", "))\n"
'
```

## Best Practices

1. **Always use `set -euo pipefail`** at the start of batch scripts to fail fast on errors
2. **Capture IDs when needed** - Use `--silent` to get just the ID from `bd create` (`--format json` is not a valid flag for `bd create`). For list commands, use `--json` flag.
3. **Validate before operating** - Check task state before bulk updates to avoid errors
4. **Provide progress feedback** - Echo status messages so users see progress in long-running scripts
5. **Use arrays for static data** - Define multiple similar tasks in Bash arrays for easy iteration
6. **Handle edge cases** - Check for null/empty results from jq before processing
7. **Add summary output** - Always end batch operations with a summary of what was done

## When to Use Batch Operations

- **Creating ≥3 related tasks** - Use batch creation instead of individual tool calls
- **Bulk status updates** - Moving multiple tasks through workflow states
- **Generating reports** - Querying and formatting task data
- **Conditional operations** - Complex logic that checks state before acting
- **Data migrations** - One-time operations on many existing tasks
