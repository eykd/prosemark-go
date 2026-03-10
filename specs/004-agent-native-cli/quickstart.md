# Quickstart: Agent-Native CLI Improvements

**Feature**: 004-agent-native-cli | **Date**: 2026-03-10

## Prerequisites

- Go 1.25.6+
- `lefthook install` (git hooks)
- `just install-tools` (beads CLI)

## Development Workflow

### Phase 1: Semantic Exit Codes

1. Create `cmd/exit_error.go` with `ExitError` type
2. Create `ExitCodeForDiagnostics` mapping function
3. Create `ExitCodeForAuditDiagnostics` for doctor command
4. Wire into each command's RunE (wrap final error in `ExitError`)
5. Update `main.go` to extract `ExitError` via `errors.As`
6. Run: `just test && just check`

### Phase 2: Error Suggestions

1. Add `Suggestion` field to `binder.Diagnostic`
2. Create `cmd/suggestions.go` with `attachSuggestions` and suggestion map
3. Call `attachSuggestions` in each command's RunE after getting diagnostics
4. Update `printDiagnostics` to print suggestions
5. Create doctor suggestion mapping for `AuditDiagnostic` codes
6. Run: `just test && just check`

### Phase 3: Dry-Run Support

1. Add `--dry-run` persistent flag to root command
2. Add `DryRun` field to `binder.OpResult`
3. In each mutation command: check flag, skip writes, set `DryRun=true`
4. Add `dry-run:` prefix to human-mode output
5. Handle `init --dry-run` (report without creating)
6. Run: `just test && just check`

### Phase 4: Help Text

1. Add exit code table to root `Long` description
2. Add state model description to root `Long`
3. Add `Example` fields to all subcommands
4. Document `--dry-run` in relevant subcommand help
5. Run: `just test && just check`

## Testing Commands

```bash
just test              # Unit tests with coverage
just test-cover        # Coverage report
just check             # All quality gates
just acceptance        # Acceptance tests
just test-all          # Unit + acceptance
just smoke             # CLI smoke test
```

## Key Files to Read First

1. `cmd/root.go` — root command setup, printDiagnostics, flag handling
2. `internal/binder/types.go` — Diagnostic, OpResult, diagnostic code constants
3. `internal/node/types.go` — AuditDiagnostic, AuditCode
4. `main.go` — current error handling (where ExitError extraction goes)
5. `cmd/delete.go` — representative mutation command pattern
