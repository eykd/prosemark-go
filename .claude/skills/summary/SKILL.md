---
name: summary
description: 'Generate a dated project status snapshot at docs/summaries/YYYY-MM-DD.md.
  Use when onboarding contributors, capturing a milestone, or auditing project state.'
---

# Project Summary Snapshot

Generate a comprehensive, dated project status document using parallel subagents.

## Process

### Phase 1 — Parallel exploration (4 subagents)

Launch all four simultaneously:

**Subagent A — Core metadata**
Read: go.mod, main.go, justfile, lefthook.yml
Collect: module name, binary name, Go version, all deps+versions, justfile recipes (one-line purpose each), lefthook hooks, git branch + last 10 commits

**Subagent B — Code structure**
List: cmd/ (each file + one-line purpose), internal/ (each package + files + one-line purpose), acceptance/ (structure), conformance/ (structure), docs/ (all files)
Read key files to understand purpose: cmd/root.go, internal/binder/types.go

**Subagent C — Counts and specs**
Glob: **/*.go excl *_test.go → source file count + list by package
Glob: **/*_test.go → test file count
Glob: .claude/skills/*/SKILL.md → skills list + count
Glob: specs/**/*.txt → acceptance spec list + count per feature
List: docs/summaries/ → prior snapshots

**Subagent D — Project state**
Run: `./bin/bd list --json` → parse open/closed/in-progress task counts and open task details
Read: specs/ subdirectories to identify feature slugs and status
Collect: active branch from git, recent PRs merged (from git log)

### Phase 2 — Synthesis subagent

Pass all Phase 1 results to one subagent to draft the full document with these sections:

1. Header — `# Project State Summary — YYYY-MM-DD`
2. Executive Summary — one paragraph: what pmk is, current feature status, test coverage
3. Project Identity — table: Module, Binary, Go Version, Dependencies, Current Branch, Active Feature
4. Architecture Overview — annotated directory tree (cmd/, internal/, acceptance/, conformance/, specs/, docs/)
5. CLI Interface — table: Command, Flags, Output; plus output convention note
6. Domain Model — Core types (Node, Diagnostic, ParseResult, OpResult) + Selector syntax
7. Feature Status — completed features table + active feature user stories table with acceptance scenario counts
8. Test Infrastructure — test streams table (unit/coverage/conformance/acceptance), coverage by package table, Impl pattern explanation, test file counts by package
9. Quality Gates — table: Gate, Tool, Status
10. Beads Issue Tracker — open/closed/in-progress counts + open issues table (ID, Type, Priority, Title, Status)
11. ATDD Pipeline — ASCII diagram of specs → IR → generated tests → results; note on stub preservation
12. Normative Specifications — list of docs/ spec files with one-line description each
13. Development Workflow — TDD cycle steps, quality check sequence (bash block), commit convention
14. Known Issues and Next Steps — numbered list from open beads tasks

### Phase 3 — Write output

Write draft to `docs/summaries/YYYY-MM-DD.md` (using today's date).

## Formatting Rules

- GitHub-flavored Markdown, tables for structured data, code blocks for trees and commands
- Actual counts and versions — no placeholders
- Concise: reference snapshot, not prose
- Section dividers (`---`) between major sections

## Reference Template

Use `docs/summaries/2026-02-28.md` as the canonical example of format and detail level.

## Output

Confirm to the user:

- File path written
- Key stats (source files, tests, skills, specs)
- Notable changes from last snapshot (if one exists)
