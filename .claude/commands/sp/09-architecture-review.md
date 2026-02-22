---
description: Review all changes from the base branch to HEAD for architectural compliance. Create remediation beads tasks on the current epic for any findings not already tracked.
---

## User Input

```text
$ARGUMENTS
```

You **MUST** consider the user input before proceeding (if not empty).

## Scope

Review scope is **all branch changes from the base branch (usually `master` or `main`) to `HEAD`**.

Determine base branch:

1. Try:

```bash
git symbolic-ref --short refs/remotes/origin/HEAD | sed 's|^origin/||'
```

2. If that fails, choose the first existing branch in this order:

- `main`
- `master`

Verify it exists locally/remotely:

```bash
git show-ref --verify --quiet refs/heads/<base> || git show-ref --verify --quiet refs/remotes/origin/<base>
```

Compute the review range:

- `<base>..HEAD`

## Steps

1. Run `.specify/scripts/bash/check-prerequisites.sh --json --require-tasks --include-tasks` and parse `FEATURE_DIR` (absolute path).

2. Retrieve the current epic id:

```bash
grep "Beads Epic" FEATURE_DIR/spec.md | grep -oE 'workspace-[a-z0-9]+|bd-[a-z0-9]+'
```

If no epic id is found, ERROR with: "No Beads Epic found for this feature. Run `/sp:01-specify` first."

3. List current open + in_progress tasks for the epic (for de-duplication):

```bash
npx bd list --parent <epic-id> --status open --json
npx bd list --parent <epic-id> --status in_progress --json
```

4. Identify changed files and summary:

```bash
git diff --name-only <base>..HEAD
git diff --stat <base>..HEAD
```

5. Run the review skill over this change set:

- Skill: `/clean-architecture-validator`
- Review range: `<base>..HEAD`
- Focus: architectural compliance
- **Style Requirements**:
  - Report problems only, never praise or positive feedback
  - Make findings scannable in 30 seconds
  - Format: file:line, severity, one-sentence problem, concise fix
  - Include copy-paste prompt ONLY if findings exist (3-5 lines)

If the skill supports reading the git diff directly, prefer providing it the diff for `<base>..HEAD`.

6. Translate findings into beads tasks:

For each distinct finding:

- Check if an existing task already covers it (match by keywords + file path).
- If not covered, create a new task under the epic:

```bash
npx bd create "Remediate: <short finding title>" -p <1|2|3> --parent <epic-id> \
  --description "**Review**: [sp:09-architecture-review]\n**Range**: <base>..HEAD\n**Files**: <file paths>\n\n**Finding**:\n<what's wrong>\n\n**Fix suggestion**:\n<concrete steps>\n\n**Acceptance**:\n- <verifiable criteria>" --json
```

Severity → priority mapping:

- CRITICAL security/architecture correctness → p1
- MAJOR → p2
- MINOR/nits → p3

7. Cross-reference with Prior Learnings:

   Search `.specify/solutions/` for solutions matching current finding categories. If the directory does not exist, skip this step silently.
   - Search `.specify/solutions/clean-architecture/` and `.specify/solutions/type-safety/` for solutions related to current findings
   - If an implementation repeats a previously solved pattern, note it in the remediation task description with a reference to the original solution document
   - Example addition to task description: `\n\n**Prior Learning**: See .specify/solutions/clean-architecture/{slug}.md for a previous solution to this pattern.`

8. Output a concise summary:

- Base branch used
- Number of findings
- Number of new tasks created
- List created task IDs + titles
- If remediation tasks were created, include: "Consider running `/compound` to document what you learned fixing these issues."
