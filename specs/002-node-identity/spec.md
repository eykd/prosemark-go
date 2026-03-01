# Feature Specification: Node Identity & Frontmatter Layer

**Feature Branch**: `002-node-identity`
**Created**: 2026-02-28
**Status**: Draft
**Input**: Feature 002 — Node Identity & Frontmatter Layer (stable node identity, UUIDv7, frontmatter, NodeRepo, pmk init/add --new/edit/doctor)
**Beads Epic**: `prosemark-go-dv8`

**Beads Phase Tasks**:

- clarify: `prosemark-go-7mf`
- plan: `prosemark-go-txa`
- red-team: `prosemark-go-yi0`
- tasks: `prosemark-go-df4`
- analyze: `prosemark-go-8u2`
- implement: `prosemark-go-hhx`
- security-review: `prosemark-go-lc5`
- architecture-review: `prosemark-go-e1b`
- code-quality-review: `prosemark-go-bkg`

## User Scenarios & Testing _(mandatory)_

### User Story 1 - Initialize a Prosemark Project (Priority: P1)

An author starts a new long-form prose project and runs `pmk init` in their project directory. The command creates the structural scaffolding — a `_binder.md` with the managed binder block and a `.prosemark.yml` configuration file — so they can immediately start organizing chapters.

**Why this priority**: Initialization is the entry point for all new projects. Without it, no other commands can function. It must be idempotent and safe to re-run.

**Independent Test**: Can be tested by running `pmk init` in an empty directory and verifying that `_binder.md` and `.prosemark.yml` are created with correct contents. An already-initialized project should refuse re-initialization without `--force`.

**Acceptance Scenarios**:

1. **Given** an empty directory with no prosemark files, **When** `pmk init` runs, **Then** `_binder.md` is created with the managed binder block and `.prosemark.yml` is created with default settings.
2. **Given** a directory where `_binder.md` already exists with user content, **When** `pmk init` runs without `--force`, **Then** the command fails with an informative error and the existing file is unchanged.
3. **Given** a directory already initialized, **When** `pmk init --force` runs, **Then** the binder block is inserted (or confirmed present) and `.prosemark.yml` is overwritten with defaults.
4. **Given** a directory where `_binder.md` does not exist but `.prosemark.yml` already exists, **When** `pmk init` runs, **Then** `_binder.md` is created and `.prosemark.yml` is left unchanged (already initialized).
5. **Given** a directory with no write permissions, **When** `pmk init` runs, **Then** the command fails with a clear filesystem error and creates no partial state.

---

### User Story 2 - Create a New Node with Stable Identity (Priority: P2)

An author adds a new chapter to their project. Running `pmk add --new --title "Chapter One"` generates a unique identity for the chapter, creates the draft file with proper frontmatter, and registers it in the binder outline — all atomically.

**Why this priority**: Node creation with stable identity is the core value proposition of this feature. It transforms the binder from a file list into a registry of first-class writing entities. All downstream features (edit, doctor, sessions) depend on nodes having UUIDs.

**Independent Test**: Can be tested by running `pmk add --new --title "Chapter One" --parent .` and verifying: (a) a `{uuid}.md` file is created with valid YAML frontmatter matching the UUID filename, (b) the binder is updated with a link to that file, (c) a failed write rolls back the file creation.

**Acceptance Scenarios**:

1. **Given** an initialized project, **When** `pmk add --new --title "Chapter One" --parent .`, **Then** a `{uuid}.md` is created with frontmatter containing `id`, `title`, `created`, and `updated` fields, and the binder gains a link `[Chapter One]({uuid}.md)` at the root.
2. **Given** `pmk add --new --title "Scene 1" --parent {existing-uuid}.md`, **When** the command runs, **Then** the new UUID node is nested under the specified parent in the binder.
3. **Given** `pmk add --new --title "Prologue" --synopsis "The world before the war."`, **When** the command runs, **Then** the frontmatter `synopsis` field contains the provided text.
4. **Given** `pmk add --new --title "Chapter Two" --edit`, **When** the command runs, **Then** the node is created and the draft file is immediately opened in `$EDITOR`.
5. **Given** `pmk add --target existing.md --parent .` (no `--new` flag), **When** the command runs, **Then** the existing file is linked into the binder without creating a new file (existing Feature 001 behavior preserved).
6. **Given** `pmk add --new` and the filesystem write of `{uuid}.md` succeeds but the binder write fails, **When** the operation rolls back, **Then** the created UUID file is deleted and the binder is unchanged.
7. **Given** `pmk add --new --target 0192f0c1-0000-7000-8000-000000000000.md`, **When** the command runs, **Then** the explicitly provided UUID is validated against the UUID pattern and used as the node identity.
8. **Given** `pmk add --new --target not-a-uuid.md`, **When** the command runs, **Then** the command fails with an error indicating the target must be a valid UUID filename.
9. **Given** `pmk add --new --title "Chapter Two" --edit` and `$EDITOR` is not set, **When** the command runs, **Then** the node file and binder entry are created successfully, after which the command exits with an error indicating `$EDITOR` is not set. The node persists in a valid state.

---

### User Story 3 - Edit a Node in the System Editor (Priority: P3)

An author wants to write or revise a chapter. Running `pmk edit {uuid} --part draft` opens the correct file in their preferred editor. When they close the editor, the node's `updated` timestamp is refreshed automatically.

**Why this priority**: Editing is the primary day-to-day workflow. The timestamp update ensures `updated` always reflects the last actual edit, supporting future analytics and session tracking.

**Independent Test**: Can be tested by mocking `$EDITOR` and verifying: (a) the correct file path is opened, (b) `updated` is advanced in the frontmatter after the editor exits, (c) errors are returned if the node ID is not in the binder or the file is missing.

**Acceptance Scenarios**:

1. **Given** a node `{uuid}` in the binder with a corresponding `{uuid}.md` file, **When** `pmk edit {uuid} --part draft`, **Then** `$EDITOR` is opened with `{uuid}.md` and `updated` is set to the current time on editor exit.
2. **Given** `pmk edit {uuid} --part notes` and `{uuid}.notes.md` exists, **When** the editor exits, **Then** `updated` in the draft's frontmatter is refreshed.
3. **Given** `pmk edit {uuid} --part notes` and `{uuid}.notes.md` does not exist, **When** the command runs, **Then** a new `{uuid}.notes.md` is created before opening the editor.
4. **Given** a UUID not present in the binder, **When** `pmk edit {uuid} --part draft`, **Then** the command fails with a "node not in binder" error.
5. **Given** a UUID in the binder but with a missing `{uuid}.md` file, **When** `pmk edit {uuid} --part draft`, **Then** the command fails with a "node file missing" error.
6. **Given** `pmk edit {uuid}` with no `--part` flag, **When** the command runs, **Then** `--part draft` is used as the default.
7. **Given** `pmk edit {uuid}` and `$EDITOR` is not set, **When** the command runs, **Then** the command fails with an error indicating `$EDITOR` is not set. No file is created or modified.

---

### User Story 4 - Validate Project Integrity (Priority: P4)

An author wants to verify their project is internally consistent — all binder references point to real files, all node files have valid frontmatter, and there are no structural anomalies. Running `pmk doctor` produces a diagnostic report.

**Why this priority**: Doctor provides structural confidence and catches corruption early. It is also the enforcement mechanism for the frontmatter contract, ensuring the identity invariants introduced in this feature remain valid over time.

**Independent Test**: Can be tested by constructing projects with known violations (missing files, bad frontmatter, duplicate IDs) and verifying that each violation produces the correct error code. A clean project should exit 0 with no output.

**Acceptance Scenarios**:

1. **Given** a clean project where all binder references exist and all frontmatter is valid, **When** `pmk doctor`, **Then** the command exits 0 with no errors or warnings.
2. **Given** a binder link to `{uuid}.md` where that file does not exist on disk, **When** `pmk doctor`, **Then** AUD001 is reported and the command exits 1.
3. **Given** a `{uuid}.md` file in the project directory not referenced in the binder, **When** `pmk doctor`, **Then** AUD002 is reported as a warning (exit code unaffected). AUD002 applies only to files matching the UUID filename pattern — non-UUID `.md` files are ignored.
4. **Given** a binder with the same `{uuid}.md` linked twice, **When** `pmk doctor`, **Then** AUD003 is reported and the command exits 1.
5. **Given** a `{uuid}.md` file whose frontmatter `id` field does not match the filename stem, **When** `pmk doctor`, **Then** AUD004 is reported and the command exits 1.
6. **Given** a `{uuid}.md` file missing the required `created`, `id`, or `updated` field in frontmatter, **When** `pmk doctor`, **Then** AUD005 is reported and the command exits 1.
7. **Given** a `{uuid}.md` file whose body content after the closing frontmatter delimiter is empty or whitespace-only, **When** `pmk doctor`, **Then** AUD006 is reported as a warning (the chapter exists but has no prose yet).
8. **Given** a project with errors and `pmk doctor --json`, **When** the command runs, **Then** the output is a JSON array of diagnostic objects with `code`, `message`, and `path` fields.
9. **Given** a project with only AUD006 placeholder warnings (no errors), **When** `pmk doctor`, **Then** the command exits 0 (warnings do not affect exit code).
12. **Given** a project with only AUD002 orphan warnings (no errors), **When** `pmk doctor`, **Then** the command exits 0 (orphan warnings do not affect exit code).
10. **Given** a binder with a non-UUID filename link (e.g. `chapter-one.md`), **When** `pmk doctor`, **Then** a warning is emitted about the non-UUID filename but the command does not error.
11. **Given** a `{uuid}.md` file whose YAML frontmatter is syntactically invalid (e.g. unclosed quotes, invalid indentation) and cannot be parsed, **When** `pmk doctor`, **Then** AUD007 is reported and the command exits 1.

---

### Edge Cases

- **Resolved**: `pmk add --new --edit` with `$EDITOR` unset — node and binder are created successfully, then the command exits with an error. The node persists in valid state. (See US2 Scenario 9.)
- **Resolved**: What happens when the UUID file write succeeds but the process is killed before the binder write completes? — SIGKILL/OOM-kill after node file creation is an unrecoverable race at the stated single-user scale. The orphaned file is detectable by `pmk doctor` as AUD002 (warning). Recovery: delete the orphaned file manually or link it via `pmk add --target`. No journal or lock file is introduced. (See plan.md §Signal-Interrupted `pmk add --new` Rollback Gap.)
- **Resolved**: Syntactically malformed YAML frontmatter triggers AUD007 (distinct from AUD005 for field-level issues). (See US4 Scenario 11, FR-019c.)
- Does `pmk doctor` scan only the immediate project root for orphan files, or also subdirectories? — **Resolved in Assumptions**: immediate project root only.
- **Resolved**: `pmk edit` with `$EDITOR` unset — command fails immediately with error; no files created or modified. (See US3 Scenario 7.)

## Requirements _(mandatory)_

### Functional Requirements

- **FR-001**: System MUST generate UUIDv7 identifiers for new nodes, formatted as canonical lowercase hex with hyphens.
- **FR-002**: System MUST create node draft files named `{uuid}.md` where the UUID matches the `id` field in the file's YAML frontmatter.
- **FR-003**: System MUST enforce that every node file begins with YAML frontmatter containing at minimum `id`, `created`, and `updated` fields.
- **FR-004**: The `id` field in frontmatter MUST equal the filename stem (without `.md` extension).
- **FR-005**: The `created` timestamp MUST be set at file creation time and MUST NOT change on subsequent edits.
- **FR-005b**: The `updated` timestamp MUST be set to the same value as `created` at node creation time. It is always present from the moment the file is created.
- **FR-006**: The `updated` timestamp MUST be refreshed to the current UTC time whenever an edit operation closes.
- **FR-007**: `pmk init` MUST create `_binder.md` with the managed binder block if not already present.
- **FR-008**: `pmk init` MUST create `.prosemark.yml` with default project configuration.
- **FR-009**: `pmk init` MUST refuse to overwrite existing initialized project files unless `--force` is provided.
- **FR-010**: `pmk add --new` MUST atomically create the node file and update the binder, rolling back the file creation if the binder write fails.
- **FR-011**: `pmk add --new` MUST accept optional `--title` and `--synopsis` flags that populate the corresponding frontmatter fields.
- **FR-012**: `pmk add --new --edit` MUST open the node file in `$EDITOR` immediately after creation.
- **FR-013**: `pmk edit <id> --part <draft|notes>` MUST open the appropriate file in `$EDITOR`. The `--part synopsis` variant is out of scope for this feature.
- **FR-013b**: If `$EDITOR` is not set when any editor-opening operation is attempted (including `pmk add --new --edit` and `pmk edit`), the command MUST exit with a clear error indicating `$EDITOR` is unset. For `pmk add --new --edit`, file and binder writes that completed before the editor step are retained; the node remains in valid state.
- **FR-014**: `pmk edit` MUST validate that the given ID exists in the binder before opening any file.
- **FR-015**: `pmk edit --part notes` MUST create `{uuid}.notes.md` if it does not yet exist.
- **FR-016**: `pmk doctor` MUST report AUD001 when a binder reference points to a missing file.
- **FR-016b**: `pmk doctor` MUST report AUD002 (warning) when a UUID-pattern file (`{uuid}.md`) exists in the project root but is not referenced in the binder. AUD002 does not affect the exit code. Non-UUID `.md` files are not subject to this check.
- **FR-017**: `pmk doctor` MUST report AUD003 when the same file appears more than once in the binder.
- **FR-018**: `pmk doctor` MUST report AUD004 when a node file's frontmatter `id` does not match its filename stem.
- **FR-019**: `pmk doctor` MUST report AUD005 when required frontmatter fields (`id`, `created`, `updated`) are absent or malformed.
- **FR-019b**: `pmk doctor` MUST report AUD006 (warning) when a `{uuid}.md` file has valid frontmatter but empty or whitespace-only body content.
- **FR-019c**: `pmk doctor` MUST report AUD007 when a `{uuid}.md` file's YAML frontmatter block is syntactically unparseable (distinct from AUD005 which covers parseable but invalid field values).
- **FR-020**: `pmk doctor` MUST exit with code 1 when any error-level diagnostic is present, and code 0 when only warnings (or nothing) are present.
- **FR-021**: `pmk doctor --json` MUST output a JSON array of diagnostic objects.
- **FR-022**: Non-UUID filenames linked in the binder MUST generate a warning but MUST NOT cause an error — backward compatibility with Feature 001 binder-only projects is preserved.

### Key Entities

- **NodeId**: A UUIDv7 value that permanently identifies a node. Immutable after creation. Encoded as canonical lowercase hex with hyphens (e.g. `0192f0c1-3e7a-7000-8000-5a4b3c2d1e0f`).
- **Node**: A first-class writing entity with a stable identity, frontmatter metadata, and body content. Stored as `{uuid}.md`. May also have an associated notes file `{uuid}.notes.md`.
- **Frontmatter**: YAML metadata block at the start of a node file. Contains `id`, `title` (optional), `synopsis` (optional), `created`, and `updated`.
- **NodePart**: The aspect of a node being edited — `draft` (the main content file) or `notes` (the notes companion file). The `synopsis` variant is deferred to a future feature.
- **NodeRepo**: The filesystem adapter responsible for creating, reading, and writing node files and their frontmatter.
- **Binder**: The `_binder.md` file that registers the structural outline and links to node files. Managed by the existing Feature 001 binder engine.

## Success Criteria _(mandatory)_

### Measurable Outcomes

- **SC-001**: Authors can initialize a new project and add their first chapter in under 60 seconds using only CLI commands.
- **SC-002**: Every node file created by `pmk add --new` passes `pmk doctor` validation without manual intervention.
- **SC-003**: `pmk doctor` detects 100% of the seven defined violation types (AUD001–AUD007) when they are present in a project.
- **SC-004**: A binder write failure during `pmk add --new` leaves zero orphaned UUID files on disk.
- **SC-005**: `pmk edit` opens the correct file within 1 second of invocation (editor launch latency excluded).
- **SC-006**: Existing Feature 001 binder-only projects continue to work with `pmk parse`, `pmk add`, `pmk delete`, and `pmk move` without modification.

## Assumptions

- `$EDITOR` environment variable is the sole mechanism for determining the user's preferred editor. No fallback chain (e.g., `$VISUAL`, `vi`) is in scope for this feature.
- `.prosemark.yml` contains at minimum a `version` field. Additional fields are out of scope for this feature unless clarified.
- Doctor scans only the immediate project directory root (not subdirectories) for orphan UUID files.
- `pmk add --new` does not create a notes file by default. Notes files are created on-demand by `pmk edit --part notes`.
- All timestamps are stored and compared in UTC (ISO 8601 format with `Z` suffix).

## Clarifications

### Session 2026-02-28 (continued)

- Q: When `pmk edit {uuid} --part synopsis` is invoked, what should happen? → A: `--part synopsis` is out of scope for this feature; removed from FR-013 and NodePart definition.
- Q: When `$EDITOR` is not set and an editor-opening operation is attempted, what should happen? → A: For `pmk add --new --edit`, create the node and binder entry first, then fail with an error; the node persists. For `pmk edit`, fail with an error immediately. (FR-013b added.)
- Q: When frontmatter YAML is syntactically unparseable (not just missing fields), which audit code applies? → A: AUD007 (new code, distinct from AUD005 for parseable-but-invalid fields). (FR-019c and US4 Scenario 11 added; SC-003 updated to 7 violation types.)
- Q: Is AUD002 (orphaned UUID file) an error (exit 1) or a warning (exit 0)? → A: Warning (exit 0), same class as AUD006. (US4 Scenario 3 updated; US4 Scenario 12 added; FR-016b updated.)
- Q: What is the initial value of `updated` when a node is first created? → A: Set to the same value as `created` at creation time; always present from the start. (FR-003, FR-005b added, FR-019, US4 Scenario 6 updated.)

## Interview

### Answer Log

1. **AUD002 scope** (2026-02-28): AUD002 applies only to files matching the UUID filename pattern (`{uuid}.md`). Non-UUID `.md` files are ignored by doctor. *(Answer: A)*
2. **AUD006 placeholder definition** (2026-02-28): A placeholder node is a `{uuid}.md` file whose body content after the closing frontmatter `---` delimiter is empty or whitespace-only. AUD006 is a warning (exit code unaffected). *(Answer: A)*
3. **`--part synopsis` scope** (2026-02-28): `pmk edit --part synopsis` is out of scope for this feature. FR-013 updated to `draft|notes` only; NodePart `synopsis` variant deferred to future feature. *(Answer: C)*
4. **`$EDITOR` not set behavior** (2026-02-28): For `pmk add --new --edit`: create node + binder, then fail with error on editor step; node persists in valid state. For `pmk edit`: fail immediately with error. FR-013b added; US2 Scenario 9 and US3 Scenario 7 added. *(Answer: B)*
5. **Syntactically malformed YAML frontmatter** (2026-02-28): Introduce AUD007 for unparseable YAML (distinct from AUD005 for field-level validation failures). FR-019c and US4 Scenario 11 added; SC-003 updated to 7 violation types. *(Answer: B)*
6. **AUD002 severity** (2026-02-28): AUD002 (orphaned UUID file) is a warning (exit 0), same class as AUD006. FR-016b and US4 Scenario 3 updated; US4 Scenario 12 added. *(Answer: B)*
7. **`updated` initial value** (2026-02-28): `updated` is set to the same value as `created` at node creation time; always present in frontmatter. FR-003 updated to require `updated`; FR-005b added; FR-019 updated to include `updated` in AUD005 check; US4 Scenario 6 updated. *(Answer: A)*

### Open Questions

_(none — interview complete)_
