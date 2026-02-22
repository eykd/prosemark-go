---
description: Query beads for ready phase tasks and invoke the appropriate skill based on [sp:XX-name] prefix. Use to progress through the spec workflow.
---

## User Input

```text
$ARGUMENTS
```

You **MUST** consider the user input before proceeding (if not empty).

## Outline

The `/sp:next` command queries beads for ready tasks, identifies phase tasks by their `[sp:XX-name]` prefix, and invokes the appropriate skill.

### 1. Parse Arguments

Check for special flags in `$ARGUMENTS`:

- `--skip`: Skip the current phase (close it without running the skill)
- `--status`: Show workflow state without invoking any skill
- `<phase-name>`: Force a specific phase (e.g., `03-plan`, `06-implement`)

### 2. Find Active Epic

```bash
npx bd list --type epic --status open --json
```

If multiple epics exist, prefer:

1. Epic matching current git branch name
2. Most recently created epic
3. Ask user to specify

If no epic found:

- ERROR: "No active epic found. Run `/sp:01-specify <feature>` to create one."

Store the epic ID for subsequent queries.

### 3. Get Ready Phase Tasks

```bash
npx bd ready --json
```

Parse the JSON output to find tasks matching the phase task pattern:

- Regex: `^\[sp:(\d{2})-([a-z-]+)\]`
- Extract: phase number (NN) and phase name (e.g., "clarify", "plan")

Filter results to only include tasks that:

1. Match the `[sp:XX-*]` pattern
2. Are children of the active epic

### 4. Handle No Ready Phase Tasks

If no phase tasks are ready:

a. Check for implementation sub-tasks (tasks under `[sp:07-implement]`):

```bash
IMPLEMENT_TASK=$(npx bd list --parent <epic-id> --json | jq -r '.[] | select(.title | contains("[sp:07-implement]"))')
if [ -n "$IMPLEMENT_TASK" ]; then
  IMPLEMENT_ID=$(echo $IMPLEMENT_TASK | jq -r '.id')
  IMPL_STATUS=$(echo $IMPLEMENT_TASK | jq -r '.status')
  if [ "$IMPL_STATUS" = "in_progress" ] || [ "$IMPL_STATUS" = "open" ]; then
    # Check for sub-tasks
    npx bd list --parent $IMPLEMENT_ID --status open --json
  fi
fi
```

b. If implementation sub-tasks are ready:

- Report: "Implementation in progress. Ready sub-tasks: [list]"
- Suggest: Run `/sp:07-implement` to continue implementation

c. If no tasks at all are ready:

- Check epic status with `npx bd list --parent <epic-id> --json`
- If all tasks closed: "Feature workflow complete! Epic ready to close."
- If tasks exist but blocked: "No ready tasks. Check dependencies with `npx bd dep tree <epic-id>`"

### 5. Select Next Phase (--status flag)

If `--status` flag is present:

- Display current workflow state:

```markdown
## Workflow Status: [feature-name]

**Epic**: [epic-id]

| Phase                       | Task ID | Status                    |
| --------------------------- | ------- | ------------------------- |
| [sp:02-clarify]             | [id]    | [open/in_progress/closed] |
| [sp:03-plan]                | [id]    | [open/in_progress/closed] |
| [sp:04-checklist]           | [id]    | [open/in_progress/closed] |
| [sp:05-tasks]               | [id]    | [open/in_progress/closed] |
| [sp:06-analyze]             | [id]    | [open/in_progress/closed] |
| [sp:07-implement]           | [id]    | [open/in_progress/closed] |
| [sp:08-security-review]     | [id]    | [open/in_progress/closed] |
| [sp:09-architecture-review] | [id]    | [open/in_progress/closed] |
| [sp:10-code-quality-review] | [id]    | [open/in_progress/closed] |

**Next Ready Phase**: [phase-name] or "None (workflow complete)"
```

- Exit without invoking any skill

### 6. Handle Skip (--skip flag)

If `--skip` flag is present:

a. Get the next ready phase task (lowest phase number)
b. Close it with skip reason:

```bash
npx bd close <phase-task-id> --reason "Skipped by user via /sp:next --skip"
```

c. Report: "Phase [sp:XX-name] skipped."
d. Check for newly ready phase:

```bash
npx bd ready --json | jq '.[] | select(.title | contains("[sp:"))'
```

e. Report: "Next phase [sp:YY-name] is now ready. Run `/sp:next` to continue."

### 7. Handle Force Phase (argument)

If a phase name is provided (e.g., `/sp:next 03-plan`):

a. Find the specified phase task:

```bash
npx bd list --parent <epic-id> --json | jq -r '.[] | select(.title | contains("[sp:<phase-name>]"))'
```

b. Check if task exists:

- If not found: ERROR "Phase task [sp:<phase-name>] not found in epic"

c. Check if task is blocked (has unfinished dependencies):

```bash
# Task status check
TASK_STATUS=$(npx bd show <task-id> --json | jq -r '.status')
```

d. If blocked:

- WARN: "Phase [sp:<phase-name>] is blocked by incomplete dependencies."
- Ask: "Force execution anyway? (yes/no)"
- If yes, proceed; if no, abort

e. Proceed to step 8 with the specified phase task

### 8. Select and Invoke Next Phase

If multiple phase tasks are ready (rare):

- Sort by phase number (02 < 03 < 04 < 05 < 06 < 09)
- Select the lowest numbered phase

Mark the selected phase as in-progress:

```bash
npx bd update <phase-task-id> --status in_progress
```

Display:

```markdown
## /sp:next

**Epic**: [epic-id]
**Next Phase**: [sp:XX-name]
**Task ID**: [phase-task-id]

Invoking `/sp:XX-name`...
```

Invoke the corresponding skill:

- `[sp:02-clarify]` → `/sp:02-clarify`
- `[sp:03-plan]` → `/sp:03-plan`
- `[sp:04-red-team]` → `/sp:04-red-team`
- `[sp:05-tasks]` → `/sp:05-tasks`
- `[sp:06-analyze]` → `/sp:06-analyze`
- `[sp:07-implement]` → `/sp:07-implement`
- `[sp:08-security-review] / [sp:09-architecture-review] / [sp:10-code-quality-review]` → `/sp:08-security-review` (or 09/10 depending on prefix)

## Skill Mapping

| Pattern                       | Skill to Invoke              |
| ----------------------------- | ---------------------------- |
| `[sp:02-clarify]`             | `/sp:02-clarify`             |
| `[sp:03-plan]`                | `/sp:03-plan`                |
| `[sp:04-red-team]`            | `/sp:04-red-team`            |
| `[sp:05-tasks]`               | `/sp:05-tasks`               |
| `[sp:06-analyze]`             | `/sp:06-analyze`             |
| `[sp:07-implement]`           | `/sp:07-implement`           |
| `[sp:08-security-review]`     | `/sp:08-security-review`     |
| `[sp:09-architecture-review]` | `/sp:09-architecture-review` |
| `[sp:10-code-quality-review]` | `/sp:10-code-quality-review` |

## Error Handling

- **No epic found**: Suggest running `/sp:01-specify` first
- **No phase tasks created**: Feature may use old workflow; suggest re-running `/sp:01-specify`
- **Beads not initialized**: Suggest `npm install --save-dev @beads/bd && npx bd init`
- **Circular dependency**: Run `npx bd dep cycles` and report
- **Phase task not found**: List available phases with `npx bd list --parent <epic-id>`

## Beads Commands Reference

| Action               | Command                                        |
| -------------------- | ---------------------------------------------- |
| List epics           | `npx bd list --type epic --status open --json` |
| Get ready tasks      | `npx bd ready --json`                          |
| Mark in progress     | `npx bd update <id> --status in_progress`      |
| Close (skip)         | `npx bd close <id> --reason "..."`             |
| View all phases      | `npx bd list --parent <epic-id> --json`        |
| View dependency tree | `npx bd dep tree <epic-id>`                    |

## Example Usage

```bash
# Progress to next ready phase
/sp:next

# Show workflow status without invoking
/sp:next --status

# Skip current phase and move to next
/sp:next --skip

# Force a specific phase
/sp:next 03-plan
```

## Example Output

```markdown
=== /sp:next ===

**Epic**: workspace-054 (user-auth)

Ready Phase Tasks:

1. [sp:03-plan] Create implementation plan for user-auth (workspace-054.2)

Selecting: [sp:03-plan] (lowest phase number)
Marking in progress...

Invoking /sp:03-plan...
```
