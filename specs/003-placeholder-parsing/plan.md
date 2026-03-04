# Implementation Plan: Placeholder Node Parsing

**Branch**: `003-placeholder-parsing` | **Date**: 2026-03-04 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/003-placeholder-parsing/spec.md`

## Summary

Add support for **placeholder nodes** in the binder parser. A placeholder is a Markdown list item whose link has an empty target: `[Title]()`. Currently the parser silently skips these items. After this change, they produce real `Node` values in the parse tree (with `Target = ""`), participate in the tree hierarchy, round-trip through serialization unchanged, and do not trigger BNDW003 (duplicate) or BNDW004 (missing file) diagnostics.

Changes are confined to **`internal/binder/parser.go`** (three small edits) plus new conformance fixtures. No new types, no serializer changes, no selector changes, no CLI changes.

---

## Technical Context

**Language/Version**: Go 1.25.6
**Primary Dependencies**: `internal/binder` package (parser, serializer, types); `regexp`, `strings`, `fmt` from stdlib
**Storage**: Filesystem (binder files); no database or network I/O
**Testing**: `go test`, table-driven unit tests, conformance fixtures, acceptance pipeline
**Target Platform**: Linux/macOS/Windows CLI (cross-compiled binary)
**Performance Goals**: N/A — single-file parsing; no throughput targets
**Constraints**: 100% coverage of non-Impl functions; zero `go vet` / `staticcheck` warnings
**Scale/Scope**: Single binder file per parse invocation; no scale concerns

---

## Constitution Check

_GATE: Must pass before implementation. Re-confirmed after design._

| Principle | Status | Notes |
|-----------|--------|-------|
| I. ATDD — GWT spec before implementation | PASS | 4 GWT `.txt` specs created (US1–US4) |
| I. Acceptance Red before Inner TDD | PASS | Acceptance tests will be generated from specs; must fail before coding |
| II. Static Analysis — zero vet/lint warnings | PASS | Changes add no new patterns that could trigger warnings |
| II. Errors handled explicitly | PASS | No new error paths; existing error handling unchanged |
| III. GoDoc on exported APIs | PASS | No new exported functions; existing docs unchanged |
| III. gofmt formatting | PASS | All new code must be formatted before commit |
| IV. Pre-commit gates enforced | PASS | lefthook runs `just check` on every commit |
| V. No deferred warnings | PASS | Fix must not introduce any vet/staticcheck findings |
| VI. CLI conventions unchanged | PASS | No CLI flag or output changes |
| VII. YAGNI / KISS | PASS | Minimal targeted change: one new regex, three guarded blocks |

No violations. Complexity tracking table not required.

---

## Project Structure

### Documentation (this feature)

```text
specs/003-placeholder-parsing/
├── plan.md              ← this file
├── spec.md              ← feature specification
├── research.md          ← Phase 0 research output
├── data-model.md        ← Phase 1 entity model
├── US1-recognize-placeholder-nodes.txt
├── US2-placeholder-nodes-with-children.txt
├── US3-placeholder-round-trip.txt
└── US4-no-spurious-diagnostics.txt
```

### Source Code Changes

```text
internal/binder/
├── parser.go            ← MODIFIED (3 targeted edits; see Implementation below)
└── parser_test.go       ← MODIFIED (new table-driven test cases)

docs/conformance/v1/parse/fixtures/
├── 121-placeholder-basic/
│   ├── binder.md
│   ├── expected-parse.json
│   └── expected-diagnostics.json
├── 122-placeholder-empty-title/   (same structure)
├── 123-placeholder-with-children/ (same structure + chapter-one.md)
├── 124-placeholder-mixed/         (same structure + real.md)
└── 125-placeholder-list-markers/  (same structure)

specs/003-placeholder-parsing/
└── US1–US4 .txt files             ← created above
```

**Structure Decision**: Single project layout, modifying two existing files and adding five fixture directories. No new packages.

---

## Phase 0: Research

**Status**: Complete. See [research.md](research.md) for full decision log.

Key findings:
- `inlineLinkRE` uses `[^)"]+` (one-or-more) → `[Title]()` never matches → parser skips item.
- Parse loop skip condition `if target == ""` must become `if linkNode == nil`.
- `seenTargets` map and BNDW004 check must be guarded by `if !isPlaceholder`.
- Serializer is line-preserving → no changes needed.
- `nodeMatchesSelector` title match already handles named placeholders.
- Fixtures 121–125 are safe (current highest is 120).

---

## Phase 1: Implementation

### 1.1 — Add `emptyTargetLinkRE` to parser.go

**File**: `internal/binder/parser.go`

Add one new package-level compiled regex alongside the existing ones:

```go
// emptyTargetLinkRE matches an inline link with an empty target: [Title]() or []().
// Tried before inlineLinkRE so that the empty-target case is handled first.
emptyTargetLinkRE = regexp.MustCompile(`^\[((?:[^\]\\]|\\.)*)\]\(\s*\)`)
```

**In `parseLink`**: Add the empty-target branch as the **first** case, before the existing `inlineLinkRE` check:

```go
// Empty-target inline link: [Title]() — placeholder node.
if m := emptyTargetLinkRE.FindStringSubmatch(s); m != nil {
    return m[1], "", &sourceInfo{/* position data from existing pattern */}
}
```

> **Note to implementer**: Examine the existing `parseLink` return type carefully. If the third return value is `*sourceInfo` or similar, adapt accordingly. The key contract is: return a non-nil third value to signal "link found, even if empty target".

**Precondition edge case**: `[Title]()` inside a code fence — the parse loop's existing `inCodeFence` gate runs before `parseLink` is called, so code-fence items are already excluded. No additional guard needed.

---

### 1.2 — Fix the continuation-line guard and the skip condition

**File**: `internal/binder/parser.go` — in the main parse loop

**Before** (two occurrences of `if target == ""`):

```go
// Continuation line fallback
if target == "" && i+1 < len(result.Lines) {
    // try parseLink on continuation
}

// Skip if no structural link found
if target == "" {
    continue
}
```

**After**:

```go
// Continuation line fallback — only when no link was found at all
if linkNode == nil && i+1 < len(result.Lines) {
    // try parseLink on continuation (unchanged logic)
}

// Skip if no structural link found (placeholder link keeps linkNode non-nil)
if linkNode == nil {
    continue
}
```

Where `linkNode` is the existing third return value from `parseLink`. After a successful match (including placeholder), `linkNode != nil`. After no match, `linkNode == nil`.

---

### 1.3 — Guard BNDW003 and BNDW004 for placeholder nodes

**File**: `internal/binder/parser.go` — immediately after the skip check, add:

```go
isPlaceholder := target == ""
```

Wrap the BNDW003 block:

```go
// BNDW003 — duplicate file reference (skip for placeholders)
if !isPlaceholder {
    if seenTargets[target] {
        diags = append(diags, Diagnostic{ /* BNDW003 */ })
    }
    seenTargets[target] = true
}
```

Wrap the BNDW004 block:

```go
// BNDW004 — missing target file (skip for placeholders)
if !isPlaceholder && project != nil {
    lookupTarget := strings.TrimPrefix(target, "./")
    if !projectFileSet[lookupTarget] {
        // existing BNDW004 / BNDW009 logic
    }
}
```

No other diagnostic codes need modification (BNDW001, BNDW002, BNDW005, BNDW006, BNDW007 are independent of the target value).

---

## Phase 2: Conformance Fixtures

Each fixture follows the standard structure: `binder.md` + `expected-parse.json` + `expected-diagnostics.json`. File-backed-node fixtures also include the referenced `.md` file(s) so `scanFixtureProject` populates the project file list correctly.

### 121-placeholder-basic

`binder.md`:
```markdown
<!-- prosemark-binder:v1 -->

- [Chapter 3]()
```

`expected-parse.json`:
```json
{"version": "1", "root": {"type": "root", "children": [{"type": "node", "title": "Chapter 3", "children": []}]}}
```

`expected-diagnostics.json`:
```json
{"version": "1", "diagnostics": []}
```

---

### 122-placeholder-empty-title

`binder.md`:
```markdown
<!-- prosemark-binder:v1 -->

- []()
```

`expected-parse.json`:
```json
{"version": "1", "root": {"type": "root", "children": [{"type": "node", "children": []}]}}
```

`expected-diagnostics.json`:
```json
{"version": "1", "diagnostics": []}
```

> Note: `title` and `target` are both absent from JSON due to `omitempty`.

---

### 123-placeholder-with-children

`binder.md`:
```markdown
<!-- prosemark-binder:v1 -->

- [Part I]()
  - [chapter-one.md](chapter-one.md "Chapter One")
```

Also include `chapter-one.md` (empty file) so BNDW004 is not emitted.

`expected-parse.json`:
```json
{"version": "1", "root": {"type": "root", "children": [{"type": "node", "title": "Part I", "children": [{"type": "node", "target": "chapter-one.md", "title": "Chapter One", "children": []}]}]}}
```

`expected-diagnostics.json`:
```json
{"version": "1", "diagnostics": []}
```

---

### 124-placeholder-mixed

`binder.md`:
```markdown
<!-- prosemark-binder:v1 -->

- [real.md](real.md "Real Chapter")
- [Planned Chapter]()
- [real.md](real.md "Real Chapter")
```

Also include `real.md` (empty).

> Note: two entries for `real.md` → BNDW003 is emitted for the duplicate real node. The placeholder is exempt and does NOT contribute to the BNDW003 count.

`expected-parse.json`:
```json
{"version": "1", "root": {"type": "root", "children": [{"type": "node", "target": "real.md", "title": "Real Chapter", "children": []}, {"type": "node", "title": "Planned Chapter", "children": []}, {"type": "node", "target": "real.md", "title": "Real Chapter", "children": []}]}}
```

`expected-diagnostics.json`:
```json
{"version": "1", "diagnostics": [{"severity": "warning", "code": "BNDW003"}]}
```

---

### 125-placeholder-list-markers

`binder.md`:
```markdown
<!-- prosemark-binder:v1 -->

- [Dash]()
* [Star]()
+ [Plus]()
1. [Ordered]()
```

`expected-parse.json`:
```json
{"version": "1", "root": {"type": "root", "children": [{"type": "node", "title": "Dash", "children": []}, {"type": "node", "title": "Star", "children": []}, {"type": "node", "title": "Plus", "children": []}, {"type": "node", "title": "Ordered", "children": []}]}}
```

`expected-diagnostics.json`:
```json
{"version": "1", "diagnostics": []}
```

---

## Phase 3: Unit Tests

**File**: `internal/binder/parser_test.go`

Add table-driven test cases for:

| Test function | Cases to add |
|--------------|-------------|
| `TestParseLink` (or equivalent) | `[Title]()` → title="Title", target="", link found; `[]()` → title="", target="", link found; `[T](x.md)` still works (regression) |
| `TestParse_PlaceholderNodes` | Single placeholder; empty-title placeholder; placeholder as parent; duplicate identical titles produce 2 nodes; all 4 list marker types |
| `TestParse_NoDiagnosticsForPlaceholder` | Placeholder + project context → no BNDW004; two identical placeholder titles → no BNDW003; real duplicate target still emits BNDW003 |
| `TestParse_PlaceholderContinuationLine` | `[Title]()` on continuation line of a list item |

All new test cases must be written TDD-style (Red → Green → Refactor).

---

## Phase 4: Acceptance Test Binding

After the conformance fixtures and unit tests pass, bind the generated acceptance test stubs:

1. Run `just acceptance` to generate stubs from the 4 GWT `.txt` files.
2. Implement each stub in `generated-acceptance-tests/003-placeholder-parsing-US*.go`.
3. Run `just acceptance` again to confirm all acceptance tests pass.
4. Run `just test-all` to confirm unit tests and acceptance tests both pass.

---

## TDD Order

Follow Red → Green → Refactor for each step:

1. **Acceptance Red**: Run `just acceptance` — all 4 spec stubs must emit `t.Skip`.
2. **Unit Red**: Add failing `TestParseLink` case for `[Title]()`.
3. **Unit Green**: Add `emptyTargetLinkRE` and the first branch in `parseLink`.
4. **Unit Red**: Add failing parse-loop test (placeholder node created).
5. **Unit Green**: Change `if target == ""` → `if linkNode == nil` (continuation + skip).
6. **Unit Red**: Add failing BNDW003 / BNDW004 suppression tests.
7. **Unit Green**: Add `isPlaceholder` guard to BNDW003 and BNDW004 blocks.
8. **Conformance fixtures**: Create fixtures 121–125 and confirm `just test` passes.
9. **Acceptance Green**: Bind acceptance test stubs; run `just acceptance`.
10. **Refactor**: Clean up any duplication, ensure GoDoc, run `just check`.

---

## Success Criteria Checklist

- [ ] SC-001: All unit tests for placeholder parsing pass.
- [ ] SC-002: Conformance fixtures 121–125 validate parse output and diagnostics.
- [ ] SC-003: Existing fixtures 001–120 continue to pass unchanged.
- [ ] SC-004: No BNDW003 or BNDW004 for any placeholder node in any parse result.
- [ ] SC-005: Round-trip (`TestConformance_ParseStability`) passes for all new fixtures.
- [ ] SC-006: 100% branch coverage on all non-Impl parser functions.
- [ ] Acceptance tests for US1–US4 pass (`just acceptance`).
- [ ] `just check` passes (test + vet + lint + fmt).
