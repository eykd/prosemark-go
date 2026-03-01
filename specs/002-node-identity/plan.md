# Implementation Plan: Node Identity & Frontmatter Layer

**Branch**: `002-node-identity` | **Date**: 2026-02-28 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/002-node-identity/spec.md`

---

## Summary

Add stable UUID-based node identity, YAML frontmatter, and the `pmk init`, `pmk add --new`, `pmk edit`, and `pmk doctor` commands to the prosemark CLI. Each node gets a permanent UUIDv7 identity encoded in its filename (`{uuid}.md`) and mirrored in its frontmatter `id` field. A new `internal/node` package provides pure frontmatter parsing and doctor audit logic. The existing Feature 001 binder engine is extended (not replaced) to support `--new` node creation in `pmk add`.

---

## Technical Context

**Language/Version**: Go 1.25.6
**Primary Dependencies**:
- `github.com/spf13/cobra` v1.10.2 (existing)
- `github.com/google/uuid` v1.6.0 (NEW — UUIDv7 generation)
- `gopkg.in/yaml.v3` (NEW — YAML frontmatter parsing)
**Storage**: Filesystem (`{uuid}.md`, `{uuid}.notes.md`, `_binder.md`, `.prosemark.yml`)
**Testing**: `go test ./...` + `just acceptance` (ATDD pipeline with GWT specs)
**Target Platform**: Linux, macOS, Windows (cross-compilable; no CGO)
**Project Type**: Single Go CLI application
**Performance Goals**: `pmk edit` opens correct file in <1s (editor launch latency excluded)
**Constraints**: 100% coverage on non-Impl functions; zero go vet + staticcheck warnings; atomic writes for all file mutations
**Scale/Scope**: Single-user local CLI; project dirs with O(100) node files

---

## Constitution Check

_GATE: Must pass before Phase 0 research. Re-checked after Phase 1 design._

### I. Acceptance Test-Driven Development ✅
- Four user stories (US1–US4) each require a GWT spec file in `specs/002-node-identity/`.
- GWT specs must be created before implementation begins (see Phase Tasks).
- All implementation completes only when the corresponding acceptance tests pass.

### II. Static Analysis and Type Safety ✅
- No new panics introduced; all errors handled explicitly.
- `context.Context` passed through domain functions.
- Interfaces defined by consumers (cmd layer): `AddChildIO`, `DoctorIO`, `EditIO`, `InitIO`.
- `go vet ./...` and `staticcheck ./...` must remain zero-warning.

### III. Code Quality Standards ✅
- GoDoc required on all new exported types, functions, and interfaces.
- Naming: `NodeId` (not `NodeID` — consistent with existing code style check via staticcheck).
- New packages: `internal/node` (domain), not `util` or `common`.
- Package `node` exports `Frontmatter`, `NodePart`, `AuditDiagnostic`, `RunDoctor`.

### IV. Pre-commit Quality Gates ✅
- lefthook hooks unchanged; new code must pass all gates.
- `*Impl` functions (OS calls) excluded from coverage per existing policy.
- `lefthook.yml` and `justfile` must stay in sync on coverage exclusions (no new package exclusions needed — `internal/node` is fully testable).

### V. Warning and Deprecation Policy ✅
- All new dependencies must have zero known vulnerabilities at time of adoption.
- No deferred warnings.

### VI. Go CLI Target Environment ✅
- No CGO. `github.com/google/uuid` and `gopkg.in/yaml.v3` are pure Go.
- Exit code 0 for success, 1 for errors; diagnostics to stderr, output to stdout.
- `path/filepath` used for all path operations.

### VII. Simplicity and Maintainability ✅
- No over-engineering: `NodeRepo` interface is NOT introduced as a separate entity. Filesystem operations are thin `*Impl` functions in `cmd/`, consistent with existing patterns.
- `internal/node` provides only pure functions needed by cmd handlers.
- No speculative fields in `.prosemark.yml` beyond `version: "1"`.

**GATE STATUS: PASSED — No violations requiring justification.**

---

## Project Structure

### Documentation (this feature)

```text
specs/002-node-identity/
├── spec.md              # Feature specification (interviews complete)
├── plan.md              # This file
├── research.md          # Phase 0: technology decisions
├── data-model.md        # Phase 1: entities, state transitions, filesystem layout
├── contracts/
│   ├── node-frontmatter.md    # Frontmatter format contract
│   ├── doctor-diagnostic.json # JSON schema for doctor --json output
│   └── commands.md            # Command interface contracts
└── tasks.md             # Phase 2 output (sp:05-tasks command — NOT created here)
```

### Source Code

```text
internal/
├── binder/                          (existing — Feature 001, no structural changes)
│   ├── types.go
│   ├── parser.go
│   ├── selector.go
│   ├── serializer.go
│   └── ops/
│       ├── addchild.go
│       ├── delete.go
│       └── move.go
└── node/                            (NEW — Feature 002 domain)
    ├── types.go                     # NodeId, Frontmatter, NodePart, AuditCode, AuditDiagnostic
    ├── frontmatter.go               # Pure: ParseFrontmatter, SerializeFrontmatter, ValidateNode
    └── doctor.go                    # Pure: RunDoctor(ctx, DoctorData) []AuditDiagnostic

cmd/
├── root.go                          (MODIFIED: register init, edit, doctor commands)
├── parse.go                         (existing)
├── addchild.go                      (MODIFIED: add --new, --synopsis, --edit flags + node creation)
├── delete.go                        (existing)
├── move.go                          (existing)
├── scan.go                          (existing)
├── init.go                          (NEW: pmk init command)
├── edit.go                          (NEW: pmk edit command)
└── doctor.go                        (NEW: pmk doctor command)

specs/002-node-identity/
├── US1-init-project.txt             (NEW: GWT acceptance spec)
├── US2-create-new-node.txt          (NEW: GWT acceptance spec)
├── US3-edit-node.txt                (NEW: GWT acceptance spec)
└── US4-validate-integrity.txt       (NEW: GWT acceptance spec)

generated-acceptance-tests/
├── 002-node-identity-US1-init-project_test.go         (generated by pipeline)
├── 002-node-identity-US2-create-new-node_test.go      (generated by pipeline)
├── 002-node-identity-US3-edit-node_test.go            (generated by pipeline)
└── 002-node-identity-US4-validate-integrity_test.go   (generated by pipeline)
```

**Structure Decision**: Single-project Go CLI. No new top-level directories. Feature 002 code extends the existing `cmd/` and `internal/` layout without restructuring.

---

## Complexity Tracking

> Constitution Check passed. No violations requiring justification.

---

## Implementation Design

### Package: `internal/node`

Provides pure, IO-free domain logic for node files. All functions are fully unit-testable.

#### `internal/node/types.go`

```go
// NodeId is a UUIDv7 identifier for a prosemark node.
type NodeId = string

// NodePart identifies which file of a node to operate on.
type NodePart string

const (
    NodePartDraft NodePart = "draft"
    NodePartNotes NodePart = "notes"
)

// Frontmatter holds the YAML metadata for a node file.
type Frontmatter struct {
    ID       string `yaml:"id"`
    Title    string `yaml:"title,omitempty"`
    Synopsis string `yaml:"synopsis,omitempty"`
    Created  string `yaml:"created"`
    Updated  string `yaml:"updated"`
}

// AuditCode is a doctor diagnostic code.
type AuditCode string

const (
    AUD001 AuditCode = "AUD001" // referenced file missing
    AUD002 AuditCode = "AUD002" // orphaned UUID file (warning)
    AUD003 AuditCode = "AUD003" // duplicate binder reference
    AUD004 AuditCode = "AUD004" // id/filename mismatch
    AUD005 AuditCode = "AUD005" // required field absent/malformed
    AUD006 AuditCode = "AUD006" // empty body (warning)
    AUD007 AuditCode = "AUD007" // unparseable YAML
    AUDW001 AuditCode = "AUDW001" // non-UUID filename in binder (warning)
)

// AuditSeverity classifies doctor diagnostics.
type AuditSeverity string

const (
    SeverityError   AuditSeverity = "error"
    SeverityWarning AuditSeverity = "warning"
)

// AuditDiagnostic is a single result from pmk doctor.
type AuditDiagnostic struct {
    Code     AuditCode
    Severity AuditSeverity
    Message  string
    Path     string
}
```

#### `internal/node/frontmatter.go`

Pure functions — no IO:
- `ParseFrontmatter(content []byte) (Frontmatter, []byte, error)` — splits file into frontmatter + body; returns error only on unparseable YAML
- `SerializeFrontmatter(fm Frontmatter, body []byte) ([]byte, error)` — serializes frontmatter + body back to file bytes
- `ValidateNode(fm Frontmatter, filenameStem string, body []byte) []AuditDiagnostic` — checks AUD004, AUD005, AUD006
- `IsUUIDFilename(filename string) bool` — returns true if filename matches `{uuid}.md` pattern

#### `internal/node/doctor.go`

Pure function — no IO:

```go
// DoctorData holds pre-loaded data for doctor analysis.
type DoctorData struct {
    BinderSrc    []byte
    UUIDFiles    []string            // UUID-pattern .md filenames in project root
    FileContents map[string][]byte   // nil value = file does not exist
}

// RunDoctor performs all audit checks and returns diagnostics sorted by severity then path.
func RunDoctor(ctx context.Context, data DoctorData) []AuditDiagnostic
```

Internal audit sequence:
1. Parse binder (`binder.Parse`)
2. Walk tree → collect referenced targets; detect duplicates (AUD003)
3. For each referenced target: check existence (AUD001); validate UUID pattern (AUDW001 if not UUID)
4. For each existing UUID-referenced file: parse frontmatter; validate (AUD004, AUD005, AUD006, AUD007)
5. Find UUID orphans: `UUIDFiles` not in referenced set (AUD002)

---

### `cmd/init.go`

```go
type InitIO interface {
    StatFile(path string) (bool, error)       // returns exists bool
    WriteFileAtomic(path, content string) error
}

func NewInitCmd(io InitIO) *cobra.Command
```

Impl default: `fileInitIO` using `os.Stat` and temp+rename atomic write.

---

### `cmd/edit.go`

```go
type EditIO interface {
    ReadBinder(path string) ([]byte, error)
    ReadNodeFile(path string) ([]byte, error)
    WriteNodeFileAtomic(path string, content []byte) error
    CreateNotesFile(path string) error         // creates empty file
    OpenEditor(editor, path string) error      // Impl: exec.Command
}

func NewEditCmd(io EditIO) *cobra.Command
```

Editor flow:
1. Check `$EDITOR` (fail fast if unset)
2. Parse binder; find ID in tree (AUD: node must exist in binder)
3. Resolve file path based on `--part`
4. If notes and missing → CreateNotesFile (Impl)
5. OpenEditor (Impl)
6. ReadNodeFile; ParseFrontmatter; update `Updated`; SerializeFrontmatter; WriteNodeFileAtomic

---

### `cmd/doctor.go`

```go
type DoctorIO interface {
    ReadBinder(path string) ([]byte, error)
    ListUUIDFiles(dir string) ([]string, error)
    ReadNodeFile(path string) ([]byte, bool, error)  // bool = exists
}

func NewDoctorCmd(io DoctorIO) *cobra.Command
```

Handler collects all data → calls `node.RunDoctor` (pure) → outputs diagnostics.

---

### `cmd/addchild.go` (modified)

New flags added: `--new bool`, `--synopsis string`, `--edit bool`.

Extended `AddChildIO` interface adds:
```go
WriteNodeFileAtomic(path string, content []byte) error
DeleteFile(path string) error  // for rollback
OpenEditor(editor, path string) error  // Impl; only called with --edit
```

When `--new`:
1. Validate `--target` if provided (UUID pattern); else generate UUIDv7
2. Build frontmatter from flags
3. WriteNodeFileAtomic (`{uuid}.md`)
4. Call `ops.AddChild` with binder bytes
5. WriteBinderAtomic
   - On binder write failure → DeleteFile(`{uuid}.md`) as rollback
6. Print success message
7. If `--edit`: check `$EDITOR`; OpenEditor; on exit refresh `updated`

---

## New Dependencies

| Library | Purpose | Version |
|---|---|---|
| `github.com/google/uuid` | UUIDv7 generation | v1.6.0 |
| `gopkg.in/yaml.v3` | YAML frontmatter marshal/unmarshal | latest stable (v3.x) |

Both are pure Go (no CGO). Add via `go get`:
```bash
go get github.com/google/uuid@v1.6.0
go get gopkg.in/yaml.v3
```

---

## GWT Acceptance Specs (to be created before implementation)

Four spec files must be created in `specs/002-node-identity/`:

| File | User Story |
|---|---|
| `US1-init-project.txt` | Initialize a Prosemark Project |
| `US2-create-new-node.txt` | Create a New Node with Stable Identity |
| `US3-edit-node.txt` | Edit a Node in the System Editor |
| `US4-validate-integrity.txt` | Validate Project Integrity |

GWT specs must capture the acceptance scenarios from `spec.md` (US1–US4) in domain language only — no code or infrastructure terms.

---

## Phase Sequence

| Phase | Task | Deliverable |
|---|---|---|
| Spec | GWT spec files for US1–US4 | `specs/002-node-identity/US*.txt` |
| Dependencies | Add uuid + yaml.v3 to go.mod | Updated `go.mod`, `go.sum` |
| Domain | `internal/node/types.go`, `frontmatter.go`, `doctor.go` | Pure domain with 100% coverage |
| Commands | `cmd/init.go`, `cmd/edit.go`, `cmd/doctor.go` | New commands wired into root |
| Add Extension | `cmd/addchild.go` (--new, --synopsis, --edit) | Atomic node creation + binder update |
| Acceptance | GWT pipeline passes all US1–US4 specs | `just acceptance` green |
| Quality | All gates pass | `just check` green |

---

## Security Considerations

### Input Validation

- **`--target` path injection**: `--target` must be validated as a bare filename only (no path separators, no `..` components). UUID pattern check alone is insufficient if the value contains `/`. Reject any `--target` value containing `filepath.Separator` or a `.` prefix before UUID validation.
- **`--project` path canonicalization**: All `--project` values must be resolved via `filepath.Abs` + `filepath.Clean` before use. This prevents `../../etc/` traversal and ensures consistent behavior with symlinked project roots.
- **YAML special characters in `--title` / `--synopsis`**: `gopkg.in/yaml.v3` handles marshaling safely, but the serialized output must be round-trip verified in unit tests with inputs containing `:`, `#`, `"`, `'`, `|`, `>`, and leading/trailing whitespace.
- **Flag length limits for `--title` / `--synopsis`**: No length cap risks storing multi-kilobyte strings in YAML frontmatter (e.g., pasting chapter body into `--synopsis` by accident). Enforce: `--title` ≤ 500 characters, `--synopsis` ≤ 2000 characters. Return an error at flag-parse time with message `"--title exceeds maximum length of 500 characters"`. These limits are enforced in the cmd layer before any IO.

### Data Protection

- **Temp file location for atomic writes**: All temp files must be created in the **same directory** as the target file (e.g., `os.CreateTemp(filepath.Dir(target), ".pmk-tmp-*")`). Creating temp files in `/tmp` causes `os.Rename` to fail with a cross-device link error when `/tmp` is on a different filesystem. This applies to `WriteFileAtomic` in all `*Impl` implementations.
- **Temp file permissions**: Temp files should be created with mode `0600` before content is written, matching the security posture of user-owned prose files.

### Destructive Operations

- **`pmk init --force` destroys existing binder**: `--force` overwrites `_binder.md`, silently discarding any existing project tree. Unlike `pmk delete --yes`, there is no explicit confirmation requirement. Mitigate by printing the overwritten file path to stderr as a warning: `Warning: overwriting existing _binder.md at {path}`. The `--force` flag itself serves as the user's confirmation, but the warning ensures it's never silent.

### Path Containment for Doctor File Reads

- **Binder path traversal in `pmk doctor`**: `RunDoctor` reads files referenced by binder link targets. If `_binder.md` contains a crafted link like `../../sensitive-data.md`, the cmd handler joining project dir + target would resolve to a path outside the project directory. Before calling `DoctorIO.ReadNodeFile`, the cmd handler MUST validate that `filepath.Join(projectDir, target)` is contained within `projectDir`: resolve both via `filepath.Abs` + `filepath.Clean`, then verify the result has `projectDir + string(filepath.Separator)` as a prefix. If the resolved path escapes the project boundary, skip the file read and emit AUDW001 with message `"binder link escapes project directory"`.
- **`pmk edit` is not affected** — it validates the ID against the binder and constructs its own path from the UUID + project dir, never using binder link text directly as a file path.

### Terminal Output Safety

- **ANSI injection in diagnostic output**: `pmk doctor`'s human-readable output echoes binder link targets (filenames) into diagnostic messages verbatim. A `_binder.md` crafted with ANSI escape sequences in link targets (e.g., `\x1b[2J` to clear the terminal) could corrupt terminal output when doctor prints diagnostics. Mitigate by sanitizing path values before formatting them into any message: replace non-printable bytes (codepoints < 0x20 or >= 0x7F except printable Unicode) with `?`. This applies to all human-readable stderr and stdout output, not to `--json` mode (which encodes them safely via `json.Marshal`).

### Notes File Permissions

- **`CreateNotesFile` mode not specified**: The plan specifies 0600 for atomic-write temp files, but `CreateNotesFile` (which creates an empty `{uuid}.notes.md`) has no documented permission mode. Implement using `os.OpenFile(path, os.O_CREATE|os.O_EXCL, 0600)` to match the security posture of draft files. Using the default `os.Create` applies 0666 minus umask, which on systems with a permissive umask (e.g., 0022) produces 0644 — world-readable notes files. Use `O_EXCL` to also guarantee the file does not yet exist (making `CreateNotesFile` atomic and safe against TOCTOU).

### JSON Output Schema Compliance

- **`AuditDiagnostic.Severity` conflicts with `doctor-diagnostic.json` schema**: The JSON contract schema (`contracts/doctor-diagnostic.json`) sets `"additionalProperties": false` and requires only `code`, `message`, and `path` fields. The `AuditDiagnostic` struct also has a `Severity` field. If the cmd handler passes the struct directly to `json.Marshal`, the output will include `"Severity"` (or `"severity"`) and fail schema validation. Resolution: define a separate JSON-output type (`DoctorDiagnosticJSON`) containing only `code`, `message`, and `path`, and convert to it before marshaling. Alternatively, tag `Severity` with `json:"-"` to exclude it from marshaling — but then severity is only available via the Go type, not the JSON output. The separate JSON type is preferred for clarity.

---

## Edge Cases & Error Handling

### Node File Pre-existence Check

- **`pmk add --new --target` with existing file**: When `--target {uuid}.md` is explicitly provided, the `--new` flow must check for file existence BEFORE calling `WriteNodeFileAtomic`. If `{uuid}.md` already exists, error immediately with `"error: node file already exists: {uuid}.md"` (exit 1) and do not modify the binder. Without this check, `WriteNodeFileAtomic` (temp+rename) would silently overwrite an existing node, destroying author content. The existence check must be atomic-safe: use `os.OpenFile` with `O_CREATE|O_EXCL` flags to detect the race between check and write. When the UUID is auto-generated (no `--target`), this check is skipped — collision probability is negligible for UUIDv7.
- **`pmk init` with empty or whitespace-only `_binder.md`**: The contracts.md states "`_binder.md` exists (any content) → Error". The `StatFile` existence check correctly catches this case — even a zero-byte file will cause `os.Stat` to succeed. The behavior is: if `_binder.md` exists at all (even empty), refuse without `--force`. This is correct and intentional — an empty file may be the result of a failed prior init. Document explicitly in `cmd/init.go` that the existence check does not inspect content.

### Atomic Write Robustness

- **Rollback failure in `pmk add --new`**: Step 5 of the `--new` flow deletes the node file if binder write fails. If `DeleteFile` itself fails (e.g., concurrent deletion, permission change), the error must be reported to stderr alongside the original binder error — do not swallow it. Output: `"error: binder write failed: {err}; also failed to roll back node file: {rollbackErr}"`. The partial state (orphaned node file) is then detectable by `pmk doctor` via AUD002.
- **Disk full during atomic write**: If `os.CreateTemp` or `file.Write` fails due to `ENOSPC`, the temp file must be cleaned up in a `defer` block before returning the error. Pattern: `defer func() { if err != nil { os.Remove(tmpPath) } }()`.
- **`_binder.md` disappears mid-operation**: If `_binder.md` is deleted between `pmk init`'s existence check and its write, the atomic write will create it fresh (desired behavior). Document this explicitly in `WriteFileAtomic` — it creates-or-replaces, not update-in-place.

### Concurrency

- **Concurrent `pmk edit` on the same node**: No file locking is provided (single-user CLI assumption per spec scale). If two editor sessions are open on the same node, the last close wins and overwrites the other's changes silently. This is acceptable for the stated scale (single-user local CLI), but `pmk doctor` can detect inconsistency post-facto. Document this assumption explicitly in `cmd/edit.go`.
- **`pmk init` race**: Two concurrent `pmk init` calls in the same directory will each check for `_binder.md` absence, then both attempt to write it. The atomic temp+rename idiom means the last writer wins with a consistent file (rename is atomic on POSIX). The earlier writer's content is silently replaced. This is acceptable for single-user local CLI.

### Editor Integration

- **`$EDITOR` non-zero exit code**: If the editor exits with a non-zero code (e.g., user quit without saving in some editors, or editor crashed), the `updated` timestamp must **not** be refreshed. The contract should be: only refresh `updated` if `OpenEditor` returns `nil` (exit code 0). This preserves the "updated = last actual edit" invariant.
- **`$EDITOR` contains arguments**: `$EDITOR` values like `"nano -R"` or `"code --wait"` must be shell-split before exec (split on whitespace; use first token as binary, remainder as prepended args). Use `strings.Fields($EDITOR)` to split, then append the file path. This is the POSIX-conventional behavior for `$EDITOR`.
- **Editor fails to launch**: If `OpenEditor` returns an error (binary not found, not executable), for `pmk edit` the node file already exists unchanged — report the error to stderr, exit 1, but do not modify the file. For `pmk add --new --edit`, the node and binder are already committed in valid state; report the editor launch error to stderr and exit 1. (RESOLVED: exit 1 is correct because the command did not fully succeed — the author expected their editor to open. The node file is valid and reachable via `pmk edit`; the failure is in the tool's execution, not the file state. This resolves the ambiguity noted in contracts.md.)
- **`pmk edit` `updated` write failure after editor exits**: If the editor exits successfully (exit 0) but `WriteNodeFileAtomic` fails while writing the refreshed `updated` timestamp, the node file on disk is unchanged from before the edit session (the editor's own saves are unaffected — the Go write is updating only the frontmatter timestamp). Report to stderr: `"warning: failed to update 'updated' timestamp: {err}"` and exit 1. The prose content is not lost (the editor already saved it). The stale `updated` timestamp is detectable by `pmk doctor` if needed.

### `pmk doctor` Resilience

- **Uninitialized project (no `_binder.md`)**: If `ReadBinder` returns a file-not-found error, `pmk doctor` must not crash with a raw I/O error. Output to stderr: `"error: project not initialized — run 'pmk init' first"` and exit 1. This is a pre-scan failure, not an audit finding, and must be handled in the cmd handler before calling `RunDoctor`.
- **Unreadable `_binder.md`**: If `ReadBinder` returns a permissions error (file exists but is not readable), output to stderr: `"error: cannot read binder: {err}"` and exit 1. Distinguish this from the not-found case so the user knows whether to init or fix permissions.
- **Unreadable UUID file**: If `ReadNodeFile` returns a permission error, emit an AUD diagnostic rather than aborting the scan. Suggested: treat as AUD007 with message `"cannot read file: {err}"` so the file appears in the report without crashing the tool.
- **YAML with deeply nested anchors (YAML bomb)**: `gopkg.in/yaml.v3` does not protect against deeply nested anchor chains by default. Since doctor reads all UUID files, a malformed or adversarially crafted file could cause excessive memory use. Mitigate by applying a file size limit before parsing: files larger than 1 MB should be rejected with AUD007 (`"frontmatter exceeds size limit"`). For a prose writing tool, 1 MB is a generous upper bound for frontmatter.
- **Symlinks in project root**: `ListUUIDFiles` must document whether it follows symlinks. Recommended: do not follow symlinks (use `os.Lstat` to identify regular files only). Symlinked UUID files should be excluded from orphan checks and documented as out of scope.

### Frontmatter Delimiter Ambiguity

- **Multi-line YAML values containing `---`**: The `ParseFrontmatter` function must not use naive line scanning for the closing `---` delimiter. A YAML value that spans multiple lines and includes `---` on a line by itself (e.g., `synopsis: "---"` or a block scalar `synopsis: |-\n  ---`) would cause a line-scanner to prematurely close the frontmatter block, corrupting the parsed output. Implementation MUST use `gopkg.in/yaml.v3`'s decoder in a two-pass approach: (1) find the closing delimiter by scanning after the opening `---\n` with awareness that `---` can appear inside quoted/block scalar values (use yaml.Decoder to attempt parse and report the byte offset consumed), or (2) find the second standalone `---\n` using a state machine that tracks whether scanning is inside a block scalar or flow string. Unit tests MUST cover: `synopsis: "---"`, `synopsis: |-\n  ---\n  other`, title containing `---`.

### Timestamp Format Validation (AUD005)

- **AUD005 must validate field format, not just presence**: "Absent or malformed" must be defined precisely. A timestamp present as `created: 2026-02-28` (valid YAML string but missing time component and Z suffix) passes a presence check but violates the RFC3339+Z contract. `ValidateNode` MUST validate that `created` and `updated` are non-empty strings that parse successfully via `time.Parse(time.RFC3339, value)` and that the string has a `Z` suffix (not `+00:00` or other UTC representations). Any field that is present but not a valid UTC RFC3339 timestamp with `Z` suffix triggers AUD005 with message `"field '{name}' is not a valid UTC timestamp"`.

### Binder Parse Failure in `pmk edit`

- **`binder.Parse` error vs. file-not-found**: The `pmk edit` flow reads then parses `_binder.md` to validate node ID membership. Distinct error cases must produce distinct user-facing messages: (1) file not found → `"error: project not initialized — run 'pmk init' first"`, (2) file exists but unreadable (permissions) → `"error: cannot read binder: {err}"`, (3) file readable but parse fails → `"error: cannot parse binder: {err}"`. Treating case (3) as a raw Go error surface (e.g., `"yaml: unmarshal error"`) is confusing. The cmd handler for `pmk edit` must handle all three cases explicitly before proceeding to ID lookup.

### UUID Case Sensitivity

- **`--target` must be lowercase**: If `--target 0192F0C1-3E7A-7000-8000-5A4B3C2D1E0F.md` (uppercase) is provided, the UUID pattern check must reject it. Emit: `"target must be a valid UUID filename (lowercase hex with hyphens) when --new is set"`. Auto-generated UUIDs from `github.com/google/uuid` are lowercase; `--target` values must match the same canonical form to prevent mixed-case filenames that cause issues on case-insensitive filesystems (macOS HFS+/APFS).
- **`ListUUIDFiles` on case-insensitive filesystems**: `IsUUIDFilename` uses a lowercase-only regex. On macOS (HFS+/APFS case-insensitive), a manually created `{UUID}.md` (uppercase stem) would not match the regex and would not appear in AUD002 orphan checks. This is an acceptable limitation for the current scope — pmk never generates uppercase filenames. Document explicitly in `IsUUIDFilename` that it matches the canonical lowercase form only.

### Notes File Cleanup on Editor Failure

- **New notes file created but editor exits non-zero**: When `pmk edit --part notes` creates a new empty `{uuid}.notes.md` (because it did not previously exist) and the editor subsequently exits with a non-zero code, the empty notes file should be deleted as rollback. An empty notes file with no author content left by a failed edit is clutter. If the delete-on-rollback itself fails, log to stderr: `"warning: failed to clean up notes file after editor error: {err}"` and exit 1. If the notes file already existed before the edit (non-new case), no rollback is performed.

### Partial Init State

- **`_binder.md` present without `.prosemark.yml`**: If a user manually creates `_binder.md` (or has a Feature 001 project) but has no `.prosemark.yml`, running `pmk init` fails with an error (binder exists). The user cannot create the missing `.prosemark.yml` without `--force`, which also overwrites the binder. The error message in this state MUST include a recovery hint: `"error: _binder.md already exists at {path}. Use --force to reinitialize (overwrites existing files)."` This makes the recovery path explicit without requiring documentation lookup.

### Timestamp Correctness

- **UTC enforcement**: All `created` and `updated` timestamps must use `time.Now().UTC()` explicitly. Relying on `time.Now()` alone produces local-timezone timestamps on systems where `time.Local` is not UTC. Add a `NowUTC() string` helper in `internal/node` that formats in RFC3339 with `Z` suffix and use it everywhere.
- **Timestamp precision**: Use second-level precision (`time.RFC3339` format: `2006-01-02T15:04:05Z`) for human readability and unambiguous parsing. Sub-second precision is unnecessary and complicates YAML parsing.

### `pmk init` Handler Logic Gap — `.prosemark.yml` Existence Check

- **High: Silent overwrite of `.prosemark.yml` when only `_binder.md` is absent**: The `InitIO` interface has a single `StatFile(path string) (bool, error)` method, and the plan only describes statting `_binder.md`. When `_binder.md` is absent but `.prosemark.yml` already exists, the handler will unconditionally call `WriteFileAtomic` for both files — overwriting user-modified project config. The contracts table explicitly requires: "`.prosemark.yml` exists but `_binder.md` does not → Create `_binder.md`; leave `.prosemark.yml` unchanged; exit 0." The cmd handler must stat `.prosemark.yml` separately and skip its write when it already exists (unless `--force` is set). Corrected logic:
  1. `StatFile(_binder.md)` — if exists and no `--force`, error.
  2. `WriteFileAtomic(_binder.md, binderContent)` unconditionally (it's either absent or `--force` is set).
  3. `StatFile(.prosemark.yml)` — if exists AND no `--force`, skip writing it.
  4. `WriteFileAtomic(.prosemark.yml, ymlContent)` only if absent or `--force`.

### `pmk edit` — `ParseFrontmatter` Failure After Editor Exit

- **Unspecified error path when editor corrupts frontmatter**: The plan's editor flow (step 6) calls `ParseFrontmatter` on the file content after the editor exits. If the user inadvertently makes the frontmatter YAML syntactically invalid and saves, `ParseFrontmatter` returns an error. The plan specifies behavior for `WriteNodeFileAtomic` failure but not for this prior parse failure. Required handling: if `ParseFrontmatter` returns an error after the editor exits with code 0, emit to stderr `"warning: could not refresh 'updated' timestamp — frontmatter is unparseable after edit: {err}"` and exit 1. Do NOT write anything to the file. The prose content the editor saved is already on disk and is not lost. The invalid frontmatter state is detectable by `pmk doctor` (AUD007). This error path must be covered by a unit test with a corrupted-frontmatter fixture.

### Signal-Interrupted `pmk add --new` Rollback Gap

- **SIGKILL / power-loss leaves orphaned node file — no automatic recovery**: The spec flags this as unresolved: "What happens when the UUID file write succeeds but the process is killed before the binder write completes?" The plan's rollback at step 5 (`DeleteFile`) only executes when the binder write returns a Go error. A `SIGKILL`, OOM kill, or power loss after `WriteNodeFileAtomic` (step 3) but before `WriteBinderAtomic` completes cannot trigger defer-based cleanup. Explicitly document in `cmd/addchild.go`:
  - Signal interruption after node file creation is a known unrecoverable race at the stated scale.
  - The orphaned file is detectable by `pmk doctor` as AUD002 (orphaned UUID file, warning).
  - Recovery: the user manually deletes the orphaned file or links it into the binder via `pmk add --target`.
  - No automatic journal or lock file is introduced — the complexity cost exceeds the benefit for a single-user local CLI at O(100) nodes.

### AUD007 Semantic Conflict for File-Read Permission Errors

- **AUD007 defined as "unparseable YAML" but reused for permission errors**: The plan states to treat `ReadNodeFile` permission errors as AUD007 with message `"cannot read file: {err}"`. However, AUD007's canonical meaning (per the JSON schema example) is `"frontmatter YAML is syntactically invalid: ..."`. Using AUD007 for a permissions-denied scenario misleads users who will search for a YAML syntax error when the actual problem is filesystem permissions. Resolution: use AUD007 only for YAML parse failures (after successful read). For read errors (permissions, I/O), emit AUDW001 with message `"cannot read file: {err}"` (AUDW001 is the existing "warning" catch-all not defined in the formal error set) OR treat it as AUD001 with message `"referenced file not readable: {err}"` (AUD001 already covers "referenced file does not exist" and is close in intent). Document explicitly which code is chosen in `cmd/doctor.go`. Do not introduce AUD008 without updating `contracts/doctor-diagnostic.json` and the JSON schema enum.

---

## Performance Considerations

### `pmk doctor` Scale

- **Stated scale**: O(100) node files. At this scale, sequential reads and linear scans are acceptable with no optimization needed.
- **File read budget**: `DoctorIO.ReadNodeFile` reads each UUID file fully into memory. With 100 × 64 KB files (generous estimate), peak memory is ~6 MB — well within budget.
- **File size guard**: Enforce the 1 MB per-file limit described in Edge Cases to prevent memory exhaustion from unexpectedly large files (protects both performance and security).

### UUID Generation

- **`github.com/google/uuid` v1.6.0 UUIDv7**: Generates monotonic UUIDs within the same millisecond using a random counter. If the system clock moves backward (NTP adjustment, DST), the library falls back to a randomly seeded counter rather than guaranteeing monotonicity across backward jumps. For a local CLI generating one UUID per invocation, this is not a problem — document that UUID monotonicity is best-effort, not guaranteed.

### Output Flushing

- **`pmk doctor --json`**: The JSON array is fully assembled in memory before output. At stated scale, this is fine. If scale grows, streaming JSON output would be preferred. No action required now, but avoid buffering patterns that would be hard to convert later (i.e., don't accumulate in a `strings.Builder`; use `json.Marshal` on the slice directly).

### Binder Cycle Guard in `RunDoctor`

- **Malformed binder with circular references could loop indefinitely**: `RunDoctor`'s audit sequence walks the binder tree (step 2) using the Feature 001 `binder.Parse` output. If a corrupted or adversarially crafted `_binder.md` produces a cyclic tree (a node referencing one of its own ancestors in the parsed structure), the tree walk could loop. At O(100) nodes the runtime impact is bounded, but a malformed binder with many self-referential entries could be a DoS vector. `RunDoctor` must maintain a `visited map[string]bool` of seen target filenames during tree traversal and skip any target already in the set (already emitting AUD003 for duplicate references, which naturally acts as the cycle detection). Confirm that the Feature 001 binder parser returns references as a flat list or a tree with no back-edges; if the parser itself can loop, the guard must be placed in the tree walk logic inside `RunDoctor`, not after parsing.
