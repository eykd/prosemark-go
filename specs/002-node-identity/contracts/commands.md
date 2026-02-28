# Contract: Command Interface

**Feature**: `002-node-identity`
**Version**: 1

---

## `pmk init`

Initialize a new Prosemark project in a directory.

### Synopsis
```
pmk init [--project DIR] [--force]
```

### Flags
| Flag | Default | Description |
|---|---|---|
| `--project DIR` | CWD | Directory to initialize |
| `--force` | false | Overwrite existing initialized files |

### Behavior

| Condition | Outcome |
|---|---|
| No `_binder.md` and no `.prosemark.yml` | Create both files; exit 0 |
| `_binder.md` exists (any content), no `--force` | Error (exit 1); no files modified |
| `.prosemark.yml` exists but `_binder.md` does not | Create `_binder.md`; leave `.prosemark.yml` unchanged; exit 0 |
| `--force` | (Over)write both files; exit 0 |
| No write permissions | Error (exit 1); no partial state |

### Output
- stdout: `Initialized {projectPath}` on success
- stderr: Error message on failure
- No JSON output variant

### Files Created

**`_binder.md`**:
```
<!-- prosemark-binder:v1 -->
```

**`.prosemark.yml`**:
```yaml
version: "1"
```

---

## `pmk add --new` (extends existing `pmk add`)

Create a new UUID node file and register it in the binder atomically.

### Synopsis
```
pmk add --new [--title TITLE] [--synopsis TEXT] [--parent SELECTOR] [--target UUID.md]
              [--edit] [--project DIR]
              [positioning flags: --first, --at N, --before SELECTOR, --after SELECTOR]
```

### New Flags
| Flag | Default | Description |
|---|---|---|
| `--new` | false | Create a new UUID node file instead of linking an existing file |
| `--synopsis TEXT` | "" | Set the synopsis frontmatter field |
| `--edit` | false | Open node file in `$EDITOR` after creation |

### Modified Flag Behavior with `--new`
| Flag | Behavior |
|---|---|
| `--target UUID.md` | If provided, must be a valid UUID filename; used as the node ID. If omitted, a new UUIDv7 is generated. |
| `--title TITLE` | Populates frontmatter `title` field and binder link text. Optional. |
| `--parent SELECTOR` | Parent node in binder (default: `.` for root). |

### Output (stdout)
```
Created {uuid}.md in {binderPath}
```
(Error output follows existing pattern: error to stderr, exit 1.)

### Atomicity Guarantee
If binder write fails after node file is created, the node file is deleted (rolled back) and an error is returned. The binder remains unchanged.

### `$EDITOR` Behavior with `--edit`
- Node file and binder entry are created first.
- If `$EDITOR` is not set, node and binder are retained in valid state; command exits with error.
- If `$EDITOR` is set, editor is opened; on exit, `updated` timestamp is refreshed.

### Validation
- `--target not-a-uuid.md` → error: "target must be a valid UUID filename when --new is set"
- Without `--new`: existing Feature 001 behavior is preserved exactly (no UUID validation, no frontmatter creation)

---

## `pmk edit`

Open a node file in the system editor and refresh the `updated` timestamp on close.

### Synopsis
```
pmk edit ID [--part draft|notes] [--project DIR]
```

### Arguments
| Argument | Description |
|---|---|
| `ID` | Node UUID (positional argument, required) |

### Flags
| Flag | Default | Description |
|---|---|---|
| `--part draft\|notes` | `draft` | Which part of the node to edit |
| `--project DIR` | CWD | Project directory containing `_binder.md` |

### Behavior

| Condition | Outcome |
|---|---|
| `$EDITOR` not set | Error immediately; no files created or modified |
| `ID` not in binder | Error: "node not in binder" |
| `{uuid}.md` missing (for `--part draft`) | Error: "node file missing" |
| `{uuid}.notes.md` missing (for `--part notes`) | Create `{uuid}.notes.md` (empty), then open editor |
| Successful editor exit | Refresh `updated` in frontmatter; write back atomically |

### Output
- On error: message to stderr, exit 1
- On success: no stdout output (editor output is the feedback)

---

## `pmk doctor`

Validate project structural integrity and frontmatter contracts.

### Synopsis
```
pmk doctor [--project DIR] [--json]
```

### Flags
| Flag | Default | Description |
|---|---|---|
| `--project DIR` | CWD | Project directory to audit |
| `--json` | false | Output diagnostics as JSON array |

### Exit Codes
| Condition | Exit Code |
|---|---|
| No errors or warnings | 0 |
| Only warnings (AUD002, AUD006, AUDW001) | 0 |
| Any error-level diagnostic (AUD001, AUD003, AUD004, AUD005, AUD007) | 1 |

### Output (default, human-readable)
```
AUD001 error   referenced file does not exist: 0192f0c1-3e7a-7000-8000-5a4b3c2d1e0f.md
AUD006 warning node file has no body content: 0192f0c1-3e7a-7000-8000-aabbccddeeff.md
```

### Output (`--json`)
JSON array of diagnostic objects (see `doctor-diagnostic.json` schema). Empty array `[]` on clean project.

### Scan Scope
- Binder references: all files linked in `_binder.md` (recursively in tree)
- Orphan scan: immediate project root only (no subdirectory traversal)
- UUID pattern: `{8}-{4}-{4}-{4}-{12}` hex format (any UUID version)
- Non-UUID `.md` files: excluded from orphan check (AUD002 only applies to UUID-pattern files)
