package cmd

// Tests for early EDITOR validation (prosemark-go-02c.50):
//
//   When `pmk add --new --edit` is used and $EDITOR is empty, the command
//   currently writes the node file and binder, then detects the missing editor,
//   then rolls everything back. This is wasteful and fragile — if rollback
//   fails the binder entry is orphaned (AUD001).
//
//   The fix should validate $EDITOR before any filesystem mutations, so that
//   no node file is created and no binder write occurs when the editor is
//   known to be unavailable.
//
// Desired behavior after fix:
//   1. No node file is written (WriteNodeFileAtomic never called).
//   2. No binder write occurs (WriteBinderAtomic never called).
//   3. The error message mentions EDITOR.
//   4. DeleteFile is NOT called (nothing to roll back).
//
// All tests below FAIL against the current implementation (RED phase).

import (
	"bytes"
	"strings"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
)

// TestNewAddChildCmd_EditorNotSet_NoNodeFileCreated verifies that when --edit
// is requested but $EDITOR is empty, no node file is written at all.
//
// RED: Currently fails because runNewMode writes the node file at line 298
// before checking $EDITOR at line 332.
func TestNewAddChildCmd_EditorNotSet_NoNodeFileCreated(t *testing.T) {
	t.Setenv("EDITOR", "")
	mock := &mockAddChildIOWithNew{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     &binder.Project{Files: []string{}, BinderDir: "."},
		},
	}
	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--new", "--title", "Chapter", "--edit", "--parent", ".", "--project", "."})

	err := c.Execute()
	if err == nil {
		t.Fatal("expected error when $EDITOR is not set")
	}
	if !strings.Contains(err.Error(), "EDITOR") {
		t.Fatalf("expected EDITOR-related error, got: %v", err)
	}

	// KEY ASSERTION: no node file should have been created at all.
	if mock.nodeWrittenPath != "" {
		t.Errorf("expected no node file to be written, but WriteNodeFileAtomic was called with path %q", mock.nodeWrittenPath)
	}
}

// TestNewAddChildCmd_EditorNotSet_NoBinderWrite verifies that when --edit is
// requested but $EDITOR is empty, the binder is never written.
//
// RED: Currently fails because runNewMode writes the binder at line 323
// before checking $EDITOR at line 332.
func TestNewAddChildCmd_EditorNotSet_NoBinderWrite(t *testing.T) {
	t.Setenv("EDITOR", "")
	mock := &mockAddChildIOWithNew{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     &binder.Project{Files: []string{}, BinderDir: "."},
		},
	}
	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--new", "--title", "Chapter", "--edit", "--parent", ".", "--project", "."})

	_ = c.Execute()

	// KEY ASSERTION: binder should never have been written.
	if mock.writtenPath != "" {
		t.Error("expected no binder write when $EDITOR is not set, but WriteBinderAtomic was called")
	}
}

// TestNewAddChildCmd_EditorNotSet_NoDeleteNeeded verifies that when --edit is
// requested but $EDITOR is empty and the check is done early, no rollback
// deletion is needed because no files were created.
//
// RED: Currently fails because runNewMode creates the node file, then detects
// missing $EDITOR, then calls DeleteFile as rollback.
func TestNewAddChildCmd_EditorNotSet_NoDeleteNeeded(t *testing.T) {
	t.Setenv("EDITOR", "")
	mock := &mockAddChildIOWithNew{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     &binder.Project{Files: []string{}, BinderDir: "."},
		},
	}
	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--new", "--title", "Chapter", "--edit", "--parent", ".", "--project", "."})

	_ = c.Execute()

	// KEY ASSERTION: DeleteFile should not be called because nothing was created.
	if mock.deletedPath != "" {
		t.Errorf("expected no DeleteFile call (nothing to roll back), but DeleteFile was called with %q", mock.deletedPath)
	}
}
