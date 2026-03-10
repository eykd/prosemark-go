# Research: Agent-Native CLI Improvements

**Feature**: 004-agent-native-cli | **Date**: 2026-03-10

## R1: Semantic Exit Code Conventions for CLI Tools

**Decision**: Use a custom exit code table: 0 (success), 1 (usage), 2 (validation), 3 (not found), 5 (conflict), 6 (transient/I/O). Code 4 reserved for future permission-denied.

**Rationale**: POSIX reserves only 0 (success) and 1 (general error). BSD `sysexits.h` defines 64–78 but is not widely adopted by modern CLI tools. Tools like `curl`, `git`, and `docker` define their own semantic ranges. Since `pmk` targets AI agent consumers, a small, well-documented set of codes (0–6) is more useful than the `sysexits.h` range. The codes are compact enough to include in `--help` output.

**Alternatives considered**:
- `sysexits.h` (64–78): Too many codes, not intuitive for agents, no "not found" vs "conflict" distinction
- Single non-zero (current): Insufficient for agent error classification
- stderr-only signaling: Agents prefer structured exit codes over parsing text

## R2: ExitError Type Design

**Decision**: Simple struct in `cmd/` package with `Code int` and `Err error`. Implements `error` and `Unwrap()`.

**Rationale**: The ExitError is a presentation-layer concern — it translates domain diagnostics into process exit codes. Placing it in `cmd/` follows the consumer-defines-interface principle. `errors.As` in `main.go` extracts the code.

**Alternatives considered**:
- Sentinel error values: Can't carry dynamic context
- New `exitcode` package: Over-engineering for a single type
- Returning exit code from `Execute()`: Would require forking Cobra's API

## R3: Doctor Command Diagnostic Adaptation

**Decision**: Create a separate `ExitCodeForAuditDiagnostics([]node.AuditDiagnostic) int` function rather than converting `AuditDiagnostic` to `binder.Diagnostic`.

**Rationale**: `AuditDiagnostic` uses `node.AuditCode` and `node.AuditSeverity` (both custom string types), while `binder.Diagnostic` uses plain strings. Converting between them would create a leaky abstraction. A parallel mapping function (same pattern, different input type) is simpler and more honest.

**Alternatives considered**:
- Shared `Diagnostic` interface: Would couple `node` and `binder` packages
- Converting AuditDiagnostic → Diagnostic: Lossy (AuditDiagnostic has `Path` field)
- Single generic function: Go generics would add complexity for two concrete types

## R4: Suggestion Attachment Strategy

**Decision**: `attachSuggestions` mutates diagnostics in-place using a `map[string]string` lookup by diagnostic code.

**Rationale**: Suggestions are a presentation concern (they reference CLI commands like `pmk parse --json`). The mapping is static and small (~15 entries). In-place mutation avoids allocation. The function is pure (deterministic, no side effects beyond mutation) and trivially testable.

**Alternatives considered**:
- Attach at diagnostic creation site: Scatters suggestion logic across packages; couples domain to CLI presentation
- Return new slice: Unnecessary allocation for a simple field set
- Suggestion as separate output: Would complicate JSON schema

## R5: Dry-Run Implementation Strategy

**Decision**: `--dry-run` as persistent flag on root. Mutation commands check it after computing results but before writing. `OpResult.DryRun` set to `true`; `Changed` forced to `false`.

**Rationale**: Persistent flag ensures all subcommands inherit it without per-command wiring. Checking after computation means all validation and diagnostics still fire (FR-014). Setting `Changed = false` is semantically correct: no files were changed. The `DryRun` field distinguishes "nothing to change" from "would change but didn't."

**Alternatives considered**:
- Per-command flag: Repetitive, easy to forget on new commands
- Middleware/interceptor pattern: Over-engineering; Go doesn't have natural middleware for Cobra
- No-op IO implementation: Would require swapping IO interfaces, complex testing
- Skip computation entirely: Would miss validation errors (violates FR-014)

## R6: Help Text Structure

**Decision**: Inline strings in Go source using Cobra's `Long` and `Example` fields. Exit code table as aligned text.

**Rationale**: Cobra renders `Long` and `Example` directly. No template engine needed. Aligned text tables render well in terminals. Examples use the standard Cobra format (indented, one per line with comment).

**Alternatives considered**:
- External help files: Adds runtime file dependency
- Go templates: Over-engineering for static text
- Separate `--exit-codes` flag: Agents should get codes from `--help`
