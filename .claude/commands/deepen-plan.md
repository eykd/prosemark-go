---
description: Enhance the current feature plan with focused research on uncertain sections. Use after /sp:03-plan for complex features, before /sp:04-red-team.
---

## User Input

```text
$ARGUMENTS
```

You **MUST** consider the user input before proceeding (if not empty).

## Purpose

Deepen the implementation plan by researching uncertain areas and incorporating prior learnings. This command is idempotent — it can be run multiple times safely. It does NOT create beads tasks or close any phase.

## When to Use

- After `/sp:03-plan` for complex features that have "NEEDS CLARIFICATION" sections
- Before `/sp:04-red-team` to strengthen the plan with concrete details
- When the plan has high-uncertainty areas that need research

## Steps

### 1. Load Plan

Run `.specify/scripts/bash/check-prerequisites.sh --json` from repo root and parse `FEATURE_DIR`.

Read `FEATURE_DIR/plan.md`. If it does not exist, ERROR: "No plan.md found. Run `/sp:03-plan` first."

### 2. Identify Uncertain Sections

Scan plan.md for:

- Sections containing "NEEDS CLARIFICATION" or "TBD" or "TODO"
- Sections with vague language ("might", "could", "possibly", "consider")
- Sections that reference technologies or patterns without concrete implementation details

List the uncertain sections for the user.

### 3. Search Prior Learnings

If `.specify/solutions/` exists, search for solutions relevant to each uncertain section:

- Match by category (e.g., uncertain D1 query patterns → search `cloudflare-workers/`)
- Match by keywords from the uncertain section
- Extract `## Solution` and `## Prevention` sections from matches

### 4. Research and Expand

For each uncertain section:

1. If prior learnings exist, incorporate the concrete patterns from the solution documents
2. If no prior learnings exist, research the unknown using available tools
3. Replace vague language with concrete implementation details
4. Add code examples where helpful

### 5. Update plan.md

Use the Edit tool to expand uncertain sections in plan.md with:

- Concrete implementation approaches
- Code patterns or configuration examples
- Links to referenced prior learnings

If prior learnings were applied, add or update the `## Applied Learnings` section.

### 6. Report

Output:

```markdown
## Plan Deepened

**Sections expanded**: {count}

{For each expanded section:}

- **{section name}**: {what was clarified}

**Prior learnings applied**: {count or "none"}
**Remaining uncertainties**: {count or "none"}

Run `/sp:04-red-team` to adversarially review the strengthened plan.
```

## Commit Changes

Run the `/commit` skill to stage and commit all changes made during this phase. Do not push.

---

Use subagents liberally and aggressively to conserve the main context window.
