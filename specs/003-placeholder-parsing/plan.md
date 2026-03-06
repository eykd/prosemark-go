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

### 1.1 — Add `emptyTargetLinkRE` and update `parseLink` signature

**File**: `internal/binder/parser.go`

**Step A — Add regex** alongside the existing package-level vars (after `inlineLinkRE`):

```go
// emptyTargetLinkRE matches an inline link with an empty target: [Title]() or []().
// Checked before inlineLinkRE so the empty-target case is handled first.
emptyTargetLinkRE = regexp.MustCompile(`^\[((?:[^\]\\]|\\.)*)\]\(\s*\)`)
```

**Step B — Modify `parseLink` signature** to add `found bool` as the third return value.

Current signature (line 345):
```go
func parseLink(content string, refDefs map[string]RefDef, wikiIndex map[string][]wikilinkEntry, binderDir string, lineNum, column int) (target, title string, diags []Diagnostic)
```

New signature:
```go
func parseLink(content string, refDefs map[string]RefDef, wikiIndex map[string][]wikilinkEntry, binderDir string, lineNum, column int) (target, title string, found bool, diags []Diagnostic)
```

**Step C — Add empty-target branch** as the FIRST case in `parseLink`, before the `inlineLinkRE` check:

```go
// Empty-target inline link: [Title]() — placeholder node.
// strings.TrimSpace normalises whitespace-only titles (e.g. "[ ]()" → title "").
if m := emptyTargetLinkRE.FindStringSubmatch(content); m != nil {
    title, found = strings.TrimSpace(unescapeTitle(m[1])), true
    return
}
```

**Step D — Complete rewrite of `parseLink` body** with `found bool` in every branch. The full replacement for lines 345–376:

```go
func parseLink(content string, refDefs map[string]RefDef, wikiIndex map[string][]wikilinkEntry, binderDir string, lineNum, column int) (target, title string, found bool, diags []Diagnostic) {
    // Empty-target inline link: [Title]() or []() — placeholder node.
    // Checked first so the empty-target case is handled before inlineLinkRE.
    if m := emptyTargetLinkRE.FindStringSubmatch(content); m != nil {
        title, found = strings.TrimSpace(unescapeTitle(m[1])), true
        return
    }
    if m := inlineLinkRE.FindStringSubmatch(content); m != nil {
        // inlineLinkRE requires [^)"]+, so target is always non-empty here.
        target, title, found = m[2], unescapeTitle(m[1]), true
        if title == "" {
            title = stemFromPath(target)
        }
    } else if m := wikilinkRE.FindStringSubmatch(content); m != nil {
        rawStem := m[1]
        alias := m[2]
        // If no pipe alias, check for trailing text after ]] (e.g. [[file.md]] Title).
        if alias == "" {
            trailing := strings.TrimSpace(content[len(m[0]):])
            if trailing != "" {
                alias = trailing
            }
        }
        target, title, diags = resolveWikilink(rawStem, alias, wikiIndex, binderDir, lineNum, column)
        // resolveWikilink returns target="" on failure (BNDE003 already in diags).
        // Only set found when resolution succeeded — failed wikilinks must stay skipped.
        found = target != ""
    } else if m := fullRefLinkRE.FindStringSubmatch(content); m != nil {
        if rd, exists := refDefs[strings.ToLower(m[2])]; exists {
            target, title, found = rd.Target, m[1], true
        }
    } else if m := collapsedRefRE.FindStringSubmatch(content); m != nil {
        if rd, exists := refDefs[strings.ToLower(m[1])]; exists {
            target, title, found = rd.Target, m[1], true
        }
    } else if m := shortcutRefRE.FindStringSubmatch(content); m != nil {
        if rd, exists := refDefs[strings.ToLower(m[1])]; exists {
            target, title, found = rd.Target, m[1], true
        }
    }
    return
}
```

**Why `found = false` for failed wikilinks matters**: the parse loop emits `linkDiags` (including BNDE003) at line 171 — before the skip check at line 174. If a failed wikilink sets `found = true`, `isPlaceholder` becomes `true` (found=true, target=""), bypassing BNDW007/BNDW003/BNDW004 guards and creating a spurious empty-target node. Keeping `found = false` for failed wikilinks preserves the existing skip path.

When no branch matches (and `emptyTargetLinkRE` did not match), `found` defaults to `false` (zero value).

**Precondition edge case**: `[Title]()` inside a code fence — the parse loop's existing `inCodeFence` gate runs before `parseLink` is called. No additional guard needed.

---

### 1.2 — Fix the call site and guards in the parse loop

**File**: `internal/binder/parser.go` — main parse loop

**Step A — Capture `found`** from the updated `parseLink` call (line 153):

```go
// Before (line 153):
target, title, linkDiags := parseLink(content, result.RefDefs, wikiIndex, binderDir, lineNum, listItemColumn)

// After:
target, title, found, linkDiags := parseLink(content, result.RefDefs, wikiIndex, binderDir, lineNum, listItemColumn)
```

**Step B — Derive `isPlaceholder`** immediately after the `parseLink` call (initial value; recalculated after Step C).

Use `var isPlaceholder bool` (not `:=`) so the variable can be reassigned without shadowing after the continuation block:

```go
var isPlaceholder bool
isPlaceholder = found && target == ""
```

This avoids the compilation error that would result from using `:=` before the continuation block and then needing to reassign with `=` after it.

**Step C — Fix the continuation-line guard** (lines 156–168). Change `if target == ""` → `if !found`. Also capture `tFound` from the continuation `parseLink` call so that a placeholder on the continuation line is promoted correctly (spec edge case: "Placeholder link appearing on a continuation line — recognized correctly").

Current code at lines 156–168:
```go
if target == "" && i+1 < len(result.Lines) {
    nextLine := result.Lines[i+1]
    if countLeadingWhitespace(nextLine) > indent && !listItemRE.MatchString(nextLine) {
        contContent := normalizeListContent(strings.TrimSpace(nextLine))
        t, ti, ld := parseLink(contContent, result.RefDefs, wikiIndex, binderDir, i+2, 0)
        consumed[i+1] = true
        if t != "" {
            target, title = t, ti
            linkDiags = ld
        }
    }
}
```

Updated code:
```go
if !found && i+1 < len(result.Lines) {
    nextLine := result.Lines[i+1]
    if countLeadingWhitespace(nextLine) > indent && !listItemRE.MatchString(nextLine) {
        contContent := normalizeListContent(strings.TrimSpace(nextLine))
        t, ti, tFound, ld := parseLink(contContent, result.RefDefs, wikiIndex, binderDir, i+2, 0)
        consumed[i+1] = true
        if t != "" {
            target, title = t, ti
            linkDiags = ld
            found = true
        } else if tFound {
            // Continuation line is a placeholder [Title]() — promote as found placeholder.
            title = ti
            linkDiags = ld
            found = true
        }
    }
}
// Recalculate isPlaceholder: found/target may have changed in the continuation block.
isPlaceholder = found && target == ""
```

Note: `isPlaceholder` must be recalculated after the continuation block because `found` and `target` may change there. Declaring it with `:=` before the block and reassigning with `=` after requires changing the declaration to `var isPlaceholder bool` or splitting the assignment.

**Step D — Fix the skip condition** (lines 174–176). Change `if target == ""` → `if !found`:

Current code:
```go
// Skip items with no resolved target.
if target == "" {
    continue
}
```

Updated code:
```go
// Skip items with no resolved target.
if !found {
    continue
}
```

A placeholder has `found=true` and `target=""`, so it is NOT skipped.

---

### 1.3 — Guard all target-dependent checks for placeholder nodes

**File**: `internal/binder/parser.go` — between the skip check (§1.2 Step D) and node creation

`isPlaceholder` is already set in §1.2 Step B. Guard each check that inspects `target`:

**Guard non-markdown target check (BNDW007, lines 179–193)**:

```go
// Before (line 179):
if !isMarkdownTarget(target) && !hasIllegalPathChars(target) && !escapesRoot(target) {

// After:
if !isPlaceholder && !isMarkdownTarget(target) && !hasIllegalPathChars(target) && !escapesRoot(target) {
```

Without this guard, `target=""` fails `isMarkdownTarget` and fires BNDW007 for every placeholder.

**Guard percent-decode and path-validation block (lines 195–212)**:

```go
// Before (lines 195–212):
decoded, decodeOK := percentDecodeTarget(target)
if !decodeOK { ... continue }
target = decoded
if diag := validateTarget(target, lineNum, listItemColumn); diag != nil { ... continue }

// After:
if !isPlaceholder {
    decoded, decodeOK := percentDecodeTarget(target)
    if !decodeOK { ... continue }
    target = decoded
    if diag := validateTarget(target, lineNum, listItemColumn); diag != nil { ... continue }
}
```

**Guard BNDW003 (duplicate file reference, lines 225–234)**:

```go
// Before (lines 225–234):
if seenTargets[target] {
    diags = append(diags, Diagnostic{ /* BNDW003 */ })
}
seenTargets[target] = true

// After:
if !isPlaceholder {
    if seenTargets[target] {
        diags = append(diags, Diagnostic{ /* BNDW003 */ })
    }
    seenTargets[target] = true
}
```

**Guard BNDW004/BNDW009 (missing/case-mismatch target, lines 236–256)**:

```go
// Before (lines 236–256):
lookupTarget := strings.TrimPrefix(target, "./")
if project != nil && !projectFileSet[lookupTarget] {
    // BNDW009 / BNDW004 logic
}

// After:
if !isPlaceholder {
    lookupTarget := strings.TrimPrefix(target, "./")
    if project != nil && !projectFileSet[lookupTarget] {
        // existing BNDW004 / BNDW009 logic
    }
}
```

BNDW001, BNDW002 (lines 258–266) and BNDW008 (lines 214–223) are safe without guards:
- BNDW008: `target="" != "_binder.md"` → does not fire; the guard exits before BNDW003/BNDW004 anyway.
- BNDW002: uses `mdInlineLinkRE.FindAllString(content, -1)` which counts `.md` inline links in raw content; `[Title]()` has no `.md` link → count is 0, `len(allMd) > 1` is false → does not fire.

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

**Files**:
- `internal/binder/parselink_test.go` — new file, `package binder` (internal, for `TestParseLink`)
- `internal/binder/parser_test.go` — existing file, `package binder_test` (for all other new tests)

### Why two files?

`parseLink` is unexported. All existing test files use `package binder_test` and cannot access unexported symbols. `TestParseLink` must directly assert on `found bool` (the new third return value). Creating `parselink_test.go` with `package binder` grants access to the unexported function. Go allows `package binder` and `package binder_test` files to coexist in the same directory.

The other three test functions (`TestParse_PlaceholderNodes`, `TestParse_NoDiagnosticsForPlaceholder`, `TestParse_PlaceholderContinuationLine`) test observable behavior through the exported `binder.Parse()` API and belong in `parser_test.go`.

---

### `TestParseLink` — in `internal/binder/parselink_test.go`

```go
package binder

import "testing"

func TestParseLink(t *testing.T) {
    tests := []struct {
        name       string
        content    string
        wantTarget string
        wantTitle  string
        wantFound  bool
        wantDiags  int
    }{
        {"placeholder with title", "[Title]()", "", "Title", true, 0},
        {"placeholder empty title", "[]()", "", "", true, 0},
        {"placeholder whitespace title", "[ ]()", "", "", true, 0},
        {"placeholder whitespace target", "[Title]( )", "", "Title", true, 0},
        {"regular inline link", "[T](x.md)", "x.md", "T", true, 0},
        {"regular inline link with tooltip", `[T](x.md "Tip")`, "x.md", "T", true, 0},
        {"unrecognised text", "just text", "", "", false, 0},
        // Ref-link with no matching def — unresolvable, stays skipped.
        {"ref link absent", "[Title][nonexistent-ref]", "", "", false, 0},
        // Known limitation: nested unescaped brackets — emptyTargetLinkRE captures partial
        // title "[Chapter [3" and leaves trailing "]()". Neither regex matches fully.
        // Authors must use escaped form [Chapter \[3\]]().
        // {"nested brackets", "[Chapter [3]]()", ...}, // skipped; known limitation
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // nil maps are safe: wikilink/ref branches only look up maps when the
            // regex matches, and none of the test inputs use wikilink/ref syntax.
            target, title, found, diags := parseLink(tt.content, nil, nil, "", 1, 0)
            if target != tt.wantTarget {
                t.Errorf("target = %q, want %q", target, tt.wantTarget)
            }
            if title != tt.wantTitle {
                t.Errorf("title = %q, want %q", title, tt.wantTitle)
            }
            if found != tt.wantFound {
                t.Errorf("found = %v, want %v", found, tt.wantFound)
            }
            if len(diags) != tt.wantDiags {
                t.Errorf("len(diags) = %d, want %d: %v", len(diags), tt.wantDiags, diags)
            }
        })
    }
}
```

---

### Other unit test cases — added to `internal/binder/parser_test.go`

| Test function | Cases to add |
|--------------|-------------|
| `TestParse_PlaceholderNodes` | Single placeholder; empty-title placeholder; placeholder as parent; duplicate identical titles produce 2 nodes; all 4 list marker types |
| `TestParse_NoDiagnosticsForPlaceholder` | Placeholder + project context → no BNDW004; two identical placeholder titles → no BNDW003; real duplicate target still emits BNDW003; placeholder → no BNDW007 |
| `TestParse_PlaceholderContinuationLine` | `[Title]()` on a primary list item line → placeholder node produced; real link on continuation line → node produced with correct target; `[Title]()` on continuation line (primary line has non-link text, e.g. `- placeholder\n  [Chapter 3]()`) → placeholder node produced (spec edge case: "Placeholder link appearing on a continuation line — recognized correctly") |

All new test cases must be written TDD-style (Red → Green → Refactor).

---

## Phase 4: Acceptance Test Binding

After the conformance fixtures and unit tests pass, bind the generated acceptance test stubs.

### Infrastructure

- **Generated file location**: `generated-acceptance-tests/003-placeholder-parsing-US<N>-<slug>_test.go`
- **Package**: `package acceptance_test`
- **Available helpers** (from `generated-acceptance-tests/helpers_test.go`):
  - `writeFile(t, dir, name, content) string` — creates a file in a temp dir
  - `runParse(t, binderPath) runResult` — runs `pmk parse --project <dir>`
  - `runConformance(t, filter) runResult` — runs `go test ./conformance/... -run <filter>`
  - `runResult.OK bool`, `runResult.Stdout string`, `runResult.Stderr string`
- **`pmk parse` JSON shape** (all fields in one object on stdout):
  ```json
  {"version":"1","root":{"type":"root","children":[...]},"diagnostics":[]}
  ```

### Binding steps

1. Run `just acceptance` to generate stubs from the 4 GWT `.txt` files.
2. Replace each stub body with the implementations below (preserving the function signature).
3. Run `just acceptance` again to confirm all acceptance tests pass.
4. Run `just test-all` to confirm unit tests and acceptance tests both pass.

---

### US1 — Recognize placeholder nodes

**File**: `generated-acceptance-tests/003-placeholder-parsing-US1-recognize-placeholder-nodes_test.go`

```go
// Scenario: A placeholder item [Chapter 3]() becomes a node in the outline tree.
func Test_A_placeholder_item_Chapter_3_becomes_a_node_in_the_outline_tree(t *testing.T) {
    dir := t.TempDir()
    writeFile(t, dir, "_binder.md", "<!-- prosemark-binder:v1 -->\n- [Chapter 3]()\n")
    result := runParse(t, dir+"/_binder.md")
    if !result.OK {
        t.Fatalf("expected exit 0\nstdout: %s\nstderr: %s", result.Stdout, result.Stderr)
    }
    if !strings.Contains(result.Stdout, `"title":"Chapter 3"`) {
        t.Errorf("expected node with title 'Chapter 3', got: %s", result.Stdout)
    }
    if strings.Contains(result.Stdout, `"target"`) {
        t.Errorf("expected no target field for placeholder, got: %s", result.Stdout)
    }
    if !strings.Contains(result.Stdout, `"diagnostics":[]`) {
        t.Errorf("expected no diagnostics, got: %s", result.Stdout)
    }
}

// Scenario: A bare placeholder []() with no title parses without error.
func Test_A_bare_placeholder_with_no_title_parses_without_error(t *testing.T) {
    dir := t.TempDir()
    writeFile(t, dir, "_binder.md", "<!-- prosemark-binder:v1 -->\n- []()\n")
    result := runParse(t, dir+"/_binder.md")
    if !result.OK {
        t.Fatalf("expected exit 0\nstdout: %s\nstderr: %s", result.Stdout, result.Stderr)
    }
    if !strings.Contains(result.Stdout, `"type":"node"`) {
        t.Errorf("expected a placeholder node, got: %s", result.Stdout)
    }
    if strings.Contains(result.Stdout, `"target"`) {
        t.Errorf("expected no target field, got: %s", result.Stdout)
    }
    if !strings.Contains(result.Stdout, `"diagnostics":[]`) {
        t.Errorf("expected no diagnostics, got: %s", result.Stdout)
    }
}

// Scenario: Multiple identical placeholder items each appear as a distinct node.
func Test_Multiple_identical_placeholder_items_each_appear_as_a_distinct_node(t *testing.T) {
    dir := t.TempDir()
    writeFile(t, dir, "_binder.md",
        "<!-- prosemark-binder:v1 -->\n- [Chapter]()\n- [Chapter]()\n")
    result := runParse(t, dir+"/_binder.md")
    if !result.OK {
        t.Fatalf("expected exit 0\nstdout: %s\nstderr: %s", result.Stdout, result.Stderr)
    }
    if count := strings.Count(result.Stdout, `"title":"Chapter"`); count != 2 {
        t.Errorf("expected 2 nodes with title 'Chapter', got %d: %s", count, result.Stdout)
    }
    if !strings.Contains(result.Stdout, `"diagnostics":[]`) {
        t.Errorf("expected no diagnostics (no BNDW003), got: %s", result.Stdout)
    }
}
```

---

### US2 — Placeholder nodes with children

**File**: `generated-acceptance-tests/003-placeholder-parsing-US2-placeholder-nodes-with-children_test.go`

```go
// Scenario: A placeholder parent can contain a file-backed child node.
func Test_A_placeholder_parent_can_contain_a_file_backed_child_node(t *testing.T) {
    dir := t.TempDir()
    writeFile(t, dir, "_binder.md",
        "<!-- prosemark-binder:v1 -->\n- [Part I]()\n  - [chapter-one.md](chapter-one.md \"Chapter One\")\n")
    writeFile(t, dir, "chapter-one.md", "")
    result := runParse(t, dir+"/_binder.md")
    if !result.OK {
        t.Fatalf("expected exit 0\nstdout: %s\nstderr: %s", result.Stdout, result.Stderr)
    }
    if !strings.Contains(result.Stdout, `"title":"Part I"`) {
        t.Errorf("expected placeholder parent node, got: %s", result.Stdout)
    }
    if !strings.Contains(result.Stdout, `"target":"chapter-one.md"`) {
        t.Errorf("expected child node with target, got: %s", result.Stdout)
    }
}

// Scenario: Placeholder nodes can be nested three levels deep.
func Test_Placeholder_nodes_can_be_nested_three_levels_deep(t *testing.T) {
    dir := t.TempDir()
    writeFile(t, dir, "_binder.md",
        "<!-- prosemark-binder:v1 -->\n- [Level 1]()\n  - [Level 2]()\n    - [Level 3]()\n")
    result := runParse(t, dir+"/_binder.md")
    if !result.OK {
        t.Fatalf("expected exit 0\nstdout: %s\nstderr: %s", result.Stdout, result.Stderr)
    }
    for _, title := range []string{"Level 1", "Level 2", "Level 3"} {
        if !strings.Contains(result.Stdout, fmt.Sprintf(`"title":%q`, title)) {
            t.Errorf("expected node %q in output, got: %s", title, result.Stdout)
        }
    }
}
```

> Note: the `fmt.Sprintf(`"title":%q`, title)` form produces `"title":"Level 1"` with correct JSON quoting.

---

### US3 — Placeholder round-trip

**File**: `generated-acceptance-tests/003-placeholder-parsing-US3-placeholder-round-trip_test.go`

The round-trip is tested at the library level via the conformance stability suite. The CLI (`pmk parse`) outputs JSON, not the serialized binder format, so it cannot directly demonstrate byte-for-byte stability. The acceptance test delegates to `runConformance` with a filter that covers fixtures 121–125:

```go
// Scenario: A binder with placeholder items serializes identically to its input.
func Test_A_binder_with_placeholder_items_serializes_identically_to_its_input(t *testing.T) {
    // Delegates to the conformance ParseStability suite for fixture dirs 121–125.
    // Fixture names start with "12" — filter matches 120–129 but only 121–125 exist.
    result := runConformance(t, "TestConformance_ParseStability/12")
    if !result.OK {
        t.Fatalf("round-trip stability failed for placeholder fixtures\nstdout: %s\nstderr: %s",
            result.Stdout, result.Stderr)
    }
}
```

---

### US4 — No spurious diagnostics

**File**: `generated-acceptance-tests/003-placeholder-parsing-US4-no-spurious-diagnostics_test.go`

```go
// Scenario: A placeholder node does not trigger a missing-file diagnostic.
func Test_A_placeholder_node_does_not_trigger_a_missing_file_diagnostic(t *testing.T) {
    dir := t.TempDir()
    writeFile(t, dir, "_binder.md", "<!-- prosemark-binder:v1 -->\n- [Planned]()\n")
    // Project has no file named "Planned" — BNDW004 must NOT fire for the placeholder.
    result := runParse(t, dir+"/_binder.md")
    if !result.OK {
        t.Fatalf("expected exit 0\nstdout: %s\nstderr: %s", result.Stdout, result.Stderr)
    }
    if !strings.Contains(result.Stdout, `"diagnostics":[]`) {
        t.Errorf("expected no BNDW004 for placeholder, got: %s", result.Stdout)
    }
}

// Scenario: Two identical placeholder items do not trigger a duplicate diagnostic.
func Test_Two_identical_placeholder_items_do_not_trigger_a_duplicate_diagnostic(t *testing.T) {
    dir := t.TempDir()
    writeFile(t, dir, "_binder.md",
        "<!-- prosemark-binder:v1 -->\n- [Draft]()\n- [Draft]()\n")
    result := runParse(t, dir+"/_binder.md")
    if !result.OK {
        t.Fatalf("expected exit 0\nstdout: %s\nstderr: %s", result.Stdout, result.Stderr)
    }
    if !strings.Contains(result.Stdout, `"diagnostics":[]`) {
        t.Errorf("expected no BNDW003 for identical placeholders, got: %s", result.Stdout)
    }
}
```

---

## TDD Order

Follow Red → Green → Refactor for each step:

1. **Acceptance Red**: Run `just acceptance` — all 4 spec stubs must emit `t.Skip`.
2. **Unit Red**: Add failing `TestParseLink` case for `[Title]()` — expect `found=true, target="", title="Title"`.
3. **Unit Green**: Add `emptyTargetLinkRE`, add `found bool` to `parseLink` signature, add empty-target branch first, mark all other branches `found=true`.
4. **Unit Red**: Add failing parse-loop test (placeholder node created in tree).
5. **Unit Green**: Capture `found` in loop, derive `isPlaceholder := found && target == ""`, change continuation guard to `!found`, change skip to `!found`.
6. **Unit Red**: Add failing BNDW007 suppression test (placeholder should not emit BNDW007).
7. **Unit Green**: Guard non-md check, percent-decode/validateTarget with `!isPlaceholder`.
8. **Unit Red**: Add failing BNDW003 / BNDW004 suppression tests.
9. **Unit Green**: Guard BNDW003 and BNDW004/BNDW009 blocks with `!isPlaceholder`.
10. **Conformance fixtures**: Create fixtures 121–125 and confirm `just test` passes.
11. **Acceptance Green**: Bind acceptance test stubs; run `just acceptance`.
12. **Refactor**: Clean up any duplication, ensure GoDoc, run `just check`.

---

## Edge Cases & Error Handling

### Continuation-Line Placeholder Recognition

**Scenario**: A list item whose primary line has no link but whose continuation line (second line, indented) is `[Title]()`.

```markdown
-
  [Chapter 3]()
```

**Risk**: The original §1.2 Step C design discards the `found` return from the continuation `parseLink` call (`_`) and only promotes the result when `t != ""`. For a placeholder `t = ""`, so `found` stays `false` and the item is skipped — contradicting the spec edge case "Placeholder link appearing on a continuation line — recognized correctly."

**Resolution**: Capture `tFound` from the continuation call. Add an `else if tFound` branch that sets `found = true` and `title = ti`. Recalculate `isPlaceholder` after the continuation block (see §1.2 Step C updated code).

**`listItemRE` constraint**: `listItemRE = ^(\s*)([-*+]|\d+[.)])\s+(.+)` requires `.+` — at least one character of content after the marker. A completely bare `- ` (dash and space only) does NOT match, so it is never processed as a list item and the continuation block is never reached. The test input therefore must use non-link text on the primary line, e.g.:

```markdown
<!-- prosemark-binder:v1 -->

- placeholder text
  [Chapter 3]()
```

In this input, `- placeholder text` matches `listItemRE` (content = `"placeholder text"`), `parseLink` returns `found=false`, the continuation block runs, and `[Chapter 3]()` is recognized as a placeholder.

**Test coverage required**: `TestParse_PlaceholderContinuationLine` must include a case where the primary line has non-link text and the continuation line is `[Title]()`.

---

### Whitespace-Only Title `[ ]()`

**Scenario**: Author writes `[ ]()` (space between brackets).

**Risk**: `emptyTargetLinkRE` matches, `m[1] = " "`, `unescapeTitle(" ")` returns `" "`. This differs from `[]()` (empty title) in a non-obvious way, and `Node.Title = " "` would serialize as `[ ]()` rather than `[]()`, which is unexpected.

**Resolution**: Apply `strings.TrimSpace` to the captured group before returning title — `strings.TrimSpace(unescapeTitle(m[1]))`. Both `[]()` and `[ ]()` then produce `title = ""`. (Already applied in §1.1 Step C.)

**Test coverage required**: `TestParseLink` case: `[ ]()` → title="", found=true.

---

### Whitespace-Only Target `[Title]( )`

**Scenario**: Author writes `[Title]( )` (space inside parentheses).

**Behavior**: `emptyTargetLinkRE` uses `\(\s*\)`, which matches whitespace-only targets. `[Title]( )` is treated identically to `[Title]()` — a placeholder with title "Title". `inlineLinkRE` uses `[^)"]+` (one-or-more non-empty chars) and does NOT match whitespace-only targets, so `emptyTargetLinkRE` wins.

**Decision**: Accept this behavior as intentional. Whitespace-only targets are semantically equivalent to empty targets and produce placeholder nodes. Document in tests.

**Test coverage required**: `TestParseLink` case: `[Title]( )` → title="Title", target="", found=true (placeholder).

---

### Nested Brackets in Placeholder Title

**Scenario**: `[Chapter [3]]()` — the regex character class `[^\]\\]` stops at the first `]`, capturing "Chapter [3". The trailing `]()` is not consumed; `emptyTargetLinkRE` does NOT match. `inlineLinkRE` also does not match. The item is skipped (not promoted to a placeholder).

**Decision**: Document as a known limitation consistent with existing inline-link behavior. No fix needed — authors should avoid nested unescaped brackets. Escaped form `[Chapter \[3\]]()` would match correctly.

**Test coverage**: Optional; add a comment to `TestParseLink` documenting the known limitation.

---

No prior learnings from `.specify/solutions/` were applicable (index is empty).

Research conducted during plan deepening (2026-03-04, updated 2026-03-04):
- **Exact line numbers confirmed** against `internal/binder/parser.go` — all `~` approximations replaced with precise ranges.
- **`parseLink` has no dedicated unit test** in `parser_test.go`; `TestParseLink` must be created as a new function.
- **Continuation-line call** uses 3-return form `t, ti, ld` — updated to `t, ti, tFound, ld` after `found bool` is added (not `_, ld`; `tFound` is needed for the `else if tFound` branch).
- **BNDW002 check** uses `mdInlineLinkRE` (counts `.md` links); confirmed safe for placeholders (count = 0).
- **Acceptance test file naming**: `generated-acceptance-tests/<feature>-<USN>-<slug>_test.go`.
- **`found` semantics for wikilinks/ref-links**: Diagnostics (BNDE003) are appended to `diags` at line 171, BEFORE the skip check at line 174. Setting `found = true` for failed wikilinks would make `isPlaceholder = true` (found=true, target=""), causing the BNDW003/BNDW004 guards to suppress diagnostics for failed wikilinks and create spurious empty-target nodes. Fix: `found = (target != "")` for wikilinks; `found = true` only inside `if rd, exists` for ref-link branches.
- **Conformance fixtures auto-discovered**: `TestConformance_ParseStability` uses `os.ReadDir` — no manual registration needed for fixtures 121–125.
- **Node.Target and Node.Title use `omitempty`**: Confirmed in `types.go`. Fixture 122 (empty-title placeholder) JSON correctly omits both fields.
- **`parseLink` complete body confirmed** (lines 345–376): The full rewrite for §1.1 Step D is now in the plan with all 5 existing branches + new `emptyTargetLinkRE` branch, showing exact `found` assignment per branch.
- **`isPlaceholder` declaration**: Must use `var isPlaceholder bool` (not `:=`) to allow reassignment after the continuation block without a shadowing compile error.
- **"Bare list marker" continuation test**: `listItemRE` requires `.+` after the marker, so `- ` alone never matches. Continuation-line placeholder test must use non-link text on the primary line (e.g., `- placeholder text\n  [Chapter 3]()`).
- **`unescapeTitle` exists** at parser.go lines 378–385; converts `\[`/`\]` storage escapes back to display form.
- **Acceptance stubs**: Pipeline auto-generates stubs with `t.Skip("acceptance test not yet bound")`. Binding replaces the skip with real test logic using `writeFile(t, dir, ...)` and `runParse(t, ...)` helpers from `helpers_test.go`.

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
