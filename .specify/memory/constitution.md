<!--
Sync Impact Report:
- Version: (new) → 1.0.0 (MAJOR - Initial constitution creation)
- Modified principles: N/A (initial creation)
- Added sections:
  - Preamble: Alignment and Purpose (Truth, Good, Beauty)
  - Zeroth Principle: Fidelity to Reality and Stewardship
  - I. Acceptance Test-Driven Development
  - II. Static Analysis and Type Safety
  - III. Code Quality Standards
  - IV. Pre-commit Quality Gates
  - V. Warning and Deprecation Policy
  - VI. Go CLI Target Environment
  - VII. Simplicity and Maintainability
  - Governance
- Removed sections: None
- Templates requiring updates:
  ⚠ .specify/templates/plan-template.md - Does not exist yet
  ⚠ .specify/templates/spec-template.md - Does not exist yet
  ⚠ .specify/templates/tasks-template.md - Does not exist yet
  ✅ CLAUDE.md - Already aligned with constitution principles
- Follow-up TODOs: Create .specify/templates/ when spec workflow is adopted
-->

# PROSEMARK-GO Constitution

## Preamble: Alignment and Purpose

This constitution governs a development process designed to
**minimize harm and maximize durable benefit** by aligning code
with reality, responsibility, and clarity.

It is ordered toward three non-negotiable ends:

### Truth (Accuracy and Correspondence)

- Code must behave as specified, tested, and reasoned about.
- Tests, static analysis, and explicit error handling exist to
  **detect error early** and prevent false confidence.
- Any process that produces passing checks while masking incorrect
  behavior is harmful.

**Harm avoided**: silent failures, false assurances, regression,
brittle systems.
**Benefit preserved**: correctness, predictability, trustworthy
behavior.

### Good (Responsibility and Stewardship)

- Development decisions affect users, operators, and future
  maintainers.
- Strict processes exist to **reduce downstream cost, risk, and
  cognitive burden**.
- Short-term convenience that creates long-term fragility is a
  violation of intent.

**Harm avoided**: user-facing defects, operational risk, technical
debt, burnout.
**Benefit preserved**: safety, maintainability, accountability,
sustainable velocity.

### Beauty (Clarity and Order)

- Clear structure, readable code, and disciplined simplicity
  improve understanding.
- Beauty here is not aesthetics but **legibility**: code that can
  be reasoned about without guesswork.
- Cleverness that obscures intent is harmful, even if correct.

**Harm avoided**: confusion, misinterpretation, accidental misuse.
**Benefit preserved**: comprehension, refactorability, calm
maintenance.

All rules that follow are tools in service of these ends.
Passing checks is necessary but not sufficient; **understanding
and fidelity are required**.

No optimization, shortcut, or interpretation is valid if it
undermines truth, responsibility, or clarity -- even if it
satisfies automated enforcement.

---

## Zeroth Principle: Fidelity to Reality and Stewardship

All development governed by this constitution exists to
**faithfully serve reality**, not to simulate progress, satisfy
tooling, or produce the appearance of rigor.

Accordingly:

- The purpose of every test, type, rule, and gate is **truthful
  correspondence between intent, behavior, and outcome**.
- Correctness is valued over cleverness, integrity over speed,
  and clarity over persuasion.
- Process exists to **reveal errors early**, not to conceal
  uncertainty or manufacture confidence.
- Rigor is a moral discipline: shortcuts, bypasses, or ritual
  compliance that undermine understanding are violations of this
  constitution, even if all checks pass.

This system is built for **stewardship**:

- Stewardship of users, who rely on correct behavior.
- Stewardship of future developers, who inherit the code.
- Stewardship of tools, which must not be treated as oracles.
- Stewardship of judgment, which cannot be delegated to
  automation.

No rule in this constitution may be interpreted in a way that
excuses:

- Willful blindness ("the tests passed").
- False certainty ("the types prove it's correct").
- Cargo-cult rigor ("this is how it's done").
- Abdication of responsibility ("the process allowed it").

When rules conflict, **faithfulness to reality and the user's
actual experience takes precedence**.

This principle is not optional, optimizable, or enforceable by
tooling. It is the condition under which all other principles
remain valid.

---

## Core Principles

### I. Acceptance Test-Driven Development (NON-NEGOTIABLE)

All implementation is driven by observable behavior. Acceptance
tests define WHAT the system must do; unit tests verify HOW it
does it internally. A task is NOT done when unit tests pass --
it is done when the **acceptance test** passes.

#### Outer Loop: Acceptance Tests (WHAT)

Acceptance tests are the primary driver of all implementation.
They define "done" in domain terms.

- Every user story MUST have a GWT (Given-When-Then) spec before
  implementation begins
- GWT specs live in `specs/` as `.txt` files, one per user story:
  `specs/US<N>-<kebab-title>.txt`
- Specs MUST use domain language only -- no code, infrastructure,
  or implementation terms
- The acceptance pipeline transforms specs into executable Go
  tests: `specs/*.txt -> parse -> IR JSON -> generate -> Go tests`
- A generated acceptance test MUST fail before any inner TDD
  cycle begins (Acceptance Red)
- Implementation is complete ONLY when the acceptance test passes
  (Acceptance Green) -- this is the **objective gate**

| Stream | Purpose | Command |
|--------|---------|---------|
| Acceptance tests | Observable behavior (WHAT) | `just acceptance` |
| Unit tests | Internal correctness (HOW) | `just test` |

**Rationale**: Acceptance tests anchor implementation to user-
observable behavior, preventing feature drift and ensuring that
internal correctness serves an externally verifiable outcome.

#### Inner Loop: TDD (HOW)

Within each acceptance cycle, Test-Driven Development is
MANDATORY for all production code. No exceptions.

- ralph.sh orchestrates the workflow:
  1. **Acceptance Red**: Generate acceptance test from spec
     (fails by design)
  2. **Inner TDD cycles** (repeat up to N):
     - **Red**: Write failing unit test
     - **Green**: Write minimal code to pass
     - **Refactor**: Improve code while keeping code green
  3. **Acceptance check**: After each inner cycle, run acceptance
     tests
  4. **Acceptance Green**: Task is DONE when acceptance tests pass
- Tasks without specs fall back to standard TDD (inner loop only)
- 100% test coverage of testable code MUST be maintained
- Table-driven tests MUST be used where multiple input/output
  combinations exist
- Tests use `_test.go` suffix in the same package

**Rationale**: The inner TDD cycle builds correctness
incrementally toward the acceptance goal. Red-Green-Refactor
prevents implementation drift while the outer acceptance gate
prevents feature drift.

#### Coverage Exemptions (Impl Pattern)

- `*Impl` functions that wrap external operations (exec.Command,
  os file system calls) are excluded from coverage calculation
- These functions MUST contain only the external call and minimal
  argument passing -- no business logic
- The coverage check script filters out Impl functions from the
  calculation automatically
- When achieving 100% coverage is difficult:
  - First, extract logic from Impl functions into testable pure
    functions
  - Second, use interfaces and dependency injection to mock
    external dependencies
  - Only as last resort, use the Impl pattern to isolate
    untestable external calls

**Scope**: All Go source code in the repository.

### II. Static Analysis and Type Safety

Go's type system and static analysis tooling MUST be leveraged
to their full extent.

- `go vet ./...` MUST produce zero warnings
- `staticcheck ./...` MUST produce zero warnings
- All errors MUST be handled explicitly (no `_` for error values)
- No panics except at process boundaries (e.g., `main()`)
- `context.Context` MUST be the first parameter for functions
  that support cancellation or deadlines
- Interfaces MUST be defined by consumers, not producers
  (accept interfaces, return structs)

**Rationale**: Go's compile-time type checking combined with
vet and staticcheck catches bugs before runtime. Explicit error
handling prevents silent failures. Consumer-defined interfaces
enable loose coupling and testability.

### III. Code Quality Standards

Consistent code style, documentation, and naming conventions
MUST be maintained.

- `gofmt` formatting is MANDATORY -- all code MUST be formatted
  before commit
- GoDoc comments MUST be present on all exported functions,
  methods, types, and interfaces
- **Naming conventions** follow Go standards:
  - Exported names: PascalCase
  - Unexported names: camelCase
  - Acronyms: consistent casing (URL not Url, HTTP not Http)
  - Interfaces: named by behavior, not prefixed with I
    (e.g., `Reader` not `IReader`)
  - Test files: `*_test.go` in the same package
- **Package design**:
  - Short, lowercase, single-word package names
  - No `util`, `common`, or `misc` packages
  - Package names MUST not stutter with exported names
    (e.g., `http.Server` not `http.HTTPServer`)
- **Import order**: stdlib -> external -> internal (separated
  by blank lines)

**Rationale**: Consistent style reduces cognitive load, improves
maintainability, and enables effective code review. GoDoc
documentation ensures APIs are self-documenting.

### IV. Pre-commit Quality Gates

Automated quality gates MUST pass before ANY commit is accepted.

- Lefthook pre-commit hooks enforce:
  - `gofmt` formatting check
  - `go vet ./...` (zero warnings)
  - `staticcheck ./...` (zero warnings)
  - `go test ./...` with 100% coverage (non-Impl functions)
- ALL quality checks MUST pass (no bypassing with `--no-verify`)
- Commit messages MUST follow conventional commits format:
  - Types: feat, fix, docs, style, refactor, perf, test, chore,
    revert
  - Format: `type: lowercase subject` (no period, max 100 chars)
- The `just check` command runs all quality gates:
  `just test && just vet && just lint && just fmt-check`

**Rationale**: Pre-commit gates catch issues before they enter
the repository, maintaining a consistently high-quality codebase.
Conventional commits enable automated changelog generation and
semantic versioning.

### V. Warning and Deprecation Policy

ALL warnings and deprecations MUST be addressed immediately.
No deferral allowed.

- `go vet` warnings MUST be fixed before proceeding
- `staticcheck` warnings MUST be fixed before proceeding
- Deprecation warnings from dependencies MUST be addressed
- Security advisories (`govulncheck` when available) MUST be
  resolved
- Test warnings or flaky tests MUST be fixed
- Never ignore or defer warnings

**Rationale**: Warnings are early indicators of problems.
Addressing them immediately prevents technical debt accumulation
and avoids compounding issues that become harder to fix later.

### VI. Go CLI Target Environment

Code MUST be compatible with Go CLI application constraints.

- Target: Go 1.23 with standard library
- CLI framework: Cobra
- No CGO dependencies unless explicitly justified
- Respect CLI conventions:
  - Exit code 0 for success, non-zero for failure
  - Errors to stderr, output to stdout
  - Support `--help` and `-h` flags on all commands
- Binary MUST be cross-compilable for Linux, macOS, and Windows
- File system operations MUST use `path/filepath` (not
  `path`) for OS-portable paths

**Rationale**: A Go CLI tool must work reliably across platforms.
CGO-free builds enable simple cross-compilation. Standard CLI
conventions ensure predictable behavior for users and scripts.

### VII. Simplicity and Maintainability

Start simple, build only what is needed, maintain clarity over
cleverness.

- YAGNI (You Aren't Gonna Need It) -- no speculative features
- KISS (Keep It Simple, Stupid) -- prefer simplicity over
  complexity
- DRY (Don't Repeat Yourself) -- abstract common functionality,
  but three similar lines of code is better than a premature
  abstraction
- Clear, descriptive names over terse abbreviations
- Comments explain "why", not "what" (code MUST be
  self-documenting)
- Avoid over-engineering: only make changes that are directly
  requested or clearly necessary

**Rationale**: Simple code is easier to understand, maintain,
test, and debug. Premature optimization and feature speculation
create unnecessary complexity and technical debt.

**Reference**: The `/prefactoring` skill provides detailed
guidance on applying these principles during design and
implementation.

---

## Governance

### Amendment Procedure

1. Propose amendment with clear rationale
2. Identify affected templates and code
3. Create migration plan for existing code if needed
4. Update constitution with version bump
5. Propagate changes to all dependent templates
6. Update CLAUDE.md if runtime guidance affected
7. Commit with message:
   `docs: amend constitution to vX.Y.Z (summary)`

### Versioning Policy

Constitution follows semantic versioning:

- **MAJOR**: Backward incompatible governance/principle removals
  or redefinitions
- **MINOR**: New principle/section added or materially expanded
  guidance
- **PATCH**: Clarifications, wording, typo fixes, non-semantic
  refinements

### Compliance Review

- Constitution supersedes all other practices and documentation
- All PRs/reviews MUST verify compliance with constitution
- Any complexity introduced MUST be justified
- Violations require either fix or constitutional amendment
- Use CLAUDE.md for runtime development guidance to Claude Code

**Version**: 1.0.0 | **Ratified**: 2026-02-22 | **Last Amended**: 2026-02-22
