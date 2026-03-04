# 03 — Notes Edit Command

## Summary

Add `pmk notes edit <id>` as a Cobra subcommand group. The `notes` command is a parent group (shows help); `notes edit` is the first subcommand. Both `pmk edit <id> --part notes` and `pmk notes edit <id>` invoke the same underlying logic.

**PRD Reference**: Section 6.7 — notes edit
**Dependency**: None (independent of placeholder parsing)

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Shared helper | Extract `editNodeFile()` from `cmd/edit.go` RunE body | DRY; both paths exercise identical logic |
| Parent command | `pmk notes` shows help text | Designed for future `notes list`, `notes show` subcommands |
| IO interface | Reuse existing `EditIO` | Notes edit needs exactly the same capabilities as draft edit |
| No new flags | `notes edit` takes `--project` only | `--part` is implicit (`notes`); `--edit` would be redundant |
| Refactor scope | Extract helper only; don't change `edit` command's public API | Minimize blast radius |

## Package Structure

### New Files

| File | Package | Purpose |
|------|---------|---------|
| `cmd/notes.go` | `cmd` | `NewNotesCmd()` — parent command group |
| `cmd/notes_test.go` | `cmd` | Parent command tests (shows help) |
| `cmd/notes_edit.go` | `cmd` | `NewNotesEditCmd()` — subcommand |
| `cmd/notes_edit_test.go` | `cmd` | Subcommand tests |

### Modified Files

| File | Change |
|------|--------|
| `cmd/edit.go` | Extract `editNodeFile()` helper from RunE body |
| `cmd/root.go` | `root.AddCommand(NewNotesCmd(fileEditIO{}))` |

## Key Types and Signatures

### Shared Helper (extracted to `cmd/edit.go`)

```go
// editNodeFile opens the specified part of a node file in $EDITOR.
// part must be "draft" or "notes".
// This is the shared implementation used by both `pmk edit --part` and `pmk notes edit`.
func editNodeFile(
    cmd *cobra.Command,
    io EditIO,
    binderPath string,
    nodeID string,
    part string,
) error
```

The helper contains the body of the current `edit` RunE from line 42 through line 118, parameterized by `part` instead of reading it from flags.

### `cmd/notes.go`

```go
// NewNotesCmd creates the notes parent command group.
func NewNotesCmd(io EditIO) *cobra.Command {
    cmd := &cobra.Command{
        Use:   "notes",
        Short: "Manage node notes files",
        Args:  cobra.NoArgs,
        RunE: func(cmd *cobra.Command, args []string) error {
            return cmd.Help()
        },
    }
    cmd.AddCommand(NewNotesEditCmd(io))
    return cmd
}
```

### `cmd/notes_edit.go`

```go
// NewNotesEditCmd creates the notes edit subcommand.
func NewNotesEditCmd(io EditIO) *cobra.Command {
    return newNotesEditCmdWithGetCWD(io, os.Getwd)
}

func newNotesEditCmdWithGetCWD(io EditIO, getwd func() (string, error)) *cobra.Command {
    cmd := &cobra.Command{
        Use:          "edit <id>",
        Short:        "Open a node notes file in $EDITOR",
        Args:         cobra.ExactArgs(1),
        SilenceUsage: true,
        RunE: func(cmd *cobra.Command, args []string) error {
            editor := os.Getenv("EDITOR")
            if len(strings.Fields(editor)) == 0 {
                return fmt.Errorf("$EDITOR is not set")
            }

            project, _ := cmd.Flags().GetString("project")
            binderPath, err := resolveBinderPath(project, getwd)
            if err != nil {
                return err
            }

            return editNodeFile(cmd, io, binderPath, args[0], "notes")
        },
    }
    cmd.Flags().String("project", "", "project directory containing _binder.md (default: current directory)")
    return cmd
}
```

## Refactoring `cmd/edit.go`

### Before (current RunE body, lines 41–119)

The entire RunE closure contains:
1. Editor validation
2. Project/binder path resolution
3. Part validation
4. Binder reading and parsing
5. Node lookup
6. Path construction
7. File existence checks / notes creation
8. Editor invocation
9. Rollback on failure
10. Frontmatter refresh

### After

**RunE** becomes:

```go
RunE: func(cmd *cobra.Command, args []string) error {
    nodeID := args[0]

    editor := os.Getenv("EDITOR")
    if len(strings.Fields(editor)) == 0 {
        return fmt.Errorf("$EDITOR is not set")
    }

    project, _ := cmd.Flags().GetString("project")
    binderPath, err := resolveBinderPath(project, getwd)
    if err != nil {
        return err
    }

    part, _ := cmd.Flags().GetString("part")
    if part != "draft" && part != "notes" {
        return fmt.Errorf("--part must be \"draft\" or \"notes\", got %q", part)
    }

    return editNodeFile(cmd, io, binderPath, nodeID, part)
},
```

**`editNodeFile()`** contains lines 60–118 of the current RunE:

```go
func editNodeFile(cmd *cobra.Command, io EditIO, binderPath, nodeID, part string) error {
    editor := os.Getenv("EDITOR")

    binderBytes, err := io.ReadBinder(binderPath)
    if err != nil {
        if errors.Is(err, os.ErrNotExist) {
            return fmt.Errorf("project not initialized — run 'pmk init' first")
        }
        return fmt.Errorf("reading binder: %w", err)
    }

    parsed, _, err := binder.Parse(cmd.Context(), binderBytes, nil)
    if err != nil {
        return fmt.Errorf("cannot parse binder: %w", err)
    }

    targetFilename := nodeID + ".md"
    if !findNodeInTree(parsed.Root, targetFilename) {
        return fmt.Errorf("node %q not found in binder", nodeID)
    }

    binderDir := filepath.Dir(binderPath)
    draftPath := filepath.Join(binderDir, nodeID+".md")
    notesPath := filepath.Join(binderDir, nodeID+".notes.md")

    var editPath string
    var notesCreated bool

    if part == "notes" {
        editPath = notesPath
        if _, readErr := io.ReadNodeFile(notesPath); readErr != nil {
            if !errors.Is(readErr, os.ErrNotExist) {
                return fmt.Errorf("reading notes file: %w", readErr)
            }
            if createErr := io.CreateNotesFile(notesPath); createErr != nil {
                return fmt.Errorf("creating notes file: %w", createErr)
            }
            notesCreated = true
        }
    } else {
        editPath = draftPath
        if _, readErr := io.ReadNodeFile(draftPath); readErr != nil {
            return fmt.Errorf("reading node file: %w", readErr)
        }
    }

    if err := io.OpenEditor(editor, editPath); err != nil {
        if notesCreated {
            if deleter, ok := io.(editDeleter); ok {
                _ = deleter.DeleteFile(notesPath)
            }
        }
        return fmt.Errorf("editor: %w", err)
    }

    if err := refreshNodeUpdated(io, draftPath); err != nil {
        return err
    }

    return nil
}
```

## CLI Registration

In `cmd/root.go`:

```go
root.AddCommand(NewNotesCmd(fileEditIO{}))
```

## Test Strategy

### Parent Command Tests (`cmd/notes_test.go`)

| # | Test Name | Scenario | Expected |
|---|-----------|----------|----------|
| 1 | `TestNotesCmd_ShowsHelp` | Run `pmk notes` with no args | Outputs help text, no error |
| 2 | `TestNotesCmd_HasEditSubcommand` | Check subcommand list | `edit` is present |

### Notes Edit Tests (`cmd/notes_edit_test.go`)

Mirror the existing `cmd/edit_test.go` scenarios for `--part notes`:

| # | Test Name | Scenario | Expected |
|---|-----------|----------|----------|
| 1 | `TestNotesEditCmd_OpensNotesInEditor` | Valid node, notes file exists | Editor called with notes path |
| 2 | `TestNotesEditCmd_CreatesNotesIfMissing` | Notes file doesn't exist | Creates notes, then opens editor |
| 3 | `TestNotesEditCmd_EditorNotSet` | `$EDITOR` empty | Error: `$EDITOR is not set` |
| 4 | `TestNotesEditCmd_BinderNotFound` | Missing `_binder.md` | Error: init message |
| 5 | `TestNotesEditCmd_NodeNotInBinder` | Node ID not in binder | Error: not found |
| 6 | `TestNotesEditCmd_EditorFails_RollbackNotes` | Editor returns error after notes created | Notes file deleted |
| 7 | `TestNotesEditCmd_RefreshesDraftUpdated` | Successful edit | Draft file `updated` timestamp refreshed |
| 8 | `TestNotesEditCmd_ProjectFlag` | `--project /some/dir` | Correct binder path resolution |

### Edit Command Regression (`cmd/edit_test.go`)

All existing `edit` tests must continue passing after the refactor. No test changes needed — the refactor only extracts a helper; the `edit` command's behavior is unchanged.

## Error Handling

| Scenario | Behavior | Source |
|----------|----------|--------|
| `$EDITOR` not set | `"$EDITOR is not set"` | `notes_edit.go` RunE |
| Binder not found | `"project not initialized — run 'pmk init' first"` | `editNodeFile()` |
| Parse failure | `"cannot parse binder: ..."` | `editNodeFile()` |
| Node not in binder | `"node %q not found in binder"` | `editNodeFile()` |
| Notes file read error (non-ENOENT) | `"reading notes file: ..."` | `editNodeFile()` |
| Notes file creation failure | `"creating notes file: ..."` | `editNodeFile()` |
| Editor failure | `"editor: ..."` + rollback if notes created | `editNodeFile()` |
| Frontmatter refresh failure | Propagated error | `refreshNodeUpdated()` |

## Implementation Steps (TDD Order)

1. **Refactor first** (safe — no new behavior):
   a. Extract `editNodeFile()` helper from `cmd/edit.go` RunE body.
   b. Update `edit` RunE to call `editNodeFile()`.
   c. Run existing `edit` tests — all must pass.
   d. Run `just check` — all quality gates pass.

2. **Red**: Write `TestNotesCmd_ShowsHelp`. Fails (command doesn't exist).
3. **Green**: Create `cmd/notes.go` with `NewNotesCmd()`. Test passes.
4. **Red**: Write `TestNotesEditCmd_OpensNotesInEditor`. Fails (subcommand doesn't exist).
5. **Green**: Create `cmd/notes_edit.go` with `NewNotesEditCmd()` calling `editNodeFile()`. Test passes.
6. **Red**: Write remaining notes edit tests (error cases, rollback, project flag).
7. **Green**: All should pass (logic already exists in shared helper).
8. **Register**: Add `NewNotesCmd(fileEditIO{})` to `cmd/root.go`.
9. **Verify**: Run `just check`. Run `just smoke` if applicable.

## Critical Files

| File | Action |
|------|--------|
| `cmd/edit.go` | Refactor: extract `editNodeFile()` helper |
| `cmd/notes.go` | Create |
| `cmd/notes_test.go` | Create |
| `cmd/notes_edit.go` | Create |
| `cmd/notes_edit_test.go` | Create |
| `cmd/root.go` | Add registration |
