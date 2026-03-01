package cmd

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
)

// TestNewAddChildCmd_NewMode_UnsupportedIO verifies that --new returns an error
// when the IO implementation does not satisfy newNodeIO.
func TestNewAddChildCmd_NewMode_UnsupportedIO(t *testing.T) {
	// mockAddChildIO satisfies AddChildIO but NOT newNodeIO.
	mock := &mockAddChildIO{
		binderBytes: emptyBinder(),
		project:     &binder.Project{Files: []string{}, BinderDir: "."},
	}
	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--new", "--title", "X", "--parent", ".", "--project", "."})

	if err := c.Execute(); err == nil {
		t.Error("expected error when IO does not support --new mode")
	}
}

// TestNewAddChildCmd_NewMode_UUIDGenError verifies that UUID generation failure
// propagates as an error and no node file is written.
func TestNewAddChildCmd_NewMode_UUIDGenError(t *testing.T) {
	orig := nodeIDGenerator
	defer func() { nodeIDGenerator = orig }()
	nodeIDGenerator = func() (string, error) {
		return "", errors.New("entropy exhausted")
	}

	mock := &mockAddChildIOWithNew{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     &binder.Project{Files: []string{}, BinderDir: "."},
		},
	}
	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--new", "--title", "Node", "--parent", ".", "--project", "."})

	if err := c.Execute(); err == nil {
		t.Error("expected error when UUID generation fails")
	}
	if mock.nodeWrittenPath != "" {
		t.Error("expected no node file written when UUID generation fails")
	}
}

// TestNewAddChildCmd_NewMode_RollbackFails verifies the error message when both
// the binder write and the subsequent rollback DeleteFile call fail.
func TestNewAddChildCmd_NewMode_RollbackFails(t *testing.T) {
	mock := &mockAddChildIOWithNew{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     &binder.Project{Files: []string{}, BinderDir: "."},
			writeErr:    errors.New("disk full"),
		},
		deleteErr: errors.New("cannot delete"),
	}
	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--new", "--title", "Doomed", "--parent", ".", "--project", "."})

	err := c.Execute()
	if err == nil {
		t.Error("expected error when binder write and rollback both fail")
	}
	if !strings.Contains(err.Error(), "rollback also failed") {
		t.Errorf("expected 'rollback also failed' in error, got: %v", err)
	}
}

// TestNewAddChildCmd_NewMode_StdoutError verifies that a write error on the
// confirmation output is propagated as an error.
func TestNewAddChildCmd_NewMode_StdoutError(t *testing.T) {
	mock := &mockAddChildIOWithNew{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     &binder.Project{Files: []string{}, BinderDir: "."},
		},
	}
	c := NewAddChildCmd(mock)
	c.SetOut(&errWriter{err: errors.New("write error")})
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--new", "--title", "Node", "--parent", ".", "--project", "."})

	if err := c.Execute(); err == nil {
		t.Error("expected error when writing confirmation to stdout fails")
	}
}

// TestNewAddChildCmd_NewMode_EditorError verifies that an editor failure after
// successful node and binder creation propagates as an error.
func TestNewAddChildCmd_NewMode_EditorError(t *testing.T) {
	t.Setenv("EDITOR", "vi")
	mock := &mockAddChildIOWithNew{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     &binder.Project{Files: []string{}, BinderDir: "."},
		},
		editorErr: errors.New("editor exited non-zero"),
	}
	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--new", "--edit", "--title", "Node", "--parent", ".", "--project", "."})

	if err := c.Execute(); err == nil {
		t.Error("expected error when editor exits with non-zero status")
	}
	if mock.nodeWrittenPath == "" {
		t.Error("expected node file to be created before editor failure")
	}
	if mock.deletedPath != "" {
		t.Error("expected no rollback when editor fails (node is in valid state)")
	}
	if len(mock.editorCalls) == 0 {
		t.Error("expected OpenEditor to be called")
	}
}

// TestNewAddChildCmd_NewMode_SynopsisWithoutTitle verifies that the synopsis is
// written to the node file even when no --title is provided.
func TestNewAddChildCmd_NewMode_SynopsisWithoutTitle(t *testing.T) {
	mock := &mockAddChildIOWithNew{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     &binder.Project{Files: []string{}, BinderDir: "."},
		},
	}
	c := NewAddChildCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--new", "--synopsis", "Brief intro.", "--parent", ".", "--project", "."})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	content := string(mock.nodeWrittenContent)
	if !strings.Contains(content, "synopsis: Brief intro.") {
		t.Errorf("expected synopsis in node content, got:\n%s", content)
	}
	if strings.Contains(content, "title:") {
		t.Errorf("node content must not include 'title:' when no title is provided, got:\n%s", content)
	}
}

// TestNewAddChildCmd_NewMode_EditorEnvUnset ensures the $EDITOR check uses
// os.Getenv (not a cached value) at runtime. This also covers the --edit
// path with $EDITOR set to verify it calls OpenEditor.
func TestNewAddChildCmd_NewMode_EditorCalledWithEnv(t *testing.T) {
	t.Setenv("EDITOR", "vi")
	mock := &mockAddChildIOWithNew{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     &binder.Project{Files: []string{}, BinderDir: "."},
		},
	}
	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetArgs([]string{"--new", "--edit", "--title", "Chapter", "--parent", ".", "--project", "."})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.editorCalls) == 0 || mock.editorCalls[0][0] != "vi" {
		t.Errorf("expected editor 'vi' to be called, got: %v", mock.editorCalls)
	}
}

// TestNewAddChildCmd_NewMode_ExplicitTargetEmptyCheck verifies the branch
// where a non-empty target is provided (UUID format is valid, covering
// the !strings.ContainsRune fast-path when target is already non-empty).
func TestNewAddChildCmd_NewMode_ExplicitValidTarget(t *testing.T) {
	const validUUID = "01234567-89ab-7def-0123-456789abcdef.md"
	mock := &mockAddChildIOWithNew{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     &binder.Project{Files: []string{}, BinderDir: "."},
		},
	}
	c := NewAddChildCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--new", "--target", validUUID, "--title", "Named", "--parent", ".", "--project", "."})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), validUUID) {
		t.Errorf("stdout should contain UUID, got: %s", out.String())
	}
}

// TestNewAddChildCmd_NewMode_CleanSynopsisAllowed verifies that a synopsis
// with no control characters is accepted by node.ValidateNewNodeInput.
func TestNewAddChildCmd_NewMode_CleanSynopsisAllowed(t *testing.T) {
	mock := &mockAddChildIOWithNew{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     &binder.Project{Files: []string{}, BinderDir: "."},
		},
	}
	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetArgs([]string{"--new", "--title", "Clean", "--synopsis", "No control chars.", "--parent", ".", "--project", "."})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error with clean synopsis: %v", err)
	}
}

// TestNewAddChildCmd_NewMode_AtFlag verifies that --new with --at N inserts the
// new node at the specified index, covering the params.At assignment branch.
func TestNewAddChildCmd_NewMode_AtFlag(t *testing.T) {
	mock := &mockAddChildIOWithNew{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     &binder.Project{Files: []string{}, BinderDir: "."},
		},
	}
	c := NewAddChildCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--new", "--title", "AtNode", "--at", "0", "--parent", ".", "--project", "."})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error with --new --at: %v", err)
	}
	if mock.nodeWrittenPath == "" {
		t.Error("expected WriteNodeFileAtomic to be called")
	}
}

// mockAddChildIOWithFailingRefresh wraps mockAddChildIOWithNew but returns an
// error on the second (and later) call to WriteNodeFileAtomic. This lets tests
// exercise the post-editor updated-refresh failure path (lines 247-249 of
// addchild.go) without affecting the initial node-creation write.
type mockAddChildIOWithFailingRefresh struct {
	mockAddChildIOWithNew
	refreshErr     error
	writeCallCount int
}

// WriteNodeFileAtomic succeeds on the first call and returns refreshErr on
// subsequent calls (simulating a disk failure during the post-editor refresh).
func (m *mockAddChildIOWithFailingRefresh) WriteNodeFileAtomic(path string, content []byte) error {
	m.writeCallCount++
	if m.writeCallCount > 1 && m.refreshErr != nil {
		return m.refreshErr
	}
	return m.mockAddChildIOWithNew.WriteNodeFileAtomic(path, content)
}

// TestNewAddChildCmd_NewMode_RefreshNodeWriteError verifies that when the
// second WriteNodeFileAtomic call (post-editor updated-refresh) fails, the
// command returns an error whose message contains "refreshing node file after
// edit:".
//
// This is the only path in the --new --edit flow not covered by existing tests:
// current mocks either succeed on all writes or fail on all writes.
func TestNewAddChildCmd_NewMode_RefreshNodeWriteError(t *testing.T) {
	mock := &mockAddChildIOWithFailingRefresh{
		mockAddChildIOWithNew: mockAddChildIOWithNew{
			mockAddChildIO: mockAddChildIO{
				binderBytes: emptyBinder(),
				project:     &binder.Project{Files: []string{}, BinderDir: "."},
			},
		},
		refreshErr: errors.New("refresh write failed"),
	}
	t.Setenv("EDITOR", "vi")

	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--new", "--title", "Chapter", "--edit", "--parent", ".", "--project", "."})

	err := c.Execute()
	if err == nil {
		t.Fatal("expected error when post-editor WriteNodeFileAtomic fails")
	}
	if !strings.Contains(err.Error(), "refreshing node file after edit:") {
		t.Errorf("error = %q, want to contain \"refreshing node file after edit:\"", err.Error())
	}
	// The node file should have been written once (initial creation succeeds).
	if mock.writeCallCount < 2 {
		t.Errorf("expected at least 2 WriteNodeFileAtomic calls, got %d", mock.writeCallCount)
	}
}

// TestNewAddChildCmd_NewMode_ErrorDiagnosticRollsBackNode verifies that when
// ops.AddChild produces an error-severity diagnostic (e.g. OPE001: parent not
// found) in --new mode, the command returns an error and rolls back the node
// file that was already created on disk.
//
// RED: the current implementation does not inspect diagnostic severity in --new
// mode and therefore does not roll back or return an error.
func TestNewAddChildCmd_NewMode_ErrorDiagnosticRollsBackNode(t *testing.T) {
	mock := &mockAddChildIOWithNew{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     &binder.Project{Files: []string{}, BinderDir: "."},
		},
	}
	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	// "nonexistent.md" is not in the binder → ops.AddChild produces an
	// OPE001 error-severity diagnostic; --new mode must treat this as failure.
	c.SetArgs([]string{
		"--new", "--title", "Orphan",
		"--parent", "nonexistent.md",
		"--project", ".",
	})

	err := c.Execute()
	if err == nil {
		t.Error("expected error when ops.AddChild reports error diagnostics in --new mode")
	}
	if mock.nodeWrittenPath == "" {
		t.Error("expected WriteNodeFileAtomic to be called before the diagnostic check")
	}
	if mock.deletedPath == "" {
		t.Error("expected DeleteFile to roll back the node file on ops error diagnostic")
	}
}

// TestFileAddChildIO_NewModeInterface verifies that fileAddChildIO satisfies
// the newNodeIO interface at compile time.
var _ newNodeIO = (*fileAddChildIO)(nil)

// TestNewAddChildCmd_NewMode_NilEnvEditor covers the case in os.Getenv where
// $EDITOR is set to empty string (unset). Already covered by US2/9 in the
// table tests, this test makes the path explicit.
func TestNewAddChildCmd_NewMode_EmptyEditorEnv(t *testing.T) {
	if err := os.Unsetenv("EDITOR"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("EDITOR") })

	mock := &mockAddChildIOWithNew{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     &binder.Project{Files: []string{}, BinderDir: "."},
		},
	}
	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--new", "--edit", "--title", "No Editor", "--parent", ".", "--project", "."})

	if err := c.Execute(); err == nil {
		t.Error("expected error when $EDITOR is unset and --edit is provided")
	}
	if mock.nodeWrittenPath == "" {
		t.Error("expected node file to be created before editor check")
	}
}
