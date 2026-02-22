---
name: acceptance-tests
description: >
  Writing GWT acceptance specs and binding generated stubs into executable tests.
  Use when creating new specs in specs/, binding generated acceptance test stubs,
  or troubleshooting acceptance pipeline failures.
triggers:
  - write spec
  - write acceptance test
  - bind acceptance test
  - GWT spec
  - acceptance stub
  - spec writing
---

# Acceptance Tests Skill

## Decision Tree

```
What do you need?
│
├─ Write a new spec file
│  → Section: Writing GWT Specs (below)
│  → Reference: gwt-writing-guide.md
│
├─ Bind a generated stub into a real test
│  → Section: Binding Generated Stubs (below)
│  → Reference: binding-patterns.md
│
├─ Run the acceptance pipeline
│  → Section: Pipeline Quick Reference (below)
│  → Skill: atdd (for full workflow details)
│
└─ Understand the TDD cycle around acceptance tests
   → Skill: atdd (ATDD cycle, ralph.sh integration)
   → Skill: go-tdd (inner Red-Green-Refactor loop)
```

## Writing GWT Specs

Specs live in `specs/` as plain text files using Given-When-Then format.

### File Naming

```
specs/US<N>-<kebab-case-title>.txt
```

One file per user story. The `US<N>` prefix must match the story number.

### Format

```
;=============================================
; Scenario: Description of the scenario
;=============================================
GIVEN some precondition.
WHEN an action occurs.
THEN an observable outcome.
```

- Separator lines: only `;` and `=` characters
- Description lines: start with `;` (but are not separators)
- Keywords: `GIVEN`, `WHEN`, `THEN` in ALL CAPS at line start
- Every step ends with a period
- Multiple scenarios per file are allowed (each gets its own separator block)

### Domain Language Discipline

Specs describe **what the user observes**, not how the system works internally.

**Good** — domain language:
```
GIVEN a project directory with three chapter files.
WHEN the user runs lmk status.
THEN the output lists three chapters.
```

**Bad** — implementation leakage:
```
GIVEN a filesystem directory containing markdown files matching glob pattern.
WHEN the CLI invokes the status cobra command handler.
THEN stdout contains JSON-formatted chapter metadata.
```

Rules:
- No code identifiers (function names, types, packages)
- No infrastructure terms (filesystem, stdout, cobra, JSON)
- No internal architecture (handler, middleware, parser)
- Use `/spec-check` to audit existing specs for leakage

See `references/gwt-writing-guide.md` for detailed examples and review checklist.

## Binding Generated Stubs

### Architecture

The acceptance pipeline generates test stubs with merge support:

```
specs/*.txt → parse → IR JSON → generate → generated-acceptance-tests/*_test.go
```

Generated stubs contain `t.Fatal("acceptance test not yet bound")` — they are
scaffolds showing what needs to be tested. The pipeline **preserves bound
implementations** across regeneration: any test function that does not contain
the unbound sentinel is kept as-is when stubs are regenerated.

**Edit generated files directly.** Replace `t.Fatal("acceptance test not yet bound")`
with real test code. The pipeline will preserve your implementations on subsequent runs.
Use `just acceptance-regen` to force-regenerate all stubs (destroys bound implementations).

### CLI Execution Pattern

Acceptance tests exercise `lmk` as a black box via `exec.Command`:

```go
func Test_Some_scenario(t *testing.T) {
    dir := t.TempDir()
    // GIVEN: set up files in dir

    cmd := exec.Command("go", "run", ".", "status")
    cmd.Dir = dir
    output, err := cmd.CombinedOutput()
    if err != nil {
        t.Fatalf("command failed: %v\n%s", err, output)
    }

    // THEN: assert on string(output)
    if !strings.Contains(string(output), "3 chapters") {
        t.Errorf("expected chapter count in output, got: %s", output)
    }
}
```

Key points:
- Use `t.TempDir()` for isolated file system setup (auto-cleaned)
- Run CLI via `exec.Command("go", "run", ".", <subcommand>, <args>...)`
- Assert on combined output as strings — the user-observable surface
- Each test function maps to one scenario from the spec

See `references/binding-patterns.md` for setup/teardown, output assertion,
and multi-step scenario patterns.

## Pipeline Quick Reference

| Action | Command |
|--------|---------|
| Full pipeline (parse + generate + run) | `just acceptance` |
| Parse specs to IR | `just acceptance-parse` |
| Generate stubs from IR | `just acceptance-generate` |
| Run generated tests | `just acceptance-run` |
| Unit + acceptance | `just test-all` |
| Audit specs for leakage | `/spec-check` |

For full ATDD workflow, cycle management, and ralph.sh integration,
see the **atdd** skill.

## References

- `references/gwt-writing-guide.md` — Detailed spec writing craft, good/bad examples, review checklist
- `references/binding-patterns.md` — Code templates for binding stubs into executable tests
- **atdd** skill — ATDD workflow, pipeline details, ralph.sh outer loop
- **go-tdd** skill — Go testing patterns, TDD micro/macro loops
