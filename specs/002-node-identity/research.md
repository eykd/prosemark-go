# Research: Node Identity & Frontmatter Layer

**Feature**: `002-node-identity`
**Phase**: Phase 0 — Research
**Date**: 2026-02-28

---

## 1. UUIDv7 Generation Library

**Decision**: Use `github.com/google/uuid` v1.6.0

**Rationale**:
- The dominant UUID library in the Go ecosystem (104,000+ dependents, 6,000 stars).
- Pure Go — no CGO, cross-compilable for Linux/macOS/Windows.
- `uuid.NewV7()` returns `(UUID, error)` — idiomatic, non-panicking.
- `UUID.String()` produces canonical lowercase hex with hyphens: `0192f0c1-3e7a-7000-8000-5a4b3c2d1e0f`.
- RFC 9562 compliant. Monotonic counter for within-millisecond ordering.
- **NOT in Go standard library** as of Go 1.25 — a third-party dependency is required.

**Alternatives considered**:
- `github.com/gofrs/uuid` — Also pure Go, RFC 9562 compliant, but fewer dependents and no meaningful advantage for this use case.
- `github.com/oklog/ulid` — NOT a UUID library; ULID is a different 26-character Crockford Base32 format. Incompatible with the UUID filename convention.
- Standard library — No UUID package exists in Go 1.25.

**API Usage**:
```go
import "github.com/google/uuid"

id, err := uuid.NewV7()
if err != nil {
    return fmt.Errorf("generate UUID: %w", err)
}
filename := id.String() + ".md" // e.g. "0192f0c1-3e7a-7000-8000-5a4b3c2d1e0f.md"
```

**UUID Validation** (for `--target` with `--new`):
```go
func isValidUUIDFilename(target string) bool {
    if !strings.HasSuffix(target, ".md") {
        return false
    }
    stem := strings.TrimSuffix(target, ".md")
    _, err := uuid.Parse(stem)
    return err == nil
}
```

---

## 2. YAML Frontmatter Parsing

**Decision**: Use `gopkg.in/yaml.v3` directly; implement frontmatter delimiter parsing with stdlib `strings`.

**Rationale**:
- `yaml.v3` is the canonical YAML library for Go; pure Go, no CGO, widely used.
- Frontmatter delimiter parsing (`--- ... ---`) is trivial with stdlib `strings` — no extra library needed.
- `yaml.Marshal` on a struct serializes fields in declaration order (de facto stable behavior in the entire Go ecosystem, even if not formally documented as a contract). For a fixed 5-field struct this is reliable and sufficient.
- `adrg/frontmatter` library avoided: read-only (no write-back), pulls in yaml.v2 as a conflicting transitive dep.
- `sigs.k8s.io/yaml` avoided: JSON-bridge approach → alphabetical field order; incompatible with desired declaration-order YAML.

**Alternatives considered**:
- `github.com/adrg/frontmatter` — Read-only, adds yaml.v2 transitive dependency. Rejected.
- `sigs.k8s.io/yaml` — Field order alphabetical (JSON bridge). Rejected.
- `github.com/goccy/go-yaml` — Feature-rich but overkill; adds a larger dependency tree. Rejected.

**Frontmatter struct**:
```go
type Frontmatter struct {
    ID       string `yaml:"id"`
    Title    string `yaml:"title,omitempty"`
    Synopsis string `yaml:"synopsis,omitempty"`
    Created  string `yaml:"created"`
    Updated  string `yaml:"updated"`
}
```

**YAML output (declaration order)**:
```yaml
---
id: 0192f0c1-3e7a-7000-8000-5a4b3c2d1e0f
title: Chapter One
synopsis: The world before the war.
created: 2026-02-28T15:04:05Z
updated: 2026-02-28T15:04:05Z
---
```

**Delimiter parsing**: The `---\n` prefix and `\n---\n` closing delimiter are detected with `strings.HasPrefix` / `strings.Index`. No regex needed.

**Handling malformed frontmatter**:
- No leading `---` → no frontmatter, entire content is body (no error)
- Missing closing `---` → AUD007 (syntactically malformed, cannot parse)
- Invalid YAML inside delimiters → `yaml.Unmarshal` returns error → AUD007
- Missing required fields in valid YAML → AUD005

---

## 3. Timestamps

**Decision**: UTC ISO 8601 with Z suffix via `time.RFC3339`.

```go
time.Now().UTC().Format(time.RFC3339)
// e.g. "2026-02-28T15:04:05Z"
```

`time.RFC3339` = `"2006-01-02T15:04:05Z07:00"`. For UTC time it always emits the `Z` suffix.

---

## 4. Binder Pragma Format

**Finding**: The managed binder block pragma is an HTML comment on its own line:
```
<!-- prosemark-binder:v1 -->
```
Source: `pragmaRE = regexp.MustCompile(`<!–-\s*prosemark-binder:v1\s*-->`)` in `internal/binder/parser.go`.

`pmk init` must create `_binder.md` containing this pragma as the first line. Without it, all binder parse operations emit `BNDW001` (missing pragma warning).

---

## 5. Existing Atomicity Pattern (Binder Writes)

**Finding**: Existing code uses temp file + rename for atomic binder writes in all cmd IO adapters. The same pattern must be applied to node file creation and can be reused for `pmk init` file creation.

---

## 6. Editor Opening Pattern

**Decision**: Use `exec.Command($EDITOR, filePath)` with stdin/stdout/stderr inherited from the parent process. Wrap as an `Impl` function to exclude from coverage.

```go
// OpenEditorImpl wraps exec.Command for the user's $EDITOR.
// Excluded from coverage (Impl pattern).
func OpenEditorImpl(editor, path string) error {
    cmd := exec.Command(editor, path)
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    return cmd.Run()
}
```

`$EDITOR` is read from `os.Getenv("EDITOR")`. If empty, commands that need the editor fail with a clear error before any editor invocation. For `pmk add --new --edit`, the check is deferred until after node+binder creation (so the node persists in valid state per FR-013b).

---

## 7. Doctor Scan Architecture

**Decision**: Collect all IO upfront in the command handler, then call a pure `RunDoctor` function with all data pre-loaded.

This follows the established pattern in the codebase (cmd layer handles IO, domain layer handles logic). The pure function is fully unit-testable without filesystem mocking.

```go
// DoctorData holds all pre-read data for doctor analysis.
type DoctorData struct {
    BinderSrc    []byte
    UUIDFiles    []string            // UUID-pattern .md filenames found in project root
    FileContents map[string][]byte   // nil value means file does not exist on disk
}

// RunDoctor performs all audit checks and returns diagnostics.
// Pure function — no IO.
func RunDoctor(ctx context.Context, data DoctorData) []AuditDiagnostic
```

**Doctor IO sequence** (cmd handler, Impl functions):
1. Read `_binder.md` → `BinderSrc`
2. ReadDir project root → filter UUID-pattern `.md` files → `UUIDFiles`
3. Walk binder tree → collect all referenced file targets
4. For each referenced file + all UUID files: read file (or record as missing) → `FileContents`
5. Call `RunDoctor(ctx, data)` (pure)
6. Output diagnostics

---

## 8. Init Idempotency

**Decision**: `pmk init` checks both `_binder.md` and `.prosemark.yml` independently.

Rules (per spec, US1 scenarios):
- `_binder.md` exists with content → refuse without `--force` (informative error; file unchanged)
- `_binder.md` exists, `.prosemark.yml` does not → `_binder.md` left unchanged, `.prosemark.yml` treated as not-initialized
- `.prosemark.yml` exists but `_binder.md` does not → create `_binder.md`; leave `.prosemark.yml` unchanged (already partially initialized)
- With `--force` → overwrite both files with defaults
- No write permissions → clear filesystem error, no partial state (atomic write pattern)

`.prosemark.yml` minimum content:
```yaml
version: "1"
```
