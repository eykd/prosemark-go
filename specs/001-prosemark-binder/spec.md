# Feature Specification: Prosemark Binder v1 Implementation

**Feature Branch**: `001-prosemark-binder`
**Created**: 2026-02-23
**Status**: Draft
**Input**: User description: "Please prepare an implementation of the specs and conformance tests described in docs/ for a pmk CLI tool called prosemark."
**Beads Epic**: `prosemark-go-48x`

**Beads Phase Tasks**:

- clarify: `prosemark-go-48x.1`
- plan: `prosemark-go-48x.2`
- red-team: `prosemark-go-48x.3`
- tasks: `prosemark-go-48x.4`
- analyze: `prosemark-go-48x.5`
- implement: `prosemark-go-48x.6`
- security-review: `prosemark-go-48x.7`
- architecture-review: `prosemark-go-48x.8`
- code-quality-review: `prosemark-go-48x.9`

## User Scenarios & Testing _(mandatory)_

### User Story 1 - Parse Binder File (Priority: P1)

An author has a `_binder.md` file organizing their long-form prose project. They want to extract the structural outline — which files exist, how they're titled, and how they nest — in a machine-readable form.

**Why this priority**: Parsing is the foundational capability. All other operations (add, delete, move) depend on correctly parsing the binder and extracting its structural tree. Conformance testing also begins here.

**Independent Test**: Can be tested by running `pmk parse` against any `_binder.md` file and verifying a JSON tree is produced matching the expected node hierarchy. Delivers value as a standalone "binder introspection" tool.

**Acceptance Scenarios**:

1. **Given** a `_binder.md` with one top-level inline-link list item targeting a `.md` file, **When** the user runs `pmk parse --json`, **Then** the output contains a root node with one child node whose target and title match the link.
2. **Given** a `_binder.md` using Obsidian wikilink syntax `[[chapter]]` and the file exists in the project, **When** parsed, **Then** the output node's title is the stem of the filename and the target resolves to the matching `.md` file.
3. **Given** a `_binder.md` with a reference-style link `[Title][ref]` and a corresponding definition, **When** parsed, **Then** the output node contains the correct title and target.
4. **Given** a `_binder.md` missing the `<!-- prosemark-binder:v1 -->` pragma, **When** parsed, **Then** parsing succeeds but a BNDW001 warning is emitted.
5. **Given** a `_binder.md` with a path containing illegal characters such as `<` or `>`, **When** parsed, **Then** a BNDE001 error is emitted.
6. **Given** a `_binder.md` with a path that escapes the project root via `../`, **When** parsed, **Then** a BNDE002 error is emitted.
7. **Given** a `_binder.md` with a link target that exists only as a case-insensitive match, **When** parsed, **Then** a BNDW009 warning is emitted.
8. **Given** a `_binder.md` with a wikilink stem matching files in multiple directories where proximity tiebreak fails, **When** parsed, **Then** a BNDE003 error is emitted.
9. **Given** a `_binder.md` with links inside a fenced code block, **When** parsed, **Then** those links produce a BNDW005 warning and are excluded from the structural tree.
10. **Given** a `_binder.md` with CRLF or bare-CR line endings, **When** parsed, **Then** the structure is extracted correctly and line endings are preserved in any round-trip output.

---

### User Story 2 - Add a Child Node (Priority: P2)

An author wants to add a new chapter or section to their binder outline, specifying where it should appear relative to existing nodes.

**Why this priority**: The `add-child` operation is the primary way authors grow their outline. It must handle all position variants and serialize consistently with existing formatting conventions.

**Independent Test**: Can be tested by running `pmk add-child` on a binder file and verifying the output file has the new node at the expected position with correct list-marker style and indentation.

**Acceptance Scenarios**:

1. **Given** a binder with one top-level node, **When** `add-child` targets the root (`.`) with a new target and title using default position, **Then** the new node is appended after the existing node.
2. **Given** a binder with several children, **When** `add-child` uses `--first` position, **Then** the new node appears before all existing siblings.
3. **Given** a binder with several children, **When** `add-child --at 1`, **Then** the new node is inserted at zero-based index 1.
4. **Given** a binder with siblings `a`, `b`, `c`, **When** `add-child --before b`, **Then** the new node appears between `a` and `b`.
5. **Given** a binder where the target file already exists as a child and `--force` is not used, **When** `add-child` runs, **Then** the operation is skipped and an OPW002 warning is emitted with the file unchanged.
6. **Given** a binder where the target file already exists as a child, **When** `add-child --force`, **Then** a duplicate node is inserted.
7. **Given** existing siblings using an ordered list with period-style markers, **When** a new node is added, **Then** the new node uses an ordinal following the max+1 rule with the same marker style.
8. **Given** existing siblings indented with tabs, **When** a nested node is added, **Then** the new node uses tab indentation to match.
9. **Given** a title containing bracket characters `[` or `]`, **When** the node is serialized, **Then** those characters are backslash-escaped in the output link text.
10. **Given** a parent selector that matches zero nodes, **When** `add-child` runs, **Then** an OPE001 error is returned and the file is not modified.

---

### User Story 3 - Delete a Node (Priority: P3)

An author wants to remove a chapter or section from their binder outline, along with its entire subtree.

**Why this priority**: Delete is a core mutation needed to clean up the outline. Correctness of cleanup rules (blank-line collapsing, sublist pruning) is essential for the lossless round-trip guarantee.

**Independent Test**: Can be tested by running `pmk delete` on a binder and confirming the target node and its children are removed, blank lines are collapsed, and the file is byte-identical to the expected output.

**Acceptance Scenarios**:

1. **Given** a binder with a leaf node, **When** `delete` targets that node, **Then** the node's list item is removed from the file.
2. **Given** a binder with a node that has children, **When** `delete` targets that node, **Then** the node and all its nested children are removed.
3. **Given** a delete that leaves an empty sublist, **When** the operation completes, **Then** the empty sublist is pruned and an OPW004 warning is emitted.
4. **Given** a binder with two consecutive blank lines created by deletion, **When** the operation completes, **Then** consecutive blank lines are collapsed to one.
5. **Given** a binder node with inline prose content beyond its structural link, **When** deleted, **Then** the non-structural content is also removed and an OPW003 warning is emitted.
6. **Given** a selector that matches zero nodes, **When** `delete` runs, **Then** an OPE001 error is returned and the file is unchanged.
7. **Given** a binder node whose structural link uses reference-style syntax with a definition elsewhere in the file, **When** deleted, **Then** the reference definition is preserved (not deleted).
8. **Given** the last top-level node is deleted, **When** the operation completes, **Then** any trailing blank lines at end-of-file are removed.

---

### User Story 4 - Move a Node (Priority: P4)

An author wants to reorganize their outline by relocating a chapter or section to a different position or parent.

**Why this priority**: Move is essential for outline reorganization. Cycle detection prevents corrupting the tree structure, and the lossless guarantee requires correct cleanup and re-serialization.

**Independent Test**: Can be tested by running `pmk move` and verifying the node and subtree appear at the new location with correct indentation, while the original location is cleaned up per blank-line rules.

**Acceptance Scenarios**:

1. **Given** a binder with two sibling nodes, **When** `move` reorders them, **Then** the source node appears at the target position.
2. **Given** a binder with a node that has children, **When** `move` relocates that node, **Then** the entire subtree moves together.
3. **Given** a move where the destination parent is a descendant of the source node, **When** the operation runs, **Then** an OPE003 cycle-detection error is returned and the file is unchanged.
4. **Given** a node using reference-link syntax, **When** moved, **Then** its link syntax is preserved in the new location.
5. **Given** a node with a tooltip, **When** moved, **Then** the tooltip text is preserved in the new location.
6. **Given** a move into a parent with different indentation convention, **When** the node is placed, **Then** its indentation matches the destination's children.
7. **Given** a node with non-structural inline content, **When** moved, **Then** the non-structural content is destroyed and an OPW003 warning is emitted.

---

### User Story 5 - Conformance Test Suite (Priority: P5)

A developer wants to verify that the `pmk` implementation passes all 135 conformance fixtures defined in the specification.

**Why this priority**: The conformance suite is the authoritative quality gate. Passing all fixtures guarantees the implementation correctly handles every specified edge case across parsing, operations, and stability.

**Independent Test**: Can be tested by running `just acceptance` against all fixtures in `docs/conformance/` and seeing a pass rate of 100% (135/135).

**Acceptance Scenarios**:

1. **Given** the 63 parse fixtures in `docs/conformance/v1/parse/fixtures/`, **When** each fixture's `binder.md` and `project.json` are processed, **Then** the output parse tree and diagnostics match `expected-parse.json` and `expected-diagnostics.json` respectively.
2. **Given** the 72 ops fixtures in `docs/conformance/v1/ops/fixtures/`, **When** each fixture's `input-binder.md` and `op.json` are processed, **Then** the output binder bytes match `expected-binder.md` (if present) or the input (if absent), and diagnostics match `expected-diagnostics.json`.
3. **Given** a stability fixture (no `op.json`), **When** the binder is parsed and re-serialized, **Then** the output is byte-for-byte identical to the input.
4. **Given** a fixture that expects an OPExx error, **When** the operation runs, **Then** the exit code is non-zero and the file is unchanged.
5. **Given** a fixture that expects only OPWxx warnings, **When** the operation runs, **Then** the exit code is 0 and the mutation is applied as expected.
6. **Given** all 135 fixtures run together, **Then** 100% pass with zero failures.

---

### Edge Cases

- What happens when a wikilink matches a file only case-insensitively on a case-sensitive filesystem?
- How does the system handle a `_binder.md` file with a UTF-8 BOM?
- What happens when a wikilink stem has zero matches (file not in project)?
- How are fragment-only wikilinks (`[[#heading]]`) handled?
- What happens when `--at N` is used and N equals the current child count (boundary append)?
- How are CRLF vs LF vs bare-CR line endings preserved in mutations?
- What happens when a binder node is inside a tilde-fence (`~~~`) code block vs a backtick-fence?
- How are percent-encoded paths in inline links handled during parsing and add-child?
- What happens when a link target has intermediate dots (e.g., `chapter.one.md`)?
- How does delete behave when a selector matches the last top-level node?

## Requirements _(mandatory)_

### Functional Requirements

- **FR-001**: The tool MUST parse `_binder.md` files into a structural node tree, recognizing CommonMark inline links, reference-style links (full, collapsed, shortcut), and Obsidian wikilinks as structural nodes when they target `.md` files inside list items.
- **FR-002**: The tool MUST emit diagnostic codes (BNDE001–003, BNDW001–010, OPE001–009, OPW001–004) in structured JSON output that includes severity, code, message, and source location (line, column, byte offset).
- **FR-003**: The tool MUST produce JSON parse output with a version field and a recursive root/node tree where each node has `type`, `target`, `title`, and `children` fields.
- **FR-004**: The tool MUST implement the `add-child` mutation with position flags: `--last` (default), `--first`, `--at <index>`, `--before <selector>`, and `--after <selector>`.
- **FR-005**: The tool MUST implement the `delete` mutation that removes the target node and its entire subtree, applying blank-line collapse and empty-sublist pruning cleanup rules.
- **FR-006**: The tool MUST implement the `move` mutation that relocates a source node (and subtree) under a destination parent at a specified position, with cycle detection that aborts on circular references.
- **FR-007**: The tool MUST resolve Obsidian wikilinks using basename search across the project map, applying proximity (shallowest-path) tiebreak and emitting BNDE003 when no tiebreak is possible.
- **FR-008**: The tool MUST serialize new nodes using Markdown inline-link syntax `[Title](target.md)`, inheriting list-marker style (unordered or ordered), ordinal (max+1), and indentation from existing siblings.
- **FR-009**: The tool MUST implement node selectors: bare stem (`foo`), relative path (`sub/foo`), root (`.`), index-qualified (`foo[0]`), and multi-segment colon-separated (`foo:bar`).
- **FR-010**: The tool MUST apply atomic (all-or-nothing) semantics to all mutations: validate all preconditions before writing any bytes; on any OPExx error, leave the file unchanged.
- **FR-011**: The tool MUST preserve line endings (LF, CRLF, bare-CR) exactly as found in the input file during any mutation.
- **FR-012**: The tool MUST pass all 135 conformance fixtures in `docs/conformance/v1/` with byte-exact mutation output and exact diagnostic-code floor matching.
- **FR-013**: The tool MUST exclude links inside fenced code blocks (backtick and tilde fences) from the structural tree, emitting BNDW005 for each.
- **FR-014**: The tool MUST NOT move or delete reference link definitions when deleting or moving nodes; orphaned definitions may remain in the file.
- **FR-015**: The tool MUST implement the idempotency contract for `add-child`: skip (OPW002) if the same target file is already a child of the parent, unless `--force` is specified.
- **FR-016**: The tool MUST apply title derivation rules: empty bracket title `[](foo.md)` uses stem; wikilink `[[foo]]` uses stem; empty alias `[[foo|]]` uses stem; wikilink `[[subfolder/foo]]` uses leaf stem only.
- **FR-017**: The tool MUST backslash-escape `[` and `]` characters in titles when serializing new nodes as inline links.
- **FR-018**: The tool MUST strip the UTF-8 BOM (U+FEFF) when reading binder files and emit BNDW010.

### Key Entities

- **Binder**: The `_binder.md` file at a project root that encodes the hierarchical outline using CommonMark list syntax.
- **Structural Node**: A CommonMark list item containing at least one valid `.md` link; the first such link determines the node's target and title.
- **Synthetic Root**: A virtual container node holding all top-level structural nodes; addressed by the `.` selector.
- **Diagnostic**: A structured record with severity, code, message, and source location emitted when the binder has issues or an operation encounters a problem.
- **Node Selector**: A string expression that identifies zero or more nodes in the tree (bare stem, relative path, root, indexed, multi-segment).
- **Project Map**: The set of `.md` file paths known to exist in the project, supplied externally and used for wikilink resolution and missing-file diagnostics.

## Success Criteria _(mandatory)_

### Measurable Outcomes

- **SC-001**: All 135 conformance fixtures pass with zero failures on the first run after implementation.
- **SC-002**: Parse output for any binder file is produced in under 100 milliseconds for files up to 10,000 lines.
- **SC-003**: Every mutation operation produces byte-identical output to the corresponding conformance fixture's `expected-binder.md`.
- **SC-004**: Re-serializing a parsed binder without any operation produces output byte-for-byte identical to the original input (all stability fixtures pass).
- **SC-005**: The diagnostic output for every fixture contains all expected codes and zero unexpected error codes.
- **SC-006**: Unit test coverage for all non-I/O functions reaches 100% per project coverage policy.

## Assumptions

- The `pmk` binary is a CLI application using an existing command scaffolding in the repository.
- The conformance runner lives in a new `conformance/` package (separate from the GWT acceptance spec pipeline in `acceptance/`) and is invoked via `just conformance-run`; `just test-all` runs both unit tests and conformance fixtures.
- The project map is supplied as an external JSON input; the tool does not scan the filesystem to discover project files during conformance testing.
- All binder files are encoded in UTF-8 (with optional BOM handled via BNDW010).
- The tool is single-user and single-process; concurrent writes to the same binder file are out of scope.
- The `pmk parse` command outputs JSON to stdout; mutations write the modified binder in-place.
- Test fixtures in `docs/conformance/` are normative and serve as the acceptance criteria for implementation correctness.

## Interview

### Open Questions

*(none)*

### Answer Log

*(none)*
