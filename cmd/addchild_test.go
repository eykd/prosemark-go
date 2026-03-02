package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
	"github.com/eykd/prosemark-go/internal/node"
)

// mockAddChildIO is a test double for AddChildIO.
type mockAddChildIO struct {
	binderBytes  []byte
	project      *binder.Project
	binderErr    error
	projectErr   error
	writeErr     error
	writtenBytes []byte
	writtenPath  string
}

func (m *mockAddChildIO) ReadBinder(_ context.Context, _ string) ([]byte, error) {
	return m.binderBytes, m.binderErr
}

func (m *mockAddChildIO) ScanProject(_ context.Context, _ string) (*binder.Project, error) {
	if m.project != nil {
		return m.project, m.projectErr
	}
	return &binder.Project{Files: []string{}, BinderDir: "."}, m.projectErr
}

func (m *mockAddChildIO) WriteBinderAtomic(_ context.Context, path string, data []byte) error {
	m.writtenPath = path
	m.writtenBytes = data
	return m.writeErr
}

// acBinder returns a minimal valid binder with one child node.
func acBinder() []byte {
	return []byte("<!-- prosemark-binder:v1 -->\n- [Chapter One](chapter-one.md)\n")
}

func TestNewAddChildCmd_HasRequiredFlags(t *testing.T) {
	c := NewAddChildCmd(nil)
	required := []string{"project", "parent", "target", "title", "first", "at", "before", "after", "force", "json"}
	for _, name := range required {
		name := name
		t.Run(name, func(t *testing.T) {
			if c.Flags().Lookup(name) == nil {
				t.Errorf("expected --%s flag on add command", name)
			}
		})
	}
}

func TestNewAddChildCmd_DefaultsToCWD(t *testing.T) {
	// When no --project flag is given, command resolves path from CWD.
	mock := &mockAddChildIO{
		binderBytes: acBinder(),
		project:     &binder.Project{Files: []string{"chapter-two.md"}, BinderDir: "."},
	}
	c := NewAddChildCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--parent", ".", "--target", "chapter-two.md"})

	if err := c.Execute(); err != nil {
		t.Fatalf("expected success with no --project (CWD default): %v", err)
	}
}

func TestNewAddChildCmd_AcceptsProjectFlag(t *testing.T) {
	mock := &mockAddChildIO{
		binderBytes: acBinder(),
		project:     &binder.Project{Files: []string{"chapter-two.md"}, BinderDir: "."},
	}
	c := NewAddChildCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--parent", ".", "--target", "chapter-two.md", "--project", "/some/dir"})

	if err := c.Execute(); err != nil {
		t.Fatalf("expected success with --project flag: %v", err)
	}
}

func TestNewAddChildCmd_GetCWDError(t *testing.T) {
	mock := &mockAddChildIO{binderBytes: acBinder()}
	c := newAddChildCmdWithGetCWD(mock, func() (string, error) {
		return "", errors.New("getwd failed")
	})
	c.SetOut(new(bytes.Buffer))
	c.SetArgs([]string{"--parent", ".", "--target", "chapter-two.md"})

	if err := c.Execute(); err == nil {
		t.Error("expected error when getwd fails")
	}
}

func TestNewAddChildCmd_PrintsConfirmationOnSuccess(t *testing.T) {
	mock := &mockAddChildIO{
		binderBytes: acBinder(),
		project:     &binder.Project{Files: []string{"chapter-two.md"}, BinderDir: "."},
	}
	c := NewAddChildCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--parent", ".", "--target", "chapter-two.md", "--project", "."})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "Added chapter-two.md to _binder.md") {
		t.Errorf("expected stdout to contain confirmation, got: %s", out.String())
	}
}

func TestNewAddChildCmd_WritesModifiedBinderOnChange(t *testing.T) {
	mock := &mockAddChildIO{
		binderBytes: acBinder(),
		project:     &binder.Project{Files: []string{"chapter-two.md"}, BinderDir: "."},
	}
	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetArgs([]string{"--parent", ".", "--target", "chapter-two.md", "--project", "."})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mock.writtenPath != "_binder.md" {
		t.Errorf("written to %q, want \"_binder.md\"", mock.writtenPath)
	}
	if len(mock.writtenBytes) == 0 {
		t.Error("expected non-empty bytes written to binder")
	}
}

func TestNewAddChildCmd_DoesNotWriteBinderWhenUnchanged(t *testing.T) {
	// Adding an existing target without --force → OPW002 (duplicate skipped, no bytes change)
	mock := &mockAddChildIO{
		binderBytes: acBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md"}, BinderDir: "."},
	}
	c := NewAddChildCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	// chapter-one.md already exists in binder → OPW002 (duplicate skipped, changed=false)
	c.SetArgs([]string{"--parent", ".", "--target", "chapter-one.md", "--project", "."})

	if err := c.Execute(); err != nil {
		t.Fatalf("expected exit 0 for OPW002, got: %v", err)
	}
	if mock.writtenPath != "" {
		t.Errorf("binder must not be written when no change occurred, but was written to %q", mock.writtenPath)
	}
}

func TestNewAddChildCmd_ReadBinderError(t *testing.T) {
	mock := &mockAddChildIO{binderErr: errors.New("disk error")}
	c := NewAddChildCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--parent", ".", "--target", "chapter-two.md", "--project", "."})

	if err := c.Execute(); err == nil {
		t.Error("expected error when ReadBinder fails")
	}
	if out.Len() > 0 {
		t.Errorf("expected no stdout on ReadBinder error, got: %s", out.String())
	}
}

func TestNewAddChildCmd_ScanProjectError(t *testing.T) {
	mock := &mockAddChildIO{
		binderBytes: acBinder(),
		projectErr:  errors.New("disk error"),
	}
	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetArgs([]string{"--parent", ".", "--target", "chapter-two.md", "--project", "."})

	if err := c.Execute(); err == nil {
		t.Error("expected error when ScanProject fails")
	}
}

func TestNewAddChildCmd_WriteError(t *testing.T) {
	mock := &mockAddChildIO{
		binderBytes: acBinder(),
		project:     &binder.Project{Files: []string{"chapter-two.md"}, BinderDir: "."},
		writeErr:    errors.New("write failed"),
	}
	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetArgs([]string{"--parent", ".", "--target", "chapter-two.md", "--project", "."})

	if err := c.Execute(); err == nil {
		t.Error("expected error when WriteBinderAtomic fails")
	}
}

func TestNewAddChildCmd_ExitsNonZeroOnOpErrors(t *testing.T) {
	// Selector that matches no node → OPE001 (error-severity diagnostic)
	mock := &mockAddChildIO{
		binderBytes: acBinder(),
	}
	c := NewAddChildCmd(mock)
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)
	c.SetOut(out)
	c.SetErr(errOut)
	c.SetArgs([]string{"--parent", "nonexistent.md", "--target", "ch.md", "--project", "."})

	err := c.Execute()
	if err == nil {
		t.Error("expected non-zero exit when op has error diagnostics")
	}
	if out.Len() > 0 {
		t.Errorf("expected no stdout on op error, got: %s", out.String())
	}
	if !strings.Contains(errOut.String(), "OPE001") {
		t.Errorf("expected errOut to contain OPE001, got: %s", errOut.String())
	}
	// Binder must NOT be written when op has error diagnostics
	if mock.writtenPath != "" {
		t.Errorf("binder must not be written when op has error diagnostics, was written to %q", mock.writtenPath)
	}
}

func TestNewAddChildCmd_ExitsZeroOnWarningOnly(t *testing.T) {
	// OPW002: adding existing target without --force → warning severity, exit 0
	mock := &mockAddChildIO{
		binderBytes: acBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md"}, BinderDir: "."},
	}
	c := NewAddChildCmd(mock)
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)
	c.SetOut(out)
	c.SetErr(errOut)
	c.SetArgs([]string{"--parent", ".", "--target", "chapter-one.md", "--project", "."})

	if err := c.Execute(); err != nil {
		t.Errorf("expected exit 0 for warning-only (OPW002), got: %v", err)
	}
	if !strings.Contains(out.String(), "skipped") {
		t.Errorf("expected stdout to contain \"skipped\", got: %s", out.String())
	}
	if !strings.Contains(errOut.String(), "OPW002") {
		t.Errorf("expected errOut to contain \"OPW002\", got: %s", errOut.String())
	}
}

func TestNewAddChildCmd_ForceFlag(t *testing.T) {
	// --force allows adding a duplicate target; binder IS changed
	mock := &mockAddChildIO{
		binderBytes: acBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md"}, BinderDir: "."},
	}
	c := NewAddChildCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--parent", ".", "--target", "chapter-one.md", "--force", "--project", "."})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error with --force on duplicate: %v", err)
	}
	if mock.writtenPath == "" {
		t.Error("expected binder to be written when --force produces a change")
	}
}

func TestNewAddChildCmd_WriteSuccessMessageError(t *testing.T) {
	mock := &mockAddChildIO{
		binderBytes: acBinder(),
	}
	c := NewAddChildCmd(mock)
	c.SetOut(&errWriter{err: errors.New("write error")})
	c.SetArgs([]string{"--parent", ".", "--target", "ch.md", "--project", "."})

	if err := c.Execute(); err == nil {
		t.Error("expected error when writing success message fails")
	}
}

func TestNewAddChildCmd_WriteSkippedMessageError(t *testing.T) {
	// Adding an existing target (OPW002, changed=false) with a failing stdout writer.
	mock := &mockAddChildIO{
		binderBytes: acBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md"}, BinderDir: "."},
	}
	c := NewAddChildCmd(mock)
	c.SetOut(&errWriter{err: errors.New("write error")})
	c.SetArgs([]string{"--parent", ".", "--target", "chapter-one.md", "--project", "."})

	if err := c.Execute(); err == nil {
		t.Error("expected error when writing skipped message fails")
	}
}

func TestNewAddChildCmd_FirstFlag(t *testing.T) {
	// --first inserts the child as the first child; command must succeed
	mock := &mockAddChildIO{
		binderBytes: acBinder(),
		project:     &binder.Project{Files: []string{"chapter-two.md"}, BinderDir: "."},
	}
	c := NewAddChildCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--parent", ".", "--target", "chapter-two.md", "--first", "--project", "."})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error with --first: %v", err)
	}
	if mock.writtenPath == "" {
		t.Error("expected binder to be written when --first inserts new child")
	}
}

func TestNewAddChildCmd_AtFlag(t *testing.T) {
	// --at 0 inserts the new child at index 0 (before all existing children).
	mock := &mockAddChildIO{
		binderBytes: acBinder(),
		project:     &binder.Project{Files: []string{"chapter-two.md"}, BinderDir: "."},
	}
	c := NewAddChildCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--parent", ".", "--target", "chapter-two.md", "--at", "0", "--project", "."})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error with --at flag: %v", err)
	}
}

func TestNewAddChildCmd_OutputsOpResultJSONOnSuccess(t *testing.T) {
	mock := &mockAddChildIO{
		binderBytes: acBinder(),
		project:     &binder.Project{Files: []string{"chapter-two.md"}, BinderDir: "."},
	}
	c := NewAddChildCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--parent", ".", "--target", "chapter-two.md", "--json", "--project", "."})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result binder.OpResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out.String())
	}
	if result.Version != "1" {
		t.Errorf("version = %q, want \"1\"", result.Version)
	}
	if !result.Changed {
		t.Error("expected Changed=true when child is added")
	}
}

func TestNewAddChildCmd_JSONEncodeError(t *testing.T) {
	mock := &mockAddChildIO{
		binderBytes: acBinder(),
		project:     &binder.Project{Files: []string{"chapter-two.md"}, BinderDir: "."},
	}
	c := NewAddChildCmd(mock)
	c.SetOut(&errWriter{err: errors.New("write error")})
	c.SetArgs([]string{"--parent", ".", "--target", "chapter-two.md", "--json", "--project", "."})

	if err := c.Execute(); err == nil {
		t.Error("expected error when JSON encoding fails")
	}
}

func TestNewAddChildCmd_ScanProjectErrorWithJSON(t *testing.T) {
	mock := &mockAddChildIO{
		binderBytes: acBinder(),
		projectErr:  errors.New("disk error"),
	}
	c := NewAddChildCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--parent", ".", "--target", "chapter-two.md", "--json", "--project", "."})

	if err := c.Execute(); err == nil {
		t.Error("expected error when ScanProject fails")
	}

	var result binder.OpResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("expected OPE009 JSON on stdout, got: %s", out.String())
	}
	if len(result.Diagnostics) == 0 || result.Diagnostics[0].Code != binder.CodeIOOrParseFailure {
		t.Errorf("expected OPE009 diagnostic, got: %v", result.Diagnostics)
	}
}

func TestNewAddChildCmd_ConflictingPositionFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "first and at conflict",
			args: []string{"--parent", ".", "--target", "ch.md", "--first", "--at", "1", "--project", "."},
		},
		{
			name: "first and before conflict",
			args: []string{"--parent", ".", "--target", "ch.md", "--first", "--before", "x.md", "--project", "."},
		},
		{
			name: "before and after conflict",
			args: []string{"--parent", ".", "--target", "ch.md", "--before", "x.md", "--after", "y.md", "--project", "."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockAddChildIO{
				binderBytes: acBinder(),
				project:     &binder.Project{Files: []string{"ch.md"}, BinderDir: "."},
			}
			c := NewAddChildCmd(mock)
			c.SetOut(new(bytes.Buffer))
			c.SetErr(new(bytes.Buffer))
			c.SetArgs(tt.args)

			if err := c.Execute(); err == nil {
				t.Errorf("expected error for conflicting position flags (%s)", tt.name)
			}
		})
	}
}

func TestNewRootCmd_RegistersAddChildSubcommand(t *testing.T) {
	root := NewRootCmd()
	var found bool
	for _, sub := range root.Commands() {
		if sub.Name() == "add" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected \"add\" subcommand registered on root command")
	}
}

// ─── --new flag: mock, helpers, and tests (US2) ───────────────────────────────

// mockAddChildIOWithNew extends mockAddChildIO with the three IO methods
// required by the --new flag workflow. These methods will be added to the
// AddChildIO interface in the GREEN phase; the embedded struct satisfies the
// current interface while the extra methods enable test assertions.
type mockAddChildIOWithNew struct {
	mockAddChildIO

	nodeWrittenPath    string
	nodeWrittenContent []byte
	nodeWriteErr       error

	// nodeWrittenPaths and nodeWrittenContents record every call to
	// WriteNodeFileAtomic in order, enabling tests that verify a second
	// atomic write occurs after the editor exits (updated-refresh).
	nodeWrittenPaths    []string
	nodeWrittenContents [][]byte

	deletedPath string
	deleteErr   error

	editorCalls [][]string
	editorErr   error
}

// WriteNodeFileAtomic records the call and returns the configured error.
func (m *mockAddChildIOWithNew) WriteNodeFileAtomic(path string, content []byte) error {
	m.nodeWrittenPath = path
	m.nodeWrittenContent = content
	m.nodeWrittenPaths = append(m.nodeWrittenPaths, path)
	m.nodeWrittenContents = append(m.nodeWrittenContents, content)
	return m.nodeWriteErr
}

// DeleteFile records the path and returns the configured error.
func (m *mockAddChildIOWithNew) DeleteFile(path string) error {
	m.deletedPath = path
	return m.deleteErr
}

// OpenEditor records the call and returns the configured error.
func (m *mockAddChildIOWithNew) OpenEditor(editor, path string) error {
	m.editorCalls = append(m.editorCalls, []string{editor, path})
	return m.editorErr
}

// ReadNodeFile returns the most recently written node file content.
func (m *mockAddChildIOWithNew) ReadNodeFile(_ string) ([]byte, error) {
	return m.nodeWrittenContent, nil
}

// emptyBinder returns a minimal initialized binder with no children.
func emptyBinder() []byte {
	return []byte("<!-- prosemark-binder:v1 -->\n")
}

// unsetEditorEnv unsets $EDITOR for the duration of the test, restoring it
// on cleanup. Used when a test requires the editor to be absent.
func unsetEditorEnv(t *testing.T) {
	t.Helper()
	orig, exists := os.LookupEnv("EDITOR")
	if err := os.Unsetenv("EDITOR"); err != nil {
		t.Fatalf("unsetenv EDITOR: %v", err)
	}
	t.Cleanup(func() {
		if exists {
			_ = os.Setenv("EDITOR", orig)
		}
	})
}

// TestNewAddChildCmd_NewModeFlags verifies that --new, --synopsis, and --edit
// are registered on the add command. Fails until the flags are added (RED).
func TestNewAddChildCmd_NewModeFlags(t *testing.T) {
	c := NewAddChildCmd(nil)
	for _, name := range []string{"new", "synopsis", "edit"} {
		name := name
		t.Run(name, func(t *testing.T) {
			if c.Flags().Lookup(name) == nil {
				t.Errorf("expected --%s flag on add command", name)
			}
		})
	}
}

// TestNewAddChildCmd_NewMode_Scenarios covers the nine US2 GWT scenarios for
// the --new flag workflow using table-driven tests.
func TestNewAddChildCmd_NewMode_Scenarios(t *testing.T) {
	parentBinder := []byte("<!-- prosemark-binder:v1 -->\n- [Chapter One](chapter-one.md)\n")

	tests := []struct {
		name           string
		args           []string
		binderBytes    []byte
		projectFiles   []string
		editorEnv      string // non-empty = set EDITOR to this value; "" = unset EDITOR
		binderWriteErr error
		nodeWriteErr   error
		wantErr        bool
		wantNodeCalled bool // WriteNodeFileAtomic expected to be called
		wantDeleted    bool // DeleteFile expected (rollback)
		wantEditorCall bool // OpenEditor expected to be called
		wantStdout     string
	}{
		{
			// US2 scenario 1: new node at root level
			name:           "US2/1 new node at root",
			args:           []string{"--new", "--title", "Chapter One", "--parent", ".", "--project", "."},
			binderBytes:    emptyBinder(),
			wantNodeCalled: true,
			wantStdout:     "Created",
		},
		{
			// US2 scenario 2: new node nested under an existing parent
			name:           "US2/2 new node nested under parent",
			args:           []string{"--new", "--title", "Scene 1", "--parent", "chapter-one.md", "--project", "."},
			binderBytes:    parentBinder,
			projectFiles:   []string{"chapter-one.md"},
			wantNodeCalled: true,
		},
		{
			// US2 scenario 3: new node created with a synopsis field
			name:           "US2/3 new node with synopsis",
			args:           []string{"--new", "--title", "Prologue", "--synopsis", "The world before the war.", "--parent", ".", "--project", "."},
			binderBytes:    emptyBinder(),
			wantNodeCalled: true,
		},
		{
			// US2 scenario 4: --edit opens the preferred editor after creation
			name:           "US2/4 new node with --edit opens editor",
			args:           []string{"--new", "--title", "Chapter Two", "--edit", "--parent", ".", "--project", "."},
			binderBytes:    emptyBinder(),
			editorEnv:      "vi",
			wantNodeCalled: true,
			wantEditorCall: true,
		},
		{
			// US2 scenario 5: Feature 001 behavior is preserved when --new is omitted.
			// A plain `pmk add --parent . --target chapter-two.md` must still add the
			// target to the binder without creating a UUID node file.
			name:           "US2/5 Feature 001 behavior preserved: plain add without --new",
			args:           []string{"--parent", ".", "--target", "chapter-two.md", "--project", "."},
			binderBytes:    emptyBinder(),
			projectFiles:   []string{"chapter-two.md"},
			wantNodeCalled: false,
			wantStdout:     "Added",
		},
		{
			// US2 scenario 6: node file rolled back when binder write fails
			name:           "US2/6 rollback node file on binder write failure",
			args:           []string{"--new", "--title", "Doomed Node", "--parent", ".", "--project", "."},
			binderBytes:    emptyBinder(),
			binderWriteErr: errors.New("disk full"),
			wantErr:        true,
			wantNodeCalled: true,
			wantDeleted:    true,
		},
		{
			// US2 scenario 7: explicitly provided valid UUID used as node identity
			name: "US2/7 explicit valid UUID target used as node identity",
			args: []string{
				"--new",
				"--target", "01234567-89ab-7def-0123-456789abcdef.md",
				"--title", "Named Node",
				"--parent", ".", "--project", ".",
			},
			binderBytes:    emptyBinder(),
			wantNodeCalled: true,
			wantStdout:     "01234567-89ab-7def-0123-456789abcdef.md",
		},
		{
			// US2 scenario 8: non-UUID target filename is rejected
			name: "US2/8 invalid target filename rejected",
			args: []string{
				"--new",
				"--target", "not-a-uuid.md",
				"--title", "Bad Node",
				"--parent", ".", "--project", ".",
			},
			binderBytes: emptyBinder(),
			wantErr:     true,
		},
		{
			// US2 scenario 9: no editor configured; node and binder committed, error returned
			name:           "US2/9 no editor configured: node committed then error",
			args:           []string{"--new", "--title", "No Editor", "--edit", "--parent", ".", "--project", "."},
			binderBytes:    emptyBinder(),
			editorEnv:      "", // will be unset
			wantErr:        true,
			wantNodeCalled: true,
			wantDeleted:    false, // node must NOT be rolled back
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			pf := tt.projectFiles
			if pf == nil {
				pf = []string{}
			}
			mock := &mockAddChildIOWithNew{
				mockAddChildIO: mockAddChildIO{
					binderBytes: tt.binderBytes,
					project:     &binder.Project{Files: pf, BinderDir: "."},
					writeErr:    tt.binderWriteErr,
				},
				nodeWriteErr: tt.nodeWriteErr,
			}

			if tt.editorEnv != "" {
				t.Setenv("EDITOR", tt.editorEnv)
			} else {
				unsetEditorEnv(t)
			}

			c := NewAddChildCmd(mock)
			out := new(bytes.Buffer)
			errOut := new(bytes.Buffer)
			c.SetOut(out)
			c.SetErr(errOut)
			c.SetArgs(tt.args)

			err := c.Execute()

			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v (stdout=%q stderr=%q)", err, tt.wantErr, out, errOut)
			}
			if tt.wantNodeCalled && mock.nodeWrittenPath == "" {
				t.Error("expected WriteNodeFileAtomic to be called")
			}
			if !tt.wantNodeCalled && mock.nodeWrittenPath != "" {
				t.Errorf("expected WriteNodeFileAtomic NOT called, got path %q", mock.nodeWrittenPath)
			}
			if tt.wantDeleted && mock.deletedPath == "" {
				t.Error("expected DeleteFile to be called for rollback")
			}
			if !tt.wantDeleted && mock.deletedPath != "" {
				t.Errorf("expected DeleteFile NOT called, got path %q", mock.deletedPath)
			}
			if tt.wantEditorCall && len(mock.editorCalls) == 0 {
				t.Error("expected OpenEditor to be called")
			}
			if !tt.wantEditorCall && len(mock.editorCalls) > 0 {
				t.Errorf("expected OpenEditor NOT called, got %v", mock.editorCalls)
			}
			if tt.wantStdout != "" && !strings.Contains(out.String(), tt.wantStdout) {
				t.Errorf("stdout = %q, want to contain %q", out.String(), tt.wantStdout)
			}
		})
	}
}

// ─── Validation edge cases ────────────────────────────────────────────────────

// TestNewAddChildCmd_NewMode_TitleTooLong verifies --title is rejected when it
// exceeds 500 characters and no node file is written.
func TestNewAddChildCmd_NewMode_TitleTooLong(t *testing.T) {
	mock := &mockAddChildIOWithNew{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     &binder.Project{Files: []string{}, BinderDir: "."},
		},
	}
	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--new", "--title", strings.Repeat("a", 501), "--parent", ".", "--project", "."})

	if err := c.Execute(); err == nil {
		t.Error("expected error when --title exceeds 500 characters")
	}
	if mock.nodeWrittenPath != "" {
		t.Error("expected no node file written when title too long")
	}
}

// TestNewAddChildCmd_NewMode_SynopsisTooLong verifies --synopsis is rejected
// when it exceeds 2000 characters and no node file is written.
func TestNewAddChildCmd_NewMode_SynopsisTooLong(t *testing.T) {
	mock := &mockAddChildIOWithNew{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     &binder.Project{Files: []string{}, BinderDir: "."},
		},
	}
	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--new", "--synopsis", strings.Repeat("s", 2001), "--parent", ".", "--project", "."})

	if err := c.Execute(); err == nil {
		t.Error("expected error when --synopsis exceeds 2000 characters")
	}
	if mock.nodeWrittenPath != "" {
		t.Error("expected no node file written when synopsis too long")
	}
}

// TestNewAddChildCmd_NewMode_TitleControlChars verifies --title is rejected
// when it contains C0 control characters.
func TestNewAddChildCmd_NewMode_TitleControlChars(t *testing.T) {
	mock := &mockAddChildIOWithNew{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     &binder.Project{Files: []string{}, BinderDir: "."},
		},
	}
	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--new", "--title", "bad\x01title", "--parent", ".", "--project", "."})

	if err := c.Execute(); err == nil {
		t.Error("expected error when --title contains control characters")
	}
	if mock.nodeWrittenPath != "" {
		t.Error("expected no node file written when title has control chars")
	}
}

// TestNewAddChildCmd_NewMode_SynopsisControlChars verifies --synopsis is
// rejected when it contains C0 control characters.
func TestNewAddChildCmd_NewMode_SynopsisControlChars(t *testing.T) {
	mock := &mockAddChildIOWithNew{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     &binder.Project{Files: []string{}, BinderDir: "."},
		},
	}
	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--new", "--synopsis", "bad\x01synopsis", "--parent", ".", "--project", "."})

	if err := c.Execute(); err == nil {
		t.Error("expected error when --synopsis contains control characters")
	}
}

// TestNewAddChildCmd_NewMode_TargetWithPathSeparator verifies --target is
// rejected when it contains a filepath separator.
func TestNewAddChildCmd_NewMode_TargetWithPathSeparator(t *testing.T) {
	mock := &mockAddChildIOWithNew{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     &binder.Project{Files: []string{}, BinderDir: "."},
		},
	}
	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{
		"--new",
		"--target", "subdir/01234567-89ab-7def-0123-456789abcdef.md",
		"--parent", ".", "--project", ".",
	})

	if err := c.Execute(); err == nil {
		t.Error("expected error when --target contains a path separator")
	}
}

// TestNewAddChildCmd_NewMode_NodeWriteError verifies the command returns an
// error and does not update the binder when WriteNodeFileAtomic fails.
func TestNewAddChildCmd_NewMode_NodeWriteError(t *testing.T) {
	mock := &mockAddChildIOWithNew{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     &binder.Project{Files: []string{}, BinderDir: "."},
		},
		nodeWriteErr: errors.New("disk full"),
	}
	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--new", "--title", "Node", "--parent", ".", "--project", "."})

	if err := c.Execute(); err == nil {
		t.Error("expected error when WriteNodeFileAtomic fails")
	}
	if mock.writtenPath != "" {
		t.Error("binder must not be written when node file creation fails")
	}
}

// TestNewAddChildCmd_NewMode_FrontmatterFields verifies the node file written
// by --new contains the expected YAML frontmatter fields.
func TestNewAddChildCmd_NewMode_FrontmatterFields(t *testing.T) {
	mock := &mockAddChildIOWithNew{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     &binder.Project{Files: []string{}, BinderDir: "."},
		},
	}
	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetArgs([]string{
		"--new",
		"--title", "My Chapter",
		"--synopsis", "A brief synopsis.",
		"--parent", ".", "--project", ".",
	})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := string(mock.nodeWrittenContent)
	for _, want := range []string{"id:", "title: My Chapter", "synopsis: A brief synopsis.", "created:", "updated:"} {
		if !strings.Contains(content, want) {
			t.Errorf("node file content missing %q\ncontent:\n%s", want, content)
		}
	}
}

// ─── RED Cycle 4: Gaps identified in REVIEW ──────────────────────────────────

// TestIsUUIDFilename_RejectsNonV7UUID verifies that node.IsUUIDFilename accepts
// only UUIDv7 filenames by requiring the third UUID group to begin with '7'.
func TestIsUUIDFilename_RejectsNonV7UUID(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantMatch bool
	}{
		{"v7 UUID accepted", "01234567-89ab-7def-0123-456789abcdef.md", true},
		{"v7 UUID max third group", "ffffffff-ffff-7fff-ffff-ffffffffffff.md", true},
		{"v4 UUID rejected", "550e8400-e29b-41d4-a716-446655440000.md", false},
		{"v1 UUID rejected", "00000000-0000-1000-8000-000000000000.md", false},
		{"v6 UUID rejected", "1ee4c0ff-c3ae-6000-8000-000000000000.md", false},
		{"uppercase rejected", "01234567-89AB-7DEF-0123-456789ABCDEF.md", false},
		{"no .md extension rejected", "01234567-89ab-7def-0123-456789abcdef", false},
		{"not a uuid rejected", "not-a-uuid.md", false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := node.IsUUIDFilename(tt.input)
			if got != tt.wantMatch {
				t.Errorf("node.IsUUIDFilename(%q) = %v, want %v",
					tt.input, got, tt.wantMatch)
			}
		})
	}
}

// TestNewAddChildCmd_NewMode_NonV7UUIDTargetRejected verifies that a non-v7
// UUID filename is rejected when supplied as --target with --new.
func TestNewAddChildCmd_NewMode_NonV7UUIDTargetRejected(t *testing.T) {
	// v4 UUID: third group starts with '4', not '7'
	mock := &mockAddChildIOWithNew{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     &binder.Project{Files: []string{}, BinderDir: "."},
		},
	}
	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{
		"--new",
		"--target", "550e8400-e29b-41d4-a716-446655440000.md",
		"--title", "Bad Version UUID",
		"--parent", ".", "--project", ".",
	})

	if err := c.Execute(); err == nil {
		t.Error("expected error when --target is a non-v7 UUID")
	}
	if mock.nodeWrittenPath != "" {
		t.Error("no node file must be written when target is a non-v7 UUID")
	}
}

// TestNewAddChildCmd_NewMode_SynopsisAbsentWhenNotProvided verifies that when
// --synopsis is not supplied, the written node file frontmatter does NOT
// include a 'synopsis:' key.
func TestNewAddChildCmd_NewMode_SynopsisAbsentWhenNotProvided(t *testing.T) {
	mock := &mockAddChildIOWithNew{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     &binder.Project{Files: []string{}, BinderDir: "."},
		},
	}
	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetArgs([]string{
		"--new",
		"--title", "No Synopsis Chapter",
		"--parent", ".", "--project", ".",
	})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := string(mock.nodeWrittenContent)
	if strings.Contains(content, "synopsis:") {
		t.Errorf("node file must NOT include 'synopsis:' when --synopsis is not provided\ncontent:\n%s",
			content)
	}
}

// TestNewAddChildCmd_NewMode_RollbackAndDeleteBothFail verifies that when
// writing the binder fails AND the rollback DeleteFile also fails, the
// returned error contains both failure messages. This covers the branch at
// addchild.go:242-243 which currently has no test coverage.
func TestNewAddChildCmd_NewMode_RollbackAndDeleteBothFail(t *testing.T) {
	mock := &mockAddChildIOWithNew{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     &binder.Project{Files: []string{}, BinderDir: "."},
			writeErr:    errors.New("binder disk full"),
		},
		deleteErr: errors.New("delete also failed"),
	}
	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--new", "--title", "Double Failure Node", "--parent", ".", "--project", "."})

	err := c.Execute()
	if err == nil {
		t.Fatal("expected error when both binder write and rollback delete fail")
	}
	if !strings.Contains(err.Error(), "rollback also failed") {
		t.Errorf("error must mention 'rollback also failed', got: %v", err)
	}
}

// TestNewAddChildCmd_NewMode_EmptyTitleRejected verifies that --new without a
// non-empty --title or --synopsis is rejected before any file I/O occurs.
func TestNewAddChildCmd_NewMode_EmptyTitleRejected(t *testing.T) {
	mock := &mockAddChildIOWithNew{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     &binder.Project{Files: []string{}, BinderDir: "."},
		},
	}
	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	// --title is omitted (defaults to empty string)
	c.SetArgs([]string{"--new", "--parent", ".", "--project", "."})

	if err := c.Execute(); err == nil {
		t.Error("expected error when --new is used without a non-empty --title")
	}
	if mock.nodeWrittenPath != "" {
		t.Error("no node file must be written when title is empty")
	}
}

// TestNewAddChildCmd_NewMode_EditRefreshesUpdated verifies that after --edit
// opens the editor and it exits successfully (code 0), the 'updated' field in
// the node file frontmatter is refreshed by a second atomic write.
//
// The implementation reads the node file back after the editor exits, updates
// the 'updated' timestamp, and writes it back atomically (second write).
func TestNewAddChildCmd_NewMode_EditRefreshesUpdated(t *testing.T) {
	mock := &mockAddChildIOWithNew{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     &binder.Project{Files: []string{}, BinderDir: "."},
		},
	}
	t.Setenv("EDITOR", "vi")

	c := NewAddChildCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{
		"--new",
		"--title", "Chapter One",
		"--edit",
		"--parent", ".", "--project", ".",
	})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// After OpenEditor returns nil the implementation must: read the node file,
	// update the 'updated' frontmatter field to time.Now().UTC(), then write it
	// back atomically.  This requires a second call to WriteNodeFileAtomic.
	if len(mock.nodeWrittenContents) < 2 {
		t.Fatalf("expected WriteNodeFileAtomic called at least twice "+
			"(initial create + post-editor updated-refresh), got %d call(s)",
			len(mock.nodeWrittenContents))
	}

	// The refreshed write must still contain an 'updated:' field.
	refreshed := string(mock.nodeWrittenContents[1])
	if !strings.Contains(refreshed, "updated:") {
		t.Errorf("refreshed node file missing 'updated:' field\ncontent:\n%s", refreshed)
	}
}
