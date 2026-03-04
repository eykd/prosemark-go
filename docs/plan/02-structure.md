# 02 — Structure Command

## Summary

Add `pmk structure` — a read-only command that displays the binder hierarchy as a human-readable Markdown outline. Titles are shown without link syntax; an optional ATX heading is extracted from the binder source. Placeholder nodes are hidden by default and shown with a `--placeholders` flag.

**PRD Reference**: Section 6.8 — structure
**Dependency**: Feature 01 (placeholder-parsing) for `--placeholders` flag behavior

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Output format | Markdown list with two-space indent | Matches PRD example output exactly |
| Heading extraction | Scan binder source for first ATX heading (`# ...`) | PRD example shows heading; simple regex extraction |
| No `--json` flag | Omit | `pmk parse` already provides machine-readable JSON output |
| Placeholder display | Hidden by default; shown with `--placeholders` | Avoids clutter for users who haven't materialized all placeholders |
| Placeholder marker | `(placeholder)` suffix | Clear, unambiguous, grep-friendly |
| Empty title fallback | Use target filename stem for file nodes; empty string for placeholders | Consistent with parser title derivation |
| Domain/CLI split | `RenderStructure()` in domain, thin CLI wrapper | Testable without Cobra; follows existing patterns |

## Package Structure

### New Files

| File | Package | Purpose |
|------|---------|---------|
| `internal/binder/structure.go` | `binder` | `RenderStructure()`, `ExtractHeading()`, `StructureOptions` |
| `internal/binder/structure_test.go` | `binder` | Domain-layer tests |
| `cmd/structure.go` | `cmd` | Cobra command, `ParseReader` reuse |
| `cmd/structure_test.go` | `cmd` | Command-layer tests |

### Modified Files

| File | Change |
|------|--------|
| `cmd/root.go` | `root.AddCommand(NewStructureCmd(newDefaultParseReader()))` |

## Key Types and Signatures

### `internal/binder/structure.go`

```go
// StructureOptions controls rendering behavior.
type StructureOptions struct {
    ShowPlaceholders bool // include placeholder nodes in output
}

// RenderStructure renders the binder tree as a human-readable Markdown outline.
// Returns a string with two-space-indented list items showing node titles.
func RenderStructure(result *ParseResult, opts StructureOptions) string

// ExtractHeading scans binder source lines for the first ATX heading (# ...).
// Returns the heading text (without the # prefix) or "" if none found.
func ExtractHeading(lines []string) string
```

### `cmd/structure.go`

```go
// NewStructureCmd creates the structure subcommand.
// Reuses ParseReader interface (same as pmk parse).
func NewStructureCmd(pr ParseReader) *cobra.Command
```

## Output Format

Given this binder:

```markdown
# Salvage of Empire

- [Part I](part1.md)
  - [Chapter 1](chapter1.md)
  - [Chapter 2](chapter2.md)
- [Part II](part2.md)
  - [Chapter 3]()
```

Default output (`pmk structure`):

```
# Salvage of Empire

- Part I
  - Chapter 1
  - Chapter 2
- Part II
```

With `--placeholders` (`pmk structure --placeholders`):

```
# Salvage of Empire

- Part I
  - Chapter 1
  - Chapter 2
- Part II
  - Chapter 3 (placeholder)
```

### Rendering Rules

1. If `ExtractHeading()` returns non-empty, output `# <heading>\n\n` first.
2. Walk `Root.Children` depth-first pre-order.
3. For each node:
   - If `Target == ""` (placeholder) and `!opts.ShowPlaceholders`: skip.
   - Indent: `strings.Repeat("  ", depth)` (two spaces per nesting level, depth starts at 0).
   - Title: use `node.Title`; if empty and `Target != ""`, use `stemFromPath(Target)`.
   - If placeholder: append ` (placeholder)` suffix.
   - Output: `<indent>- <title>\n`
4. Return accumulated string (no trailing newline beyond last item).

## CLI Flags and Registration

```go
cmd.Flags().String("project", "", "project directory containing _binder.md (default: current directory)")
cmd.Flags().Bool("placeholders", false, "include placeholder nodes in output")
```

**Registration** in `cmd/root.go`:

```go
root.AddCommand(NewStructureCmd(newDefaultParseReader()))
```

## Command Flow

1. Resolve binder path from `--project` flag (reuse `resolveBinderPath`).
2. Read binder via `ParseReader.ReadBinder()`.
3. Scan project via `ParseReader.ScanProject()`.
4. Parse binder via `binder.Parse()`.
5. Extract heading via `binder.ExtractHeading(result.Lines)`.
6. Render structure via `binder.RenderStructure(result, opts)`.
7. Write to `cmd.OutOrStdout()`.

## Test Strategy

### Domain Tests (`internal/binder/structure_test.go`)

| # | Test Name | Scenario | Expected |
|---|-----------|----------|----------|
| 1 | `TestRenderStructure_EmptyTree` | Root with no children | Empty string |
| 2 | `TestRenderStructure_SingleNode` | One top-level node | `- Title\n` |
| 3 | `TestRenderStructure_FlatList` | Three top-level nodes | Three `- Title\n` lines |
| 4 | `TestRenderStructure_Nested` | Two levels of nesting | Correct two-space indentation |
| 5 | `TestRenderStructure_DeepNesting` | Three+ levels | Correct indentation (4, 6 spaces) |
| 6 | `TestRenderStructure_EmptyTitleFallback` | Node with empty title, non-empty target | Shows filename stem |
| 7 | `TestRenderStructure_PlaceholdersHidden` | Tree with placeholders, default opts | Placeholders omitted |
| 8 | `TestRenderStructure_PlaceholdersShown` | Tree with placeholders, `ShowPlaceholders: true` | Shown with `(placeholder)` suffix |
| 9 | `TestRenderStructure_PlaceholderEmptyTitle` | Placeholder with empty title, shown | Shows `(placeholder)` only |
| 10 | `TestRenderStructure_MixedPlaceholdersAndNodes` | Interleaved types | Correct filtering/display |
| 11 | `TestExtractHeading_Found` | `# My Book` in lines | Returns `"My Book"` |
| 12 | `TestExtractHeading_NotFound` | No ATX heading | Returns `""` |
| 13 | `TestExtractHeading_MultipleHeadings` | Two `#` lines | Returns first only |
| 14 | `TestExtractHeading_HeadingInFence` | `# Title` inside code fence | Skipped; returns `""` or next heading |

### Command Tests (`cmd/structure_test.go`)

| # | Test Name | Scenario | Expected |
|---|-----------|----------|----------|
| 1 | `TestStructureCmd_BasicOutput` | Valid binder with heading | Heading + structure in stdout |
| 2 | `TestStructureCmd_NoHeading` | Binder without heading | Structure only, no heading line |
| 3 | `TestStructureCmd_PlaceholdersHidden` | Default (no flag) | Placeholders omitted |
| 4 | `TestStructureCmd_PlaceholdersShown` | `--placeholders` flag | Placeholders visible |
| 5 | `TestStructureCmd_BinderNotFound` | Missing `_binder.md` | Error message about `pmk init` |
| 6 | `TestStructureCmd_ParseError` | Invalid binder content | Error propagation |

Mock pattern: reuse `ParseReader` interface with a mock struct, same pattern as `cmd/parse_test.go`.

## Error Handling

| Scenario | Behavior | Exit Code |
|----------|----------|-----------|
| Binder not found | `"project not initialized — run 'pmk init' first"` | 1 |
| Parse error | Propagate error message | 1 |
| Empty binder (no nodes) | Output heading only (or nothing) | 0 |
| Binder with parse warnings | Ignore warnings; render structure | 0 |

## Implementation Steps (TDD Order)

1. **Red**: Write `TestExtractHeading_Found` and `TestExtractHeading_NotFound`. Fails (function doesn't exist).
2. **Green**: Implement `ExtractHeading()` with ATX heading regex. Tests pass.
3. **Red**: Write `TestRenderStructure_EmptyTree` and `TestRenderStructure_SingleNode`. Fails.
4. **Green**: Implement `RenderStructure()` with basic depth-first walk. Tests pass.
5. **Red**: Write nesting tests (`Nested`, `DeepNesting`). Should pass with recursive walk.
6. **Red**: Write `TestRenderStructure_EmptyTitleFallback`. May need stem fallback logic.
7. **Green**: Add stem fallback in render loop.
8. **Red**: Write placeholder tests (`PlaceholdersHidden`, `PlaceholdersShown`).
9. **Green**: Add `StructureOptions` and placeholder filtering/suffix logic.
10. **Refactor**: Clean up domain layer.
11. **Red**: Write `TestStructureCmd_BasicOutput`.
12. **Green**: Implement `cmd/structure.go` wiring.
13. **Red**: Write remaining cmd tests.
14. **Green**: Register in `cmd/root.go`. Run `just check`.

## Critical Files

| File | Action |
|------|--------|
| `internal/binder/structure.go` | Create |
| `internal/binder/structure_test.go` | Create |
| `cmd/structure.go` | Create |
| `cmd/structure_test.go` | Create |
| `cmd/root.go` | Add registration |
