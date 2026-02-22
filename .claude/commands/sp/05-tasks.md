---
description: Generate beads tasks for the feature based on available design artifacts. Creates tasks in beads hierarchy instead of markdown checkboxes.
handoffs:
  - label: Analyze For Consistency
    agent: sp:06-analyze
    prompt: Run a project analysis for consistency
    send: true
  - label: Implement Project
    agent: sp:07-implement
    prompt: Start the implementation in phases
    send: true
---

## User Input

```text
$ARGUMENTS
```

You **MUST** consider the user input before proceeding (if not empty).

## Outline

1. **Setup**: Run `.specify/scripts/bash/check-prerequisites.sh --json` from repo root and parse FEATURE_DIR and AVAILABLE_DOCS list. All paths must be absolute. For single quotes in args like "I'm Groot", use escape syntax: e.g 'I'\''m Groot' (or double-quote if possible: "I'm Groot").

2. **Load design documents**: Read from FEATURE_DIR:
   - **Required**: plan.md (tech stack, libraries, structure), spec.md (user stories with priorities)
   - **Optional**: data-model.md (entities), contracts/ (API endpoints), research.md (decisions), quickstart.md (test scenarios)
   - Note: Not all projects have all documents. Generate tasks based on what's available.

3. **Retrieve Beads Epic ID**:

   a. Read the epic ID from spec.md front matter:

   ```bash
   grep "Beads Epic" FEATURE_DIR/spec.md | grep -oE 'workspace-[a-z0-9]+|bd-[a-z0-9]+'
   ```

   b. If not found in spec.md, search beads for epic by feature name:

   ```bash
   npx bd list --type epic --status open --json
   ```

   - Parse JSON to find epic matching the feature branch name
   - Extract the epic ID

   c. If no epic exists, create one:

   ```bash
   npx bd create "Feature: <feature-name>" -t epic -p 0 --json
   ```

   - Store the returned ID for use in task creation

   d. Store epic ID for subsequent task creation steps

4. **Execute task generation workflow**:
   - Load plan.md and extract tech stack, libraries, project structure
   - Load spec.md and extract user stories with their priorities (P1, P2, P3, etc.)
   - If data-model.md exists: Extract entities and map to user stories
   - If contracts/ exists: Map endpoints to user stories
   - If research.md exists: Extract decisions for setup tasks
   - Generate tasks organized by user story (see Task Generation Rules below)
   - Generate dependency graph showing user story completion order
   - Create parallel execution examples per user story
   - Validate task completeness (each user story has all needed tasks, independently testable)

5. **Generate GWT Acceptance Specs**:

   For each user story extracted from spec.md:

   a. Create `specs/` directory if it doesn't exist:

   ```bash
   mkdir -p specs
   ```

   b. Write `specs/US<N>-<kebab-title>.txt` with:
      - `;===` header with the user story description
      - One scenario per acceptance criterion from spec.md
      - GIVEN/WHEN/THEN steps using domain language only

   c. Validate specs contain no implementation language (quick self-check against GWT Acceptance Spec Rules below)

   d. Commit spec files:

   ```bash
   git add specs/ && git commit -m "feat: add GWT acceptance specs for US<N>"
   ```

   See **GWT Acceptance Spec Rules** below for format and language requirements.

6. **Create Beads Tasks** (as children of the [sp:07-implement] phase task):

   > **Note**: Every user story MUST have a corresponding `specs/US<N>-<kebab-title>.txt` file created in step 5.

   **First, find the implement phase task** (created by `/sp:01-specify`):

   ```bash
   IMPLEMENT_TASK_ID=$(npx bd list --parent <epic-id> --status open --json | jq -r '.[] | select(.title | contains("[sp:07-implement]")) | .id')
   ```

   Store this ID - all user story tasks will be created as children of this task.

   **Skill Mapping Reference** - Use these skills based on task type:

   | Task Pattern                                | Skills                                          |
   | ------------------------------------------- | ----------------------------------------------- |
   | `Create.*entity`, `.*domain model`          | `/ddd-domain-modeling`                           |
   | `Implement.*repository`, `.*data access`    | `/ddd-domain-modeling`                           |
   | `Write.*test`, `.*spec.*`                   | Project test skill (see CLAUDE.md)               |
   | `Setup.*`, `Configure.*`                    | `/prefactoring`                                  |
   | `.*security.*`, `.*auth.*`                  | `/security-review`                               |

   > **Note**: Match skills to the project's technology stack. Check available skills with `/help` and reference CLAUDE.md for project-specific testing and implementation guidance.

   For each user story from spec.md:

   a. Create a task for the user story **as a child of the implement task** with description:

   ```bash
   npx bd create "US<N>: <user-story-title>" -p <priority> --parent $IMPLEMENT_TASK_ID \
     --description "**Spec**: specs/$BRANCH/spec.md §US-<N>
   **Goal**: <user-story-goal-from-spec>
   **Acceptance**: <acceptance-criteria-summary>" --json
   ```

   - `<N>`: User story number (1, 2, 3...)
   - `<priority>`: Map P1→1, P2→2, P3→3, etc.
   - `--parent $IMPLEMENT_TASK_ID`: Link to the **implement phase task** (NOT the epic)

   b. For each implementation step within the user story, create a sub-task with description:

   ```bash
   npx bd create "<step-description>" -p <priority> --parent <user-story-task-id> \
     --description "**Spec**: specs/$BRANCH/spec.md §US-<N>, plan.md §<section>
   **Skills**: <skill-list-from-mapping>
   **Files**: <target-file-paths>
   **Acceptance**: <specific-criteria>" --json
   ```

   - `<user-story-task-id>`: The user story task ID from step (a)
   - `<skill-list-from-mapping>`: Skills from the mapping table above
   - `<target-file-paths>`: Specific files to create/modify
   - `<specific-criteria>`: Measurable acceptance criteria

   c. Establish dependencies between sequential tasks:

   ```bash
   npx bd dep add <dependent-task-id> <blocking-task-id>
   ```

   - Add dependencies where one task must complete before another
   - Tasks marked [P] should NOT have dependencies between them (parallel execution)

   d. For parallel tasks (marked [P] in task plan):
   - Create without dependencies between them
   - They will all appear in `bd ready` once their common parent is ready

7. **Verify Task Hierarchy**:

   ```bash
   npx bd dep tree <epic-id>
   ```

   - Verify the hierarchy: Epic → User Story Tasks → Implementation Sub-tasks
   - Check for any circular dependencies: `npx bd dep cycles`

8. **Close Phase Task in Beads**:

   After creating all implementation tasks, close the 05-tasks phase task to unblock the implement phase.

   a. Find the tasks phase task:

   ```bash
   npx bd list --parent <epic-id> --status open --json | jq -r '.[] | select(.title | contains("[sp:05-tasks]")) | .id'
   ```

   b. Close the task with a completion summary:

   ```bash
   npx bd close <tasks-task-id> --reason "Created <N> tasks across <M> user stories under [sp:07-implement]"
   ```

   c. The [sp:06-analyze] phase task is now ready (its dependency on 05-tasks is satisfied).

   d. Report: "Phase [sp:05-tasks] complete. Run `/sp:next` or `/sp:06-analyze` to validate artifacts."

9. **Report**: Output summary including:
   - **Beads epic ID** and total tasks created in beads
   - **Implement task ID** (`$IMPLEMENT_TASK_ID`) containing all user story tasks
   - Task count per user story (with task IDs)
   - Parallel opportunities identified
   - Independent test criteria for each story
   - Suggested MVP scope (typically just User Story 1)
   - **Next step**: Run `/sp:next` or `/sp:06-analyze` to validate cross-artifact consistency
   - **How to view ready tasks**: `npx bd ready --json`

Context for task generation: $ARGUMENTS

The tasks should be immediately executable via beads - each task must be specific enough that an LLM can complete it without additional context.

## Task Generation Rules

**CRITICAL**: Tasks MUST be organized by user story to enable independent implementation and testing.

**Acceptance Specs are MANDATORY**: Every user story MUST have a corresponding `specs/US<N>-<kebab-title>.txt` file generated in step 5.

### Beads Task Format

When creating tasks in beads, use this naming convention:

```text
User Story Task: "US<N>: <title>"
Sub-task: "<action> <target> in <file-path>"
```

**Examples**:

- User Story: `US1: Initialize Beads in Repository`
- Sub-task: `Create User model in src/models/user.py`
- Sub-task: `Implement authentication middleware in src/middleware/auth.py`

### Priority Mapping

| Spec Priority | Beads Priority | Description             |
| ------------- | -------------- | ----------------------- |
| Epic          | 0              | Feature-level (highest) |
| P1            | 1              | Critical user story     |
| P2            | 2              | High priority           |
| P3            | 3              | Medium priority         |
| P4+           | 4+             | Lower priority          |

### Dependency Rules

1. **Sequential tasks**: Use `bd dep add <child> <parent>` to create blocking relationships
2. **Parallel tasks**: Do NOT add dependencies between them - they'll appear together in `bd ready`
3. **Cross-story dependencies**: Minimize these; each story should be independently completable
4. **Setup/Foundational**: These block all user story tasks

### Task Organization

1. **From User Stories (spec.md)** - PRIMARY ORGANIZATION:
   - Each user story (P1, P2, P3...) becomes a beads task under the epic
   - Implementation steps become sub-tasks under each user story task
   - Map all related components to their story:
     - Models needed for that story
     - Services needed for that story
     - Endpoints/UI needed for that story
     - If tests requested: Tests specific to that story
   - Mark story dependencies (most stories should be independent)

2. **From Contracts**:
   - Map each contract/endpoint → to the user story it serves
   - If tests requested: Each contract → contract test sub-task before implementation

3. **From Data Model**:
   - Map each entity to the user story(ies) that need it
   - If entity serves multiple stories: Put in earliest story or Setup phase
   - Relationships → service layer tasks in appropriate story phase

4. **From Setup/Infrastructure**:
   - Shared infrastructure → Setup phase tasks
   - Foundational/blocking tasks → Foundational phase tasks
   - Story-specific setup → within that story's sub-tasks

### Phase Structure

- **Phase 1**: Setup (project initialization) - Tasks directly under epic
- **Phase 2**: Foundational (blocking prerequisites) - Tasks directly under epic
- **Phase 3+**: User Stories in priority order (P1, P2, P3...)
  - Each user story is a task under the epic
  - Implementation steps are sub-tasks under the user story task
  - Each phase should be a complete, independently testable increment
- **Final Phase**: Polish & Cross-Cutting Concerns - Tasks directly under epic

## Beads Error Handling

If beads commands fail during task creation:

1. **Epic not found**: Create a new epic for the feature
2. **Task creation fails**: Log error, continue with remaining tasks, report failures at end
3. **Dependency cycle detected**: Remove the problematic dependency, log warning
4. **bd command not found**: Suggest `npm install --save-dev @beads/bd`

If beads commands fail completely, report failures and suggest troubleshooting steps.

## GWT Acceptance Spec Rules

**MANDATORY**: Every user story MUST have a corresponding `specs/US<N>-<kebab-title>.txt` file.

**Format**:
- Use `;===` separator lines around scenario descriptions
- Use GIVEN, WHEN, THEN keywords (uppercase) at start of lines
- End each statement with a period
- Leave blank lines between steps for readability

**Example**:

```text
;===============================================================
; User can add a new outline item.
;===============================================================
GIVEN an empty outline.

WHEN the user adds an item titled "Chapter 1".

THEN the outline contains 1 item.
THEN the item is titled "Chapter 1".
```

**Language rules (STRICT)**:
- ONLY domain language (user actions, business rules, observable outcomes)
- NO: class names, function names, file paths, package names
- NO: HTTP methods, API endpoints, database tables, SQL
- NO: framework terms (Cobra, handler, middleware, router)
- NO: data structures (array, map, slice, struct)
- A non-developer should understand every statement
