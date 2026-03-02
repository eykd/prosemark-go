# Contract: Node Frontmatter

**Feature**: `002-node-identity`
**Version**: 1

---

## Format

Every node draft file (`{uuid}.md`) MUST begin with a YAML frontmatter block delimited by `---` lines:

```
---
{yaml content}
---

{body content}
```

**Opening delimiter**: `---` on its own line, immediately at byte offset 0 of the file.
**Closing delimiter**: `---` on its own line, preceded by a newline.
**Body**: Everything after the closing `---\n` delimiter (may be empty, which triggers AUD006 warning).

---

## Required Fields

| Field | YAML Key | Type | Constraints |
|---|---|---|---|
| Node ID | `id` | string | Must equal the filename stem (UUID format) |
| Created | `created` | string | UTC ISO 8601 with `Z` suffix; immutable after creation |
| Updated | `updated` | string | UTC ISO 8601 with `Z` suffix; refreshed on every edit |

---

## Optional Fields

| Field | YAML Key | Type | Notes |
|---|---|---|---|
| Title | `title` | string | Display name for the node |
| Synopsis | `synopsis` | string | Short description or summary |

---

## Field Order

When serialized by `pmk`, fields appear in this declaration order:
1. `id`
2. `title` (omitted if empty)
3. `synopsis` (omitted if empty)
4. `created`
5. `updated`

---

## Canonical Example (all fields)

```yaml
---
id: 0192f0c1-3e7a-7000-8000-5a4b3c2d1e0f
title: Chapter One
synopsis: The world before the war.
created: 2026-02-28T15:04:05Z
updated: 2026-02-28T15:04:05Z
---

Body content begins here.
```

## Minimal Example (required fields only)

```yaml
---
id: 0192f0c1-3e7a-7000-8000-5a4b3c2d1e0f
created: 2026-02-28T15:04:05Z
updated: 2026-02-28T15:04:05Z
---

```

---

## Violation Codes

| Condition | Code |
|---|---|
| YAML block is syntactically unparseable | AUD007 |
| `id`, `created`, or `updated` absent or empty | AUD005 |
| `id` does not match filename stem | AUD004 |
| Body is empty or whitespace-only | AUD006 (warning) |

---

## Notes File

A node may have an associated notes file `{uuid}.notes.md`. It is **not** required to have YAML frontmatter. The notes file is created on demand by `pmk edit {uuid} --part notes`. The doctor does not audit notes files.
