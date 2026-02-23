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

## Schema Overview

| Schema File | Validates |
|-------------|-----------|
| `schema/parse-result.schema.json` | `expected-parse.json` |
| `schema/diagnostics.schema.json` | `expected-diagnostics.json` |
| `schema/op-result.schema.json` | Op output (used by runner, not stored in fixture files) |
| `schema/op-spec.schema.json` | `op.json` |
| `schema/project.schema.json` | `project.json` |

**Total fixtures**: 76. All fixtures are implemented.

## Test Catalog Summary

### Parse Domain (001–040, 071)

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

### Ops Domain (041–069, 070, 072–075)

| Range | Operation |
|-------|-----------|
| 041–050 | add-child |
| 051–055 | delete |
| 056–060 | move |
| 061 | move / cycle-detection |
| 062–067, 070 | ops-error (OPExx abort conditions) |
| 068–069 | stability (round-trip) |
| 072 | delete / multi-match warning |
| 073 | move / multi-match warning |
| 074 | add-child / ordered-list serialization |
| 075 | add-child / selector index disambiguation |
