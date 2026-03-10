# Data Model: Agent-Native CLI Improvements

**Feature**: 004-agent-native-cli | **Date**: 2026-03-10

## Modified Entities

### Diagnostic (internal/binder/types.go)

**Current fields**: Severity, Code, Message, Location

**Added field**:
| Field | Type | JSON Tag | Description |
|-------|------|----------|-------------|
| Suggestion | string | `json:"suggestion,omitempty"` | Actionable recovery hint for agents |

**Validation**: None (populated by `attachSuggestions`, omitted if empty)

### OpResult (internal/binder/types.go)

**Current fields**: Version, Changed, Diagnostics

**Added field**:
| Field | Type | JSON Tag | Description |
|-------|------|----------|-------------|
| DryRun | bool | `json:"dryRun,omitempty"` | True when command ran in dry-run mode |

**Validation**: None (set by command layer)

**State transitions**:
- Normal execution: `DryRun=false, Changed=true/false` (existing behavior)
- Dry-run execution: `DryRun=true, Changed=false` (new)
- Error execution: `DryRun=true/false, Changed=false` (errors prevent changes)

## New Entities

### ExitError (cmd/exit_error.go)

| Field | Type | Description |
|-------|------|-------------|
| Code | int | Semantic exit code (0–6) |
| Err | error | Underlying error |

**Methods**:
- `Error() string` — delegates to `Err.Error()`
- `Unwrap() error` — returns `Err` (supports `errors.As`/`errors.Is`)

**Not persisted**; exists only in the error return chain from command → main.

### Exit Code Mapping (cmd/exit_error.go)

Static map from diagnostic code to exit code integer:

```
OPE010              → 1 (usage)
OPE004, OPE005      → 2 (validation)
BNDE001–003         → 2 (validation)
AUD*                → 2 (validation)
OPE001, OPE007, OPE008 → 3 (not found)
OPE002, OPE003, OPE006 → 5 (conflict)
OPE009              → 6 (transient/I/O)
```

Unmapped codes default to exit 1.

### Suggestion Mapping (cmd/suggestions.go)

Static `map[string]string` from diagnostic code to suggestion text. See plan.md Phase 2 for the full table.

## Relationships

```
main.go
  └─ errors.As(*ExitError) → exit code
       └─ ExitError.Code ← ExitCodeForDiagnostics([]Diagnostic)
                              └─ exitCodeMap[diagnostic.Code]

Command RunE
  ├─ ops.AddChild/Delete/Move → []Diagnostic
  │     └─ attachSuggestions(diags) → diags[i].Suggestion populated
  │     └─ ExitCodeForDiagnostics(diags) → ExitError.Code
  ├─ --dry-run check → OpResult.DryRun = true, skip writes
  └─ return &ExitError{Code, err}

Doctor RunE
  ├─ node.RunDoctor → []AuditDiagnostic
  │     └─ ExitCodeForAuditDiagnostics(diags) → ExitError.Code
  └─ return &ExitError{Code, err}
```

## JSON Schema Impact

### OpResult (docs/conformance/v1/schema/op-result.json)

Add optional properties:
- `dryRun`: `{"type": "boolean"}` — present only when true

### Diagnostic (within op-result and parse-result schemas)

Add optional property:
- `suggestion`: `{"type": "string"}` — present only when non-empty

Both additions are backward-compatible due to `omitempty` semantics.
