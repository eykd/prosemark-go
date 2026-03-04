# 05 — Compile Command

## Summary

Add `pmk compile` — concatenates node draft files in binder order to stdout. The command strips YAML frontmatter from each file, joins bodies with `\n\n` separators, and skips placeholder nodes (nodes with `Target == ""`). Missing files are skipped with a warning diagnostic.

**PRD Reference**: Section 6.11 — compile
**Dependency**: Feature 01 (placeholder-parsing) for skipping placeholder nodes cleanly

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Output target | stdout only | PRD specifies stdout; users redirect with `>` |
| Frontmatter stripping | Use `node.ParseFrontmatter()` | Existing, tested function; handles edge cases |
| No-frontmatter files | Use entire content as body | Gracefully handles human-named nodes without frontmatter |
| Separator | `\n\n` between bodies | PRD specifies double newline; clean paragraph break |
| Trailing newline | Single `\n` at end | Standard text file convention |
| Placeholder handling | Skip silently (no diagnostic) | Placeholders are expected structural elements |
| Missing files | Skip with warning diagnostic | Non-fatal; allows partial compilation |
| Empty bodies | Skipped (no separator emitted) | Avoids extra blank lines from empty drafts |
| Traversal order | Depth-first pre-order | Same as `collect_binder_refs.go`; matches PRD "binder order" |
| No `--output` flag | Omit | Shell redirection is sufficient; avoids flag complexity |

## Package Structure

### New Files

| File | Package | Purpose |
|------|---------|---------|
| `internal/binder/compile.go` | `binder` | `CompileNodes()` domain function |
| `internal/binder/compile_test.go` | `binder` | Domain-layer tests |
| `cmd/compile.go` | `cmd` | Cobra command, `CompileIO` interface |
| `cmd/compile_test.go` | `cmd` | Command-layer tests |

### Modified Files

| File | Change |
|------|--------|
| `cmd/root.go` | `root.AddCommand(NewCompileCmd(fileCompileIO{}))` |

## Key Types and Signatures

### `internal/binder/compile.go`

```go
// CompileNodes concatenates node draft file bodies in depth-first pre-order.
// readFile is called for each node's Target to retrieve file content.
// Placeholder nodes (Target == "") are skipped.
// Missing or empty files are skipped with warning diagnostics.
// Returns the compiled manuscript text and any diagnostics.
func CompileNodes(
    root *Node,
    readFile func(target string) ([]byte, error),
) (string, []Diagnostic)
```

### `cmd/compile.go`

```go
// CompileIO handles I/O for the compile command.
type CompileIO interface {
    ReadBinder(ctx context.Context, path string) ([]byte, error)
    ScanProject(ctx context.Context, binderPath string) (*binder.Project, error)
    ReadNodeFile(path string) ([]byte, error)
}

// NewCompileCmd creates the compile subcommand.
func NewCompileCmd(io CompileIO) *cobra.Command
```

## Domain Operation: `CompileNodes()`

### Algorithm

```go
func CompileNodes(root *Node, readFile func(target string) ([]byte, error)) (string, []Diagnostic) {
    var buf strings.Builder
    var diags []Diagnostic
    first := true

    var walk func(n *Node)
    walk = func(n *Node) {
        if n.Type == "node" && n.Target != "" {
            // Non-placeholder node: read and append body.
            // Placeholder nodes (Target == "") fall through to child walk.
            content, err := readFile(n.Target)
            if err != nil {
                diags = append(diags, Diagnostic{
                    Severity: "warning",
                    Code:     CodeMissingTargetFile,
                    Message:  fmt.Sprintf("skipping %s: %v", n.Target, err),
                    Location: &Location{Line: n.Line},
                })
            } else {
                body := extractBody(content)
                if strings.TrimSpace(body) != "" {
                    if !first {
                        buf.WriteString("\n\n")
                    }
                    buf.WriteString(body)
                    first = false
                }
            }
        }

        // Always walk children — even for placeholders acting as structural groups.
        for _, child := range n.Children {
            walk(child)
        }
    }

    for _, child := range root.Children {
        walk(child)
    }

    if buf.Len() > 0 {
        buf.WriteString("\n")
    }

    return buf.String(), diags
}

// extractBody strips YAML frontmatter from content and returns the body.
// If no frontmatter is found, returns the entire content as-is.
func extractBody(content []byte) string {
    _, body, err := node.ParseFrontmatter(content)
    if err != nil {
        // No valid frontmatter — use entire content.
        return string(content)
    }
    return string(body)
}
```

### Traversal Order

Depth-first pre-order walk, visiting `Root.Children` in order. This matches the traversal used by `collect_binder_refs.go` and produces the natural reading order of the manuscript.

Example tree:
```
Root
├── Part I
│   ├── Chapter 1
│   └── Chapter 2
└── Part II
    └── Chapter 3
```

Compile order: Part I → Chapter 1 → Chapter 2 → Part II → Chapter 3

### Frontmatter Stripping

Uses `node.ParseFrontmatter(content)` from `internal/node/frontmatter.go`:
- Returns `(Frontmatter, body []byte, error)`.
- If `error != nil` (no frontmatter block), the entire content is the body.
- This gracefully handles:
  - UUID nodes with standard frontmatter
  - Human-named nodes without frontmatter
  - Empty files

### Placeholder Skipping

After Feature 01, placeholders are nodes with `Target == ""`. The compile function simply checks `n.Target == ""` and skips without emitting a diagnostic.

**Important**: Placeholder nodes with children are skipped, but their children are still walked. This handles the case where a placeholder acts as a structural grouping element:

```markdown
- [Part III]()
  - [Chapter 7](ch7.md)
```

Here, Part III is skipped but Chapter 7 is still compiled.

## CLI Command Flow

1. Resolve binder path from `--project`.
2. Read binder via `io.ReadBinder()`.
3. Scan project via `io.ScanProject()`.
4. Parse binder via `binder.Parse()`.
5. Build `readFile` closure that reads from the project directory:
   ```go
   binderDir := filepath.Dir(binderPath)
   readFile := func(target string) ([]byte, error) {
       return io.ReadNodeFile(filepath.Join(binderDir, target))
   }
   ```
6. Call `binder.CompileNodes(result.Root, readFile)`.
7. Print diagnostics (warnings) to stderr.
8. Write compiled text to stdout.
9. Exit 0 (warnings are non-fatal).

## CLI Flags

```go
cmd.Flags().String("project", "", "project directory containing _binder.md (default: current directory)")
```

Minimal flags — compile is a straightforward read-only operation.

```go
Use:  "compile",
Args: cobra.NoArgs,
```

## Test Strategy

### Domain Tests (`internal/binder/compile_test.go`)

| # | Test Name | Input | Expected |
|---|-----------|-------|----------|
| 1 | `TestCompileNodes_EmptyTree` | Root with no children | Empty string, no diagnostics |
| 2 | `TestCompileNodes_SingleNode` | One node with frontmatter | Body only, no frontmatter |
| 3 | `TestCompileNodes_SingleNodeNoFrontmatter` | Node file without `---` block | Entire content as body |
| 4 | `TestCompileNodes_MultipleNodes` | Three nodes | Bodies joined with `\n\n` |
| 5 | `TestCompileNodes_NestedDepthFirst` | Parent + children | Correct DFS pre-order |
| 6 | `TestCompileNodes_MissingFile` | readFile returns error | Warning diagnostic, other nodes compiled |
| 7 | `TestCompileNodes_EmptyBody` | Node with only frontmatter | Skipped (no separator) |
| 8 | `TestCompileNodes_WhitespaceOnlyBody` | Frontmatter + whitespace | Skipped |
| 9 | `TestCompileNodes_PlaceholderSkipped` | Node with `Target: ""` | Skipped, no diagnostic |
| 10 | `TestCompileNodes_PlaceholderWithChildren` | Placeholder parent, file children | Placeholder skipped, children compiled |
| 11 | `TestCompileNodes_MixedPlaceholdersAndNodes` | Interleaved types | Only file nodes compiled |
| 12 | `TestCompileNodes_TrailingNewline` | Any non-empty output | Ends with single `\n` |
| 13 | `TestCompileNodes_DeepNesting` | 3+ levels | Correct DFS order |

### Command Tests (`cmd/compile_test.go`)

| # | Test Name | Scenario | Expected |
|---|-----------|----------|----------|
| 1 | `TestCompileCmd_HappyPath` | Valid binder with nodes | Compiled output on stdout |
| 2 | `TestCompileCmd_EmptyBinder` | No nodes | Empty stdout |
| 3 | `TestCompileCmd_MissingFile` | Node file doesn't exist | Warning on stderr, partial output |
| 4 | `TestCompileCmd_BinderNotFound` | Missing `_binder.md` | Error exit |
| 5 | `TestCompileCmd_ParseError` | Invalid binder | Error exit |
| 6 | `TestCompileCmd_ProjectFlag` | `--project /some/dir` | Correct path resolution |
| 7 | `TestCompileCmd_PlaceholdersSkipped` | Binder with placeholders | Placeholders not in output |
| 8 | `TestCompileCmd_OutputToStdout` | Redirect capture | Full manuscript on stdout |

## Error Handling Matrix

| Scenario | Behavior | Exit Code |
|----------|----------|-----------|
| Binder not found | Error: "project not initialized" | 1 |
| Parse failure | Error: propagated | 1 |
| Node file missing | Warning on stderr; compile continues | 0 |
| Node file read error | Warning on stderr; compile continues | 0 |
| Empty binder | Empty stdout | 0 |
| All files missing | Warnings on stderr; empty stdout | 0 |
| Frontmatter parse error | Use entire content as body | 0 |

## Implementation Steps (TDD Order)

1. **Red**: Write `TestCompileNodes_EmptyTree`. Fails (function doesn't exist).
2. **Green**: Create `internal/binder/compile.go` with stub. Test passes.
3. **Red**: Write `TestCompileNodes_SingleNode` with frontmatter content.
4. **Green**: Implement `extractBody()` + single-node walk. Test passes.
5. **Red**: Write `TestCompileNodes_SingleNodeNoFrontmatter`.
6. **Green**: Add fallback for missing frontmatter. Test passes.
7. **Red**: Write `TestCompileNodes_MultipleNodes`.
8. **Green**: Add `\n\n` separator logic. Test passes.
9. **Red**: Write `TestCompileNodes_NestedDepthFirst`.
10. **Green**: Implement recursive DFS walk. Test passes.
11. **Red**: Write `TestCompileNodes_MissingFile`.
12. **Green**: Add error handling with warning diagnostic. Test passes.
13. **Red**: Write `TestCompileNodes_PlaceholderSkipped` and `TestCompileNodes_PlaceholderWithChildren`.
14. **Green**: Add `Target == ""` skip with child continuation. Test passes.
15. **Red**: Write remaining domain tests (empty body, whitespace, trailing newline).
16. **Green**: Refine edge cases. All tests pass.
17. **Refactor**: Clean up domain layer.
18. **Red**: Write `TestCompileCmd_HappyPath`.
19. **Green**: Implement `cmd/compile.go`. Test passes.
20. **Red**: Write remaining cmd tests.
21. **Green**: All pass.
22. **Register**: Add to `cmd/root.go`. Run `just check`.

## Critical Files

| File | Action |
|------|--------|
| `internal/binder/compile.go` | Create |
| `internal/binder/compile_test.go` | Create |
| `cmd/compile.go` | Create |
| `cmd/compile_test.go` | Create |
| `cmd/root.go` | Add registration |

## Notes on `node.ParseFrontmatter` Import

The `compile.go` file in `internal/binder/` will need to import `internal/node` for `ParseFrontmatter()`. Check that this doesn't create a circular dependency:
- `internal/binder` → `internal/node`: OK (node doesn't import binder... but `collect_binder_refs.go` in `internal/node` imports `internal/binder`!)

**Circular dependency risk**: `internal/node/collect_binder_refs.go` imports `internal/binder`. If `internal/binder/compile.go` imports `internal/node`, this creates a cycle.

**Resolution options**:
1. **Accept the function parameter**: Pass `parseFrontmatter func([]byte) (Frontmatter, []byte, error)` into `CompileNodes()` — avoids the import.
2. **Inline frontmatter stripping**: Duplicate the simple regex-based stripping in `compile.go` — avoids dependency.
3. **Extract to shared package**: Move `ParseFrontmatter` to a `internal/shared` or `internal/frontmatter` package.

**Recommended**: Option 1 — accept a function parameter. This follows the existing pattern of injecting `readFile` and keeps the function signature clean:

```go
func CompileNodes(
    root *Node,
    readFile func(target string) ([]byte, error),
    stripFrontmatter func([]byte) ([]byte, error),
) (string, []Diagnostic)
```

Where `stripFrontmatter` is:
```go
func defaultStripFrontmatter(content []byte) ([]byte, error) {
    _, body, err := node.ParseFrontmatter(content)
    if err != nil {
        return content, nil // no frontmatter = use entire content
    }
    return body, nil
}
```

The CLI layer passes this in; the domain layer stays import-free.
