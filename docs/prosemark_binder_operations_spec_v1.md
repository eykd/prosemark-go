# Prosemark Binder Operations & Transform Semantics v1

This document defines mutation, traversal, and addressing semantics for `_binder.md` under Binder Format v1.

---

## 1. Core Principles

The binder is a concrete syntax tree (CST) containing structural list-item occurrences.

Operations MUST be surgical. Implementations MUST modify only the minimal required structural lines. All other content MUST remain byte-for-byte identical.

Mutations MUST be atomic. If any part of an operation fails validation, the file MUST be left unchanged. Implementations MUST validate all preconditions before writing any bytes.

Lint errors in the binder file do not block operations. Lint is advisory. Operations fail only on their own error conditions.

---

## 2. Node Identity

- A node is a list-item occurrence.
- The first structural link defines the node's `target` (resolved file path).
- Multiple nodes MAY share the same target.
- Node identity for duplicate detection is based solely on resolved file path. Fragments, link syntax, and title do not affect identity.

---

## 3. Node Selectors

### 3.1 Overview

A **node selector** is a string that identifies one or more structural node occurrences in the binder. Selectors are used by all operations to specify which nodes to act on.

### 3.2 Selector Grammar

```
Selector       ::= "." | Segment ( ":" Segment )*
Segment        ::= FileReference [ "[" Index "]" ]
FileReference  ::= <file stem or relative path without .md extension>
Index          ::= <non-negative integer>
```

Segments are separated by `:` (colon). The `/` (forward slash) is reserved for file path separators within a segment.

### 3.3 Segment Resolution

Each segment matches structural nodes by resolved file path.

- A bare stem (e.g., `chapter-03`) matches any node whose target file has that stem, regardless of directory.
- A relative path (e.g., `subfolder/chapter-03`) matches only nodes whose target resolves to that specific relative path.

If a bare stem is ambiguous (matches files in different directories within the same selector scope), the operation MUST fail with an ambiguous selector error. Full relative paths MUST be used to disambiguate.

### 3.4 Index Semantics

An index filters among sibling nodes that match the segment's file reference under the same parent.

- `chapter-03[0]` — the first `chapter-03` child (among siblings sharing that parent).
- `chapter-03[2]` — the third such child.

Indices are zero-based and refer to document order among matching siblings.

When an index is omitted, the segment matches **all** occurrences at that level. A selector without any indices may therefore match multiple nodes.

### 3.5 Multi-Match Behavior

All operations accept multi-match selectors. When a selector matches multiple nodes, the operation is applied to each match in preorder depth-first traversal order.

- The operation applies to all matched nodes.
- OPW001 is always emitted. For CLI implementations, destructive operations (delete, move) MUST prompt for interactive confirmation when multi-match occurs; `--yes` suppresses the prompt. Add-child is not a destructive operation: CLI implementations MUST NOT prompt for confirmation on add-child multi-match; the operation proceeds silently while still emitting OPW001. Library implementations proceed without prompting but MUST still emit OPW001.

**Index scope in multi-segment selectors**: Indices in a selector apply independently at each segment level during tree traversal. `foo[1]:bar` means "among all nodes at the current level matching `foo`, take the second one (zero-based index 1), then within that node's children, match all nodes whose target file stem is `bar`." Only the segment that bears an `[N]` qualifier is narrowed; other segments continue to match all nodes at their respective levels.

### 3.6 Selector Examples

Given the binder:

```markdown
- [Part One](part-one.md)
  - [Chapter 1](chapter-01.md)
  - [Chapter 3](chapter-03.md)
  - [Chapter 3](sub/chapter-03.md)
- [Part Two](part-two.md)
  - [Chapter 3](chapter-03.md)
```

| Selector | Matches |
|---|---|
| `chapter-03` | All four `chapter-03`-stem nodes (bare stem matches any directory) |
| `sub/chapter-03` | The single `sub/chapter-03.md` node (relative path disambiguates by directory) |
| `part-one:chapter-03` | Both `chapter-03`-stem nodes under `part-one.md` (`chapter-03.md` and `sub/chapter-03.md`) |
| `part-one:chapter-03[0]` | First `chapter-03`-stem child (in document order) under `part-one.md` |
| `part-one[0]:chapter-03[0]` | Exactly one node: first `chapter-03`-stem child under first `part-one.md` |
| `part-two:chapter-03` | The single `chapter-03.md` child under `part-two.md` |

### 3.7 Root-Level Selectors

A single-segment selector (e.g., `part-one`) matches nodes at the top level (children of the synthetic root).

A multi-segment selector (e.g., `part-one:chapter-03`) matches the final segment among children of nodes matched by the preceding segments.

### 3.8 Root Selector

The literal string `.` is a reserved selector that matches the **synthetic root** node. It is valid only as a `parent-selector` in add-child operations (inserting top-level children). It is not valid as a source or destination selector for delete or move, and is not valid in multi-segment selectors. Attempting to use `.` as a source selector in `delete` or `move` operations resolves to zero matches and emits OPE001 (SelectorNoMatch). When `.` is used as parent-selector, multi-match semantics do not apply (the root is unique).

### 3.9 Fragment Handling in Selectors

Fragments in node targets carry no structural weight (see format spec Section A.4). When a selector segment matches by file stem or relative path, ALL nodes referencing that file are considered matches regardless of their fragment component. A node referencing `chapter.md#intro` and a node referencing `chapter.md#outro` are both matched by the selector `chapter`.

Index qualifiers (`[N]`) count across all fragment variants: `chapter[0]` means the first node (in document order) whose target file is `chapter.md`, whether it references `chapter.md`, `chapter.md#intro`, `chapter.md#outro`, or any other fragment or no fragment.

---

## 4. Add-Child Operation

### 4.1 Synopsis

```
add-child <parent-selector> <target> --title <title> [position flags] [--force]
```

### 4.2 Parameters

- **parent-selector**: A node selector identifying the parent(s) under which to insert.
- **target**: The relative file path of the new child (must be a `.md` file; a non-`.md` extension emits OPE004; the path must also conform to binder path rules per the format spec Section 4.5, otherwise OPE004 is also emitted).
- **title** (required): The display text for the link.

### 4.3 Behavior

For each node matching `parent-selector`:

1. Check whether a child node with the same resolved target already exists.
2. If a duplicate exists and `--force` is not set → skip (no-op for that parent). A warning MUST be emitted (OPW002 DuplicateSkipped).
3. If a duplicate exists and `--force` is set → insert regardless, creating an additional occurrence.
4. Otherwise → insert a new structural node as a child.

### 4.4 Insertion Position

Position is controlled by the following flags, which are mutually exclusive:

| Flag | Behavior |
|---|---|
| `--last` | Append after the last structural child. **(default)** |
| `--first` | Insert before the first structural child. |
| `--at <index>` | Insert at the given zero-based index among structural children. `--at N` where N equals the current count of structural children is valid and equivalent to `--last` (appends after the last child). OPE008 is raised only when N is strictly greater than the count. |
| `--before <selector>` | Insert immediately before the matched sibling. |
| `--after <selector>` | Insert immediately after the matched sibling. |

Positional parameters count only structural children. Non-structural list items are ignored for positioning purposes.

Insertion occurs at the structurally correct position. Non-structural list items that happen to be adjacent are not displaced; the new node is placed in the correct structural position relative to the structural children.

### 4.5 Serialization

New nodes are always serialized as standard Markdown inline links:

```
- [Title](target.md)
```

**List marker**: Match the last encountered sibling (the sibling immediately above the insertion point in document order). If there is no previous sibling, match the next sibling. If there are no siblings, use `-`. If the previous and next siblings use conflicting marker types (ordered vs. unordered), use the previous sibling's marker type. If only a next sibling exists and it is ordered, use the next sibling's marker type.

**Ordered list markers** (`1.`, `2.`, etc.): use the maximum ordinal among the parent's existing structural children plus 1. If the parent has no structural children, use `1.`. The marker style (period vs. paren) MUST match the immediately preceding sibling's style; if there is no preceding sibling, use the next sibling's style.

**Indentation**: Match the last encountered sibling's indentation character and unit. If the reference sibling uses tab characters for indentation, the new node MUST use tab characters for each indentation level. If the reference sibling uses spaces, use the same number of spaces as the unit. If there is no previous sibling, match the next sibling. If there are no siblings, derive from the parent's nesting depth using a 2-space indent unit (the default).

**Title escaping**: The title text provided by the caller MUST be escaped for use in Markdown link syntax: `[` and `]` characters within the title MUST be backslash-escaped (e.g., `A [note]` becomes `A \[note\]`). Link tooltip attributes (the optional quoted title in `[text](url "tooltip")` syntax) are NOT emitted for new nodes. When moving a node, any tooltip in the original source is preserved as-is.

The serialization resolution order for all marker, style, and indentation attributes follows this priority chain:

1. **Previous sibling exists**: use its marker type, marker style, and indentation unit.
2. **No previous sibling, next sibling exists**: use the next sibling's marker type, marker style, and indentation unit.
3. **No siblings**: use `-` marker and a 2-space indent per nesting level (default).

---

## 5. Delete Operation

### 5.1 Synopsis

```
delete <selector> [--yes]
```

### 5.2 Behavior

For each node matching `selector`:

1. Remove the list item occurrence and its entire nested subtree.
2. If the deleted node's list item contained non-structural content (annotations, free text, task checkboxes), emit a warning noting the destroyed content.
3. If the deletion leaves the parent with an empty sublist, prune the empty sublist.
4. After removing the node and its subtree lines, examine the lines immediately surrounding the gap. Apply blank-line cleanup in this order:
   - **Definition**: A *run* is a maximal sequence of consecutive lines that are empty or contain only whitespace characters.
   - **Rule 1**: A run of two or more blank lines anywhere in the file MUST be collapsed to exactly one blank line.
   - **Rule 2 (EOF exception)**: If the run occurs at the very end of the file (nothing but optional whitespace follows), it MUST be collapsed to zero blank lines (the file MUST NOT end with trailing blank lines).
   - **Rule 3**: Blank lines that existed *before* the deleted node (blank lines appearing immediately before the deleted node's list item in document order) are retained unless Rule 1 or Rule 2 applies to them after the deletion.

Other occurrences referencing the same file remain untouched.

Reference link definitions (`[label]: url`) associated with deleted nodes are NOT removed. See Section 6.4 for details.

### 5.3 Confirmation

In interactive CLI mode, if the selector matches any nodes, the CLI MUST display the matched nodes and prompt for confirmation before proceeding. The `--yes` flag overrides this prompt.

---

## 6. Move Operation

### 6.1 Synopsis

```
move <source-selector> <destination-parent-selector> [position flags] [--yes]
```

Position flags are the same as for add-child (Section 4.4).

### 6.2 Behavior

For each node matching `source-selector`:

1. Remove the node and its nested subtree from the source location.
2. Reinsert at the specified position under the destination parent.

If `source-selector` and `destination-parent-selector` resolve such that the source and destination parent are the same node, the operation is a **reorder** among siblings.

### 6.3 Formatting Rules

- **Relative indentation** within the moved subtree MUST be preserved. Absolute indentation MUST be adjusted to match the destination nesting depth.
- **Link syntax** within the moved subtree MUST be preserved as-is (wikilinks remain wikilinks, inline links remain inline links).
- **List markers** MUST be updated to match the destination context, following the same resolution rules as add-child serialization (Section 4.5): previous sibling → next sibling → defaults. Only the moved node's own marker is updated; the markers and ordinals of nodes within the moved subtree are preserved as-is.

### 6.4 Reference Link Definitions

Reference link definitions (`[label]: url`) are document-global and are NOT moved or deleted along with the nodes that reference them. After a move or delete operation, any reference definitions that were only used by the moved/deleted nodes remain in the document as orphaned definitions. Implementations MAY emit a warning for orphaned definitions but MUST NOT be required to do so. This behavior preserves lossless round-tripping for reference-style links.

### 6.5 Validation

A move MUST fail if it would make a node a descendant of itself (cycle detection). A move where the source node and the destination parent resolve to the same tree occurrence (a node moved to become its own child) is also rejected as OPE003.

### 6.6 Confirmation

In interactive CLI mode, the CLI MUST display the source and destination and prompt for confirmation. The `--yes` flag overrides this prompt.

### 6.7 Cleanup

The source site follows the same cleanup rules as delete (Section 5.2): prune empty sublists, clean up residual blank lines, emit OPW003 (NonStructuralContentDestroyed) when non-structural content existed in the moved node's list item beyond the structural link and its subtree.

---

## 7. Ensure Semantics

Add-child operations are idempotent by default.

Duplicate detection uses resolved file path only (per Section 2). Link syntax, title, and fragment differences do not prevent duplicate detection.

If a matching structural relationship already exists:

- No insertion occurs (unless `--force` is set).
- No reordering of existing children occurs.
- Existing children MUST NOT be moved.

---

## 8. Stability Guarantee

If no structural mutation is requested, writing `_binder.md` MUST produce identical bytes.

Any operation that results in no net structural change (e.g., an add-child where the child already exists and `--force` is not set) MUST also produce identical bytes.

Binder transformations MUST be deterministic and minimal.

---

## 9. Traversal Semantics

> **Informational — not tested by conformance suite.** This section describes traversal semantics for future compile/export operations. Conformance runners MUST NOT test Section 9 behavior. Implementations may implement compile/export traversal as a non-normative extension.

### 9.1 Occurrence Traversal (Compile/Export)

Traversal order for compile/export is binder occurrence order:

- Preorder depth-first traversal of structural nodes.
- Duplicates are included.
- File reuse does not imply deduplication.

### 9.2 Node Graph Traversal

Operations that require unique file processing MAY perform deduplicated traversal by resolved file path.

This behavior is operation-specific and outside binder format scope.

---

## 10. Error Handling

### 10.1 Errors (operation aborts, file unchanged)

- Selector resolves to zero matches (no matching node found).
- Ambiguous bare-stem selector (stem matches files in different directories).
- Move would create a cycle (node moved under its own subtree).
- Insertion target is not a valid `.md` binder path (per format spec Section 4.5).
- Insertion target is `_binder.md` itself.
- Target node is inside a code fence.
- `--before` or `--after` sibling selector resolves to zero matches.
- `--at` index is out of bounds.
- Any I/O or parse failure on `_binder.md`.

Because mutations are atomic, any error aborts the entire operation. No partial writes occur.

### 10.2 Warnings (operation proceeds)

- Selector matches multiple nodes (multi-match warning).
- Add-child skipped due to existing duplicate (no `--force`).
- Non-structural content destroyed during delete or move.
- Empty sublist pruned during delete or move cleanup.

---

## 11. Mutation and Code Fences

Mutations MUST refuse to act on structural nodes detected inside code fences.

If a selector matches a node that appears inside a code fence, the operation MUST fail with OPE006 (NodeInCodeFence). This applies to all three mutation operations: add-child (when the parent selector targets a node inside a code fence), delete, and move.

---

## 12. Conformance

An implementation is Binder Operations v1 compliant if:

- It implements add-child, delete, and move as defined in this specification.
- It respects the selector grammar and multi-match semantics.
- It enforces atomic mutations.
- It satisfies the stability guarantee.
- It passes the Prosemark Binder Operations v1 conformance test suite.

---

## 13. Future Extensions

Future versions MAY introduce:

- Per-occurrence persistent identifiers
- Batch/transaction operations
- Normalization modes
- Additional operations (e.g., rename, bulk restructure)

Such features MUST remain backward compatible with Binder v1 parsing rules and the selector grammar defined here.
