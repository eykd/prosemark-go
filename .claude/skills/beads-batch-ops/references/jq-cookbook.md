# jq Cookbook for Beads

Practical jq patterns for processing beads JSONL output in batch operations.

## Prerequisites

All beads commands support `--format json` for machine-readable output:

```bash
npx bd list --format json
npx bd ready --format json
npx bd show <id> --format json
```

## Basic Extraction Patterns

### Get All Task IDs

```bash
npx bd list --format json | jq -r '.[].id'
```

### Get Task Titles and IDs

```bash
npx bd list --format json | jq -r '.[] | "\(.id): \(.title)"'
```

### Extract Specific Fields

```bash
# Just titles
npx bd list --format json | jq -r '.[].title'

# Title, status, and priority
npx bd list --format json | jq -r '.[] | "\(.title) | \(.status) | \(.priority // "none")"'
```

## Filtering Patterns

### Filter by Status

```bash
# All open tasks
npx bd list --format json | jq '.[] | select(.status == "open")'

# All closed tasks
npx bd list --format json | jq '.[] | select(.status == "closed")'

# Not ready tasks
npx bd list --format json | jq '.[] | select(.status != "ready")'
```

### Filter by Multiple Conditions

```bash
# High priority tasks that are ready
npx bd list --format json | jq '.[] | select(.priority == "high" and .status == "ready")'

# Open tasks in specific phase
npx bd list --format json | jq '.[] | select(.status == "open" and .phase == "implementation")'

# Tasks with no dependencies
npx bd list --format json | jq '.[] | select(.blockedBy == null or (.blockedBy | length == 0))'
```

### Filter by Tag Presence

```bash
# Tasks with "bug" tag
npx bd list --format json | jq '.[] | select(.tags[]? == "bug")'

# Tasks with any tags
npx bd list --format json | jq '.[] | select(.tags != null and (.tags | length > 0))'

# Tasks without tags
npx bd list --format json | jq '.[] | select(.tags == null or (.tags | length == 0))'
```

## Aggregation Patterns

### Count by Status

```bash
npx bd list --format json | jq -r '
  group_by(.status) |
  map({status: .[0].status, count: length}) |
  .[] |
  "\(.status): \(.count)"
'
```

### Count by Priority

```bash
npx bd list --format json | jq -r '
  group_by(.priority // "none") |
  map({priority: .[0].priority // "none", count: length}) |
  .[] |
  "\(.priority): \(.count)"
'
```

### Count by Phase

```bash
npx bd list --format json | jq -r '
  group_by(.phase // "unspecified") |
  map({phase: .[0].phase // "unspecified", count: length}) |
  .[] |
  "\(.phase): \(.count)"
'
```

## Sorting Patterns

### Sort by Created Date (Newest First)

```bash
npx bd list --format json | jq 'sort_by(.createdAt) | reverse | .[]'
```

### Sort by Priority (High → Low)

```bash
npx bd list --format json | jq '
  map(
    . + {priorityOrder: (
      if .priority == "high" then 1
      elif .priority == "medium" then 2
      elif .priority == "low" then 3
      else 4
      end
    )}
  ) |
  sort_by(.priorityOrder) |
  .[]
'
```

### Sort by Title (Alphabetical)

```bash
npx bd list --format json | jq 'sort_by(.title) | .[]'
```

## Transformation Patterns

### Convert to Markdown Checklist

```bash
npx bd ready --format json | jq -r '.[] | "- [ ] \(.title) (`\(.id)`)"'
```

### Generate CSV Output

```bash
# Header
echo "ID,Title,Status,Priority"

# Data rows
npx bd list --format json | jq -r '
  .[] |
  [.id, .title, .status, (.priority // "none")] |
  @csv
'
```

### Create JSON Summary Object

```bash
npx bd list --format json | jq '{
  total: length,
  by_status: (group_by(.status) | map({(.[0].status): length}) | add),
  ready_count: ([.[] | select(.status == "ready")] | length),
  in_progress_count: ([.[] | select(.status == "in_progress")] | length)
}'
```

## Complex Queries

### Find Orphaned Tasks (No Parent, Not an Epic)

```bash
npx bd list --format json | jq '.[] | select(.parent == null and (.children == null or (.children | length == 0)))'
```

### Find Epics with Their Child Count

```bash
npx bd list --format json | jq -r '
  .[] |
  select(.children != null and (.children | length > 0)) |
  "\(.title): \(.children | length) children"
'
```

### Find Blocked Tasks with Blocking Task Details

This requires two queries and merging:

```bash
# Get all tasks
all_tasks=$(npx bd list --format json)

# Find blocked tasks and resolve blocker info
echo "$all_tasks" | jq --argjson all "$all_tasks" '
  .[] |
  select(.blockedBy != null and (.blockedBy | length > 0)) |
  {
    id: .id,
    title: .title,
    blockedBy: [
      .blockedBy[] as $blocker_id |
      $all |
      .[] |
      select(.id == $blocker_id) |
      {id: .id, title: .title, status: .status}
    ]
  }
'
```

## Conditional Logic in jq

### Use Alternative Values

```bash
# Use "none" if priority is null
npx bd list --format json | jq -r '.[] | .priority // "none"'

# Use empty array if tags is null
npx bd list --format json | jq '.[] | .tags // []'
```

### Conditional Field Selection

```bash
# Show different output based on status
npx bd list --format json | jq -r '
  .[] |
  if .status == "closed" then
    "✅ \(.title)"
  elif .status == "in_progress" then
    "🚧 \(.title)"
  else
    "⏸️  \(.title)"
  end
'
```

## Integration with Bash

### Store IDs in Array

```bash
# Store ready task IDs in Bash array
mapfile -t ready_ids < <(npx bd ready --format json | jq -r '.[].id')

# Iterate over array
for id in "${ready_ids[@]}"; do
  echo "Processing: $id"
  npx bd start "$id"
done
```

### Conditional Execution Based on Query

```bash
# Check if any tasks are in progress
in_progress=$(npx bd list --format json | jq '[.[] | select(.status == "in_progress")] | length')

if [ "$in_progress" -gt 0 ]; then
  echo "Tasks in progress: $in_progress"
else
  echo "No tasks in progress"
fi
```

### Capture Single Value

```bash
# Get the first ready task ID
next_task=$(npx bd ready --format json | jq -r '.[0].id // empty')

if [ -n "$next_task" ]; then
  echo "Starting task: $next_task"
  npx bd start "$next_task"
else
  echo "No ready tasks"
fi
```

## Debugging jq Queries

### Pretty Print JSON

```bash
npx bd list --format json | jq '.'
```

### View Array Length

```bash
npx bd list --format json | jq 'length'
```

### Inspect First Item

```bash
npx bd list --format json | jq '.[0]'
```

### Test Filter Without Processing

```bash
# See what would be selected
npx bd list --format json | jq '[.[] | select(.status == "ready")] | length'
```

## Common Pitfalls

### Issue: "Cannot index null"

**Problem**: Trying to access a field that doesn't exist on some objects.

**Solution**: Use the optional operator `?` or null coalescing `//`:

```bash
# Bad
jq '.[] | .priority'

# Good
jq '.[] | .priority // "none"'
jq '.[] | .priority?'
```

### Issue: Empty Output

**Problem**: Filter is too restrictive or field names are wrong.

**Solution**: Test incrementally:

```bash
# Step 1: See all tasks
npx bd list --format json | jq '.'

# Step 2: See first task structure
npx bd list --format json | jq '.[0]'

# Step 3: Test filter
npx bd list --format json | jq '.[] | select(.status == "ready")'
```

### Issue: Unexpected Type Errors

**Problem**: Trying to use string operators on arrays or vice versa.

**Solution**: Check types:

```bash
# See type of a field
npx bd list --format json | jq '.[0].tags | type'

# Handle arrays vs strings
npx bd list --format json | jq '.[] | .tags[]?'  # Iterate array safely
```

## Performance Tips

1. **Filter early** - Use `select()` before `sort_by()` or other operations
2. **Limit output** - Use `.[0:10]` to take first 10 items if you don't need all
3. **Avoid repeated queries** - Store JSON in a variable if you'll process it multiple times

```bash
# Good: Query once
tasks=$(npx bd list --format json)
echo "$tasks" | jq 'filter expression 1'
echo "$tasks" | jq 'filter expression 2'

# Bad: Query multiple times
npx bd list --format json | jq 'filter expression 1'
npx bd list --format json | jq 'filter expression 2'
```

## Reference

- [jq Manual](https://jqlang.github.io/jq/manual/)
- [jq Playground](https://jqplay.org/) - Test queries online
- Beads JSON schema: See `references/beads-schema.md`
