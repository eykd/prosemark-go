package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mockEditIO is a test double for EditIO.
//
// ReadNodeFile is path-aware when nodeFiles is non-nil: each entry is keyed by
// filepath.Base(path). A missing key falls back to nodeFileBytes/nodeFileErr.
// A nil map value forces nodeFileErr for that path.
type mockEditIO struct {
	binderBytes    []byte
	binderErr      error
	nodeFiles      map[string][]byte // base filename → content; missing key falls back to defaults
	nodeFileBytes  []byte
	nodeFileErr    error
	writeErr       error
	createNotesErr error
	editorErr      error

	writtenPath  string
	writtenBytes []byte
	notesCreated string
	editorCalls  [][]string // each element is [editor, path]
	readPaths    []string   // tracks every ReadNodeFile call
}

func (m *mockEditIO) ReadBinder(path string) ([]byte, error) {
	return m.binderBytes, m.binderErr
}

func (m *mockEditIO) ReadNodeFile(path string) ([]byte, error) {
	m.readPaths = append(m.readPaths, path)
	if m.nodeFiles != nil {
		base := filepath.Base(path)
		if content, ok := m.nodeFiles[base]; ok {
			if content == nil {
				err := m.nodeFileErr
				if err == nil {
					err = os.ErrNotExist
				}
				return nil, err
			}
			return content, nil
		}
		// Key not found → file missing
		err := m.nodeFileErr
		if err == nil {
			err = os.ErrNotExist
		}
		return nil, err
	}
	return m.nodeFileBytes, m.nodeFileErr
}

func (m *mockEditIO) WriteNodeFileAtomic(path string, content []byte) error {
	m.writtenPath = path
	m.writtenBytes = content
	return m.writeErr
}

func (m *mockEditIO) CreateNotesFile(path string) error {
	m.notesCreated = path
	return m.createNotesErr
}

func (m *mockEditIO) OpenEditor(editor, path string) error {
	m.editorCalls = append(m.editorCalls, []string{editor, path})
	return m.editorErr
}

// mockEditIOWithDelete extends mockEditIO with a DeleteFile method to support
// notes-file rollback assertions.
type mockEditIOWithDelete struct {
	mockEditIO
	deletedPath string
	deleteErr   error
}

func (m *mockEditIOWithDelete) DeleteFile(path string) error {
	m.deletedPath = path
	return m.deleteErr
}

// ─── Shared assertion helper ──────────────────────────────────────────────

// editExpected holds the expected outcomes for an edit command test case.
type editExpected struct {
	err               bool
	editorCalled      bool
	editorPathSuffix  string
	noteCreateAttempt bool
	writeCalled       bool
	writePathSuffix   string
}

// assertEditOutcome verifies mock state against expected outcomes.
func assertEditOutcome(t *testing.T, mock *mockEditIO, err error, exp editExpected) {
	t.Helper()

	if (err != nil) != exp.err {
		t.Errorf("error = %v, wantErr %v", err, exp.err)
	}
	if exp.editorCalled && len(mock.editorCalls) == 0 {
		t.Error("expected OpenEditor to be called")
	}
	if !exp.editorCalled && len(mock.editorCalls) > 0 {
		t.Errorf("expected OpenEditor NOT called, got %v", mock.editorCalls)
	}
	if exp.editorPathSuffix != "" && len(mock.editorCalls) > 0 {
		gotPath := mock.editorCalls[0][1]
		if !strings.HasSuffix(gotPath, exp.editorPathSuffix) {
			t.Errorf("OpenEditor path = %q, want suffix %q", gotPath, exp.editorPathSuffix)
		}
	}
	if exp.noteCreateAttempt && mock.notesCreated == "" {
		t.Error("expected CreateNotesFile to be called")
	}
	if !exp.noteCreateAttempt && mock.notesCreated != "" {
		t.Errorf("expected CreateNotesFile NOT called, got %q", mock.notesCreated)
	}
	if exp.writeCalled && mock.writtenPath == "" {
		t.Error("expected WriteNodeFileAtomic to be called")
	}
	if !exp.writeCalled && mock.writtenPath != "" {
		t.Errorf("expected WriteNodeFileAtomic NOT called, got %q", mock.writtenPath)
	}
	if exp.writePathSuffix != "" && mock.writtenPath != "" {
		if !strings.HasSuffix(mock.writtenPath, exp.writePathSuffix) {
			t.Errorf("WriteNodeFileAtomic path = %q, want suffix %q", mock.writtenPath, exp.writePathSuffix)
		}
	}
}

// ─── Test fixtures ─────────────────────────────────────────────────────────

const editTestNodeUUID = "01234567-89ab-7def-0123-456789abcdef"

// editBinderWithNode returns a binder that contains editTestNodeUUID.
func editBinderWithNode() []byte {
	return []byte("<!-- prosemark-binder:v1 -->\n- [Chapter One](" + editTestNodeUUID + ".md)\n")
}

// validEditNodeContent returns valid node file bytes for editTestNodeUUID.
func validEditNodeContent() []byte {
	return []byte(
		"---\n" +
			"id: " + editTestNodeUUID + "\n" +
			"title: Chapter One\n" +
			"created: 2026-01-01T00:00:00Z\n" +
			"updated: 2026-01-01T00:00:00Z\n" +
			"---\n" +
			"Body text.\n",
	)
}

// ─── Flag / structure tests ─────────────────────────────────────────────────

func TestNewEditCmd_HasRequiredFlags(t *testing.T) {
	c := NewEditCmd(nil)
	for _, name := range []string{"project", "part"} {
		t.Run(name, func(t *testing.T) {
			if c.Flags().Lookup(name) == nil {
				t.Errorf("expected --%s flag on edit command", name)
			}
		})
	}
}

func TestNewEditCmd_RequiresIDArg(t *testing.T) {
	mock := &mockEditIO{}
	c := NewEditCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--project", "."})
	if err := c.Execute(); err == nil {
		t.Error("expected error when positional ID argument is missing")
	}
}

func TestNewEditCmd_DefaultsToCWD(t *testing.T) {
	t.Setenv("EDITOR", "vi")
	mock := &mockEditIO{
		binderBytes:   editBinderWithNode(),
		nodeFileBytes: validEditNodeContent(),
	}
	c := NewEditCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{editTestNodeUUID})
	// CWD may not have a valid binder; the important check is no panic.
	_ = c.Execute()
}

func TestNewEditCmd_GetCWDError(t *testing.T) {
	t.Setenv("EDITOR", "vi")
	mock := &mockEditIO{binderBytes: editBinderWithNode()}
	c := newEditCmdWithGetCWD(mock, func() (string, error) {
		return "", errors.New("getwd failed")
	})
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{editTestNodeUUID})
	if err := c.Execute(); err == nil {
		t.Error("expected error when getwd fails")
	}
}

// ─── US3 scenario table ─────────────────────────────────────────────────────

func TestNewEditCmd_Scenarios(t *testing.T) {
	tests := []struct {
		name string
		args []string

		// Environment
		editorEnv   string
		unsetEditor bool

		// IO mock state
		binderBytes    []byte
		binderErr      error
		nodeFiles      map[string][]byte // path-aware; nil = use nodeFileBytes/nodeFileErr
		nodeFileBytes  []byte
		nodeFileErr    error
		writeErr       error
		createNotesErr error
		editorErr      error

		// Expected outcomes
		want editExpected
	}{
		{
			// US3/1: edit draft — opens draft file, refreshes updated timestamp on close.
			name:          "US3/1 edit draft: opens draft, refreshes updated",
			args:          []string{editTestNodeUUID, "--project", "."},
			editorEnv:     "vi",
			binderBytes:   editBinderWithNode(),
			nodeFileBytes: validEditNodeContent(),
			want: editExpected{
				editorCalled:     true,
				editorPathSuffix: editTestNodeUUID + ".md",
				writeCalled:      true,
				writePathSuffix:  editTestNodeUUID + ".md",
			},
		},
		{
			// US3/2: edit notes — opens notes file, refreshes draft's updated timestamp.
			// Notes file already exists (createNotesErr = os.ErrExist → skip creation).
			name:           "US3/2 edit notes: opens notes file, refreshes draft updated",
			args:           []string{editTestNodeUUID, "--part", "notes", "--project", "."},
			editorEnv:      "vi",
			binderBytes:    editBinderWithNode(),
			nodeFileBytes:  validEditNodeContent(),
			createNotesErr: os.ErrExist,
			want: editExpected{
				editorCalled:     true,
				editorPathSuffix: editTestNodeUUID + ".notes.md",
				writeCalled:      true,
				writePathSuffix:  editTestNodeUUID + ".md",
			},
		},
		{
			// US3/3: edit notes creates notes file when it does not yet exist.
			name:        "US3/3 edit notes creates missing notes file",
			args:        []string{editTestNodeUUID, "--part", "notes", "--project", "."},
			editorEnv:   "vi",
			binderBytes: editBinderWithNode(),
			nodeFiles: map[string][]byte{
				editTestNodeUUID + ".md": validEditNodeContent(),
				// notes file NOT present → ReadNodeFile returns ErrNotExist
			},
			want: editExpected{
				noteCreateAttempt: true,
				editorCalled:      true,
				editorPathSuffix:  editTestNodeUUID + ".notes.md",
				writeCalled:       true,
				writePathSuffix:   editTestNodeUUID + ".md",
			},
		},
		{
			// US3/4: node not in binder → error "node not in binder"; no IO.
			name:        "US3/4 node not in binder",
			args:        []string{"99999999-89ab-7def-0123-456789abcdef", "--project", "."},
			editorEnv:   "vi",
			binderBytes: editBinderWithNode(),
			want:        editExpected{err: true},
		},
		{
			// US3/5: draft file missing → error before editor opens.
			name:        "US3/5 draft file missing",
			args:        []string{editTestNodeUUID, "--project", "."},
			editorEnv:   "vi",
			binderBytes: editBinderWithNode(),
			nodeFileErr: errors.New("open: no such file or directory"),
			want:        editExpected{err: true},
		},
		{
			// US3/6: no --part flag → defaults to draft.
			name:          "US3/6 no --part defaults to draft",
			args:          []string{editTestNodeUUID, "--project", "."},
			editorEnv:     "vi",
			binderBytes:   editBinderWithNode(),
			nodeFileBytes: validEditNodeContent(),
			want: editExpected{
				editorCalled:     true,
				editorPathSuffix: editTestNodeUUID + ".md",
				writeCalled:      true,
				writePathSuffix:  editTestNodeUUID + ".md",
			},
		},
		{
			// US3/7: no $EDITOR → error immediately; no files created or modified.
			name:        "US3/7 no EDITOR configured",
			args:        []string{editTestNodeUUID, "--project", "."},
			unsetEditor: true,
			binderBytes: editBinderWithNode(),
			want:        editExpected{err: true},
		},
		{
			// Editor exits non-zero → updated field must NOT be refreshed.
			name:          "editor exits non-zero: no timestamp refresh",
			args:          []string{editTestNodeUUID, "--project", "."},
			editorEnv:     "vi",
			binderBytes:   editBinderWithNode(),
			nodeFileBytes: validEditNodeContent(),
			editorErr:     errors.New("editor exited with status 1"),
			want:          editExpected{err: true, editorCalled: true},
		},
		{
			// ReadBinder error → fail before any other IO.
			name:      "ReadBinder error",
			args:      []string{editTestNodeUUID, "--project", "."},
			editorEnv: "vi",
			binderErr: errors.New("cannot read binder"),
			want:      editExpected{err: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.unsetEditor {
				unsetEditorEnv(t)
			} else if tt.editorEnv != "" {
				t.Setenv("EDITOR", tt.editorEnv)
			}

			mock := &mockEditIO{
				binderBytes:    tt.binderBytes,
				binderErr:      tt.binderErr,
				nodeFiles:      tt.nodeFiles,
				nodeFileBytes:  tt.nodeFileBytes,
				nodeFileErr:    tt.nodeFileErr,
				writeErr:       tt.writeErr,
				createNotesErr: tt.createNotesErr,
				editorErr:      tt.editorErr,
			}

			c := NewEditCmd(mock)
			out := new(bytes.Buffer)
			errOut := new(bytes.Buffer)
			c.SetOut(out)
			c.SetErr(errOut)
			c.SetArgs(tt.args)

			err := c.Execute()

			assertEditOutcome(t, mock, err, tt.want)
		})
	}
}

// ─── Edge-case tests ────────────────────────────────────────────────────────

// TestNewEditCmd_InvalidPart verifies that --part values other than "draft" or
// "notes" are rejected before any IO occurs.
func TestNewEditCmd_InvalidPart(t *testing.T) {
	t.Setenv("EDITOR", "vi")
	mock := &mockEditIO{binderBytes: editBinderWithNode()}
	c := NewEditCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{editTestNodeUUID, "--part", "invalid", "--project", "."})

	if err := c.Execute(); err == nil {
		t.Error("expected error when --part is not 'draft' or 'notes'")
	}
	if len(mock.editorCalls) > 0 || mock.writtenPath != "" || mock.notesCreated != "" {
		t.Error("expected no IO operations when --part is invalid")
	}
}

// TestNewEditCmd_ParseFrontmatterFailAfterEdit verifies that if the editor
// exits with code 0 but the saved file has unparseable frontmatter, the command
// returns an error and does NOT write anything.
func TestNewEditCmd_ParseFrontmatterFailAfterEdit(t *testing.T) {
	t.Setenv("EDITOR", "vi")
	mock := &mockEditIO{
		binderBytes:   editBinderWithNode(),
		nodeFileBytes: []byte("no YAML frontmatter here\n"),
	}
	c := NewEditCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{editTestNodeUUID, "--project", "."})

	err := c.Execute()
	if err == nil {
		t.Error("expected error when frontmatter is unparseable after edit")
	}
	if mock.writtenPath != "" {
		t.Error("expected WriteNodeFileAtomic NOT called when frontmatter is unparseable")
	}
}

// TestNewEditCmd_NotesRollbackOnEditorNonZero verifies that when --part notes
// causes a new notes file to be created and the editor subsequently exits with
// a non-zero code, the notes file is deleted as rollback.
func TestNewEditCmd_NotesRollbackOnEditorNonZero(t *testing.T) {
	t.Setenv("EDITOR", "vi")
	mock := &mockEditIOWithDelete{
		mockEditIO: mockEditIO{
			binderBytes: editBinderWithNode(),
			nodeFiles: map[string][]byte{
				editTestNodeUUID + ".md": validEditNodeContent(),
				// notes NOT present → CreateNotesFile is triggered
			},
			editorErr: errors.New("editor exited with status 1"),
		},
	}
	c := NewEditCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{editTestNodeUUID, "--part", "notes", "--project", "."})

	err := c.Execute()
	if err == nil {
		t.Error("expected error when editor exits non-zero")
	}
	// If a new notes file was created, it must be deleted on failure.
	if mock.notesCreated != "" && mock.deletedPath == "" {
		t.Error("expected DeleteFile rollback for newly-created notes file on editor failure")
	}
	// Draft must not have been written.
	if mock.writtenPath != "" {
		t.Error("expected WriteNodeFileAtomic NOT called when editor exits non-zero")
	}
}

// TestNewEditCmd_EditorShellSplit verifies that $EDITOR is shell-split before
// exec: "code --wait" passes the full value to OpenEditor (the Impl is
// responsible for splitting with strings.Fields).
func TestNewEditCmd_EditorShellSplit(t *testing.T) {
	t.Setenv("EDITOR", "code --wait")
	mock := &mockEditIO{
		binderBytes:   editBinderWithNode(),
		nodeFileBytes: validEditNodeContent(),
	}
	c := NewEditCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{editTestNodeUUID, "--project", "."})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.editorCalls) == 0 {
		t.Fatal("expected OpenEditor to be called")
	}
	// The implementation passes the full $EDITOR string to OpenEditor; the
	// OpenEditorImpl Impl performs the shell split with strings.Fields.
	gotEditor := mock.editorCalls[0][0]
	if gotEditor != "code --wait" {
		t.Errorf("OpenEditor called with editor=%q, want %q", gotEditor, "code --wait")
	}
}

// TestNewEditCmd_WhitespaceOnlyEditorRejected verifies that $EDITOR values
// consisting entirely of whitespace are treated as "not configured" and
// rejected before OpenEditor is called. edit.go currently uses `editor == ""`
// which does not trim whitespace; strings.Fields would split "   " into no
// tokens, making the exec impossible. The check must happen at the RunE level
// so that OpenEditor is never invoked with an unrunnable value.
func TestNewEditCmd_WhitespaceOnlyEditorRejected(t *testing.T) {
	t.Setenv("EDITOR", "   ") // whitespace-only; passes editor == "" but has no tokens
	mock := &mockEditIO{
		binderBytes:   editBinderWithNode(),
		nodeFileBytes: validEditNodeContent(),
	}
	c := NewEditCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{editTestNodeUUID, "--project", "."})

	err := c.Execute()
	if err == nil {
		t.Error("expected error when $EDITOR is whitespace-only, got nil")
	}
	// OpenEditor must NOT be called when the editor string has no tokens.
	if len(mock.editorCalls) > 0 {
		t.Errorf("expected OpenEditor NOT to be called for whitespace-only EDITOR, got calls: %v", mock.editorCalls)
	}
}

// TestNewEditCmd_WriteAtomicFailAfterEdit verifies that a WriteNodeFileAtomic
// failure after a successful editor exit is reported as an error.
func TestNewEditCmd_WriteAtomicFailAfterEdit(t *testing.T) {
	t.Setenv("EDITOR", "vi")
	mock := &mockEditIO{
		binderBytes:   editBinderWithNode(),
		nodeFileBytes: validEditNodeContent(),
		writeErr:      errors.New("disk full"),
	}
	c := NewEditCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{editTestNodeUUID, "--project", "."})

	if err := c.Execute(); err == nil {
		t.Error("expected error when WriteNodeFileAtomic fails after editor exits")
	}
}

// TestNewEditCmd_UpdatedTimestampRefreshed verifies that the node file written
// after a successful edit has an "updated:" field that differs from the
// original fixture (i.e. the timestamp was actually refreshed).
func TestNewEditCmd_UpdatedTimestampRefreshed(t *testing.T) {
	t.Setenv("EDITOR", "vi")
	mock := &mockEditIO{
		binderBytes:   editBinderWithNode(),
		nodeFileBytes: validEditNodeContent(),
	}
	c := NewEditCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{editTestNodeUUID, "--project", "."})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.writtenPath == "" {
		t.Fatal("expected WriteNodeFileAtomic to be called")
	}
	written := string(mock.writtenBytes)
	if !strings.Contains(written, "updated:") {
		t.Error("written node content missing 'updated:' field")
	}
	// The written content should differ from the fixture (timestamp changed).
	if bytes.Equal(mock.writtenBytes, validEditNodeContent()) {
		t.Error("expected written content to differ from original (updated timestamp should be refreshed)")
	}
}

// ─── Title-based selector tests ─────────────────────────────────────────────

const editTestNodeUUID2 = "aaaaaaaa-bbbb-7ccc-dddd-eeeeeeeeeeee"

// editBinderWithTwoNodes returns a binder containing two nodes with distinct titles.
func editBinderWithTwoNodes() []byte {
	return []byte(
		"<!-- prosemark-binder:v1 -->\n" +
			"- [Chapter One](" + editTestNodeUUID + ".md)\n" +
			"- [Chapter Two](" + editTestNodeUUID2 + ".md)\n",
	)
}

// editBinderWithDuplicateTitles returns a binder with two nodes that have the same title.
func editBinderWithDuplicateTitles() []byte {
	return []byte(
		"<!-- prosemark-binder:v1 -->\n" +
			"- [Chapter One](" + editTestNodeUUID + ".md)\n" +
			"- [Chapter One](" + editTestNodeUUID2 + ".md)\n",
	)
}

func validEditNodeContent2() []byte {
	return []byte(
		"---\n" +
			"id: " + editTestNodeUUID2 + "\n" +
			"title: Chapter Two\n" +
			"created: 2026-01-01T00:00:00Z\n" +
			"updated: 2026-01-01T00:00:00Z\n" +
			"---\n" +
			"Body text two.\n",
	)
}

func TestNewEditCmd_TitleBasedSelector(t *testing.T) {
	tests := []struct {
		name string
		args []string

		binderBytes   []byte
		nodeFiles     map[string][]byte
		nodeFileBytes []byte

		want editExpected
	}{
		{
			// Title selector: "Chapter One" resolves to the correct node UUID.
			name:        "title selector resolves to correct node",
			args:        []string{"Chapter One", "--project", "."},
			binderBytes: editBinderWithTwoNodes(),
			nodeFiles: map[string][]byte{
				editTestNodeUUID + ".md":  validEditNodeContent(),
				editTestNodeUUID2 + ".md": validEditNodeContent2(),
			},
			want: editExpected{
				editorCalled:     true,
				editorPathSuffix: editTestNodeUUID + ".md",
				writeCalled:      true,
				writePathSuffix:  editTestNodeUUID + ".md",
			},
		},
		{
			// Case-insensitive title matching: "chapter two" resolves to "Chapter Two".
			name:        "case-insensitive title match",
			args:        []string{"chapter two", "--project", "."},
			binderBytes: editBinderWithTwoNodes(),
			nodeFiles: map[string][]byte{
				editTestNodeUUID + ".md":  validEditNodeContent(),
				editTestNodeUUID2 + ".md": validEditNodeContent2(),
			},
			want: editExpected{
				editorCalled:     true,
				editorPathSuffix: editTestNodeUUID2 + ".md",
				writeCalled:      true,
				writePathSuffix:  editTestNodeUUID2 + ".md",
			},
		},
		{
			// Title that matches no node produces a not-found error.
			name:        "title matches no node",
			args:        []string{"Nonexistent Chapter", "--project", "."},
			binderBytes: editBinderWithTwoNodes(),
			want:        editExpected{err: true},
		},
		{
			// Ambiguous title (multiple nodes with the same title, different targets)
			// should produce an error.
			name:        "ambiguous title with different targets",
			args:        []string{"Chapter One", "--project", "."},
			binderBytes: editBinderWithDuplicateTitles(),
			want:        editExpected{err: true},
		},
		{
			// UUID still works as before (backwards compatibility).
			name:        "UUID selector still works",
			args:        []string{editTestNodeUUID, "--project", "."},
			binderBytes: editBinderWithTwoNodes(),
			nodeFiles: map[string][]byte{
				editTestNodeUUID + ".md":  validEditNodeContent(),
				editTestNodeUUID2 + ".md": validEditNodeContent2(),
			},
			want: editExpected{
				editorCalled:     true,
				editorPathSuffix: editTestNodeUUID + ".md",
				writeCalled:      true,
				writePathSuffix:  editTestNodeUUID + ".md",
			},
		},
		{
			// Title selector with --part notes opens the correct notes file.
			name:        "title selector with --part notes",
			args:        []string{"Chapter Two", "--part", "notes", "--project", "."},
			binderBytes: editBinderWithTwoNodes(),
			nodeFiles: map[string][]byte{
				editTestNodeUUID + ".md":  validEditNodeContent(),
				editTestNodeUUID2 + ".md": validEditNodeContent2(),
			},
			want: editExpected{
				noteCreateAttempt: true,
				editorCalled:      true,
				editorPathSuffix:  editTestNodeUUID2 + ".notes.md",
				writeCalled:       true,
				writePathSuffix:   editTestNodeUUID2 + ".md",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("EDITOR", "vi")

			mock := &mockEditIO{
				binderBytes:   tt.binderBytes,
				nodeFiles:     tt.nodeFiles,
				nodeFileBytes: tt.nodeFileBytes,
			}

			c := NewEditCmd(mock)
			out := new(bytes.Buffer)
			errOut := new(bytes.Buffer)
			c.SetOut(out)
			c.SetErr(errOut)
			c.SetArgs(tt.args)

			err := c.Execute()

			assertEditOutcome(t, mock, err, tt.want)
		})
	}
}

// ─── Root command wiring ────────────────────────────────────────────────────

// TestNewRootCmd_RegistersEditSubcommand verifies that "edit" is registered on
// the root command.
func TestNewRootCmd_RegistersEditSubcommand(t *testing.T) {
	root := NewRootCmd()
	var found bool
	for _, sub := range root.Commands() {
		if sub.Name() == "edit" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected \"edit\" subcommand registered on root command")
	}
}

// ─── Compile-time interface checks ─────────────────────────────────────────

// TestFileEditIO_ImplementsEditIO is a compile-time assertion that fileEditIO
// satisfies the EditIO interface.
var _ EditIO = (*fileEditIO)(nil)
