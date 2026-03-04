# 01 — Placeholder Parsing

## Summary

Extend the binder parser to recognize empty-target inline links (`[Title]()`) as first-class placeholder nodes. Currently, `inlineLinkRE` and `allInlineLinkRE` require one or more characters in the link target (`[^)"]+`), so `[Title]()` is silently skipped. After this feature, placeholders become proper `*binder.Node` entries in the parse tree with `Target: ""`.

**PRD Reference**: Section 4.3 — Placeholders

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Regex change scope | Change `+` → `*` in two regexes | Minimal change; empty capture group naturally yields `""` |
| Node representation | `Target: ""` on a normal `*Node` | No new type needed; `omitempty` already handles JSON serialization |
| Validation bypass | Guard target-validation block with `if target != ""` | Placeholders have no path to validate, no file to check |
| Duplicate detection | Skip for placeholders | Multiple `[Ch 3]()` are distinct planning entries, not duplicates |
| Title derivation | Use bracket text as-is; no stem fallback | Placeholders have no filename to derive a stem from |
| Empty title | `[]()` produces `Title: ""` | Consistent with "no text = no title" |

## Parser Changes

### File: `internal/binder/parser.go`

#### 1. Regex modifications (lines 17, 27)

```go
// Before:
inlineLinkRE    = regexp.MustCompile(`^\[((?:[^\]\\]|\\.)*)\]\(([^)"]+)(?:\s+"[^"]*")?\s*\)`)
allInlineLinkRE = regexp.MustCompile(`\[((?:[^\]\\]|\\.)*)\]\(([^)"]+)(?:\s+"[^"]*")?\s*\)`)

// After:
inlineLinkRE    = regexp.MustCompile(`^\[((?:[^\]\\]|\\.)*)\]\(([^)"]*?)(?:\s+"[^"]*")?\s*\)`)
allInlineLinkRE = regexp.MustCompile(`\[((?:[^\]\\]|\\.)*)\]\(([^)"]*?)(?:\s+"[^"]*")?\s*\)`)
```

Note: `+` → `*?` (lazy) to avoid greedily matching into optional title portion.

#### 2. `parseLink` function (line 346)

The `inlineLinkRE` match branch already handles empty title fallback via `stemFromPath`. For placeholders where `target == ""`, `stemFromPath("")` returns `""`, which is correct. The existing code works without modification:

```go
if m := inlineLinkRE.FindStringSubmatch(content); m != nil {
    target, title = m[2], unescapeTitle(m[1])
    if title == "" {
        title = stemFromPath(target) // stemFromPath("") == ""
    }
}
```

#### 3. Parse loop — placeholder guard (lines 173–266)

After `parseLink` returns, the parse loop currently skips items with `target == ""` (line 174). For placeholders, `target` will be `""` but the link was still recognized by the regex. We need to distinguish "no link found" from "placeholder link with empty target".

**Approach**: `parseLink` returns a fourth value `linkFound bool`, or more simply, introduce a sentinel. The cleanest approach: check in the parse loop whether the raw content matches a placeholder pattern before the `target == ""` skip.

**Refined approach**: Add a `isPlaceholder` return from `parseLink`:

```go
func parseLink(...) (target, title string, diags []Diagnostic, isPlaceholder bool) {
    if m := inlineLinkRE.FindStringSubmatch(content); m != nil {
        target, title = m[2], unescapeTitle(m[1])
        if target == "" {
            isPlaceholder = true
            return // title already set from bracket text
        }
        if title == "" {
            title = stemFromPath(target)
        }
    }
    // ... rest unchanged
    return
}
```

Then in the parse loop (around lines 153–176):

```go
target, title, linkDiags, isPlaceholder := parseLink(...)

// ... continuation-line fallback (also needs isPlaceholder propagation)

diags = append(diags, linkDiags...)

// Skip items with no resolved target (unless placeholder).
if target == "" && !isPlaceholder {
    continue
}

// --- Begin target-validation block (skip for placeholders) ---
if !isPlaceholder {
    // Non-md fallback (lines 178–193)
    // Percent-decode (lines 195–206)
    // validateTarget (lines 208–212)
    // Self-referential check (lines 214–223)
    // Duplicate detection (lines 225–234)
    // Missing file check (lines 236–256)
    // Multiple structural links check (lines 258–266)
}
```

Node creation (line 268) proceeds unchanged — `Target` will be `""` for placeholders.

#### 4. Continuation-line fallback (lines 155–168)

The continuation-line search also calls `parseLink`. Propagate `isPlaceholder`:

```go
if target == "" && !isPlaceholder && i+1 < len(result.Lines) {
    // ...
    t, ti, ld, ip := parseLink(...)
    if t != "" || ip {
        target, title, isPlaceholder = t, ti, ip
        linkDiags = ld
    }
}
```

## No Changes Needed

These components already handle placeholders correctly:

| Component | File | Why |
|-----------|------|-----|
| `Node` struct | `types.go` | `Target` has `omitempty`; empty string serializes correctly |
| `Serialize()` | `serializer.go` | Line-preserving; round-trips `[Title]()` unchanged |
| `EvalSelector()` | `selector.go` | Placeholders are unmatchable by path (correct); matchable by title via `strings.EqualFold(n.Title, fileRef)` |
| `ops/delete.go` | `ops/delete.go` | Works on any node regardless of target |
| `ops/move.go` | `ops/move.go` | Works on any node regardless of target |
| `findFirstMdLink()` | `parser.go` | Only searches for `.md` links; placeholders are not `.md` |

## Format Spec Updates

### File: `docs/prosemark_binder_format_spec_v1.md`

#### Section 4.1 — Add placeholder definition

Add after the structural node definition:

> A **placeholder node** is a list item containing an inline link with an empty target: `[Title]()`. Placeholder nodes are structural nodes with `target: ""`. They represent planned content that has not yet been materialized into a file.

#### Section 4.4 — Valid structural link targets

Add a paragraph:

> An inline link with an empty target (`[Title]()`) is a valid structural link that produces a **placeholder node**. Placeholder nodes have no file path and are exempt from path validation, duplicate detection, and file-existence checks. The `materialize` operation (see operations spec) converts a placeholder into a file-backed node.

#### Section 5.1 — Tree construction

Add rule:

> Placeholder links (`[Title]()`) produce nodes with `target: ""`. They participate in the tree hierarchy like any other node — they may have children and may appear at any nesting level.

## Conformance Fixtures

Add new fixtures in `docs/conformance/v1/parse/fixtures/`:

### `066-placeholder-basic/`

```
binder.md:
<!-- prosemark-binder:v1 -->

- [Chapter 3]()

expected-parse.json:
{
  "version": "1",
  "root": {
    "type": "root",
    "children": [
      {
        "type": "node",
        "target": "",
        "title": "Chapter 3",
        "children": []
      }
    ]
  }
}

expected-diagnostics.json:
{
  "version": "1",
  "diagnostics": []
}
```

### `067-placeholder-empty-title/`

`[]()` — node with empty target and empty title.

### `068-placeholder-with-children/`

Placeholder parent with file-backed children nested beneath it.

### `069-placeholder-mixed/`

Placeholders interspersed with real nodes at multiple nesting levels.

### `070-placeholder-list-markers/`

Placeholders using `-`, `*`, `+`, `1.` list markers.

## Test Strategy

### File: `internal/binder/parser_test.go`

Table-driven tests added to existing test file:

| # | Test Name | Input | Expected |
|---|-----------|-------|----------|
| 1 | `TestParse_Placeholder_BasicNode` | `- [Chapter 3]()` | Node with `Target: ""`, `Title: "Chapter 3"` |
| 2 | `TestParse_Placeholder_EmptyTitle` | `- []()` | Node with `Target: ""`, `Title: ""` |
| 3 | `TestParse_Placeholder_WithChildren` | `- [Part]()` + `  - [Ch1](ch1.md)` | Placeholder parent, file-backed child |
| 4 | `TestParse_Placeholder_Mixed` | Real nodes + placeholders interleaved | Correct tree with both types |
| 5 | `TestParse_Placeholder_NestedLevels` | Placeholders at 3+ indent levels | Correct nesting |
| 6 | `TestParse_Placeholder_RoundTrip` | Parse → Serialize | Byte-identical output |
| 7 | `TestParse_Placeholder_ListMarkers` | `-`, `*`, `+`, `1.` markers | All produce nodes |
| 8 | `TestParse_Placeholder_NoDuplicateWarning` | Two `[Ch 3]()` entries | No BNDW003 emitted |
| 9 | `TestParse_Placeholder_NoMissingFileWarning` | `- [Ch]()` with project context | No BNDW004 emitted |
| 10 | `TestParse_Placeholder_ContinuationLine` | Placeholder link on continuation line | Recognized correctly |

### Regression

All existing conformance fixtures (065 current fixtures) must continue passing unchanged.

## Error Handling

| Scenario | Behavior | Code |
|----------|----------|------|
| `[Title]()` in list item | Parsed as placeholder node | (no diagnostic) |
| `[]()` in list item | Parsed as placeholder with empty title | (no diagnostic) |
| `[Title]()` inside code fence | Emitted as BNDW005 (existing behavior) | BNDW005 |
| `[Title]()` outside list item | Skipped (existing behavior for non-list content) | (none) |
| Placeholder with children | Children attach normally | (none) |

## Implementation Steps (TDD Order)

1. **Red**: Write `TestParse_Placeholder_BasicNode` — expects `Target: ""`, `Title: "Chapter 3"`. Run; fails because regex rejects empty target.
2. **Green**: Change `inlineLinkRE` regex: `+` → `*?`. Update `parseLink` to return `isPlaceholder`. Add guard in parse loop. Test passes.
3. **Red**: Write `TestParse_Placeholder_EmptyTitle` — expects empty title. Should pass immediately (verify).
4. **Red**: Write `TestParse_Placeholder_NoDuplicateWarning`. Run; verify no BNDW003.
5. **Red**: Write `TestParse_Placeholder_NoMissingFileWarning` with project context. Run; verify no BNDW004.
6. **Green**: Confirm the `if !isPlaceholder` guard covers all validation. Fix if needed.
7. **Red**: Write `TestParse_Placeholder_WithChildren`. Should pass (tree-building unchanged).
8. **Red**: Write `TestParse_Placeholder_RoundTrip`. Should pass (serializer is line-preserving).
9. **Red**: Write remaining test cases. All should pass.
10. **Refactor**: Clean up; add `allInlineLinkRE` regex change. Run `just check`.
11. **Conformance**: Add fixture directories. Run conformance tests.
12. **Spec**: Update `docs/prosemark_binder_format_spec_v1.md` sections 4.1, 4.4, 5.1.

## Critical Files

| File | Action |
|------|--------|
| `internal/binder/parser.go` (lines 17, 27, 153–176, 345–376) | Modify |
| `internal/binder/parser_test.go` | Add tests |
| `docs/prosemark_binder_format_spec_v1.md` (§4.1, §4.4, §5.1) | Update |
| `docs/conformance/v1/parse/fixtures/066-*` through `070-*` | Add |
