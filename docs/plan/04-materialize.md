# 04 — Materialize Command

## Summary

Add `pmk materialize <placeholder-title>` — converts a placeholder binder entry into a real node. The command finds a placeholder node by title, generates a UUIDv7 filename, updates the link target in the binder, creates draft and notes files, and writes the binder atomically.

**PRD Reference**: Section 6.5 — materialize
**Dependency**: Feature 01 (placeholder-parsing) — placeholders must be recognized as nodes in the parse tree

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Selector strategy | Match by title only (case-insensitive) | Placeholders have no target path; title is the only identifier |
| In-place update | Regex replace `]()` → `](<target>)` on `RawLine` | Preserves list markers, indentation, surrounding content |
| File creation | Same pattern as `pmk add --new` | Consistent UX: frontmatter, notes file, backlink |
| Rollback | Delete created files if binder write fails | Same pattern as `addchild.go` |
| New diagnostic code | `OPE011` — "node is not a placeholder" | Distinct from OPE001 (no match); user selected a real node |
| Multiple matches | Error (OPE002-like) | Ambiguous; user must disambiguate via binder editing |
| Title override | `--title` flag changes the bracket text | Allows renaming during materialization |
| Notes file creation | Use `WriteNodeFileAtomic` (not `CreateNotesFile`) | `EditIO.CreateNotesFile` creates empty files; materialize needs backlink content. `add --new` doesn't create notes files at all, so materialize is the first command to create draft + notes in one operation. |

## Package Structure

### New Files

| File | Package | Purpose |
|------|---------|---------|
| `internal/binder/ops/materialize.go` | `ops` | `Materialize()` domain operation |
| `internal/binder/ops/materialize_test.go` | `ops` | Domain-layer tests |
| `cmd/materialize.go` | `cmd` | Cobra command, `MaterializeIO` interface |
| `cmd/materialize_test.go` | `cmd` | Command-layer tests |

### Modified Files

| File | Change |
|------|--------|
| `internal/binder/types.go` | Add `MaterializeParams`, `CodeNotAPlaceholder` |
| `cmd/root.go` | `root.AddCommand(NewMaterializeCmd(newDefaultMaterializeIO()))` |

## Key Types and Signatures

### `internal/binder/types.go` — Additions

```go
// MaterializeParams holds parameters for the materialize operation.
type MaterializeParams struct {
    Selector string // placeholder title to match (case-insensitive)
    Target   string // generated filename (e.g., "uuid.md")
    Title    string // override title (empty = keep original)
}

// Operation error for materialize.
const CodeNotAPlaceholder = "OPE011" // selector matched a node that is not a placeholder
```

### `internal/binder/ops/materialize.go`

```go
// Materialize converts a placeholder node into a file-backed node.
// It finds the placeholder by title, updates the link target in the binder source,
// and returns the modified binder bytes along with diagnostics.
//
// The caller is responsible for creating the actual draft and notes files.
// This separation keeps the domain layer free of filesystem operations.
func Materialize(
    ctx context.Context,
    src []byte,
    project *binder.Project,
    params binder.MaterializeParams,
) ([]byte, []binder.Diagnostic)
```

### `cmd/materialize.go`

```go
// MaterializeIO handles I/O for the materialize command.
// Note: uses WriteNodeFileAtomic for both draft and notes file creation,
// matching the newNodeIO pattern from addchild.go. The existing EditIO.CreateNotesFile
// creates empty files; materialize needs to write backlink content, so it uses
// WriteNodeFileAtomic instead.
type MaterializeIO interface {
    ReadBinder(ctx context.Context, path string) ([]byte, error)
    ScanProject(ctx context.Context, binderPath string) (*binder.Project, error)
    WriteBinderAtomic(ctx context.Context, path string, data []byte) error
    WriteNodeFileAtomic(path string, content []byte) error
    DeleteFile(path string) error
    OpenEditor(editor, path string) error
    ReadNodeFile(path string) ([]byte, error)
}

// NewMaterializeCmd creates the materialize subcommand.
func NewMaterializeCmd(io MaterializeIO) *cobra.Command
```

## Domain Operation: `Materialize()`

### Algorithm

1. **Parse** the binder source via `parseBinderFn()`.
2. **Find placeholder** by scanning all nodes in the parse tree:
   - Match: `node.Target == ""` AND `strings.EqualFold(node.Title, params.Selector)`
   - Collect all matches.
3. **Validate matches**:
   - Zero matches: return `OPE001` (selector no match).
   - Multiple matches: return `OPE002` (ambiguous).
   - Single match but `Target != ""`: return `OPE011` (not a placeholder).
4. **Update the raw line**:
   - Find `]()` in `node.RawLine` and replace with `](<params.Target>)`.
   - If `params.Title != ""` (title override): also replace bracket text.
   - Update `result.Lines[node.Line-1]` with the modified line.
5. **Serialize** via `binder.Serialize(result)`.
6. Return modified bytes and collected diagnostics.

### In-Place Line Update

```go
// Replace empty target with generated filename.
// Before: "- [Chapter 3]()"
// After:  "- [Chapter 3](uuid.md)"
oldLine := result.Lines[match.Line-1]
newLine := strings.Replace(oldLine, "]()", "]("+params.Target+")", 1)

// Optional title override:
if params.Title != "" {
    escapedTitle := escapeTitle(params.Title)
    // Replace [OldTitle]( with [NewTitle](
    oldBracket := "[" + escapeTitle(match.Title) + "]("
    newBracket := "[" + escapedTitle + "]("
    newLine = strings.Replace(newLine, oldBracket, newBracket, 1)
}

result.Lines[match.Line-1] = newLine
```

### Placeholder Matching

The selector system (`EvalSelector`) already supports title matching via `strings.EqualFold(n.Title, fileRef)`. However, for `materialize` we need an additional constraint: the matched node must be a placeholder (`Target == ""`). Rather than modifying the generic selector, the `Materialize` function does its own targeted search:

```go
func findPlaceholders(root *binder.Node, title string) []*binder.Node {
    var matches []*binder.Node
    var walk func(n *binder.Node)
    walk = func(n *binder.Node) {
        if n.Type == "node" && strings.EqualFold(n.Title, title) {
            matches = append(matches, n)
        }
        for _, child := range n.Children {
            walk(child)
        }
    }
    walk(root)
    return matches
}
```

Then validate:

```go
matches := findPlaceholders(result.Root, params.Selector)
if len(matches) == 0 {
    return src, []binder.Diagnostic{{Severity: "error", Code: binder.CodeSelectorNoMatch, ...}}
}
if len(matches) > 1 {
    return src, []binder.Diagnostic{{Severity: "error", Code: binder.CodeAmbiguousBareStem, ...}}
}
match := matches[0]
if match.Target != "" {
    return src, []binder.Diagnostic{{Severity: "error", Code: binder.CodeNotAPlaceholder, ...}}
}
```

## CLI Command Flow

1. Resolve binder path from `--project`.
2. Read binder via `io.ReadBinder()`.
3. Scan project via `io.ScanProject()`.
4. Generate UUID target: `id, _ := nodeIDGenerator()` → `target = id + ".md"`.
5. Build `MaterializeParams{Selector: args[0], Target: target, Title: titleFlag}`.
6. Call `ops.Materialize()` → get modified bytes + diagnostics.
7. If diagnostics have errors: print diagnostics, return error.
8. Create draft file with frontmatter (same pattern as `addchild.go --new`):
   ```go
   fm := node.Frontmatter{ID: stem, Title: title, Synopsis: synopsis, Created: now, Updated: now}
   content := node.SerializeFrontmatter(fm)
   io.WriteNodeFileAtomic(draftPath, content)
   ```
9. Create notes file with backlink: `io.WriteNodeFileAtomic(notesPath, []byte("[["+stem+"]]\n"))`.
10. Write binder atomically: `io.WriteBinderAtomic()`.
11. If binder write fails: rollback — delete draft and notes files.
12. If `--edit` flag: open draft in `$EDITOR`, refresh `updated` timestamp.
13. Output result (JSON or human-readable).

## CLI Flags

```go
cmd.Flags().String("project", "", "project directory containing _binder.md (default: current directory)")
cmd.Flags().String("title", "", "override the placeholder title")
cmd.Flags().String("synopsis", "", "set synopsis in frontmatter (max 2000 chars)")
cmd.Flags().Bool("edit", false, "open the new draft in $EDITOR after materialization")
cmd.Flags().Bool("json", false, "output result as JSON")
```

**Positional argument**: `<placeholder-title>` — the title text to match.

```go
Use:  "materialize <placeholder-title>",
Args: cobra.ExactArgs(1),
```

## Test Strategy

### Domain Tests (`internal/binder/ops/materialize_test.go`)

| # | Test Name | Input | Expected |
|---|-----------|-------|----------|
| 1 | `TestMaterialize_HappyPath` | `[Chapter 3]()` + selector "Chapter 3" | Target replaced, title preserved |
| 2 | `TestMaterialize_NoMatch` | Selector "nonexistent" | OPE001, src unchanged |
| 3 | `TestMaterialize_NotAPlaceholder` | Selector matches `[Ch](ch.md)` | OPE011, src unchanged |
| 4 | `TestMaterialize_MultipleMatches` | Two `[Chapter 3]()` entries | OPE002, src unchanged |
| 5 | `TestMaterialize_CaseInsensitive` | `[chapter 3]()` + selector "Chapter 3" | Match found |
| 6 | `TestMaterialize_TitleOverride` | `--title "The Finale"` | Both bracket text and target updated |
| 7 | `TestMaterialize_DifferentListMarkers` | `* [Ch]()`, `+ [Ch2]()` | Correct replacement |
| 8 | `TestMaterialize_Nested` | Placeholder nested under file node | Correct line updated |
| 9 | `TestMaterialize_EscapedBrackets` | `[Ch \[3\]]()` | Brackets handled correctly |
| 10 | `TestMaterialize_PreservesOtherLines` | Multi-node binder | Only placeholder line changed |

### Command Tests (`cmd/materialize_test.go`)

| # | Test Name | Scenario | Expected |
|---|-----------|----------|----------|
| 1 | `TestMaterializeCmd_HappyPath` | Valid placeholder | Files created, binder updated |
| 2 | `TestMaterializeCmd_JSONOutput` | `--json` flag | OpResult JSON on stdout |
| 3 | `TestMaterializeCmd_NoMatch` | Bad title | Error exit, diagnostics on stderr |
| 4 | `TestMaterializeCmd_NotAPlaceholder` | Title matches real node | OPE011 error |
| 5 | `TestMaterializeCmd_BinderWriteFail_Rollback` | Write error | Draft and notes files deleted |
| 6 | `TestMaterializeCmd_EditFlag` | `--edit` | Editor opened with draft path |
| 7 | `TestMaterializeCmd_TitleOverride` | `--title "New Name"` | Bracket text updated |
| 8 | `TestMaterializeCmd_SynopsisFlag` | `--synopsis "..."` | Frontmatter includes synopsis |

## Error Handling Matrix

| Code | Severity | Condition | Behavior |
|------|----------|-----------|----------|
| OPE001 | error | No placeholder matches selector | Abort; return src unchanged |
| OPE002 | error | Multiple placeholders match | Abort; return src unchanged |
| OPE009 | error | I/O or parse failure | Abort; return src unchanged |
| OPE011 | error | Matched node is not a placeholder | Abort; return src unchanged |
| (none) | — | Binder write fails | Rollback: delete created files |
| (none) | — | Editor fails | Non-fatal; files remain |

## Implementation Steps (TDD Order)

1. **Red**: Add `MaterializeParams` and `CodeNotAPlaceholder` to `types.go`. Write `TestMaterialize_HappyPath`. Fails.
2. **Green**: Implement `Materialize()` with placeholder search, line update, serialize. Test passes.
3. **Red**: Write `TestMaterialize_NoMatch`. Should pass (OPE001 from empty match set).
4. **Red**: Write `TestMaterialize_NotAPlaceholder`. May need OPE011 check.
5. **Green**: Add `Target != ""` guard. Test passes.
6. **Red**: Write `TestMaterialize_MultipleMatches`. Should pass (OPE002).
7. **Red**: Write `TestMaterialize_TitleOverride`. Implement title replacement.
8. **Green**: Add bracket-text replacement logic. Test passes.
9. **Red**: Write remaining domain tests. All should pass.
10. **Refactor**: Clean up domain layer.
11. **Red**: Write `TestMaterializeCmd_HappyPath`. Fails.
12. **Green**: Implement `cmd/materialize.go`. Test passes.
13. **Red**: Write remaining cmd tests (rollback, flags, JSON).
14. **Green**: Implement rollback, flag handling. Tests pass.
15. **Register**: Add to `cmd/root.go`. Run `just check`.

## Critical Files

| File | Action |
|------|--------|
| `internal/binder/types.go` | Add `MaterializeParams`, `CodeNotAPlaceholder` |
| `internal/binder/ops/materialize.go` | Create |
| `internal/binder/ops/materialize_test.go` | Create |
| `cmd/materialize.go` | Create |
| `cmd/materialize_test.go` | Create |
| `cmd/root.go` | Add registration |
