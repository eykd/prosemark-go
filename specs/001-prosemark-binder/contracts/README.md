# CLI Contracts: Prosemark Binder v1

The normative JSON schemas live in `docs/conformance/v1/schema/`:

| Schema | File | Purpose |
|--------|------|---------|
| Parse result | `parse-result.schema.json` | Output of `pmk parse --json` (root + children) |
| Diagnostics | `diagnostics.schema.json` | Array of diagnostic objects |
| Operation spec | `op-spec.schema.json` | Input op.json for mutations |
| Operation result | `op-result.schema.json` | Output of `pmk <op> --json` |
| Project map | `project.schema.json` | Input project.json |

---

## Parse Command

```
pmk parse --json <binder-path> --project <project-json-path>
```

**stdout** (exit 0 or exit non-zero on OPExx):
```json
{
  "version": "1",
  "root": {
    "type": "root",
    "children": [
      {
        "type": "node",
        "target": "ch1.md",
        "title": "Chapter One",
        "children": []
      }
    ]
  },
  "diagnostics": []
}
```

Note: The `diagnostics` array is appended at the CLI layer on top of `parse-result.schema.json`.

**Exit codes**:
- `0`: parse succeeded (diagnostics may include warnings; no error-severity BNDE codes)
- non-zero: at least one error-severity diagnostic (BNDE001–003)

---

## add-child Command

```
pmk add-child --json <binder-path> --project <project-json-path> \
    --parent <selector> --target <path> --title <text> \
    [--last | --first | --at <n> | --before <sel> | --after <sel>] \
    [--force]
```

**stdout** (op-result.schema.json):
```json
{
  "version": "1",
  "changed": true,
  "diagnostics": []
}
```

Binder file is modified **in-place** at `<binder-path>`.

**Exit codes**:
- `0`: operation succeeded or was skipped (OPW002)
- non-zero: OPExx error; file unchanged

---

## delete Command

```
pmk delete --json <binder-path> --project <project-json-path> \
    --selector <selector> --yes
```

**stdout** (op-result.schema.json): same shape as add-child.

**Exit codes**: same convention.

---

## move Command

```
pmk move --json <binder-path> --project <project-json-path> \
    --source <selector> --dest <selector> \
    [--last | --first | --at <n> | --before <sel> | --after <sel>] \
    --yes
```

**stdout** (op-result.schema.json): same shape as add-child.

**Exit codes**: same convention.

---

## Human-readable mode

When `--json` is absent, commands produce concise human-readable output to stdout/stderr.
The exact format is not normative (not tested by conformance suite).
