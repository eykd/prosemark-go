# Data Model: Node Identity & Frontmatter Layer

**Feature**: `002-node-identity`
**Phase**: Phase 1 — Design
**Date**: 2026-02-28

---

## Entities

### NodeId

A UUIDv7 value that permanently identifies a node. Immutable after creation.

| Property | Value |
|---|---|
| Type | `string` (type alias in Go) |
| Format | Canonical lowercase hex with hyphens: `xxxxxxxx-xxxx-xxxx-7xxx-xxxxxxxxxxxx` |
| Generation | `github.com/google/uuid` `NewV7()` |
| Example | `0192f0c1-3e7a-7000-8000-5a4b3c2d1e0f` |
| Validation | Parseable by `uuid.Parse()` — any UUID version accepted for user-supplied values |
| Filename | `{id}.md` — filename stem equals the NodeId string |

### Frontmatter

YAML metadata block at the start of every node file. Delimited by `---` on its own line.

| Field | Type | Required | Mutable | Description |
|---|---|---|---|---|
| `id` | string | Yes | No | NodeId — must equal filename stem |
| `title` | string | No | Yes | Display title of the node |
| `synopsis` | string | No | Yes | Short description or summary |
| `created` | string | Yes | No | UTC ISO 8601 creation timestamp (`Z` suffix) |
| `updated` | string | Yes | Yes | UTC ISO 8601 last-edit timestamp (`Z` suffix) |

**Field order in serialized output** (declaration order via `yaml.v3`):
```
id, title (if present), synopsis (if present), created, updated
```

**Example**:
```yaml
---
id: 0192f0c1-3e7a-7000-8000-5a4b3c2d1e0f
title: Chapter One
synopsis: The world before the war.
created: 2026-02-28T15:04:05Z
updated: 2026-02-28T15:04:05Z
---

[body content here]
```

**Initial values at creation**:
- `created` = timestamp at creation time (UTC)
- `updated` = same value as `created` (not left empty)
- `title` = value from `--title` flag (omitted from YAML if empty string)
- `synopsis` = value from `--synopsis` flag (omitted from YAML if empty string)

### NodePart

An enumeration of the aspects of a node that can be edited.

| Value | File | Description |
|---|---|---|
| `draft` | `{uuid}.md` | Main content file (default) |
| `notes` | `{uuid}.notes.md` | Notes companion file (created on demand) |

`synopsis` part is deferred to a future feature.

### AuditDiagnostic

A diagnostic produced by `pmk doctor`.

| Field | Type | Description |
|---|---|---|
| `code` | string | Audit code: AUD001–AUD007 or AUDW001 |
| `message` | string | Human-readable description |
| `path` | string | File or binder reference path (relative to project root) |
| `severity` | string | `"error"` or `"warning"` (not in JSON output; determines exit code) |

**JSON output** (for `pmk doctor --json`): Array of objects with `code`, `message`, `path` (severity excluded from JSON).

### Audit Codes

| Code | Severity | Trigger |
|---|---|---|
| AUD001 | error | Binder references a file that does not exist on disk |
| AUD002 | warning | UUID-pattern file exists in project root but is not referenced in binder |
| AUD003 | error | Same file appears more than once in the binder |
| AUD004 | error | Node file's frontmatter `id` does not match its filename stem |
| AUD005 | error | Required frontmatter field (`id`, `created`, or `updated`) is absent or malformed |
| AUD006 | warning | Node file has valid frontmatter but empty/whitespace-only body |
| AUD007 | error | Node file's YAML frontmatter block is syntactically unparseable |
| AUDW001 | warning | Non-UUID filename linked in binder (backward-compat with Feature 001 projects) |

---

## Project Configuration

### `.prosemark.yml`

Minimum content created by `pmk init`:
```yaml
version: "1"
```

Additional fields are out of scope for this feature.

### `_binder.md` (initialized)

Minimum content created by `pmk init`:
```markdown
<!-- prosemark-binder:v1 -->
```

The pragma comment is required; its absence causes `BNDW001` warnings on all binder operations.

---

## Binder Integration

### Node File Link Format

When `pmk add --new` adds a UUID node to the binder, it creates a standard markdown link:
```markdown
- [Chapter One](0192f0c1-3e7a-7000-8000-5a4b3c2d1e0f.md)
```

This is processed by the existing Feature 001 `AddChild` operation with `Target = "{uuid}.md"` and `Title = "Chapter One"`.

### Backward Compatibility

Non-UUID filename links in the binder (from Feature 001 projects) produce `AUDW001` warnings but do not cause errors. All existing `pmk parse`, `pmk add`, `pmk delete`, and `pmk move` operations continue to work unmodified.

---

## Filesystem Layout

```
{project-root}/
├── _binder.md                                    # Managed outline (Feature 001)
├── .prosemark.yml                                # Project config (Feature 002)
├── 0192f0c1-3e7a-7000-8000-5a4b3c2d1e0f.md     # Node draft file
├── 0192f0c1-3e7a-7000-8000-5a4b3c2d1e0f.notes.md  # Node notes file (on demand)
├── chapter-one.md                                # Legacy non-UUID file (backward compat)
└── ...
```

Doctor scans **only the immediate project root** for orphan UUID files (not subdirectories).

---

## State Transitions

### Node Lifecycle

```
[New] pmk add --new → Created (uuid.md + binder entry)
[Created] pmk edit → Edited (updated timestamp refreshed)
[Created] pmk delete → Removed from binder (uuid.md stays on disk)
```

### `pmk add --new` Atomicity

```
1. Generate UUID
2. Write {uuid}.md with frontmatter
   ↓ (if write fails → abort, nothing to roll back)
3. Call AddChild on binder (in-memory)
4. Write binder atomically
   ↓ (if write fails → delete {uuid}.md, return error)
5. Success
6. [Optional, if --edit] Open $EDITOR
   ↓ (if $EDITOR unset → error, but node persists in valid state)
7. [On editor exit] Refresh updated timestamp
```

### `pmk edit` Sequence

```
1. Check $EDITOR is set (fail fast if not)
2. Find node ID in binder (fail if not found)
3. If --part notes and notes file missing → create notes file
4. Open $EDITOR with target file
5. On editor exit → read frontmatter, update `updated`, write back
```
