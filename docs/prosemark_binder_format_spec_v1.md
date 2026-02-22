# Prosemark Binder Format v1

<!-- prosemark-binder:v1 -->

## 1. Status

This document defines the normative format and parsing model for `_binder.md` in Prosemark Binder Format v1.

The binder format is designed for:

- Full CommonMark compatibility
- Obsidian wikilink compatibility
- Lossless round-tripping
- Deterministic structural extraction
- Wide third-party implementability

Mutation semantics (insert, remove, reorder, reparent) are defined in the Binder Operations specification.

---

## 2. Normative References

- CommonMark specification (block + inline parsing) is normative.
- GFM (GitHub Flavored Markdown) extensions are allowed in `_binder.md`. All GFM-specific syntax (e.g., checkbox state in task lists, tables, strikethrough) is non-structural free text. Such content is preserved verbatim but invisible to the structural model.
- Prosemark Wikilink Extension (defined below) is normative.
- This document defines structural extraction and lint behavior.

Implementations MUST parse `_binder.md` using a CommonMark-compliant parser.

---

## 3. File Identification

### 3.1 Project Root

The **project root** is the current working directory of a Prosemark session.

A project MUST contain exactly one `_binder.md`, located at the project root. `_binder.md` files in subdirectories are ignored.

### 3.2 Encoding

`_binder.md` MUST be UTF-8 encoded.

A leading BOM (U+FEFF) MUST be stripped per CommonMark behavior. Lint MAY warn on BOM presence.

### 3.3 Pragma

A binder file SHOULD contain the pragma:

```
<!-- prosemark-binder:v1 -->
```

The pragma MUST be the exact string `<!-- prosemark-binder:v1 -->` with no variation in whitespace, casing, or additional content. It MUST appear on its own line.

The pragma MAY appear anywhere in the file.

If absent, the file is still parsed as Binder v1, but lint SHOULD emit a warning.

---

## 4. Structural Model Overview

### 4.1 Structural Node

A **structural node** is defined as:

> A CommonMark list item containing at least one valid structural link whose target resolves to a `.md` file.

Each list item occurrence defines a distinct node.

Nodes are anonymous outline occurrences.

The first valid structural link in the list item defines the node's content reference.

Multiple nodes MAY reference the same file.

Both ordered and unordered list items are valid structural containers. There is no semantic distinction between them. Node ordering is determined by document order.

### 4.2 Node Properties

Each structural node carries two named properties:

- **`target`**: the resolved file path.
- **`title`**: the display text from the link.

Title is an attribute of the node, not the referenced file. Two nodes referencing the same file may have different titles.

Title derivation rules:

- Markdown link with text: `[My Title](foo.md)` → title is `My Title`.
- Markdown link with empty text: `[](foo.md)` → title is `foo` (filename stem).
- Wikilink with alias: `[[foo|My Title]]` → title is `My Title`.
- Wikilink without alias: `[[foo]]` → title is `foo` (raw target string without `.md`).
- Wikilink with path: `[[subfolder/foo]]` → title is `foo` (filename stem only, directory stripped).

### 4.3 Structural Link

A structural link is either:

1. A standard Markdown inline link:
   `[Title](target.md)`

2. A CommonMark reference-style link:
   `[Title][ref]` with a corresponding link reference definition.

3. An Obsidian wikilink:
   `[[target]]`
   `[[target|Alias]]`
   `[[target#Heading]]`
   `[[target#Heading|Alias]]`

Reference-style links resolve at the CommonMark AST level and are treated identically to inline links for structural purposes.

The first valid structural link in a list item determines the node's `target`.

All subsequent links in the same list item are ignored for structure but preserved verbatim.

If multiple structural links appear in one list item, lint MUST emit a warning.

### 4.4 Valid Structural Link Targets

Only `.md` targets produce structural nodes. A structural link whose target does not resolve to a `.md` file does not create a structural node; lint SHOULD warn when such a link appears in a list item.

A structural link targeting `_binder.md` itself is ignored for structure and does not produce a node. Lint MUST warn on self-referential links.

### 4.5 Binder Path

A **binder path** is a restricted portable path subset. All link targets MUST conform to the following rules:

- Must be a relative path (no leading `/` or drive letter).
- Forward slash `/` is the only path separator.
- No `..` components that resolve above the project root.
- No null bytes or control characters (U+0000–U+001F).
- No characters in the set `< > : " | ? * \`.
- No trailing dots or spaces in path segments.
- UTF-8 encoded.

### 4.6 Structural Links Outside List Items

Structural links occurring outside list items (e.g., in paragraphs, headings, block quotes, or other non-list block containers) are ignored for structure.

Lint MUST warn when a structural link is detected outside a list item.

---

## 5. Outline Structure Extraction

### 5.1 Tree Construction

1. Parse `_binder.md` as CommonMark.
2. Walk the block AST.
3. For each list item block:
   - If it contains ≥1 structural link with a valid `.md` target → create a node.
   - Otherwise ignore for structure.

### 5.2 Nesting Rules

Parent/child relationships are determined strictly by CommonMark list nesting rules.

Nested block elements (paragraphs, headings, sublists, etc.) inside list items are respected exactly as defined by CommonMark.

### 5.3 Synthetic Root

All structural list items occurring at top-level indentation (not nested under another structural node) become children of a synthetic root node.

The binder therefore defines a forest under a synthetic root.

### 5.4 Empty and Degenerate Binder Files

An empty file, a pragma-only file, or a file with no structural links all produce a valid binder with a synthetic root and zero children.

No lint errors or warnings are emitted for empty structural trees.

### 5.5 Wikilink Resolution Algorithm

The wikilink resolution algorithm matches Obsidian's behavior:

1. The search scope is the project root.
2. Match by basename across the project.
3. When multiple files share a basename, prefer the one closest to the linking file (same directory first, then shortest relative path).
4. If still ambiguous after proximity tiebreak, emit a lint error.

---

## 6. Graph Semantics

- Each structural list item occurrence is a distinct node.
- The link target is an attribute of that node.
- Multiple nodes MAY reference the same file.
- The binder structure itself is a tree (under a synthetic root).

The binder does not define cycles; file reuse does not imply structural cycles.

---

## 7. Preservation Requirements

### 7.1 Line Endings

Parsers MUST accept LF, CRLF, and CR per CommonMark.

Implementations MUST preserve original line endings on untouched lines.

### 7.2 General Preservation Rules

Implementations MUST:

- Preserve all original whitespace and indentation for untouched lines.
- Preserve all non-structural content verbatim.
- Preserve structural lines verbatim except where explicitly modified.
- Preserve original line endings on untouched lines.

Implementations MUST NOT:

- Normalize indentation.
- Reformat list markers.
- Reorder nodes implicitly.

---

## 8. Lint Rules

### 8.1 Errors (non-zero exit)

- Illegal path characters in link target (per Section 4.5).
- Link target resolves outside project root.
- Ambiguous wikilink resolution (per Section 5.5).

### 8.2 Warnings

- Missing binder pragma.
- Multiple structural links in one list item.
- Multiple nodes referencing the same file (operates at the file level regardless of fragments; fires even if fragments differ).
- Link target file does not exist.
- Structural node detected inside code fence.
- Structural link detected outside a list item.
- Non-`.md` target in a list item link.
- Self-referential link targeting `_binder.md`.
- Case-insensitive match: link target does not match any file by exact case but matches by case-insensitive comparison.
- BOM presence.

CommonMark-valid but visually surprising nesting MUST NOT be treated as error.

### 8.3 Case Sensitivity

Link target resolution is case-sensitive (normative).

Lint MUST warn when a target does not match any file by exact case but does match by case-insensitive comparison.

---

## 9. Conformance

An implementation is Binder v1 compliant if:

- It parses structural nodes exactly as defined.
- It preserves non-structural text losslessly.
- It passes the Prosemark Binder v1 conformance test suite.

Future revisions MUST version the pragma accordingly.

Mutation semantics are defined in the Binder Operations specification.

---

# Appendix A: Prosemark Structural Link Grammar (EBNF)

This appendix defines the inline link constructs recognized by Prosemark for structural extraction.

Note: This EBNF does NOT define CommonMark list parsing or nesting. CommonMark block parsing is normative for list-item detection and hierarchy.

## A.1 Structural Link (High-Level)

A structural link is either a Markdown inline link, a Markdown reference-style link, or an Obsidian wikilink.

```
StructuralLink ::= MarkdownLink | ReferenceLink | Wikilink
```

Only the FIRST StructuralLink encountered in a list item defines the node's target.

Only links whose resolved target is a `.md` file are valid structural links.

---

## A.2 Markdown Inline Link

This grammar models the subset relevant to binder structural extraction.

```
MarkdownLink ::= "[" LinkText "]" "(" LinkDest [ LinkTitle ] ")"

LinkText      ::= { any character except "]" }

LinkDest      ::= Path [ "#" Fragment ]

Path          ::= { any character except ")" and whitespace }

Fragment      ::= { any character except ")" and whitespace }

LinkTitle     ::= SPACE ( '"' { any character except '"' } '"'
                         | "'" { any character except "'" } "'"
                         | "(" { any character except ")" } ")" )
```

### Reference-Style Links

Reference-style links (full, collapsed, and shortcut forms as defined by CommonMark) resolve at the AST level and are treated identically to inline links for structural purposes.

```
ReferenceLink ::= FullReference | CollapsedReference | ShortcutReference
```

Resolution of the reference label to a link destination follows CommonMark rules. Once resolved, the resulting target and text are processed identically to an inline link.

### Link Title Handling

The optional title component in a Markdown link (e.g., `[Text](target.md "tooltip")`) is silently ignored for structural purposes.

### Target Derivation Rules

- Markdown link targets are resolved relative to the project root.
- Markdown link targets are taken literally — no implicit `.md` appending.
- If `Path` does not end in `.md`, the link does not produce a structural node; lint SHOULD warn.
- Percent-encoding in the target MUST be decoded before path resolution.
- Resolution rules (including path normalization and project-root checks) must follow lint rules in Section 8.

---

## A.3 Obsidian Wikilink

All standard Obsidian wikilink forms are supported.

```
Wikilink ::= "[[" WikilinkTarget "]]"

WikilinkTarget ::= TargetCore [ "|" Alias ]

TargetCore ::= Path [ "#" Fragment ]

Alias ::= { any character except "]" }
```

### Embed Form

Embed syntax is recognized but treated identically for structural purposes.

```
EmbedWikilink ::= "![[" WikilinkTarget "]]"
```

The `!` prefix is stripped for structural extraction. Embed and non-embed links produce identical structural nodes. The `!` prefix is preserved verbatim in the source.

### Target Derivation Rules

Given:

```
[[foo bar]]
```

Target is derived as:

```
foo bar.md
```

If `Path` already ends with `.md`, it MUST NOT be modified.

Fragment and alias components do NOT affect the node's identity target.

### Wikilink Resolution

Wikilink targets are resolved using the algorithm defined in Section 5.5.

---

## A.4 Fragments and Node Identity

Fragment components (`#Heading`) are preserved in the source but carry no structural weight.

The node's identity target is the file path only. Fragments do not distinguish nodes for the purposes of the "multiple nodes referencing the same file" lint warning (Section 8.2).

---

## A.5 Structural Node Predicate

A CommonMark list item defines a structural node if and only if:

1. It contains at least one StructuralLink (MarkdownLink, ReferenceLink, or Wikilink).
2. The StructuralLink target resolves to a syntactically valid binder path (per Section 4.5).
3. The resolved target is a `.md` file.
4. The resolved target is not `_binder.md` itself.

If multiple StructuralLinks occur in the same list item:

- The first is authoritative.
- Lint MUST emit a warning.

---

## A.6 Explicit Non-Goals

This EBNF does NOT attempt to define:

- CommonMark list parsing
- Indentation rules
- Code fence handling
- Block quote behavior
- Full CommonMark inline grammar

These are delegated to the CommonMark specification.
