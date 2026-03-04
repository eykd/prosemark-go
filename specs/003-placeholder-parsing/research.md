# Research: Placeholder Node Parsing

**Branch**: `003-placeholder-parsing` | **Date**: 2026-03-04

## Decision Log

### D-001: How to detect empty-target links in `parseLink`

**Decision**: Add a dedicated `emptyTargetLinkRE` regex tried *before* `inlineLinkRE`.

**Rationale**: The existing `inlineLinkRE` uses `[^)"]+` (one-or-more) for the target capture group, which requires at least one character. Changing this in-place to `*` (zero-or-more) risks subtle regressions with the optional tooltip pattern `(?:\s+"[^"]*")?`. A separate, tightly-scoped regex for the empty-target case is the safest, most targeted change.

**New regex**:
```go
emptyTargetLinkRE = regexp.MustCompile(`^\[((?:[^\]\\]|\\.)*)\]\(\s*\)`)
```
This matches `[Title]()` or `[Title](  )` (optional whitespace in target). It captures the title and implies an empty target.

**Alternatives considered**:
- Modify `inlineLinkRE` target group `+` → `*`: rejected (risk of tooltip-only link `[T]("title")` being parsed as placeholder with target `"title"`).
- Add fallback after all existing checks fail: rejected (continuation-line logic would still fire on a placeholder before the fallback could run).

---

### D-002: Distinguishing "no link found" from "placeholder found"

**Decision**: Use `linkNode != nil` (the existing third return value of `parseLink`) as the `linkFound` sentinel.

**Rationale**: `parseLink` already returns `("", "", nil)` when no structural link is found. After adding the `emptyTargetLinkRE` path, a placeholder returns `(title, "", non-nil)`. Using the existing return contract avoids adding a new boolean to `parseLink`'s signature.

**Implication**: The continuation-line fallback check changes from `if target == ""` to `if linkNode == nil`. The node-creation skip changes from `if target == ""` to `if linkNode == nil`. These are the only two call-site changes needed in the parse loop.

---

### D-003: Guarding BNDW003 (duplicate) and BNDW004 (missing file) for placeholders

**Decision**: Introduce `isPlaceholder := target == ""` immediately after `linkNode != nil` is established. Wrap BNDW003 and BNDW004 emission blocks with `if !isPlaceholder`.

**Rationale**:
- BNDW003 tracks `seenTargets[target]`. All placeholders share `target = ""`. If we added `seenTargets[""] = true` for the first placeholder, every subsequent placeholder would trigger a spurious duplicate warning. Skipping the entire BNDW003 block (both check and mark) for placeholders eliminates this.
- BNDW004 checks `projectFileSet[target]`. An empty target cannot match any project file and would always fire. Skipping for placeholders eliminates this spurious warning.

**Alternatives considered**:
- Use a special sentinel target (e.g., `"__placeholder__"`): rejected (pollutes the Node struct, complicates selector logic and JSON output).
- Use a new `IsPlaceholder bool` field on Node: rejected (YAGNI — no code path outside the parse loop needs to distinguish; the absence of a target is sufficient).

---

### D-004: Serializer changes

**Decision**: No serializer changes required.

**Rationale**: The serializer (`binder.Serialize`) replays `ParseResult.Lines` verbatim — it does not reconstruct Markdown from the Node tree. Each Node stores its `RawLine` (the original source line). Placeholder nodes are no different from any other node in this respect: the raw line is stored during parsing and replayed during serialization. Round-trip byte-identity is automatic.

---

### D-005: Selector changes

**Decision**: No selector changes required.

**Rationale**: `nodeMatchesSelector` already includes a title-based match: `strings.EqualFold(n.Title, fileRef)`. This means named placeholders like `[Chapter 3]()` are already matchable via `Chapter 3` as a selector segment. Empty-title placeholders (`[]()`) are unmatchable by any valid selector segment, which is acceptable — there is no meaningful way to address a node with no title or path.

---

### D-006: Conformance fixture numbering

**Decision**: Use numbers 121–125 for new placeholder-parsing fixtures.

**Rationale**: The current highest fixture number is 120 (`120-cr-line-endings`). Numbers 066–070 are referenced in `docs/plan/01-placeholder-parsing.md` (written when fixtures only went to ~65), but the fixture set has grown substantially since then. Using 121–125 guarantees no conflict without requiring a gap audit.

**Fixtures**:
| Number | Directory | Scenario |
|--------|-----------|----------|
| 121 | `121-placeholder-basic` | Single `[Chapter 3]()` — node with title, empty target, no diagnostics |
| 122 | `122-placeholder-empty-title` | `[]()` — node with empty title and empty target |
| 123 | `123-placeholder-with-children` | Placeholder parent with file-backed child |
| 124 | `124-placeholder-mixed` | Placeholders and real nodes interspersed |
| 125 | `125-placeholder-list-markers` | `-`, `*`, `+`, `1.` markers all produce placeholder nodes |

---

### D-007: Node struct changes

**Decision**: No new fields on `Node`.

**Rationale**: The existing `Target string` field with `json:"target,omitempty"` correctly represents both cases:
- File-backed node: `Target = "foo.md"` → appears in JSON as `"target": "foo.md"`.
- Placeholder node: `Target = ""` → omitted from JSON (omitempty); node is still present in the tree.

The assumption in spec.md ("The `Node` struct's existing `Target string` field correctly handles the empty-string case") is confirmed.

---

## Codebase Findings

### `inlineLinkRE` (current)
```go
inlineLinkRE = regexp.MustCompile(`^\[((?:[^\]\\]|\\.)*)\]\(([^)"]+)(?:\s+"[^"]*")?\s*\)`)
```
Target group `([^)"]+)` requires one-or-more chars → empty target never matches.

### Parse loop skip (current)
```go
if target == "" {
    continue
}
```
After fix → changed to `if linkNode == nil { continue }`.

### BNDW003 emission site (current, in parser.go)
```go
if seenTargets[target] {
    diags = append(diags, ...)
}
seenTargets[target] = true
```
After fix → wrapped in `if !isPlaceholder { ... }`.

### BNDW004 emission site (current, in parser.go)
```go
if project != nil && !projectFileSet[lookupTarget] {
    // emit BNDW004 or BNDW009
}
```
After fix → wrapped in `if !isPlaceholder { ... }`.

### Conformance test runner
Uses **subset matching** for parse results (extra keys in actual are ignored). Uses **exact matching** for diagnostics (count, code, severity must match; message/location checked only if present in expected).

### Round-trip test
`TestConformance_ParseStability` already runs for all fixtures. New placeholder fixtures automatically get round-trip tested.
