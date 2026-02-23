# Implementation Plan: Prosemark Binder v1

**Branch**: `001-prosemark-binder` | **Date**: 2026-02-23 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/001-prosemark-binder/spec.md`

## Summary

Implement the `pmk` CLI tool's Prosemark Binder v1 functionality: a source-preserving parser for
`_binder.md` files that extracts hierarchical node trees with structured diagnostics, three
mutation operations (add-child, delete, move), and a conformance runner that verifies all 135
fixtures. Technical approach: custom line-scanner parser in `internal/binder` preserving all
source bytes for lossless round-trips; a node selector engine; mutation operations that splice
source lines with inherited formatting; and a test-binary conformance runner integrated into the
existing justfile.

---

## Technical Context

**Language/Version**: Go 1.25
**Primary Dependencies**: cobra v1.10.2, spf13/pflag v1.0.9 (no new external dependencies needed)
**Storage**: Files (`_binder.md` read/written in-place; `project.json` read-only)
**Testing**: go test, table-driven tests, 100% coverage for non-Impl functions
**Target Platform**: Linux, macOS, Windows (CGO-free)
**Project Type**: CLI (single binary)
**Performance Goals**: Parse <100 ms for up to 10,000-line binder files
**Constraints**: Byte-exact mutation output; lossless round-trip; LF/CRLF/bare-CR preservation
**Scale/Scope**: Single-file, single-process; 135 conformance fixtures (63 parse + 72 ops)

---

## Constitution Check

_GATE: Must pass before Phase 0 research. Re-checked post-design below._

| Principle | Check | Status |
|-----------|-------|--------|
| I. ATDD — GWT specs exist before implementation | GWT specs defined in `spec.md`; `specs/US*.txt` files authored in sp:05-tasks before any code | ✅ PASS |
| II. Static Analysis — go vet + staticcheck zero warnings | All new packages run through both tools; no CGO | ✅ PASS |
| III. Code Quality — gofmt, GoDoc, naming conventions | Enforced throughout; interfaces named by behavior | ✅ PASS |
| IV. Pre-commit gates — lefthook enforced | Existing hooks cover all quality gates | ✅ PASS |
| V. Warnings addressed immediately — no deferrals | BNDW/BNDE codes surface during TDD cycles | ✅ PASS |
| VI. Go CLI Target — Cobra, exit codes, cross-platform filepath | Uses existing Cobra scaffold; `path/filepath` throughout | ✅ PASS |
| VII. Simplicity — YAGNI, custom parser over heavyweight CommonMark lib | Custom line-scanner is simpler for byte-exact round-trips | ✅ PASS |

**No violations. Design is constitutional.**

---

## Project Structure

### Documentation (this feature)

```text
specs/001-prosemark-binder/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
└── contracts/           # Phase 1 output
    ├── README.md        # CLI protocol summary
    └── schemas/         # Symlink/reference to docs/conformance/v1/schema/
```

### Source Code (repository root)

```text
main.go                          # Delegates to cmd.Execute()

cmd/
├── root.go                      # Root `pmk` command setup
├── parse.go                     # `pmk parse --json --project`
├── addchild.go                  # `pmk add-child --json --project [params]`
├── delete.go                    # `pmk delete --json --project [params]`
└── move.go                      # `pmk move --json --project [params]`

internal/binder/
├── types.go                     # Node, Diagnostic, Location, Project, ParseResult, OpResult, OpSpec, *Params
├── parser.go                    # Parse(ctx, bytes, project) → (ParseResult, []Diagnostic, error)
├── serializer.go                # Serialize(parseResult, srcLines) → []byte
├── selector.go                  # ParseSelector, EvalSelector
└── ops/
    ├── addchild.go              # AddChild(ctx, src, project, AddChildParams) → ([]byte, []Diagnostic, error)
    ├── delete.go                # Delete(ctx, src, project, DeleteParams) → ([]byte, []Diagnostic, error)
    └── move.go                  # Move(ctx, src, project, MoveParams) → ([]byte, []Diagnostic, error)

conformance/
└── runner_test.go               # Walk docs/conformance/v1/, invoke built pmk binary, compare outputs
```

**Structure Decision**: Single-project Go CLI. New packages are `cmd/` (thin Cobra wrappers),
`internal/binder/` (all domain logic), and `conformance/` (integration test runner). No new
external dependencies.

---

## Implementation Phases

### Phase A: Core Parser (US1)

Build `internal/binder/types.go`, `parser.go`, and `serializer.go`.

**Parser responsibilities:**
1. Strip UTF-8 BOM if present; emit BNDW010
2. Detect and record line endings per line (LF / CRLF / bare-CR)
3. Scan for fenced code blocks (backtick and tilde fences at any depth)
4. Detect pragma `<!-- prosemark-binder:v1 -->`; emit BNDW001 if absent
5. Scan list items at all indentation levels; determine nesting from indentation
6. For each list item, extract the first structural link:
   - Inline link: `[Title](target.md)` and `[Title](target.md "tooltip")`
   - Reference-style (full, collapsed, shortcut): resolve against reference definitions
   - Obsidian wikilink: `[[target]]`, `[[target|alias]]`, `[[target|]]`
7. Apply path validation rules: illegal chars (BNDE001), path-escapes-root (BNDE002)
8. Resolve wikilinks against project file map (proximity tiebreak, BNDE003 on ambiguity)
9. Emit secondary diagnostics: BNDW002–BNDW009 as appropriate
10. Build recursive `Node` tree with source location metadata

**Serializer responsibilities (round-trip):**
- Reconstruct output from original source lines (all untouched lines byte-identical)
- Used for stability tests (no op.json fixtures)

### Phase B: Node Selector Engine (US1/shared)

Build `internal/binder/selector.go`.

**Selector grammar:**
```
Selector    ::= "." | Segment ( ":" Segment )*
Segment     ::= FileRef ( "[" Index "]" )?
FileRef     ::= BareStem | RelativePath
BareStem    ::= [^/\[\]:]+          (no path separators)
RelativePath::= BareStem "/" ...    (with path components)
Index       ::= [0-9]+
```

**Evaluation rules:**
- `.` → synthetic root node
- Bare stem → match nodes whose target basename (no `.md`) equals stem
- Relative path → match nodes whose target equals the relative path
- `[N]` → select the Nth match (0-based) from the matched set
- `:` → recursive descent: evaluate second segment among children of first result
- Multi-match emits OPW001; OPE001 on zero-match; OPE002 on ambiguous bare stem

### Phase C: add-child Operation (US2)

Build `internal/binder/ops/addchild.go`.

**Algorithm:**
1. Parse binder; abort on IO/parse error (OPE009)
2. Evaluate parentSelector; zero matches → OPE001; ambiguous bare stem → OPE002
3. Validate target path (illegal chars → OPE004; targets `_binder.md` → OPE005;
   node in code fence → OPE006)
4. Check idempotency: if target already a child and !force → OPW002, return unchanged
5. Determine sibling at position:
   - `--before <sel>` / `--after <sel>` → evaluate selector among parent's children; OPE007 on miss
   - `--at N` where N > child count → OPE008
6. Derive list marker and indentation from reference sibling (see research.md)
7. Derive title: use supplied title; if empty, use target stem
8. Escape `[` and `]` in title with backslash
9. Construct new list item line(s); insert into source at correct position
10. Write modified bytes; emit diagnostics

### Phase D: delete Operation (US3)

Build `internal/binder/ops/delete.go`.

**Algorithm:**
1. Parse binder; abort on IO/parse error (OPE009)
2. Evaluate selector; zero matches → OPE001
3. Collect all lines belonging to node and entire subtree
4. If node has inline prose beyond structural link → OPW003
5. Remove collected lines
6. Apply cleanup rules:
   - Prune any empty sublist that results (OPW004)
   - Collapse consecutive blank lines to one
   - Remove trailing blank lines at EOF
7. Write modified bytes

### Phase E: move Operation (US4)

Build `internal/binder/ops/move.go`.

**Algorithm:**
1. Parse binder; abort on IO/parse error (OPE009)
2. Evaluate sourceSelector and destinationParentSelector
3. Detect cycle: if destination is descendant of source → OPE003, abort
4. If source has non-structural inline content → OPW003
5. Extract source node lines (preserving relative indentation within subtree)
6. Apply delete-site cleanup rules at source location
7. Determine position flags at destination (same as add-child position logic)
8. Derive list marker and indentation for the moved node's root line
9. Re-indent the moved subtree to match destination nesting depth
10. Preserve link syntax (inline/reference/wikilink) and tooltip attributes
11. Insert at destination; write modified bytes

### Phase F: CLI Commands (all US)

Build `cmd/` package with parse, add-child, delete, move commands.

**Each command:**
- Reads binder bytes from `<binder-path>` argument
- Reads project JSON from `--project <path>` flag
- Accepts `--json` flag for machine-readable JSON output
- On success (exit 0): writes JSON result to stdout (parse result or op-result)
- On error (exit non-zero): writes error info to stderr; binder file unchanged
- Passes `context.Context` through to library functions

**`pmk parse` stdout schema** (parse-result.schema.json + diagnostics):
```json
{
  "version": "1",
  "root": { "type": "root", "children": [...] },
  "diagnostics": [...]
}
```

**`pmk <op>` stdout schema** (op-result.schema.json):
```json
{
  "version": "1",
  "changed": true,
  "diagnostics": [...]
}
```

### Phase G: Conformance Runner (US5)

Build `conformance/runner_test.go` as a Go integration test.

**Runner behavior:**
1. Build `bin/pmk` via `go build -o bin/pmk .` before tests
2. Walk `docs/conformance/v1/parse/fixtures/*/` for parse fixtures
3. Walk `docs/conformance/v1/ops/fixtures/*/*/` for ops fixtures
4. For each fixture:
   - **Parse fixtures**: invoke `pmk parse --json`, compare parse tree and diagnostics
   - **Ops fixtures with op.json**: invoke `pmk <op> --json`, compare output bytes and diagnostics
   - **Stability fixtures (no op.json)**: invoke `pmk parse --json` + re-serialize, compare bytes
5. Pass/fail per runner contract (§4); PASS if all checks pass; report diffs on failure
6. Exit non-zero if any fixture fails

**justfile integration** (add to existing justfile):
```makefile
conformance-run:
    go test -v -timeout=120s ./conformance/...

test-all:
    just test && just conformance-run
```

---

## Constitution Check (Post-Design)

Re-evaluating after Phase 1 design:

| Principle | Post-Design Assessment |
|-----------|----------------------|
| I. ATDD | GWT specs will be authored in sp:05 before implementation begins; conformance runner provides the objective gate for US5 |
| II. Static Analysis | All packages will pass vet+staticcheck; no CGO; context.Context used throughout |
| III. Code Quality | GoDoc on all exported APIs; table-driven tests throughout |
| IV. Pre-commit gates | Existing lefthook hooks enforce all gates; conformance-run added separately (not in pre-commit due to binary requirement) |
| V. No deferred warnings | All diagnostics addressed during TDD inner cycles |
| VI. CLI Target | All operations use `path/filepath`; cross-compile compatible |
| VII. Simplicity | Custom parser preferred over third-party CommonMark library; no speculative features |

**Post-design check: PASS. No constitutional violations.**
