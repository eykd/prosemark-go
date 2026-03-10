# Feature Specification: Agent-Native CLI Improvements

**Feature Branch**: `004-agent-native-cli`
**Created**: 2026-03-10
**Status**: Draft
**Beads Epic**: `prosemark-go-02c`
**Beads Workflow Phase Tasks**:

- [sp:02-clarify]: `prosemark-go-02c.1` (CLOSED)
- [sp:03-plan]: `prosemark-go-02c.2` (CLOSED)
- [sp:04-red-team]: `prosemark-go-02c.3`
- [sp:05-tasks]: `prosemark-go-02c.4`
- [sp:06-analyze]: `prosemark-go-02c.5`
- [sp:07-implement]: `prosemark-go-02c.6`
- [sp:08-security-review]: `prosemark-go-02c.7`
- [sp:09-architecture-review]: `prosemark-go-02c.8`
- [sp:10-code-quality-review]: `prosemark-go-02c.9`

**Beads Implementation Tasks** (block sp:07-implement):

- Phase 1 (exit codes): `prosemark-go-znw`
- Phase 2 (suggestions): `prosemark-go-96n` (depends on Phase 1)
- Phase 3 (dry-run): `prosemark-go-0x0` (depends on Phase 1)
- Phase 4 (help text): `prosemark-go-tcg` (depends on Phases 1, 3)

## User Scenarios & Testing _(mandatory)_

### User Story 1 — Semantic exit codes distinguish failure types (Priority: P1)

An AI agent invokes `pmk delete --selector nonexistent --yes` and receives exit code 1. It cannot distinguish "node not found" from "I/O failure" from "validation error," so it cannot decide whether to retry, correct its input, or report an infrastructure problem. With semantic exit codes, the agent receives exit code 3 (not found) and knows to try a different selector.

**Why this priority**: Exit codes are the most fundamental control-flow signal for agents. Every other improvement (suggestions, dry-run) builds on the agent being able to classify failures first.

**Independent Test**: Run any pmk mutation command with inputs that trigger each diagnostic error class and verify the process exits with the correct semantic code.

**Acceptance Scenarios**:

1. **Given** a binder with no node matching selector "nonexistent", **When** `pmk delete --selector nonexistent --yes` is run, **Then** the process exits with code 3 (not found).
2. **Given** a valid binder, **When** `pmk addchild --selector root --first --at 0 --yes` is run (conflicting flags), **Then** the process exits with code 1 (usage error).
3. **Given** a project directory with no `_binder.md`, **When** `pmk parse` is run, **Then** the process exits with code 6 (transient/I/O failure).
4. **Given** a binder with a node whose path contains illegal characters, **When** `pmk parse` is run, **Then** the process exits with code 2 (validation error).
5. **Given** a binder with an ambiguous bare stem selector matching multiple nodes, **When** `pmk delete --selector ambiguous --yes` is run, **Then** the process exits with code 5 (conflict).
6. **Given** a valid operation, **When** `pmk parse` succeeds, **Then** the process exits with code 0 (success).

---

### User Story 2 — Error messages include recovery guidance (Priority: P1)

An AI agent receives an error diagnostic `OPE001: selector matched no nodes` but has no guidance on what to do next. With suggestions, the diagnostic includes `Run 'pmk parse --json' to list available nodes and their selectors`, enabling the agent to self-correct.

**Why this priority**: Suggestions transform errors from dead ends into actionable next steps, dramatically reducing agent error-recovery loops.

**Independent Test**: Trigger each diagnostic error code and verify the output (both human and JSON modes) includes a suggestion string.

**Acceptance Scenarios**:

1. **Given** a selector that matches no nodes, **When** `pmk delete --selector nonexistent --yes` is run in human mode, **Then** the error output includes a suggestion mentioning `pmk parse --json`.
2. **Given** a selector that matches no nodes, **When** `pmk delete --selector nonexistent --yes --json` is run, **Then** the JSON output contains a `suggestion` field on the OPE001 diagnostic.
3. **Given** conflicting position flags, **When** `pmk addchild --selector root --first --at 0 --yes` is run, **Then** the error includes a suggestion about specifying only one positioning flag.
4. **Given** a missing binder file, **When** `pmk parse` is run, **Then** the error includes a suggestion to check that `_binder.md` exists and run `pmk doctor`.

---

### User Story 3 — Dry-run previews mutations without writing (Priority: P1)

An AI agent wants to verify that a delete operation will affect the correct nodes before committing. It runs `pmk delete --selector chapter-3 --yes --dry-run --json` and receives a full `OpResult` showing what would change, with `dryRun: true` and `changed: false`, without modifying any files.

**Why this priority**: Dry-run enables agents to preview and validate operations before committing, reducing destructive mistakes. It depends on exit codes (Phase 1) being in place.

**Independent Test**: Run any mutation command with `--dry-run` and verify that (a) the output is identical to a real run except for `dryRun`/`changed` fields and (b) no files are modified on disk.

**Acceptance Scenarios**:

1. **Given** a valid binder and a delete target, **When** `pmk delete --selector chapter-3 --yes --dry-run --json` is run, **Then** the JSON output has `dryRun: true` and `changed: false`, and the binder file is unmodified.
2. **Given** a valid binder, **When** `pmk addchild --selector root --target new.md --title "New" --yes --dry-run` is run in human mode, **Then** the output includes a `dry-run:` prefix and no files are created or modified.
3. **Given** a valid binder and a move target, **When** `pmk move --selector chapter-2 --before chapter-1 --yes --dry-run --json` is run, **Then** the JSON output has `dryRun: true` and the binder file is unmodified.
4. **Given** an invalid selector, **When** `pmk delete --selector nonexistent --yes --dry-run` is run, **Then** the command still reports the error with the correct exit code (dry-run does not suppress errors).
5. **Given** a non-existent project, **When** `pmk init --dry-run` is run, **Then** the output describes what would be created without creating any files.

---

### User Story 4 — Help text is self-sufficient for agents (Priority: P1)

An AI agent runs `pmk --help` to discover the tool's capabilities. The output includes a table of exit codes, environment variable documentation, a state model description, and examples for every subcommand — enough information for the agent to use the tool without reading external docs.

**Why this priority**: Rich help text completes the discoverability story. It has lower urgency because agents can currently use external docs and trial-and-error, but self-sufficient help reduces exploration overhead.

**Independent Test**: Run `pmk --help` and each subcommand's `--help` and verify that exit codes, env vars, state model, and examples are present.

**Acceptance Scenarios**:

1. **Given** the pmk binary, **When** `pmk --help` is run, **Then** the output contains an exit codes table listing codes 0–6 with meanings.
2. **Given** the pmk binary, **When** `pmk --help` is run, **Then** the output describes the state model (`.prosemark.yml`, `_binder.md`).
3. **Given** the pmk binary, **When** `pmk addchild --help` is run, **Then** the output includes at least two usage examples.
4. **Given** the pmk binary, **When** `pmk delete --help` is run, **Then** the output documents the `--dry-run` flag.

---

### Edge Cases

- Missing `--yes` flag on mutation commands: handled as a Cobra required-flag error (exit 1), not via OPE009. OPE009 is reserved exclusively for transient/I/O failures.
- Dry-run + --yes interaction: `--dry-run` bypasses the `--yes` requirement. Since no changes are made, confirmation is unnecessary. Agents can preview operations without providing `--yes`.
- Init command output: `init` uses `OpResult` (same as mutation commands) for consistent JSON output. Created files are reported as info-level diagnostics.
- Exit code mapping with multiple diagnostics of different severity: the first error diagnostic's mapped exit code wins (positional order).
- Exit code for warnings-only: exit 0 (warnings are not errors).
- Dry-run with `--json` and diagnostics: diagnostics are still emitted in the output.
- Dry-run on read-only commands (`parse`, `doctor`): flag is accepted but has no effect (these commands never write).
- Suggestion for unknown/unmapped diagnostic codes: no suggestion attached (field omitted, not empty string).
- Help text line width: no hard wrapping enforced (terminal handles it).
- Multiple error diagnostics with different exit code mappings: the first error diagnostic's exit code wins.

## Requirements _(mandatory)_

### Functional Requirements

#### Phase 1: Semantic Exit Codes

- **FR-001**: The CLI MUST exit with code 0 on success, code 1 for usage errors, code 2 for validation errors, code 3 for not-found errors, code 5 for conflict errors, and code 6 for transient/I/O errors.
- **FR-002**: An `ExitError` type MUST wrap an exit code and an inner error, and `main.go` MUST extract it via `errors.As` to determine the exit code.
- **FR-003**: Each command's `RunE` MUST return an `ExitError` with the appropriate code when diagnostics contain errors, using an `ExitCodeForDiagnostics` mapping function.
- **FR-004**: The diagnostic-to-exit-code mapping MUST be: OPE010 and Cobra flag errors → 1; OPE004, OPE005, BNDE001–003, AUD errors → 2; OPE001, OPE007, OPE008 → 3; OPE002, OPE003, OPE006 → 5; OPE009 → 6.
- **FR-005**: When multiple error diagnostics with different exit codes are present, the function MUST return a deterministic exit code (first error diagnostic's mapping wins).
- **FR-006**: Warning-only diagnostics MUST NOT cause a non-zero exit code.

#### Phase 2: Error Suggestions

- **FR-007**: The `Diagnostic` struct MUST include a `Suggestion string` field with JSON tag `json:"suggestion,omitempty"`.
- **FR-008**: An `attachSuggestions` function MUST populate the `Suggestion` field for known diagnostic codes based on a predefined mapping.
- **FR-009**: In human mode, `printDiagnostics` MUST print the suggestion on a separate line after the diagnostic message when a suggestion is present.
- **FR-010**: In JSON mode, the suggestion MUST appear in the diagnostic's JSON representation.
- **FR-011**: Diagnostic codes without a mapped suggestion MUST have an empty `Suggestion` field (omitted from JSON).

#### Phase 3: Dry-Run Support

- **FR-012**: A `--dry-run` persistent flag MUST be available on the root command.
- **FR-013**: When `--dry-run` is set, mutation commands (`addchild`, `delete`, `move`) MUST skip all file writes (binder writes and new node file creation).
- **FR-014**: When `--dry-run` is set, all validation and diagnostic computation MUST still execute fully.
- **FR-015**: The `OpResult` struct MUST include a `DryRun bool` field with JSON tag `json:"dryRun,omitempty"`.
- **FR-016**: In human mode with `--dry-run`, output MUST be prefixed with `dry-run:` to indicate no changes were made.
- **FR-017**: The `init` command MUST use `OpResult` for output (both human and JSON modes), with created files reported as info-level diagnostics. With `--dry-run`, it MUST report what files would be created without creating them.
- **FR-018**: Read-only commands (`parse`, `doctor`) MUST accept `--dry-run` without error but behave identically (no-op).

#### Phase 4: Help Text Enrichment

- **FR-019**: The root command's `Long` description MUST include a table of exit codes (0–6) with meanings.
- **FR-020**: The root command's `Long` description MUST document the state model (`.prosemark.yml`, `_binder.md` locations and roles).
- **FR-021**: Every subcommand MUST include an `Example` field with at least two usage examples.
- **FR-022**: Subcommands that support `--dry-run` MUST document it in their help text.

### Key Entities

- **ExitError**: A cmd-package error type wrapping an integer exit code and an inner error. Extracted in `main.go` via `errors.As` to determine the process exit code.
- **Suggestion**: A string field on `Diagnostic` containing an actionable recovery hint for agents. Populated by the `attachSuggestions` function based on a diagnostic-code-to-suggestion mapping.
- **DryRun flag**: A persistent boolean flag on the root command that suppresses file writes in mutation commands while preserving all validation and diagnostic logic.

## Success Criteria _(mandatory)_

### Measurable Outcomes

- **SC-001**: All six exit codes (0, 1, 2, 3, 5, 6) are exercised by unit tests that verify the process exit code for each diagnostic error class.
- **SC-002**: Every operation error diagnostic code (OPE001–OPE010) has a mapped suggestion verified by unit test.
- **SC-003**: Every parse/lint error diagnostic code (BNDE001–003) has a mapped suggestion verified by unit test.
- **SC-004**: `--dry-run` on `addchild`, `delete`, and `move` produces correct `OpResult` JSON with `dryRun: true` and does not modify any files, verified by unit tests.
- **SC-005**: `pmk --help` output contains the strings "Exit Codes", "State Model", and at least three environment-related terms.
- **SC-006**: Every subcommand's `--help` output contains at least one `Example` block.
- **SC-007**: `just check` passes with all quality gates (fmt, vet, lint, 100% coverage).
- **SC-008**: `just acceptance` passes (all existing acceptance tests remain green).
- **SC-009**: Existing behavior is preserved — no regression in any command's happy-path output.

## Assumptions

- The `ExitError` type can be a simple struct in the `cmd` package; no new package is needed.
- Adding `Suggestion` to `Diagnostic` is backward-compatible because of `omitempty`.
- Adding `DryRun` to `OpResult` is backward-compatible because of `omitempty`.
- The `--dry-run` flag as a persistent flag on root does not conflict with any existing flags.
- Exit code 4 (permission denied) is reserved but not mapped to any current diagnostic code; it may be used in future features.
- The `doctor` command uses `node.AuditDiagnostic` (not `binder.Diagnostic`); its exit code mapping may require a separate helper or adapter.

## Clarifications

### Session 2026-03-10

- Q: When multiple error diagnostics with different exit codes are present, which strategy determines the final exit code? → A: First error diagnostic's mapped exit code wins (positional order).
- Q: OPE009 is mapped to exit 6 (I/O) but also used for missing --yes. How should missing --yes be handled? → A: Treat missing --yes as a Cobra-level required flag error (exit 1); remove it from OPE009 usage. OPE009 remains exclusively for transient/I/O failures.
- Q: Should --dry-run bypass the --yes requirement on mutation commands? → A: Yes. --dry-run bypasses --yes since no changes are made; agents can preview without confirmation.
- Q: What JSON structure should `init --dry-run --json` output? → A: Switch init to use OpResult (same as mutation commands) with info-level diagnostics for created files.

## Interview

### Open Questions

_(none remaining)_

### Answer Log

- **Q1** (2026-03-10): Multiple-error exit code resolution strategy → **A: First-error-wins (positional)**
- **Q2** (2026-03-10): OPE009 dual meaning (I/O failure vs missing --yes) → **A: Missing --yes is Cobra required-flag error (exit 1); OPE009 stays I/O-only**
- **Q3** (2026-03-10): Dry-run + --yes interaction → **A: --dry-run bypasses --yes; no confirmation needed for previews**
- **Q4** (2026-03-10): Init dry-run JSON format → **A: Use OpResult with info-level diagnostics for created files**
