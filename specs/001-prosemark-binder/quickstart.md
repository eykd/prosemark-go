# Quickstart: Prosemark Binder v1 Development

**Branch**: `001-prosemark-binder` | **Date**: 2026-02-23

---

## Development Environment

```bash
# Install git hooks (run once after clone)
lefthook install

# Verify all quality gates pass
just check

# Run unit tests
just test

# Run with coverage report
just test-cover

# Run conformance suite (after building binary)
just conformance-run

# Run everything
just test-all
```

---

## TDD Workflow

Follow Red → Green → Refactor for every function. The ATDD outer loop:

```bash
# 1. Acceptance Red: ensure acceptance test stub exists and fails
just acceptance

# 2. Inner TDD cycles: write failing unit test, implement, refactor
go test -run TestParse ./internal/binder/...

# 3. After inner cycles pass, check acceptance
just acceptance

# 4. Done when acceptance passes
```

---

## Package Layout

```
internal/binder/          ← all domain logic (pure functions, no I/O)
internal/binder/ops/      ← mutation operations
cmd/                      ← thin Cobra command wrappers (I/O only)
conformance/              ← integration tests against built binary
```

**Rule**: `internal/binder` and `internal/binder/ops` MUST NOT import `os`, `exec`, or
perform any I/O. All file reads/writes happen in `cmd/` commands or in `*Impl` functions.

---

## Adding a New Test

1. For parser behavior, add a table case in `internal/binder/parser_test.go`
2. For selector behavior, add a case in `internal/binder/selector_test.go`
3. For operations, add a case in `internal/binder/ops/addchild_test.go` (etc.)
4. For CLI integration, use `conformance/runner_test.go` which runs live fixtures

---

## Running a Single Conformance Fixture

```bash
# Build binary
just build

# Parse fixture
./bin/pmk parse --json docs/conformance/v1/parse/fixtures/001-empty-file/binder.md \
    --project docs/conformance/v1/parse/fixtures/001-empty-file/project.json

# Operation fixture
./bin/pmk add-child --json docs/conformance/v1/ops/fixtures/add-child/041-append-to-root/input-binder.md \
    --project docs/conformance/v1/ops/fixtures/add-child/041-append-to-root/project.json \
    --parent . --target ch2.md --title "Chapter Two"
```

---

## Key Conformance Fixture Categories

| Range | Category |
|-------|----------|
| 001–040 | Parse: basic, pragma, links, wikilinks, lists |
| 041–050 | add-child: positions, markers, indentation |
| 051–055 | delete: leaf, subtree, cleanup |
| 056–061 | move: reorder, reparent, cycle detection |
| 062–067, 070 | ops-error: abort conditions (OPExx) |
| 068–069 | stability: round-trip |
| 071–086 | Parse: edge cases (line endings, BOM, code fences) |
| 087–108 | Mixed ops: advanced scenarios |
| 109–135 | Latest additions per spec rectification |

---

## Coverage Exemptions

Functions named `*Impl` (wrapping `os.ReadFile`, `os.WriteFile`, `exec.Command`, etc.) are
excluded from the 100% coverage requirement per the Impl pattern in `CLAUDE.md`. Keep all
business logic outside `*Impl` functions so it can be unit-tested with byte-slice inputs.

---

## Quality Gate Commands

```bash
just fmt            # Auto-format all Go code
just fmt-check      # Verify gofmt compliance (used in pre-commit)
just vet            # Run go vet
just lint           # Run staticcheck
just test-cover-check  # Verify 100% coverage of non-Impl functions
just check          # Run all quality gates (fmt-check, vet, lint, test-cover-check)
just smoke          # Verify CLI error visibility
```
