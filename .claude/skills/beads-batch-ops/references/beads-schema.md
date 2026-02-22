# Beads JSON Schema Reference

This document describes the JSON structure returned by beads commands when using `--format json`.

## Task Object Structure

```json
{
  "id": "task-001",
  "title": "Implement user authentication",
  "description": "Add JWT-based auth system with login/logout endpoints",
  "status": "open",
  "priority": "high",
  "phase": "implementation",
  "tags": ["auth", "security"],
  "parent": "epic-001",
  "children": ["task-002", "task-003"],
  "blockedBy": ["task-004"],
  "blocks": ["task-005"],
  "createdAt": "2026-01-22T10:30:00Z",
  "updatedAt": "2026-01-22T14:45:00Z",
  "startedAt": "2026-01-22T11:00:00Z",
  "closedAt": null,
  "metadata": {
    "customField": "value"
  }
}
```

## Field Descriptions

### Core Fields

| Field         | Type     | Required | Description                                    |
| ------------- | -------- | -------- | ---------------------------------------------- |
| `id`          | string   | Yes      | Unique task identifier                         |
| `title`       | string   | Yes      | Short task title                               |
| `description` | string   | No       | Detailed task description                      |
| `status`      | string   | Yes      | Current task status (see Status Values)        |
| `priority`    | string   | No       | Task priority: `high`, `medium`, `low`, `none` |
| `phase`       | string   | No       | Project phase (custom values)                  |
| `tags`        | string[] | No       | Array of tag strings                           |

### Relationships

| Field       | Type     | Description                            |
| ----------- | -------- | -------------------------------------- |
| `parent`    | string   | ID of parent task (null if top-level)  |
| `children`  | string[] | Array of child task IDs                |
| `blockedBy` | string[] | Array of task IDs that block this task |
| `blocks`    | string[] | Array of task IDs this task blocks     |

### Timestamps

| Field       | Type   | Description                       |
| ----------- | ------ | --------------------------------- |
| `createdAt` | string | ISO 8601 timestamp of creation    |
| `updatedAt` | string | ISO 8601 timestamp of last update |
| `startedAt` | string | ISO 8601 timestamp when started   |
| `closedAt`  | string | ISO 8601 timestamp when closed    |

### Custom Data

| Field      | Type   | Description                      |
| ---------- | ------ | -------------------------------- |
| `metadata` | object | Free-form JSON object for extras |

## Status Values

Beads uses a flexible status system. Common values:

- `open` - Task created but not ready to start
- `ready` - Task ready to be worked on
- `in_progress` - Task currently being worked on
- `blocked` - Task blocked by dependencies
- `closed` - Task completed
- `cancelled` - Task cancelled/abandoned

Custom statuses can be used based on project workflow.

## Priority Values

- `high` - High priority
- `medium` - Medium priority
- `low` - Low priority
- `null` - No priority set

## Array vs Single Task Format

### List Commands (`bd list`, `bd ready`)

Return an **array of tasks**:

```json
[
  {
    "id": "task-001",
    "title": "First task",
    ...
  },
  {
    "id": "task-002",
    "title": "Second task",
    ...
  }
]
```

Access with jq: `.[]` to iterate, `.[0]` for first item, `.[] | select(...)` to filter.

### Show Command (`bd show <id>`)

Returns a **single task object**:

```json
{
  "id": "task-001",
  "title": "First task",
  ...
}
```

Access with jq: `.` for the whole object, `.title` for specific field.

### Create Command (`bd create`)

Returns a **single task object** (the newly created task):

```json
{
  "id": "task-new-001",
  "title": "New task",
  ...
}
```

Useful for capturing the new task's ID:

```bash
new_id=$(npx bd create --title "Task" --description "Desc" --format json | jq -r '.id')
```

## Null vs Missing Fields

Fields may be:

1. **Missing** - Not present in JSON object
2. **Null** - Present with `null` value
3. **Empty** - Present with empty value (`""`, `[]`, `{}`)

When filtering or processing:

```bash
# Handle null priority
jq '.[] | .priority // "none"'

# Handle missing or null tags
jq '.[] | .tags // []'

# Check if field exists and is not null
jq '.[] | select(.priority != null)'

# Safe access with optional operator
jq '.[] | .tags[]?'
```

## Working with Timestamps

### Parse ISO 8601 Dates

```bash
# Extract date part
npx bd list --format json | jq -r '.[].createdAt | split("T")[0]'

# Compare dates (tasks created after specific date)
npx bd list --format json | jq '.[] | select(.createdAt > "2026-01-20")'

# Sort by date
npx bd list --format json | jq 'sort_by(.createdAt)'
```

### Calculate Duration

```bash
# Tasks closed within last 24 hours (requires date command)
cutoff=$(date -u -d '24 hours ago' +%Y-%m-%dT%H:%M:%SZ)
npx bd list --format json | jq --arg cutoff "$cutoff" '.[] | select(.closedAt > $cutoff)'
```

## Working with Relationships

### Find Tasks by Parent

```bash
# All children of epic-001
npx bd list --format json | jq '.[] | select(.parent == "epic-001")'
```

### Find Epics (Tasks with Children)

```bash
# Tasks that have children
npx bd list --format json | jq '.[] | select(.children != null and (.children | length > 0))'
```

### Find Blocked Tasks

```bash
# Tasks blocked by anything
npx bd list --format json | jq '.[] | select(.blockedBy != null and (.blockedBy | length > 0))'

# Tasks blocked by specific task
npx bd list --format json | jq '.[] | select(.blockedBy[]? == "task-001")'
```

### Resolve Blocker Details

```bash
# Get blocked task with blocker details
all_tasks=$(npx bd list --format json)

echo "$all_tasks" | jq --argjson all "$all_tasks" '
  .[] |
  select(.blockedBy != null) |
  {
    id: .id,
    title: .title,
    blockedBy: [
      .blockedBy[] as $bid |
      $all | .[] | select(.id == $bid) | {id, title, status}
    ]
  }
'
```

## Common Query Patterns

### Tasks Ready to Work On

```bash
# Ready status with no blockers
npx bd list --format json | jq '.[] | select(
  .status == "ready" and
  (.blockedBy == null or (.blockedBy | length == 0))
)'
```

### High Priority Open Tasks

```bash
npx bd list --format json | jq '.[] | select(
  .priority == "high" and
  (.status == "open" or .status == "ready")
)'
```

### Tasks in Epic

```bash
# Get epic ID, then find children
epic_id="epic-001"
npx bd list --format json | jq --arg eid "$epic_id" '.[] | select(.parent == $eid)'
```

## Performance Considerations

### Large Task Lists

When working with hundreds or thousands of tasks:

1. **Filter server-side if possible** - Use beads query features rather than processing all tasks
2. **Limit output early** - Use `.[:100]` to take first 100 items
3. **Project only needed fields** - `map({id, title, status})` instead of full objects
4. **Cache results** - Store JSON in variable for multiple queries

```bash
# Good: Store once, query multiple times
tasks=$(npx bd list --format json)
echo "$tasks" | jq 'query 1'
echo "$tasks" | jq 'query 2'

# Bad: Query database each time
npx bd list --format json | jq 'query 1'
npx bd list --format json | jq 'query 2'
```

## Tips for jq Queries

1. **Test incrementally** - Start with `.[]` then add filters
2. **Check types** - Use `| type` to see what you're working with
3. **Use `?` for optional access** - `.tags[]?` won't error if tags is null
4. **Use `//` for defaults** - `.priority // "none"` provides fallback
5. **Pretty print during debug** - Add `| .` at end to format output

## See Also

- `references/jq-cookbook.md` - Practical jq examples
- `references/batch-patterns.md` - Full batch operation scripts using this schema
