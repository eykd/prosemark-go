# Research: Prosemark Binder v1

**Branch**: `001-prosemark-binder` | **Date**: 2026-02-23

---

## 1. Parser Strategy

**Decision**: Custom line-scanner parser (source-preserving)

**Rationale**: The conformance suite requires byte-exact mutation output and lossless round-trips.
A third-party CommonMark library would normalize whitespace, reorder attributes, or otherwise
alter non-structural content. A custom line scanner that:
- Tracks each line's original bytes and line-ending sequence
- Records source positions (line, column, byte offset) for all structural nodes
- Preserves non-structural content verbatim

is both simpler (no third-party AST) and correct for this use case.

**Alternatives considered**:
- `github.com/yuin/goldmark` — full CommonMark AST; heavy; normalizes output; cannot guarantee
  byte-exact round-trips. Rejected.
- `github.com/gomarkdown/markdown` — similar issues. Rejected.
- Writing a full CommonMark-compliant parser — excessive; we only need to handle a narrow
  structural extraction problem. Rejected.

---

## 2. Line Ending Handling

**Decision**: Per-line detection and preservation

**Rationale**: CommonMark accepts LF, CRLF, and bare-CR. The spec requires preserving each
line's original ending. Store endings per line during parse so mutations can emit new lines with
the same ending as their surrounding siblings.

**Algorithm**:
1. Read file as raw bytes
2. Split on `\r\n`, `\r`, `\n` (in that precedence order) while recording which sequence
   terminated each line
3. For new lines inserted by mutations, inherit the line ending of the immediately preceding
   sibling (or the file's majority ending if no sibling)

---

## 3. Fenced Code Block Detection

**Decision**: State-machine scan independent of list parsing

**Rationale**: A list item inside a fenced code block must be excluded from the structural tree
(BNDW005). Detection requires tracking open/close fence state across lines.

**Algorithm**:
- Scan for lines matching `/^\s*(`{3,}|~{3,})/` at the start of processing
- Track whether we are inside a fence; a matching close fence (same char, same or greater count,
  no trailing non-whitespace) exits the fence state
- Any list items found inside a fenced region are excluded and generate BNDW005

---

## 4. Wikilink Resolution

**Decision**: Project file map lookup with proximity tiebreak

**Algorithm**:
1. Strip fragment component (`[[foo#heading]]` → stem `foo`)
2. Append `.md` to get the candidate filename
3. Search `project.files` for paths whose `filepath.Base()` equals the candidate filename
4. Zero matches → synthesize `<stem>.md`; emit BNDW004
5. One match → use that path
6. Multiple matches → proximity tiebreak:
   - Count path components (fewer = closer to root = preferred)
   - Remaining ties: byte-order of full relative path (forward slashes)
   - Still ambiguous → emit BNDE003; exclude node from parse result

**Fragment-only wikilinks** (`[[#heading]]`): empty stem is invalid; emit BNDE001.

---

## 5. Reference-Style Link Resolution

**Decision**: Two-pass approach: first collect all reference definitions, then resolve links

**Algorithm**:
1. Pass 1: scan all lines for reference definitions `[label]: url "optional title"`
   (CommonMark allows them anywhere in the document)
2. Pass 2: when a list item contains `[text][label]`, `[text][]`, or `[text]` (shortcut),
   look up the label in the definitions map (case-insensitive per CommonMark)
3. Resolved URL is treated identically to an inline link target

---

## 6. Indentation and Nesting Level Detection

**Decision**: Indentation-unit inference per document

**Algorithm** (matches operations spec §4.5):
1. Track each list item's raw leading whitespace (spaces or tabs)
2. A list item at column 0 is a top-level child of root
3. A list item whose leading whitespace is a strict extension of its parent's is a child
4. On first encounter of nested items, infer the indentation unit:
   - If tabs used → tab-per-level
   - If spaces → count spaces delta between parent and child = indent unit (commonly 2 or 4)
5. Indentation unit is tracked per-sibling-group for serialization of new nodes

---

## 7. Serialization for Mutations

**Decision**: Line-splice approach (operate on the original line array)

**Rationale**: Rather than regenerating the entire file from the AST, mutations:
1. Parse to get the indexed line array and node tree (with line-range metadata)
2. Compute the minimal set of lines to add/remove/replace
3. Splice those lines into the original array
4. Concatenate with original line endings

This guarantees all untouched lines are byte-identical to input, satisfying SC-004.

**New node serialization** (add-child):
- List marker: previous sibling's marker → next sibling's → default (`-`)
- Ordinal for ordered lists: max existing ordinal + 1; marker style (`.` or `)`) from sibling
- Indentation: previous sibling's leading whitespace → next sibling's → parent depth × 2 spaces
- Link format: always `[Escaped Title](target.md)` for new nodes
- Title escaping: `[` → `\[`, `]` → `\]` in the title string

**Moved node serialization**:
- Preserve original link syntax (inline / reference / wikilink)
- Preserve tooltip if present in original
- Update only root list marker and indentation to match destination context
- Preserve relative indentation within the moved subtree

---

## 8. Atomic Mutation Semantics

**Decision**: Validate all preconditions before touching any bytes

**Algorithm**:
1. Parse binder fully
2. Run all selector evaluations and precondition checks, collecting all potential errors
3. If any OPExx error is found → return diagnostics and leave the file unchanged
4. Only if all preconditions pass → perform the splice and write
5. On IO write error → emit OPE009

This satisfies FR-010 (atomic all-or-nothing semantics).

---

## 9. Conformance Runner Integration

**Decision**: Separate `conformance/` package with `runner_test.go`; add `just conformance-run`

**Rationale**: The conformance runner needs to build and invoke the `pmk` binary as a subprocess
(per the runner contract §3.5 CLI Subprocess Protocol). This is an integration test that cannot
run in the same `go test ./...` pass as unit tests (requires a built binary). A separate package
with a `TestMain` that builds the binary first is clean and maintainable.

**Integration with `just test-all`**: `just test-all` already exists and runs both unit and
acceptance tests. We add `just conformance-run` as an additional target and update `just test-all`
to include it.

---

## 10. No New External Dependencies

**Decision**: Implement everything with Go stdlib + existing Cobra

**Rationale**: The parser needs:
- `regexp` for link/wikilink extraction
- `encoding/json` for JSON I/O
- `path/filepath` for OS-portable path operations
- `unicode/utf8` for BOM detection
- `bytes`, `strings`, `sort` for slicing and sorting

All are stdlib. Adding a CommonMark library would introduce maintenance overhead and risk byte-
exact round-trip failures.
