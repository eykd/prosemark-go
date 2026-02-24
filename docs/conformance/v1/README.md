# Prosemark Binder Conformance Suite — Version 1

This directory contains conformance fixtures and supporting documents for Prosemark Binder v1.

## Fixture Authoring Guide

### Parse Fixtures (`parse/fixtures/`)

Each fixture is a directory named `NNN-kebab-slug` containing:

| File | Required | Description |
|------|----------|-------------|
| `binder.md` | yes | Exact bytes of the `_binder.md` input |
| `project.json` | yes | Files existing in project root (validates against `schema/project.schema.json`) |
| `expected-parse.json` | yes | Expected parser output (validates against `schema/parse-result.schema.json`) |
| `expected-diagnostics.json` | yes | Expected diagnostics; empty array is valid (validates against `schema/diagnostics.schema.json`) |

**Validation example** (using `check-jsonschema`):

```bash
check-jsonschema --schemafile schema/parse-result.schema.json \
    parse/fixtures/NNN-slug/expected-parse.json

check-jsonschema --schemafile schema/diagnostics.schema.json \
    parse/fixtures/NNN-slug/expected-diagnostics.json
```

### Ops Fixtures (`ops/fixtures/<operation>/`)

Each ops fixture is a directory under `ops/fixtures/<operation>/NNN-slug/` containing:

| File | Required | Description |
|------|----------|-------------|
| `input-binder.md` | yes | Input `_binder.md` bytes |
| `project.json` | yes | Project file map |
| `op.json` | yes* | Operation specification (validates against `schema/op-spec.schema.json`) |
| `expected-binder.md` | cond. | Expected output bytes after mutation |
| `expected-diagnostics.json` | yes | Expected diagnostics (OPExx errors and/or OPWxx warnings) |

\*Stability fixtures (under `ops/fixtures/stability/`) may omit `op.json` to indicate a round-trip test.

**When operation aborts** (OPExx error expected): `expected-binder.md` MUST be absent. The runner verifies the implementation left the file byte-identical to the input.

**When operation is a no-op** (e.g. add-child on existing child without `--force`): `expected-binder.md` may be absent (runner asserts no change) or present and byte-identical to `input-binder.md`.

### Selector Syntax

Fixture `op.json` files use selectors to identify nodes. The selector syntax is defined in the operations specification. Common patterns used in fixtures:

- `.` — the synthetic root node
- `foo.md` — full relative target path
- `foo` — bare stem (may trigger OPE002 if ambiguous, or OPW001 if multi-match)

### Diagnostic Matching

See `runner-contract.md` § Pass/Fail Rules for the full matching algorithm. Key rule: expected diagnostics are a **subset requirement** — every `{severity, code}` pair in `expected-diagnostics.json` MUST appear in actual output. Unexpected warnings are permitted; unexpected errors cause failure.

### Numbering

The `NNN` prefix in fixture directory names is a stable, three-digit, zero-padded integer. Do not renumber existing fixtures. Append new fixtures at the highest existing NNN + 1 within the domain.

Fixtures 001–040 are the original parse-domain block; 041–070 are the original ops-domain block. From 071 onward, both domains are interleaved: new edge cases for either domain are appended at the next available number regardless of domain. Fixture numbers do not encode domain membership — the directory path (`parse/fixtures/` vs. `ops/fixtures/`) is authoritative.

## Schema Overview

| Schema File | Validates |
|-------------|-----------|
| `schema/parse-result.schema.json` | `expected-parse.json` |
| `schema/diagnostics.schema.json` | `expected-diagnostics.json` |
| `schema/op-result.schema.json` | Op output (used by runner, not stored in fixture files) |
| `schema/op-spec.schema.json` | `op.json` |
| `schema/project.schema.json` | `project.json` |

**Total fixtures**: 135. All fixtures are implemented.

## Test Catalog Summary

### Parse Domain (001–040, 071, 076–086, 106, 109–112, 115–120)

| Range | Category |
|-------|----------|
| 001–002 | Empty and pragma-only inputs |
| 003–008 | Link syntax variants (inline, reference styles) |
| 009–013 | Wikilink variants (bare, alias, heading, embed, path) |
| 014–017 | List structure and nesting |
| 018–019 | Duplicate and multi-link items |
| 020–021 | Pragma variants |
| 022–025 | Lint errors (BNDE codes) |
| 026–033 | Lint warnings (BNDW codes) |
| 034–040 | Edge cases (line endings, GFM checkboxes, indentation) |
| 071 | Wikilink zero-match (BNDW004 synthesized target) |
| 076–077 | Percent-encoded link targets |
| 078–079 | Code fence edge cases |
| 080 | Non-.md link followed by .md link |
| 081–082 | Pragma position variants |
| 083–084 | GFM extension interactions |
| 085 | Wikilink heading with alias (`[[target#heading\|Alias]]`) |
| 086 | Fragment-only wikilink (`[[#heading]]` → BNDE001 empty-stem path) |
| 106 | Wikilink fragment zero-match: synthesized target strips fragment (`[[nonexistent#section]]` → `nonexistent.md`) |
| 109 | Tilde-fence code block exclusion (BNDW005 with `~~~` delimiter) |
| 110 | Wikilink empty alias (`[[foo\|]]` → title falls back to stem) |
| 111 | Fragment-only wikilink BNDE001 (dedicated conformance fixture) |
| 112 | Fragment zero-match with alias (`[[nonexist#section\|Title]]` → synthesized target, alias wins as title) |
| 115 | Stem derivation with intermediate dots (`chapter.one.md` → stem `chapter.one`) |
| 116 | Duplicate references via different fragment variants (`[[ch#intro]]` + `[[ch#body]]` → BNDW003) |
| 117 | Lint error: control character in link target path (percent-encoded U+0001 decoded → BNDE001) |
| 118 | Lint error: trailing dot in path segment (`section./file.md` → BNDE001) |
| 119 | Inline link with tooltip attribute (tooltip silently ignored; title from link text) |
| 120 | CR-only line endings (U+000D standalone; accepted per §7.1; structural result identical to LF) |

### Ops Domain (041–069, 070, 072–075, 087–108, 113–114, 121–135)

| Range | Operation |
|-------|-----------|
| 041–050 | add |
| 051–055 | delete |
| 056–060 | move |
| 061 | move / cycle-detection |
| 062–067, 070 | ops-error (OPExx abort conditions) |
| 068–069 | stability (round-trip) |
| 072 | delete / multi-match warning |
| 073 | move / multi-match warning |
| 074 | add / ordered-list serialization |
| 075 | add / selector index disambiguation |
| 087–088 | ops-error / OPE009 I/O and parse failure |
| 089  | add / percent-encoded target |
| 090  | add / CRLF line ending preservation |
| 091 | delete / mixed CRLF line endings |
| 092–093  | add / ordered list marker inheritance |
| 094  | add / tab indentation matching |
| 095 | move / reference-link syntax preservation |
| 096 | delete / fragment disambiguation |
| 097  | add / multi-segment indexed selector |
| 098–099 | delete / prune cascade and whitespace cleanup |
| 100 | delete / blank-line reduction before deletion |
| 101  | add / isolated parent default serialization |
| 102 | delete / duplicate node single occurrence |
| 103 | move / deep cycle detection |
| 104 | delete / GFM checkbox content destruction |
| 105  | add / title bracket escaping |
| 107 | ops-error / OPE006 for add (parent selector targets node in code fence) |
| 108 | delete / delete last top-level node (empty structural tree) |
| 113  | add / `--at N` where N equals child count (boundary: valid, appends last) |
| 114  | add / non-sequential ordered-list ordinals (max+1 rule) |
| 121  | add / relative-path selector (`subfolder/chapter-03` targets exact directory) |
| 122  | add / conflicting sibling markers: previous sibling ordered, next unordered → ordered wins |
| 123  | add / indentation inherited from next sibling when `--first` with no previous sibling |
| 124  | add / paren-style ordered marker (`1)`) inherited from next sibling when `--first` |
| 125  | add / lint warnings in binder do not block operations (BNDW001 emitted, op proceeds) |
| 126  | add / idempotent: existing middle child not reordered; OPW002 emitted |
| 127 | delete / reference link definition preserved after deleting node that referenced it |
| 128 | delete / fragment-aware index scope: `ch[1]` deletes second occurrence across fragment variants |
| 129 | move / non-structural content (checkbox) destroyed during move emits OPW003 |
| 130 | move / subtree markers preserved: only moved node's own marker updated; children retain ordinals |
| 131 | move / tooltip in source link preserved verbatim after move |
| 132 | ops-error / OPE004 for non-`.md` target in add |
| 133 | ops-error / OPE001 when root selector `.` used as delete source |
| 134 | ops-error / OPE001 when root selector `.` used as move source |
| 135 | ops-error / OPE006 for move when source node is inside a code fence |
