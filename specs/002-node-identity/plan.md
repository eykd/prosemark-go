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

### Timestamp Correctness

- **UTC enforcement**: All `created` and `updated` timestamps must use `time.Now().UTC()` explicitly. Relying on `time.Now()` alone produces local-timezone timestamps on systems where `time.Local` is not UTC. Add a `NowUTC() string` helper in `internal/node` that formats in RFC3339 with `Z` suffix and use it everywhere.
- **Timestamp precision**: Use second-level precision (`time.RFC3339` format: `2006-01-02T15:04:05Z`) for human readability and unambiguous parsing. Sub-second precision is unnecessary and complicates YAML parsing.

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
