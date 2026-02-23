# Prosemark Binder Diagnostic Code Registry — v1

All diagnostic codes emitted by a conforming Prosemark Binder implementation. Codes are organized by domain and severity.

**Code format**: `[BND|OP][E|W][0-9]{3}`
- `BND` — binder parse/lint domain
- `OP` — operation domain
- `E` — error (halts processing or aborts operation)
- `W` — warning (advisory, processing continues)

---

## Parse / Lint Errors (BNDExx)

These are emitted during parsing. When any BNDE code is raised, the parse result is considered invalid and implementations SHOULD return a non-zero exit code.

| Code | Name | Condition |
|------|------|-----------|
| `BNDE001` | IllegalPathChars | Illegal path characters in a link target (§4.5 of format spec). Examples: `<`, `>`, `"`, `\|`, `?`, `*` on Windows-restricted paths. The offending node is excluded from the parse result. |
| `BNDE002` | PathEscapesRoot | Link target resolves outside the project root (e.g. `../../evil.md`). The offending node is excluded from the parse result. |
| `BNDE003` | AmbiguousWikilink | A wikilink bare-stem resolves to two or more files in the project with the same basename but different directories, and the proximity tiebreak does not produce a unique match (§5.5). The offending node is excluded from the parse result. |

---

## Parse / Lint Warnings (BNDWxx)

These are emitted during parsing. Processing continues and a parse result is produced.

| Code | Name | Condition |
|------|------|-----------|
| `BNDW001` | MissingPragma | The file has content but does not contain the `<!-- prosemark-binder:v1 -->` pragma comment anywhere in the file. An empty file does not trigger this warning. |
| `BNDW002` | MultipleStructuralLinks | A single list item contains more than one structural link. Only the first link is treated as structural; additional links in the same item are ignored. |
| `BNDW003` | DuplicateFileReference | Two or more nodes in the binder tree reference the same target file. Both nodes are included in the parse result; they are distinct tree entries. |
| `BNDW004` | MissingTargetFile | A structural link's target file is not present in `project.json`. The node is still included in the parse result. |
| `BNDW005` | LinkInCodeFence | A list item containing a structural link is inside a fenced code block (`` ``` `` or `~~~`). The item is not treated as structural. |
| `BNDW006` | LinkOutsideListItem | A structural link appears in a paragraph, blockquote, heading, or other non-list context. The link is not treated as structural. |
| `BNDW007` | NonMarkdownTarget | A link in a list item points to a non-`.md` file. The item is not treated as structural. |
| `BNDW008` | SelfReferentialLink | A link in a list item targets `_binder.md` itself. The item is not treated as structural. |
| `BNDW009` | CaseInsensitiveMismatch | A link target matches a project file only under case-insensitive comparison (the exact case in the link does not match the file path). The node is included but flagged. |
| `BNDW010` | BOMPresence | A UTF-8 byte order mark (BOM, U+FEFF) was detected at the start of the file. The BOM is stripped before parsing; the rest of the file is parsed normally. |

---

## Operation Errors (OPExx)

These are emitted when an operation cannot be applied. The operation is **aborted** and the binder file is left byte-identical to its input.

| Code | Name | Condition |
|------|------|-----------|
| `OPE001` | SelectorNoMatch | The selector (source, parent, or destination) resolves to zero nodes in the binder tree. |
| `OPE002` | AmbiguousBareStemSelector | A bare-stem selector (e.g. `chapter-03`) matches files in two or more directories in the project, and no proximity tiebreak resolves the ambiguity. |
| `OPE003` | CycleDetected | A move operation would place a node under one of its own descendants, creating a cycle in the tree. |
| `OPE004` | InvalidTargetPath | The `target` parameter for `add-child` contains illegal path characters or otherwise fails validation (§4.5). |
| `OPE005` | TargetIsBinder | The `target` parameter for `add-child` resolves to `_binder.md` itself. |
| `OPE006` | NodeInCodeFence | The selector matches a node that is inside a fenced code block. Structural mutations within code fences are not permitted. |
| `OPE007` | SiblingNotFound | The `--before` or `--after` sibling selector did not match any sibling of the insertion point. |
| `OPE008` | IndexOutOfBounds | The `--at` position index is greater than the number of existing children at the insertion point. |
| `OPE009` | IOOrParseFailure | A file I/O error or parse failure occurred during the operation. The error message SHOULD include the underlying cause. |

---

## Operation Warnings (OPWxx)

These are emitted during an operation. The operation **proceeds** and the binder file is modified (unless the operation is also a no-op).

| Code | Name | Condition |
|------|------|-----------|
| `OPW001` | MultiMatch | The selector matched more than one node in the binder tree. The operation is applied to all matched nodes. |
| `OPW002` | DuplicateSkipped | An `add-child` operation was skipped for a specific parent because the target node already exists as a child and `--force` was not specified. |
| `OPW003` | NonStructuralContentDestroyed | A `delete` or `move` operation removed a list item that contained non-structural inline content (e.g. annotation text alongside the structural link). The content is permanently lost. |
| `OPW004` | EmptySublistPruned | Removing or moving the last structural child of a node left that node's sublist empty. The empty sublist markup was automatically removed from the binder file. |

---

## Notes

- Codes within a range (e.g. BNDW001–BNDW010) are not necessarily ordered by importance; they are ordered by the section of the spec that describes them.
- Implementations MAY emit additional implementation-defined codes outside this registry, provided they do not use the `BND` or `OP` prefix (use an implementation-specific prefix instead).
- The conformance runner treats unexpected `severity: "error"` codes as test failures; unexpected `severity: "warning"` codes are permitted.
