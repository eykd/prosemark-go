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

---

### User Story 4 - Validate Project Integrity (Priority: P4)

An author wants to verify their project is internally consistent — all binder references point to real files, all node files have valid frontmatter, and there are no structural anomalies. Running `pmk doctor` produces a diagnostic report.

**Why this priority**: Doctor provides structural confidence and catches corruption early. It is also the enforcement mechanism for the frontmatter contract, ensuring the identity invariants introduced in this feature remain valid over time.

**Independent Test**: Can be tested by constructing projects with known violations (missing files, bad frontmatter, duplicate IDs) and verifying that each violation produces the correct error code. A clean project should exit 0 with no output.

**Acceptance Scenarios**:

1. **Given** a clean project where all binder references exist and all frontmatter is valid, **When** `pmk doctor`, **Then** the command exits 0 with no errors or warnings.
2. **Given** a binder link to `{uuid}.md` where that file does not exist on disk, **When** `pmk doctor`, **Then** AUD001 is reported and the command exits 1.
3. **Given** a `{uuid}.md` file in the project directory not referenced in the binder, **When** `pmk doctor`, **Then** AUD002 is reported. [NEEDS CLARIFICATION: Should AUD002 apply only to files matching the UUID filename pattern (e.g. `{uuid}.md`), or to all `.md` files in the project directory?]
4. **Given** a binder with the same `{uuid}.md` linked twice, **When** `pmk doctor`, **Then** AUD003 is reported and the command exits 1.
5. **Given** a `{uuid}.md` file whose frontmatter `id` field does not match the filename stem, **When** `pmk doctor`, **Then** AUD004 is reported and the command exits 1.
6. **Given** a `{uuid}.md` file missing the required `created` or `id` field in frontmatter, **When** `pmk doctor`, **Then** AUD005 is reported and the command exits 1.
7. **Given** a binder node detected as a placeholder, **When** `pmk doctor`, **Then** AUD006 is reported. [NEEDS CLARIFICATION: What defines a "placeholder node" — is it a node file with empty body content after the frontmatter, a binder entry with special placeholder syntax, or something else?]
8. **Given** a project with errors and `pmk doctor --json`, **When** the command runs, **Then** the output is a JSON array of diagnostic objects with `code`, `message`, and `path` fields.
9. **Given** a project with only AUD006 placeholder warnings (no errors), **When** `pmk doctor`, **Then** the command exits 0 (warnings do not affect exit code).
10. **Given** a binder with a non-UUID filename link (e.g. `chapter-one.md`), **When** `pmk doctor`, **Then** a warning is emitted about the non-UUID filename but the command does not error.

---

### Edge Cases

- What happens when `pmk add --new --edit` is invoked but `$EDITOR` is not set?
- What happens when the UUID file write succeeds but the process is killed before the binder write completes?
- What happens when frontmatter YAML is syntactically malformed (not just missing fields)?
- Does `pmk doctor` scan only the immediate project root for orphan files, or also subdirectories?
- What happens when `pmk edit` is invoked but `$EDITOR` is not set?

## Requirements _(mandatory)_

### Functional Requirements

- **FR-001**: System MUST generate UUIDv7 identifiers for new nodes, formatted as canonical lowercase hex with hyphens.
- **FR-002**: System MUST create node draft files named `{uuid}.md` where the UUID matches the `id` field in the file's YAML frontmatter.
- **FR-003**: System MUST enforce that every node file begins with YAML frontmatter containing at minimum `id` and `created` fields.
- **FR-004**: The `id` field in frontmatter MUST equal the filename stem (without `.md` extension).
- **FR-005**: The `created` timestamp MUST be set at file creation time and MUST NOT change on subsequent edits.
- **FR-006**: The `updated` timestamp MUST be refreshed to the current UTC time whenever an edit operation closes.
- **FR-007**: `pmk init` MUST create `_binder.md` with the managed binder block if not already present.
- **FR-008**: `pmk init` MUST create `.prosemark.yml` with default project configuration.
- **FR-009**: `pmk init` MUST refuse to overwrite existing initialized project files unless `--force` is provided.
- **FR-010**: `pmk add --new` MUST atomically create the node file and update the binder, rolling back the file creation if the binder write fails.
- **FR-011**: `pmk add --new` MUST accept optional `--title` and `--synopsis` flags that populate the corresponding frontmatter fields.
- **FR-012**: `pmk add --new --edit` MUST open the node file in `$EDITOR` immediately after creation.
- **FR-013**: `pmk edit <id> --part <draft|notes|synopsis>` MUST open the appropriate file in `$EDITOR`.
- **FR-014**: `pmk edit` MUST validate that the given ID exists in the binder before opening any file.
- **FR-015**: `pmk edit --part notes` MUST create `{uuid}.notes.md` if it does not yet exist.
- **FR-016**: `pmk doctor` MUST report AUD001 when a binder reference points to a missing file.
- **FR-017**: `pmk doctor` MUST report AUD003 when the same file appears more than once in the binder.
- **FR-018**: `pmk doctor` MUST report AUD004 when a node file's frontmatter `id` does not match its filename stem.
- **FR-019**: `pmk doctor` MUST report AUD005 when required frontmatter fields (`id`, `created`) are absent or malformed.
- **FR-020**: `pmk doctor` MUST exit with code 1 when any error-level diagnostic is present, and code 0 when only warnings (or nothing) are present.
- **FR-021**: `pmk doctor --json` MUST output a JSON array of diagnostic objects.
- **FR-022**: Non-UUID filenames linked in the binder MUST generate a warning but MUST NOT cause an error — backward compatibility with Feature 001 binder-only projects is preserved.

### Key Entities

- **NodeId**: A UUIDv7 value that permanently identifies a node. Immutable after creation. Encoded as canonical lowercase hex with hyphens (e.g. `0192f0c1-3e7a-7000-8000-5a4b3c2d1e0f`).
- **Node**: A first-class writing entity with a stable identity, frontmatter metadata, and body content. Stored as `{uuid}.md`. May also have an associated notes file `{uuid}.notes.md`.
- **Frontmatter**: YAML metadata block at the start of a node file. Contains `id`, `title` (optional), `synopsis` (optional), `created`, and `updated`.
- **NodePart**: The aspect of a node being edited — `draft` (the main content file), `notes` (the notes companion file), or `synopsis` (the synopsis field within frontmatter).
- **NodeRepo**: The filesystem adapter responsible for creating, reading, and writing node files and their frontmatter.
- **Binder**: The `_binder.md` file that registers the structural outline and links to node files. Managed by the existing Feature 001 binder engine.

## Success Criteria _(mandatory)_

### Measurable Outcomes

- **SC-001**: Authors can initialize a new project and add their first chapter in under 60 seconds using only CLI commands.
- **SC-002**: Every node file created by `pmk add --new` passes `pmk doctor` validation without manual intervention.
- **SC-003**: `pmk doctor` detects 100% of the six defined violation types (AUD001–AUD006) when they are present in a project.
- **SC-004**: A binder write failure during `pmk add --new` leaves zero orphaned UUID files on disk.
- **SC-005**: `pmk edit` opens the correct file within 1 second of invocation (editor launch latency excluded).
- **SC-006**: Existing Feature 001 binder-only projects continue to work with `pmk parse`, `pmk add`, `pmk delete`, and `pmk move` without modification.

## Assumptions

- `$EDITOR` environment variable is the sole mechanism for determining the user's preferred editor. No fallback chain (e.g., `$VISUAL`, `vi`) is in scope for this feature.
- `.prosemark.yml` contains at minimum a `version` field. Additional fields are out of scope for this feature unless clarified.
- Doctor scans only the immediate project directory root (not subdirectories) for orphan UUID files.
- `pmk add --new` does not create a notes file by default. Notes files are created on-demand by `pmk edit --part notes`.
- All timestamps are stored and compared in UTC (ISO 8601 format with `Z` suffix).

## Interview

### Answer Log

_(empty)_

### Open Questions

1. **AUD002 scope**: Should the "orphan file not referenced in binder" check (AUD002) apply only to files matching the UUID filename pattern (e.g. `{uuid}.md`), or to all `.md` files in the project directory? The filesystem layout example includes a non-UUID file, suggesting the distinction matters.

2. **AUD006 placeholder definition**: What constitutes a "placeholder node" for AUD006? Candidates: (a) a node file with empty body content after frontmatter, (b) a binder entry that uses special placeholder syntax from the Binder v1 spec, or (c) something else.

**NEXT QUESTION:** #1
