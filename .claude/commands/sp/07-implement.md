---
description: Execute the implementation plan by processing tasks from beads. Uses bd ready to get available tasks and bd close to mark completion.
---

## User Input

```text
$ARGUMENTS
```

You **MUST** consider the user input before proceeding (if not empty).

## Outline

1. Run `.specify/scripts/bash/check-prerequisites.sh --json` from repo root and parse FEATURE_DIR and AVAILABLE_DOCS list. All paths must be absolute. For single quotes in args like "I'm Groot", use escape syntax: e.g 'I'\''m Groot' (or double-quote if possible: "I'm Groot").

2. Load and analyze the implementation context:
   - **REQUIRED**: Read plan.md for tech stack, architecture, and file structure
   - **REQUIRED**: Read spec.md for requirements and acceptance criteria
   - **IF EXISTS**: Read data-model.md for entities and relationships
   - **IF EXISTS**: Read contracts/ for API specifications and test requirements
   - **IF EXISTS**: Read research.md for technical decisions and constraints
   - **IF EXISTS**: Read quickstart.md for integration scenarios

3. **Retrieve Beads Epic ID**:

   a. Read the epic ID from spec.md front matter:

   ```bash
   grep "Beads Epic" FEATURE_DIR/spec.md | grep -oE 'workspace-[a-z0-9]+|bd-[a-z0-9]+'
   ```

   b. If not found, search beads for the epic:

   ```bash
   npx bd list --type epic --status open --json
   ```

   c. Store epic ID for task queries

4. **Get Ready Tasks from Beads**:

   a. Query beads for tasks ready to work on:

   ```bash
   npx bd ready --json
   ```

   - This returns tasks with no blocking dependencies
   - Parse JSON to get task IDs, titles, and priorities

   b. If no ready tasks, check remaining open tasks:

   ```bash
   npx bd list --parent <epic-id> --status open --json
   ```

   - If all tasks complete, report completion
   - If tasks exist but none ready, there may be blocking dependencies

   c. Display ready tasks to user:

   ```text
   Ready Tasks:
   1. [workspace-abc123] US1: Initialize Beads (priority: 1)
   2. [workspace-abc124] Setup project structure (priority: 1)
   ```

5. **Project Setup Verification**:
   - **REQUIRED**: Create/verify ignore files based on actual project setup
   - Check for .gitignore, .dockerignore, .eslintignore, .prettierignore as needed
   - Apply technology-specific patterns from plan.md

6. **Execute Implementation** following the task plan:

   For each ready task:

   a. **Mark task in progress**:

   ```bash
   npx bd update <task-id> --status in_progress --json
   ```

   b. **Execute the task implementation**:

   **For code changes** - Apply strict red-green-refactor TDD:
   - **RED**: Write a failing test FIRST
   - **GREEN**: Write MINIMAL code to make the test pass
   - **REFACTOR**: Improve code while maintaining green tests
   - Use the project's test runner during development (see CLAUDE.md)
   - Run the project's quality gates before marking task complete (see CLAUDE.md)

   - Use skills referenced in task description for guidance
   - Validate the implementation works

   c. **Commit changes before closing task**:
   - Stage all changes: `git add -A`
   - Create conventional commit with descriptive message
   - Pre-commit hooks MUST run and MUST pass
   - If hooks fail, fix issues and retry the commit
   - Never use `--no-verify` to skip hooks

   d. **Mark task complete**:

   ```bash
   npx bd close <task-id> --reason "<brief-completion-summary>" --json
   ```

   - Provide a brief summary of what was done

   e. **Check for newly ready tasks**:

   ```bash
   npx bd ready --json
   ```

   - Completing a task may unblock dependent tasks
   - Display any newly available tasks

7. **Implementation execution rules**:
   - **Setup first**: Initialize project structure, dependencies, configuration
   - **Tests before code**: If tests are included, write them first
   - **Core development**: Implement models, services, CLI commands, endpoints
   - **Integration work**: Database connections, middleware, logging, external services
   - **Polish and validation**: Unit tests, performance optimization, documentation

8. **Progress tracking and error handling**:
   - Report progress after each completed task
   - Show beads task status: `npx bd stats --json`
   - Halt execution if any critical task fails
   - For parallel tasks, continue with successful tasks, report failed ones
   - Provide clear error messages with context for debugging
   - Suggest next steps if implementation cannot proceed

9. **Completion validation and Self-Close**:

   a. Find the implement phase task ID:

   ```bash
   IMPLEMENT_TASK_ID=$(npx bd list --parent <epic-id> --json | jq -r '.[] | select(.title | contains("[sp:07-implement]")) | .id')
   ```

   b. Check remaining sub-tasks under the implement phase task:

   ```bash
   npx bd list --parent $IMPLEMENT_TASK_ID --status open --json
   ```

   c. If open sub-tasks remain:
   - Report remaining work count and next ready task
   - Suggest next ready task via `bd ready`
   - Do NOT close the implement phase task

   d. If ALL sub-tasks under implement are closed:

   ```bash
   # Verify all sub-tasks complete
   OPEN_COUNT=$(npx bd list --parent $IMPLEMENT_TASK_ID --status open --json | jq 'length')
   if [ "$OPEN_COUNT" -eq 0 ]; then
     # Verify implemented features match the original specification
     # Validate that tests pass and coverage meets requirements
     # Confirm the implementation follows the technical plan

     # Close the implement phase task
     npx bd close $IMPLEMENT_TASK_ID --reason "All implementation tasks complete"
   fi
   ```

   e. After closing implement task, the [sp:08-security-review] task becomes ready:

   ```bash
   npx bd ready --json | jq '.[] | select(.title | contains("[sp:08-security-review]"))'
   ```

   f. Report: "Implementation complete. Run `/sp:next` (security → architecture → quality review) for code review."

   g. Display final beads summary:

   ```bash
   npx bd stats --json
   npx bd dep tree <epic-id>
   ```

Note: This command uses beads exclusively for task tracking. Run `/sp:05-tasks` if beads tasks do not exist.

## Beads Task Lifecycle

```text
┌──────────┐     bd update      ┌─────────────┐     bd close     ┌────────┐
│   open   │ ────────────────→  │ in_progress │ ──────────────→  │ closed │
└──────────┘   --status         └─────────────┘    --reason       └────────┘
                in_progress
```

## Beads Commands Reference

| Action               | Command                                               |
| -------------------- | ----------------------------------------------------- |
| Get ready tasks      | `npx bd ready --json`                                 |
| Mark in progress     | `npx bd update <id> --status in_progress`             |
| Mark complete        | `npx bd close <id> --reason "summary"`                |
| View task            | `npx bd show <id>`                                    |
| List open tasks      | `npx bd list --parent <epic-id> --status open --json` |
| View statistics      | `npx bd stats --json`                                 |
| View dependency tree | `npx bd dep tree <epic-id>`                           |

## Error Handling

If beads commands fail:

1. **bd: command not found**: Suggest `npm install --save-dev @beads/bd`
2. **No ready tasks but open tasks exist**: Check dependencies with `bd dep tree`
3. **Task update fails**: Log error, continue with next task, report at end
4. **Epic not found**: Run `/sp:05-tasks` to create beads tasks
