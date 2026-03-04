# Data Model: Placeholder Node Parsing

**Branch**: `003-placeholder-parsing` | **Date**: 2026-03-04

## Summary

No new types are introduced. The feature extends the behavior of the existing `Node` struct by allowing its `Target` field to be empty (`""`). A `Node` with `Target == ""` is a **placeholder node** — planned content not yet materialised into a file.

---

## Existing Entity: `Node` (internal/binder/types.go)

```go
type Node struct {
    // Exported to JSON
    Type     string  `json:"type"`             // "root" | "node"
    Target   string  `json:"target,omitempty"` // empty for placeholder nodes
    Title    string  `json:"title,omitempty"`  // display text from [Title]()
    Children []*Node `json:"children"`         // ordered; never nil

    // Source metadata (not in JSON)
    Line        int    `json:"-"`
    Column      int    `json:"-"`
    ByteOffset  int    `json:"-"`
    EndLine     int    `json:"-"`
    SubtreeEnd  int    `json:"-"`
    Indent      int    `json:"-"`
    IndentChar  byte   `json:"-"`
    ListMarker  string `json:"-"` // "-", "*", "+", "1.", etc.
    RawLine     string `json:"-"` // original source line (preserved for serializer)
    InCodeFence bool   `json:"-"`
}
```

### Placeholder Node Semantics

| Field | File-backed node | Placeholder node |
|-------|-----------------|-----------------|
| `Type` | `"node"` | `"node"` |
| `Target` | `"foo.md"` (non-empty) | `""` (empty) |
| `Title` | title text or derived from stem | title text (or `""` for `[]()`) |
| `Children` | zero or more | zero or more (same as any node) |
| JSON `"target"` key | present | **absent** (omitempty) |

### Identity and Uniqueness

- Placeholder nodes with the same title are **distinct nodes** (no deduplication, no BNDW003 warning).
- Placeholder nodes are not tracked in the `seenTargets` map during parsing.
- Placeholder nodes cannot be uniquely addressed by path in `EvalSelector`; named placeholders are addressable by title via case-insensitive match.

### Lifecycle

- Created during `binder.Parse` when an empty-target inline link `[Title]()` is found in a list item.
- Persisted verbatim during `binder.Serialize` (line-preserving serializer; no structural changes).
- Mutation operations (add-child, delete, move) work on any `*Node` regardless of `Target` — no changes required.

---

## Existing Entity: `ParseResult` (internal/binder/types.go)

No changes. `ParseResult.Lines` stores the raw source lines that the serializer replays. Placeholder nodes' lines are stored identically to any other node's lines.

---

## Existing Entity: `Diagnostic` (internal/binder/types.go)

No changes to the diagnostic type. Two existing diagnostic codes are **not emitted** for placeholder nodes:

| Code | Name | Suppressed for placeholders? |
|------|------|------------------------------|
| BNDW003 | `CodeDuplicateFileRef` | Yes — empty target would cause false duplicate |
| BNDW004 | `CodeMissingTargetFile` | Yes — empty target cannot match any project file |
| All others | — | Unchanged behavior |

---

## JSON Output Example

**Binder input**:
```markdown
<!-- prosemark-binder:v1 -->

- [Part I]()
  - [chapter-one.md](chapter-one.md "Chapter One")
```

**`pmk parse` JSON output**:
```json
{
  "version": "1",
  "root": {
    "type": "root",
    "children": [
      {
        "type": "node",
        "title": "Part I",
        "children": [
          {
            "type": "node",
            "target": "chapter-one.md",
            "title": "Chapter One",
            "children": []
          }
        ]
      }
    ]
  }
}
```

Note: `"target"` is absent for the placeholder node (omitempty), present for the file-backed node.
