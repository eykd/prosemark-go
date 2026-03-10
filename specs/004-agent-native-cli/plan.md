# Implementation Plan: Agent-Native CLI Improvements

**Branch**: `004-agent-native-cli` | **Date**: 2026-03-10 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/004-agent-native-cli/spec.md`

## Summary

Add semantic exit codes, error recovery suggestions, dry-run support, and enriched help text to `pmk` so AI agents can programmatically classify failures, self-correct, preview mutations, and discover capabilities without external documentation.

The implementation spans four phases: (1) introduce `ExitError` type and diagnostic-to-exit-code mapping in `cmd/`, modify `main.go` to extract semantic codes; (2) add `Suggestion` field to `Diagnostic` with a code-to-suggestion mapping; (3) add `--dry-run` persistent flag that suppresses file writes while preserving validation; (4) enrich help text with exit code tables, state model docs, and examples.

## Technical Context

**Language/Version**: Go 1.25.6
**Primary Dependencies**: cobra v1.10.2, spf13/pflag v1.0.9
**Storage**: Filesystem (`_binder.md`, `.prosemark.yml`, `{uuid}.md` node files)
**Testing**: `go test` with 100% coverage (non-Impl), acceptance pipeline via `just acceptance`
**Target Platform**: Cross-platform CLI (Linux, macOS, Windows)
**Project Type**: Single Go module CLI application
**Performance Goals**: N/A (CLI tool, no latency-sensitive paths)
**Constraints**: No new external dependencies; backward-compatible JSON output (`omitempty`)
**Scale/Scope**: ~40 files in `cmd/`, ~15 files in `internal/`

## Constitution Check

_GATE: Must pass before Phase 0 research. Re-check after Phase 1 design._

| Principle | Status | Notes |
|-----------|--------|-------|
| I. ATDD | PASS | GWT specs defined in spec.md; acceptance pipeline will be used |
| I. Inner TDD | PASS | All new types and functions will follow Red-Green-Refactor |
| I. Coverage | PASS | 100% coverage maintained; no new Impl functions needed |
| II. Static Analysis | PASS | No new warnings expected; all errors handled explicitly |
| III. Code Quality | PASS | Standard Go naming, GoDoc on exports |
| IV. Pre-commit Gates | PASS | `just check` enforced; conventional commits |
| V. Warnings | PASS | No deferred warnings |
| VI. Go CLI Target | PASS | Semantic exit codes align with CLI conventions (VI: "Exit code 0 for success, non-zero for failure") |
| VII. Simplicity | PASS | Minimal new types; no speculative abstractions |

No violations. No complexity tracking needed.

## Project Structure

### Documentation (this feature)

```text
specs/004-agent-native-cli/
├── spec.md              # Feature specification
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
└── quickstart.md        # Phase 1 output
```

### Source Code (repository root)

```text
cmd/
├── exit_error.go        # NEW: ExitError type + ExitCodeForDiagnostics mapping
├── exit_error_test.go   # NEW: tests for exit code mapping
├── suggestions.go       # NEW: attachSuggestions function + code-to-suggestion map
├── suggestions_test.go  # NEW: tests for suggestion attachment
├── root.go              # MODIFIED: add --dry-run persistent flag, printDiagnostics shows suggestions
├── addchild.go          # MODIFIED: wrap errors in ExitError, honor --dry-run
├── delete.go            # MODIFIED: wrap errors in ExitError, honor --dry-run
├── move.go              # MODIFIED: wrap errors in ExitError, honor --dry-run
├── init.go              # MODIFIED: wrap errors in ExitError, honor --dry-run
├── parse.go             # MODIFIED: wrap errors in ExitError, accept --dry-run (no-op)
├── doctor.go            # MODIFIED: wrap errors in ExitError, accept --dry-run (no-op)
└── edit.go              # MODIFIED: wrap errors in ExitError (no dry-run needed)

internal/binder/
└── types.go             # MODIFIED: add Suggestion field to Diagnostic, DryRun field to OpResult

main.go                  # MODIFIED: extract ExitError via errors.As for semantic exit codes
```

## Phase 1: Semantic Exit Codes

### Design Decisions

1. **ExitError lives in `cmd/`**: It's a presentation-layer concern (translating domain diagnostics to process exit codes). No new package needed.

2. **ExitCodeForDiagnostics is a pure function**: Takes `[]binder.Diagnostic` → returns `int`. First error diagnostic's mapped code wins. Warnings-only → 0.

3. **Doctor command adapter**: Doctor uses `node.AuditDiagnostic` (different type). The doctor command already converts to `DoctorDiagnosticJSON` for output. We'll add a `ExitCodeForAuditDiagnostics([]node.AuditDiagnostic) int` function with its own mapping (AUD errors → 2).

4. **Cobra flag errors**: Cobra's built-in validation returns errors directly (not diagnostics). `main.go` will check for `ExitError` first; if absent, default to exit 1 (preserving current behavior for Cobra errors). Commands that detect conflicting flags (OPE010) already produce diagnostics and will map to exit 1.

### Exit Code Table

| Code | Meaning | Diagnostic Codes |
|------|---------|-------------------|
| 0 | Success | (none, or warnings only) |
| 1 | Usage error | OPE010, Cobra flag errors |
| 2 | Validation error | OPE004, OPE005, BNDE001, BNDE002, BNDE003, AUD* errors |
| 3 | Not found | OPE001, OPE007, OPE008 |
| 4 | (reserved) | — |
| 5 | Conflict | OPE002, OPE003, OPE006 |
| 6 | Transient/I/O | OPE009 |

### Key Types

```go
// cmd/exit_error.go
type ExitError struct {
    Code int
    Err  error
}
func (e *ExitError) Error() string { return e.Err.Error() }
func (e *ExitError) Unwrap() error { return e.Err }

func ExitCodeForDiagnostics(diags []binder.Diagnostic) int
func ExitCodeForAuditDiagnostics(diags []node.AuditDiagnostic) int
```

### main.go Changes

```go
if err := rootCmd.Execute(); err != nil {
    var exitErr *cmd.ExitError
    if errors.As(err, &exitErr) {
        fmt.Fprintln(os.Stderr, exitErr.Err)
        os.Exit(exitErr.Code)
    }
    fmt.Fprintln(os.Stderr, err)
    os.Exit(1)
}
```

## Phase 2: Error Suggestions

### Design Decisions

1. **Suggestion field on Diagnostic**: `Suggestion string \`json:"suggestion,omitempty"\`` — backward-compatible via omitempty.

2. **attachSuggestions is a pure function**: Takes `[]Diagnostic`, mutates in-place (or returns new slice). Called in each command's RunE after diagnostics are produced, before output.

3. **Doctor suggestions**: Doctor converts `AuditDiagnostic` to `DoctorDiagnosticJSON`. We'll add suggestion attachment at the conversion point using a separate `auditSuggestionMap`.

4. **Suggestion map**: A `map[string]string` keyed by diagnostic code. Unknown codes get no suggestion (field omitted from JSON).

### Suggestion Mapping

| Code | Suggestion |
|------|-----------|
| OPE001 | "Run 'pmk parse --json' to list available nodes and their selectors." |
| OPE002 | "Use a full path selector (e.g., 'parent/child.md') to disambiguate." |
| OPE003 | "The destination is a descendant of the source. Choose a different destination." |
| OPE004 | "Check that the target path contains only valid filename characters." |
| OPE005 | "The binder file cannot be added as a node. Choose a different target." |
| OPE006 | "The node is inside a code fence. Move it outside the fenced block." |
| OPE007 | "The sibling selector matched no nodes. Run 'pmk parse --json' to verify." |
| OPE008 | "The index is out of bounds. Run 'pmk parse --json' to check child count." |
| OPE009 | "Check that '_binder.md' exists and is readable. Run 'pmk doctor' to diagnose." |
| OPE010 | "Specify only one positioning flag: --first, --at, --before, or --after." |
| BNDE001 | "Remove illegal characters from the file path." |
| BNDE002 | "Paths must not escape the project root with '../'." |
| BNDE003 | "Use a full path instead of a wikilink to resolve the ambiguity." |
| AUD001 | "Create the referenced file or remove it from the binder." |
| AUD002 | "Add the orphaned file to the binder or delete it." |
| AUD003 | "Remove duplicate references from the binder." |
| AUD004 | "Rename the file to match its frontmatter ID, or update the ID." |
| AUD005 | "Add required frontmatter fields (id, title, created, updated)." |
| AUD007 | "Fix the YAML frontmatter syntax." |
| AUD008 | "Create or fix '.prosemark.yml'. Run 'pmk init' to generate one." |

### printDiagnostics Changes

Human mode adds a suggestion line:
```
error: selector matched no nodes (OPE001)
  suggestion: Run 'pmk parse --json' to list available nodes and their selectors.
```

## Phase 3: Dry-Run Support

### Design Decisions

1. **Persistent flag on root**: `--dry-run` as a persistent bool flag on the root command. All subcommands inherit it.

2. **DryRun field on OpResult**: `DryRun bool \`json:"dryRun,omitempty"\`` — backward-compatible.

3. **Suppression point**: Each mutation command checks `--dry-run` after computing the result but before writing files. The `OpResult` is populated normally except `Changed` is always `false` and `DryRun` is `true`.

4. **Human mode prefix**: When `--dry-run` is active, human-mode output lines are prefixed with `dry-run: ` (e.g., `dry-run: would delete chapter-3 from _binder.md`).

5. **Read-only commands**: `parse` and `doctor` accept `--dry-run` without error (persistent flag) but behave identically since they never write.

6. **Init command**: Reports what files would be created without creating them.

### Mutation Command Pattern

```go
// After computing modifiedBytes, diags:
dryRun, _ := cmd.Flags().GetBool("dry-run")
if dryRun {
    result.DryRun = true
    result.Changed = false
    // Skip all file writes
    // Output result normally
    return nil
}
// ... existing write logic ...
```

## Phase 4: Help Text Enrichment

### Design Decisions

1. **Root Long description**: Include exit code table and state model description in `cmd.Long`.

2. **Subcommand Examples**: Add `Example` field to each command with at least two examples.

3. **Dry-run documentation**: Commands supporting `--dry-run` document it in their help.

4. **No external templates**: Help text is inline strings in Go source (Cobra convention).

### Root Help Additions

- Exit codes table (0–6 with meanings)
- State model description (`.prosemark.yml`, `_binder.md`)
- Environment variables (`EDITOR`, `PMK_PROJECT`)

## Implementation Order

1. **Phase 1** (exit codes): `ExitError` type → mapping function → wire into commands → update `main.go`
2. **Phase 2** (suggestions): `Suggestion` field → mapping → `attachSuggestions` → update output
3. **Phase 3** (dry-run): `--dry-run` flag → `DryRun` field → suppress writes → human prefix
4. **Phase 4** (help): Root long desc → subcommand examples → dry-run docs

Phases 2 and 3 depend on Phase 1 (exit codes must be in place). Phases 2 and 3 are independent of each other. Phase 4 depends on Phases 1 and 3 (documents exit codes and --dry-run).

## Edge Cases & Error Handling

_Added by red team review (2026-03-10)._

### OPE009 Dual Meaning

OPE009 is currently used for both "I/O or parse failure" (mapped to exit code 6) AND "missing --yes confirmation" in delete/move commands. A missing `--yes` flag is a usage error, not a transient I/O failure. **Resolution**: Verify that the `--yes` check produces a distinct diagnostic code (or a Cobra-level error) that maps to exit code 1 (usage), not OPE009. If delete/move currently emit OPE009 for missing `--yes`, either: (a) introduce a new code like OPE011 for missing confirmation, mapped to exit 1; or (b) confirm this is handled at the Cobra required-flag level and never reaches the diagnostic mapper.

### Dry-Run + --yes Interaction

Mutation commands (`delete`, `move`) require `--yes` for non-interactive confirmation. When `--dry-run` is active, no changes are made, so requiring `--yes` adds unnecessary friction for agents previewing operations. **Resolution**: When `--dry-run` is set, `--yes` should NOT be required. The command should proceed through validation and produce the preview result without needing confirmation. Document this interaction in the `--dry-run` flag help text.

### Edit Command Inherits --dry-run

The `edit` command opens `$EDITOR` interactively. As a persistent flag on root, `--dry-run` is inherited by `edit`. The plan says "no dry-run needed" for edit, but doesn't specify what happens if a user passes `--dry-run` to `edit`. **Resolution**: `edit` should accept `--dry-run` silently (same as `parse`/`doctor`) since it doesn't perform destructive writes to the binder. The editor session is the user's intent, not a mutation pmk controls. No special handling needed beyond accepting the flag.

### Init Dry-Run Output Format

`init` currently prints ad-hoc messages rather than using `OpResult`. **Resolution**: Refactor `init` to use `OpResult` for output (same as mutation commands). Created files are reported as info-level diagnostics. In dry-run mode, `DryRun: true` and `Changed: false`; files that would be created appear as info-level diagnostics. This gives agents a single JSON schema across all commands.

### Suggestion Attachment Centralization

Each command's `RunE` must individually call `attachSuggestions` after producing diagnostics. If any command omits this call, suggestions silently don't appear — a regression risk as commands are added. **Resolution**: Rather than centralizing (which would add complexity), mitigate by: (1) adding a unit test per command that verifies suggestions appear on known diagnostic codes, and (2) documenting the `attachSuggestions` call as a required step in the command implementation pattern.

### Diagnostic Merge Order

"First error diagnostic's mapped exit code wins" depends on deterministic ordering of merged parse + operation diagnostics in `OpResult.Diagnostics`. **Resolution**: Document that parse diagnostics come first (they're produced first), followed by operation diagnostics. This is the natural order and matches the current `OpResult` construction pattern. Add a test that verifies ordering when both parse and operation errors are present.

### Unmapped Diagnostic Code Default

If a new diagnostic code is added in the future but not added to the exit code mapping, `ExitCodeForDiagnostics` would silently return 0 (success) for an error diagnostic — hiding the failure from agents. **Resolution**: The mapping function must return a sensible default for unmapped error-level diagnostics. Use exit code 1 (general error) as the fallback for any error diagnostic whose code is not in the mapping table. Add a test that verifies an unknown error diagnostic code produces exit code 1, not 0.

### Dry-Run Exit Code Consistency

The plan implies but does not explicitly state that exit codes work identically in dry-run mode. An agent relying on exit codes to decide whether a real run would succeed needs dry-run to produce the same exit code as a real run. **Resolution**: Explicitly document that `--dry-run` does not affect exit code computation. If validation produces errors, the same exit code is returned regardless of `--dry-run`. Add a test that verifies dry-run exit codes match real-run exit codes for the same invalid inputs.

### ExitError Survives Error Wrapping

`main.go` uses `errors.As(err, &exitErr)` to extract `ExitError`. If any command wraps its return error (e.g., `fmt.Errorf("addchild: %w", exitErr)`), `errors.As` must still find the `ExitError` through the wrapping chain. **Resolution**: (1) Ensure `ExitError` implements `Unwrap()` (already in the plan). (2) Commands should return `ExitError` directly as the outermost error, not wrap it further. (3) Add a test in `main.go` tests that verifies `errors.As` extracts `ExitError` from a wrapped error chain.

### Empty Diagnostics Edge Case

`ExitCodeForDiagnostics(nil)` and `ExitCodeForDiagnostics([]Diagnostic{})` must return 0. This is the "no errors" case. **Resolution**: Add explicit unit tests for nil and empty slice inputs returning exit code 0. This is likely the natural behavior of the implementation but should be verified.

### AuditDiagnostic Suggestion Path

Doctor uses `node.AuditDiagnostic` (a different type from `binder.Diagnostic`). The plan mentions a separate `auditSuggestionMap` and conversion to `DoctorDiagnosticJSON`. **Resolution**: Ensure the suggestion attachment happens at the right point in the doctor pipeline — after converting `AuditDiagnostic` to `DoctorDiagnosticJSON` but before serialization. The `DoctorDiagnosticJSON` type must also have a `Suggestion` field. Add a test that verifies doctor output includes suggestions for known AUD codes.

## Security Considerations

_Added by red team review (2026-03-10)._

### Error Message Content

Suggestion strings include specific `pmk` commands (e.g., `Run 'pmk parse --json'`). For a local CLI tool, this is safe and desirable — suggestions are recovery hints, not secrets. No information leakage concern since the tool operates on local files the user already has access to.

### Dry-Run Validation Integrity

`--dry-run` must NOT bypass validation or diagnostic computation. The plan correctly specifies that "all validation and diagnostic computation MUST still execute fully" (FR-014). This is critical: if dry-run skipped validation, an agent could use dry-run to bypass safety checks and then assume the operation is safe. **Verify**: Unit tests must confirm that dry-run produces identical diagnostics to a real run for the same inputs.

### Dry-Run as Scripted Oracle

An agent might use `--dry-run` in a loop to probe many operations rapidly (e.g., testing every possible selector). Since dry-run still reads the binder file from disk on each invocation, this is bounded by filesystem I/O and process startup time — acceptable for a CLI tool. No mitigation needed, but worth noting that dry-run has no cheaper cost path than a real run (minus the write).

## Post-Design Constitution Re-Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. ATDD | PASS | Acceptance scenarios defined; will generate GWT specs |
| I. Inner TDD | PASS | New types (ExitError) and functions (ExitCodeForDiagnostics, attachSuggestions) are pure and trivially testable |
| I. Coverage | PASS | No new Impl functions; all new code is testable |
| II. Static Analysis | PASS | No unsafe patterns |
| III. Code Quality | PASS | GoDoc on all new exports |
| IV. Pre-commit | PASS | All gates apply |
| V. Warnings | PASS | No deferred work |
| VI. Go CLI | PASS | Semantic exit codes are a CLI best practice |
| VII. Simplicity | PASS | Minimal new types; maps for lookups; no abstractions beyond what's needed |
