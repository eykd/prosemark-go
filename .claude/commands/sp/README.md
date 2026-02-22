# Spec-Kit Commands with Beads Integration

This directory contains the spec-kit workflow commands integrated with [beads](https://github.com/steveyegge/beads) for task tracking and workflow orchestration.

## Command Reference

| Command                      | Description                      | Beads Integration                                                       |
| ---------------------------- | -------------------------------- | ----------------------------------------------------------------------- |
| `/sp:00-constitution`        | Project constitution management  | None                                                                    |
| `/sp:01-specify`             | Create feature specification     | Creates epic + all phase tasks                                          |
| `/sp:02-clarify`             | Clarify requirements             | Closes phase task when done                                             |
| `/sp:03-plan`                | Create implementation plan       | Closes phase task when done                                             |
| `/sp:04-red-team`            | Adversarial review               | Enhances plan.md, closes phase task                                     |
| `/sp:05-tasks`               | Generate implementation tasks    | Creates US tasks under implement phase                                  |
| `/sp:06-analyze`             | Analyze artifacts                | Auto-fixes consistency issues, validates coverage, queries beads status |
| `/sp:07-implement`           | Execute implementation           | Closes self when all sub-tasks done                                     |
| `/sp:08-security-review`     | Security review (base..HEAD)     | Creates remediation tasks in beads                                      |
| `/sp:09-architecture-review` | Architecture review (base..HEAD) | Creates remediation tasks in beads                                      |
| `/sp:10-code-quality-review` | Code quality review (base..HEAD) | Creates remediation tasks in beads                                      |
| `/sp:next`                   | **Orchestrate workflow**         | Queries `bd ready`, invokes next skill                                  |

## Workflow: Beads Dependency Chain

The entire workflow is driven by beads task dependencies. `/sp:01-specify` creates ALL phase tasks upfront, and each phase closes itself when done to unblock the next.

```text
/sp:01-specify
     │
     ├── Creates Epic: "Feature: <name>"
     │
     └── Creates Phase Tasks with Dependencies:
           │
           ├── [sp:02-clarify] Clarify requirements
           │         │
           ├── [sp:03-plan] Create plan  ←── depends on 02
           │         │
           ├── [sp:04-red-team] Adversarial review  ←── depends on 03
           │         │
           ├── [sp:05-tasks] Generate tasks  ←── depends on 04
           │         │
           ├── [sp:06-analyze] Analyze artifacts  ←── depends on 05
           │         │
           ├── [sp:07-implement] Execute implementation  ←── depends on 06
           │         │
           │         ├── US1: User story 1 (sub-task)
           │         ├── US2: User story 2 (sub-task)
           │         └── ...
           │
           ├── [sp:08-security-review] Security review  ←── depends on 07
           │         │
           ├── [sp:09-architecture-review] Architecture review  ←── depends on 08
           │         │
           └── [sp:10-code-quality-review] Code quality review  ←── depends on 09

Progress: Run `/sp:next` to query `bd ready` and invoke the next phase
```

## Phase Task Naming Convention

Phase tasks use the `[sp:NN-name]` prefix for automatic skill invocation:

| Phase Task                        | Skill Invoked                |
| --------------------------------- | ---------------------------- |
| `[sp:02-clarify] ...`             | `/sp:02-clarify`             |
| `[sp:03-plan] ...`                | `/sp:03-plan`                |
| `[sp:04-red-team] ...`            | `/sp:04-red-team`            |
| `[sp:05-tasks] ...`               | `/sp:05-tasks`               |
| `[sp:06-analyze] ...`             | `/sp:06-analyze`             |
| `[sp:07-implement] ...`           | `/sp:07-implement`           |
| `[sp:08-security-review] ...`     | `/sp:08-security-review`     |
| `[sp:09-architecture-review] ...` | `/sp:09-architecture-review` |
| `[sp:10-code-quality-review] ...` | `/sp:10-code-quality-review` |

## Using /sp:next

The `/sp:next` command orchestrates the workflow automatically:

```bash
# Progress to the next ready phase
/sp:next

# Show workflow status without invoking
/sp:next --status

# Skip the current phase and move to next
/sp:next --skip

# Force a specific phase
/sp:next 03-plan
```

## Beads Integration

These commands use beads for task management:

1. **Task Storage**: Tasks stored in beads (`.beads/`) with git-backed persistence
2. **Dependencies**: Phase tasks created with `bd dep add` to form the workflow chain
3. **Progress**: Each phase closes itself via `bd close` to unblock the next
4. **Queries**: `/sp:next` uses `bd ready` to find the next available phase

## Prerequisites

- `@beads/bd` installed as devDependency: `npm install --save-dev @beads/bd`
- Git repository (required for beads)
- Beads initializes automatically on first use via `/sp:01-specify`

## Quick Start

```bash
# Create a new feature specification (creates epic + all phase tasks)
/sp:01-specify Add user authentication

# Progress through the workflow automatically
/sp:next              # Invokes /sp:02-clarify
/sp:next              # Invokes /sp:03-plan
/sp:next              # Invokes /sp:04-red-team
/sp:next              # Invokes /sp:05-tasks
/sp:next              # Invokes /sp:06-analyze
/sp:next              # Invokes /sp:07-implement
# ... repeat /sp:next for each implementation task ...
/sp:next              # Invokes /sp:08-security-review
/sp:next              # Invokes /sp:09-architecture-review
/sp:next              # Invokes /sp:10-code-quality-review

# Or invoke phases directly
/sp:02-clarify
/sp:03-plan
# etc.
```

## Checking Task Status

```bash
# View ready tasks (unblocked)
npx bd ready

# View all tasks for a feature epic
npx bd list --parent <epic-id> --json

# View dependency tree (shows phase chain)
npx bd dep tree <epic-id>

# Get statistics
npx bd stats

# Check workflow status
/sp:next --status
```

## Red Team Phase (sp:04-red-team)

The red team phase performs **adversarial review** of requirements and design BEFORE implementation:

**Purpose**: Strengthen the plan by thinking like an attacker/critic/tester

**Process**:

1. Reviews spec.md and plan.md from adversarial perspective
2. Identifies security gaps, edge cases, performance bottlenecks, accessibility barriers
3. **Enhances plan.md directly** with findings (no separate tasks to triage)
4. Adds sections: Security Considerations, Edge Cases & Error Handling, Performance Considerations, Accessibility Requirements

**Key Distinction**:

- **sp:04-red-team**: Reviews DESIGN (spec.md + plan.md) → Enhances plan.md
- **sp:08-security-review**: Reviews CODE (git diff) → Creates beads tasks

**When It Runs**: After sp:03-plan, before sp:05-tasks (so tasks are generated from enhanced plan)

**Skip If**: Feature is very simple and doesn't benefit from adversarial thinking (`/sp:next --skip`)

## Compound Learning (`/compound`)

The `/compound` command captures solutions to problems so future planning and review phases can reference them. Knowledge compounds over time — each feature cycle leaves the project smarter.

**When to use**: After solving a tricky bug, fixing review findings, or discovering a non-obvious pattern.

**What it does**:

1. Identifies the problem (from user input, recent commits, or closed tasks)
2. Analyzes the solution and root cause
3. Auto-categorizes into one of 8 categories (cloudflare-workers, test-coverage, clean-architecture, hugo-build, type-safety, security, performance, tooling)
4. Generates a solution document in `.specify/solutions/{category}/{slug}.md`
5. Updates `.specify/solutions/INDEX.md`

**How it feeds back**:

- `/sp:03-plan` searches prior learnings before planning (Phase 0.5)
- `/sp:04-red-team` checks for known vulnerability patterns before adversarial analysis
- `/sp:08`, `/sp:09`, `/sp:10` cross-reference findings with prior solutions
- Review completion reports suggest running `/compound` when remediation tasks were resolved

**Key design**: Standalone command, not a numbered phase. Can be invoked at any time — mid-implementation, after review, after incidents.

## Deepen Plan (`/deepen-plan`)

The `/deepen-plan` command enhances the current feature plan with focused research on uncertain sections.

**When to use**: After `/sp:03-plan` for complex features, before `/sp:04-red-team`.

**What it does**:

1. Loads plan.md and identifies sections with "NEEDS CLARIFICATION", "TBD", or vague language
2. Searches `.specify/solutions/` for prior learnings relevant to each uncertain section
3. Researches unknowns and expands sections with concrete implementation details
4. Updates plan.md in place

**Key design**: Idempotent — can be run multiple times. Does NOT create beads tasks or close any phase.

## Code Review (Not Part of sp Workflow)

The `/code-review` skill is available for iterative code review during development, but is **not part of the sp workflow**:

- **`/code-review`** - Reviews source files directly (auto-discovers or specify files)
  - Use during development for iterative feedback
  - Creates beads tasks for findings under the current epic
  - Runs in background with Haiku model

The sp workflow includes **final review phases** that review git diffs (base..HEAD):

- **`/sp:08-security-review`** - Security review after implementation
- **`/sp:09-architecture-review`** - Architecture review after security
- **`/sp:10-code-quality-review`** - Quality review after architecture

**Key difference**: `/code-review` reviews files directly for iterative feedback, while sp:08/09/10 review all changes in the branch as final validation before merge.
