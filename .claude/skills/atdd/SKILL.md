---
name: atdd
description: Acceptance Test-Driven Development workflow with GWT specs, acceptance pipeline, and two-stream testing
triggers:
  - acceptance test
  - GWT spec
  - ATDD
  - acceptance criteria
  - Given When Then
---

# ATDD Workflow

Acceptance Test-Driven Development (ATDD) adds an outer acceptance test loop around the inner Red-Green-Refactor TDD cycle. This ensures implementations satisfy observable behavior requirements, not just unit-level correctness.

## Two Test Streams

| Stream | Purpose | Location | Scope |
|--------|---------|----------|-------|
| Acceptance tests | WHAT the system does | `generated-acceptance-tests/` | User-observable behavior |
| Unit tests | HOW it does it | Alongside production code | Internal correctness |

## GWT Spec Format

Specs live in `specs/` as `.txt` files using Uncle Bob's format:

```text
;===============================================================
; User can add a new outline item.
;===============================================================
GIVEN an empty outline.

WHEN the user adds an item titled "Chapter 1".

THEN the outline contains 1 item.
THEN the item is titled "Chapter 1".
```

### Rules
- One file per user story: `specs/US<N>-<kebab-title>.txt`
- Domain language ONLY — no code, infrastructure, or framework terms
- A non-developer must understand every statement
- Committed to git (specs are the source of truth)
- Run `/spec-check` to audit for implementation leakage

## Acceptance Pipeline

The pipeline transforms GWT specs into executable Go tests:

```
specs/*.txt  →  parse  →  IR JSON  →  generate  →  Go test files
```

### Commands

| Command | Description |
|---------|-------------|
| `just acceptance` | Full pipeline: parse, generate, run |
| `just acceptance-parse` | Parse `specs/*.txt` to IR JSON |
| `just acceptance-generate` | Generate Go tests from IR |
| `just acceptance-run` | Run generated acceptance tests |
| `just test-all` | Run both unit and acceptance tests |

### Pipeline CLI

```bash
go run ./acceptance/cmd/pipeline -action=parse     # specs → IR
go run ./acceptance/cmd/pipeline -action=generate  # IR → tests
go run ./acceptance/cmd/pipeline -action=run       # parse + generate + run
```

## ATDD Cycle Structure

```
execute_atdd_cycle(task):
    spec = find_spec_for_task(task)  # specs/US<N>-*.txt

    if no spec found:
        fall back to unit TDD cycle  # backward compatible

    # Check if acceptance tests already pass
    if acceptance tests PASS: close task, done

    # BIND: write acceptance test implementations from spec
    BIND: replace t.Fatal("...") stubs with real test code

    # Inner TDD loop (up to 15 cycles)
    for each inner cycle:
        RED:      write smallest failing unit test
        GREEN:    write minimal code to pass
        REFACTOR: improve without changing behavior

        # Check acceptance after each inner cycle
        if acceptance tests PASS: close task, done

    # Exhausted: mark BLOCKED
```

### Key principles
- Acceptance tests define DONE — not LLM judgment
- BIND step writes test implementations before inner TDD starts
- Each inner cycle makes the smallest possible change
- Tasks without specs fall back to standard TDD (backward compatible)
- Inner cycle limit is 15 (vs 5 for pure TDD) to allow incremental progress
- The pipeline preserves bound test implementations across regeneration

## Spec Guardian

The spec-guardian agent audits specs for implementation leakage:
- Run with `/spec-check` or `/spec-check specs/US1-add-item.txt`
- Checks for code references, infrastructure terms, framework language, etc.
- Outputs a table of findings with suggested rewrites

## Integration with sp:05-tasks

When `/sp:05-tasks` runs, it now generates GWT acceptance specs (step 5) for every user story before creating beads tasks. This ensures acceptance criteria are defined before implementation begins.
