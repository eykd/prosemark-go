# Feature Specification: Placeholder Node Parsing

**Feature Branch**: `003-placeholder-parsing`
**Created**: 2026-03-04
**Status**: Draft
**Beads Epic**: `prosemark-go-2zj`
**Beads Phase Tasks**:

- implement: `prosemark-go-2zj.1`

## User Scenarios & Testing _(mandatory)_

### User Story 1 — Recognize placeholder nodes in a binder (Priority: P1)

A prose author planning a long-form document wants to sketch the outline before creating actual files. They write `[Chapter 3]()` as a list item in the binder file to represent a planned-but-not-yet-created chapter. The system should parse this as a real node in the outline tree rather than silently ignoring it.

**Why this priority**: This is the core capability. Without it, authors cannot use the binder to plan ahead; all other stories depend on this working first.

**Independent Test**: Parse a binder file containing one or more `[Title]()` items and verify each becomes a node in the parse result with an empty target and the correct title.

**Acceptance Scenarios**:

1. **Given** a binder file with a single list item `- [Chapter 3]()`, **When** the file is parsed, **Then** the result contains a node with title "Chapter 3" and an empty file path.
2. **Given** a binder file with `- []()`, **When** the file is parsed, **Then** the result contains a node with an empty title and an empty file path, with no errors or warnings emitted.
3. **Given** a binder file containing multiple `[Ch 3]()` entries, **When** the file is parsed, **Then** all entries appear as distinct nodes and no duplicate-detection warning is emitted.

---

### User Story 2 — Placeholder nodes with children (Priority: P2)

An author organises their outline so that a placeholder chapter (`[Part I]()`) contains several real or planned child sections. The system should allow placeholder nodes to act as parents in the hierarchy just like any file-backed node.

**Why this priority**: Outlines are hierarchical by nature. Placeholders without nesting support would be limited to flat lists, severely restricting their utility.

**Independent Test**: Parse a binder where a `[Title]()` item has indented children and verify the resulting tree reflects the parent–child relationship.

**Acceptance Scenarios**:

1. **Given** a binder with `- [Part I]()` followed by `  - [chapter-one.md](chapter-one.md "Chapter One")`, **When** parsed, **Then** the tree has a placeholder parent with one file-backed child.
2. **Given** a binder with placeholder nodes nested three levels deep, **When** parsed, **Then** each placeholder appears at the correct depth in the tree.

---

### User Story 3 — Placeholder nodes round-trip through serialization (Priority: P2)

An author who edits the binder file (adding or reordering nodes) expects that saving the file produces exactly the same text as they wrote — including any `[Title]()` placeholder items.

**Why this priority**: Mutation operations (add-child, move, delete) rewrite the binder file. If placeholders are not preserved faithfully, authors will lose their planning text.

**Independent Test**: Parse a binder with placeholders then serialize the result; confirm the output is byte-for-byte identical to the input.

**Acceptance Scenarios**:

1. **Given** a binder file with placeholder items at various nesting levels, **When** parsed and then serialized back to text, **Then** the output is identical to the input.

---

### User Story 4 — No spurious diagnostics for placeholders (Priority: P3)

An author expects that placeholder nodes do not trigger "missing file" or "duplicate target" warnings, since they intentionally have no file path.

**Why this priority**: Spurious warnings would erode trust in the diagnostics system and force authors to ignore or suppress legitimate warnings.

**Independent Test**: Parse a binder with placeholder nodes in a project directory and confirm no BNDW003 (duplicate) or BNDW004 (missing file) diagnostics are emitted for those items.

**Acceptance Scenarios**:

1. **Given** a binder with `- [Ch 3]()` and a real project context, **When** parsed, **Then** no BNDW004 (missing file) diagnostic is emitted for the placeholder.
2. **Given** a binder with two identical entries `- [Ch 3]()`, **When** parsed, **Then** no BNDW003 (duplicate) diagnostic is emitted.

---

### Edge Cases

- `[]()` — empty title AND empty target; parses as a node with both fields empty, no error.
- `[Title]()` inside a code fence — treated as code content, not a structural node (existing BNDW005 behavior unchanged).
- `[Title]()` outside a list item — not recognized as a structural link (existing behavior for non-list content).
- Placeholder link appearing on a continuation line (second line of a multi-line list item) — recognized correctly.
- All list marker variants (`-`, `*`, `+`, `1.`) — all produce placeholder nodes.
- Multiple `[Ch 3]()` entries — all appear as distinct nodes; no duplicate warning.

## Requirements _(mandatory)_

### Functional Requirements

- **FR-001**: The parser MUST recognize `[Title]()` (inline link with empty target) as a structural node, producing a node with an empty file path and the bracket text as the title.
- **FR-002**: The parser MUST recognize `[]()` as a structural node with both title and file path empty, emitting no diagnostic.
- **FR-003**: Placeholder nodes MUST be exempt from file-existence validation (no BNDW004 diagnostic).
- **FR-004**: Multiple placeholder entries with the same bracket text MUST NOT trigger duplicate-detection diagnostics (no BNDW003).
- **FR-005**: Placeholder nodes MUST be allowed to have child nodes nested beneath them at any depth.
- **FR-006**: The serializer MUST preserve placeholder items byte-for-byte when a parsed binder is written back.
- **FR-007**: All list marker styles (`-`, `*`, `+`, ordered) MUST produce placeholder nodes when the link target is empty.
- **FR-008**: Placeholder links in non-structural positions (code fences, non-list lines) MUST NOT be promoted to structural nodes.
- **FR-009**: The parse result for a placeholder node MUST carry an empty file-path field that is distinguishable from "no node" by its presence in the tree.

### Key Entities

- **Placeholder Node**: A binder node with an empty file path. Represents planned content not yet materialised into a file. Participates in the tree hierarchy identically to file-backed nodes.
- **Structural Link**: An inline Markdown link that appears in a list item within a binder file and contributes a node to the parse tree. May have an empty or non-empty file path.

## Success Criteria _(mandatory)_

### Measurable Outcomes

- **SC-001**: All planned unit tests for placeholder parsing pass with zero failures.
- **SC-002**: All new conformance fixture directories (121–125) validate against their expected parse and diagnostics output.
- **SC-003**: All existing conformance fixtures (001–120) continue to pass unchanged after the parser change.
- **SC-004**: No BNDW003 or BNDW004 diagnostics appear for any placeholder node in any parse result.
- **SC-005**: Round-trip serialization produces byte-identical output for binder files containing placeholder nodes.
- **SC-006**: 100% branch coverage is maintained on all non-Impl parser functions after the change.

## Assumptions

- The `Node` struct's existing `Target string` field correctly handles the empty-string case; no new types are needed.
- The serializer is line-preserving and does not need modification to round-trip placeholder items.
- `EvalSelector()` already handles nodes with empty targets correctly (placeholders unmatchable by path, matchable by title).
- The delete and move operations work on any node regardless of target and do not need modification.

## Interview

### Open Questions

_(none — spec is complete based on detailed plan in `docs/plan/01-placeholder-parsing.md`)_

### Answer Log

_(none needed)_
