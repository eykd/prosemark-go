## Project Overview

`pmk` is a CLI application for long-form prose projects using a new outline organization strategy. Written in Go.

## Development Process

**Strict TDD is mandatory.** Follow Red -> Green -> Refactor for all production code:
1. Write failing test first
2. Write minimal code to pass
3. Refactor while keeping tests green

Use table-driven tests where appropriate.

**Coverage Policy**: 100% coverage of testable code is required. The Impl pattern exempts external operations:
- `*Impl` functions wrap OS/exec calls and are excluded from coverage calculation
- See `.claude/skills/go-tdd/references/coverage.md` for exemption guidelines

## Commands (via justfile)

```bash
just test              # Run all unit tests
just test-cover        # Run tests with coverage report
just test-cover-check  # Verify coverage meets threshold
just vet               # Run go vet
just lint              # Run staticcheck
just fmt               # Format code with gofmt
just check             # Run all quality gates (test, vet, lint, fmt check)
just acceptance        # Full acceptance pipeline: parse, generate, run
just test-all          # Run both unit tests and acceptance tests
```

Run a single test:
```bash
go test -run TestName ./path/to/package
```

## Quality Gates

All must pass before commit (enforced via lefthook pre-commit hooks and GitHub Actions):
- `gofmt` formatting
- `go vet ./...` (zero warnings)
- `staticcheck ./...` (zero warnings)
- `go test ./...` with 100% coverage required for non-Impl functions

Coverage exemptions (per `.claude/skills/go-tdd/references/coverage.md`):
- `*Impl` functions that wrap external commands (exec.Command, os operations)
- These are tested via integration tests, not unit tests
- The coverage check filters out Impl functions from the calculation
- **Build config consistency**: Coverage package exclusions are defined in both `lefthook.yml` (pre-commit) and `justfile` (test-cover-check). These must stay in sync — update both when adding package exclusions.

### CLI Smoke Test

After changes to `main.go`, `cmd/root.go`, or command wiring, run `just smoke` to verify errors are visible to users. This is also enforced in pre-commit.

After cloning, install git hooks:
```bash
lefthook install
```

## Code Standards

- All errors must be handled explicitly (no `_` for errors)
- No panics except at process boundaries (e.g., `main()`)
- Interfaces defined by consumers, not producers
- `context.Context` required for cancellation/deadlines
- GoDoc comments on all exported APIs

## ATDD Workflow

This project uses Acceptance Test-Driven Development (ATDD) to anchor implementation to observable behavior. Two test streams run in parallel:

| Stream | Purpose | Command |
|--------|---------|---------|
| Unit tests | Internal correctness (HOW) | `just test` |
| Acceptance tests | Observable behavior (WHAT) | `just acceptance` |

### GWT Acceptance Specs

Specs live in `specs/` as `.txt` files using Given-When-Then format:
- One file per user story: `specs/US<N>-<kebab-title>.txt`
- Domain language only — no code or infrastructure terms
- Generated during `sp:05-tasks`, committed to git
- Run `/spec-check` to audit specs for implementation leakage

### Acceptance Pipeline

The `acceptance/` package transforms GWT specs into executable Go tests:
```
specs/*.txt → parse → IR JSON → generate → Go test files
```

Pipeline CLI: `go run ./acceptance/cmd/pipeline -action=<parse|generate|run>`

## Active Technologies
- Go 1.25 + Cobra (CLI framework)
- Additional dependencies will expand as features are implemented


---

Always use subagents liberally and aggressively to conserve the main context window.