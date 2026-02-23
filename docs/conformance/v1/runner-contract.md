# Prosemark Binder Conformance Runner Contract — v1

**Status**: Normative  
**Applies to**: Prosemark Binder format v1

This document defines the protocol any implementation MUST follow when running the v1 conformance suite. It is implementation-agnostic — the runner may be written in any language and may invoke the implementation as a library or subprocess.

---

## 1. Fixture Discovery

The runner walks two fixture trees:

- **Parse domain**: `parse/fixtures/*/` — each immediate subdirectory is a parse fixture
- **Ops domain**: `ops/fixtures/*/*/` — each leaf subdirectory (two levels deep) is an ops fixture

A directory is a **valid parse fixture** if it contains all of:
- `binder.md`
- `project.json`
- `expected-parse.json`
- `expected-diagnostics.json`

A directory is a **valid ops fixture** if it contains all of:
- `input-binder.md`
- `project.json`
- `expected-diagnostics.json`

And additionally either `op.json` (mutation test) or `expected-binder.md` without `op.json` (stability/round-trip test).

Directories missing required files are **skipped** with a warning, not a failure.

---

## 2. Inputs

### Parse Fixture Inputs

| Input | Source |
|-------|--------|
| Binder bytes | `binder.md` read as raw bytes |
| Project file map | `project.json` parsed as JSON |

### Ops Fixture Inputs

| Input | Source |
|-------|--------|
| Input binder bytes | `input-binder.md` read as raw bytes |
| Project file map | `project.json` parsed as JSON |
| Operation specification | `op.json` parsed as JSON (absent for stability tests) |

---

## 3. Execution Steps

### Step 1: Parse Phase (all fixtures)

Invoke the implementation's **parse function** with:
- Binder bytes (from `binder.md` or `input-binder.md`)
- Project file map (from `project.json`)

Collect:
- **Actual parse result** — the parsed node tree as a JSON object conforming to `schema/parse-result.schema.json`
- **Actual parse diagnostics** — zero or more diagnostic objects as a JSON array

### Step 2: Ops Phase (ops fixtures with `op.json` only)

Invoke the implementation's **operation function** with:
- Input binder bytes
- Project file map
- Operation specification from `op.json`

Collect:
- **Actual output binder bytes** — the (possibly mutated) binder file bytes
- **Actual op diagnostics** — zero or more diagnostic objects

For the final diagnostics comparison, merge parse diagnostics and op diagnostics into a single list.

### Step 3: Stability Phase (ops fixtures without `op.json`)

Invoke the implementation's **serialize function** with the parse result from Step 1.

> The serialize function MUST apply serialization rules defined in the operations spec (list marker inheritance, 2-space indent unit per nesting level, CRLF preservation).

Collect:
- **Actual output binder bytes** — the re-serialized binder

---

## 3.5 CLI Subprocess Protocol

When the implementation is invoked as a CLI subprocess (rather than a library),
the conformance runner MUST use the `--json` flag to obtain machine-readable output.

### Parse invocation

    <binary> parse --json <binder-path> --project <project-json-path>

stdout: a single JSON object conforming to `schema/parse-result.schema.json`,
with an additional top-level `diagnostics` array (same items as
`schema/diagnostics.schema.json`).

### Operation invocation

    <binary> <operation> --json <binder-path> --project <project-json-path> [params]

stdout: a single JSON object conforming to `schema/op-result.schema.json`.
The `changed` field indicates whether the binder bytes were modified.
The `diagnostics` array contains all parse and operation diagnostics merged.
The implementation writes the (possibly mutated) binder in-place at `<binder-path>`.

### Exit codes

| Exit code | Meaning |
|-----------|---------|
| `0` | Completed without error; file may or may not have changed; diagnostics may include warnings |
| non-zero | Aborted with error: at least one OPExx diagnostic or an I/O failure; binder file is unchanged |

### Human-readable mode

When `--json` is absent, the implementation SHOULD produce concise human-readable
output. Human-readable output format is not normative.

### Library implementations

Implementations that expose a library API MAY use the JSON schemas as their
data-contract types directly, without defining a subprocess protocol.

---

## 4. Pass/Fail Rules

A fixture **passes** if and only if ALL applicable checks below pass.

### 4.1 Parse Tree Check

**Applies to**: all fixtures.

The actual parse result MUST exactly match the expected parse result in `expected-parse.json`:

- `root.children` arrays are compared recursively, in **document order**
- Each `Node` is compared by `target`, `title`, and `children` (recursively)
- Extra properties in actual output beyond `type`, `target`, `title`, `children` are **ignored**
- `children` array lengths MUST match exactly

### 4.2 Diagnostic Subset Check

**Applies to**: all fixtures.

Every `{severity, code}` pair present in `expected-diagnostics.json` MUST appear at least once in actual diagnostics. This is a **subset requirement** — the expected list is a floor, not an exact match.

Additional rules:
- Unexpected `severity: "error"` diagnostics in actual output cause **failure**
- Unexpected `severity: "warning"` diagnostics are **permitted** (implementations may emit more warnings than the minimum floor)
- When `location` is present in an expected diagnostic, the actual diagnostic with the same `{severity, code}` MUST have a `location` that matches exactly

### 4.3 Diagnostic Ordering

**Informational only** — not a pass/fail criterion.

Conforming implementations SHOULD emit diagnostics ordered by `location.line` ascending, then `location.column` ascending. Fixtures that specify `location` are authored with this ordering.

### 4.4 Mutation Output Check

**Applies to**: ops fixtures where `expected-binder.md` is present.

Actual output binder bytes MUST be **byte-for-byte identical** to `expected-binder.md` content.

### 4.5 No-Change Check

**Applies to**: ops fixtures where `expected-binder.md` is absent.

Actual output binder bytes MUST be **byte-for-byte identical** to `input-binder.md` content (the implementation MUST NOT have modified the file).

### 4.6 Abort Check

**Applies to**: ops fixtures where an OPExx diagnostic is in `expected-diagnostics.json`.

When an error-severity OPE code is expected:
1. The implementation MUST have returned an error / non-zero exit
2. The output binder bytes MUST be byte-for-byte identical to `input-binder.md` (no partial write)

---

## 5. Result Reporting

The runner MUST report each fixture as one of:

| Result | Meaning |
|--------|---------|
| `PASS` | All applicable checks passed |
| `FAIL` | One or more checks failed (report which check and the diff) |
| `SKIP` | Fixture directory was missing required files |
| `ERROR` | Runner itself encountered an unexpected error (e.g. subprocess crash) |

Exit code: `0` if all discovered fixtures pass (or are skipped); non-zero if any fixture fails.

---

## 6. CI Justfile Targets

Implementations MAY add the following targets to their `justfile`:

```makefile
# Validate all fixture JSON files against their schemas (no implementation needed)
conformance-validate:
    # walk parse/fixtures and ops/fixtures, validate each JSON file

# Run all fixtures against the local implementation
conformance-run:
    # invoke runner binary/script, exit non-zero on failure

# Regenerate expected outputs from the reference implementation
conformance-generate:
    # run each fixture through reference impl, write actual outputs as expected
```

---

## 7. Versioning and Stability

- Fixture NNN numbers are **stable** — they are never renumbered
- `expected-parse.json` and `expected-binder.md` contents are considered normative
- Additions to the fixture set are additive and do not break existing runners
- Breaking changes to fixture format or pass/fail rules require a new suite version (`v2/`)
