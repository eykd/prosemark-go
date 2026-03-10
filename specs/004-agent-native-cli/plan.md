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
