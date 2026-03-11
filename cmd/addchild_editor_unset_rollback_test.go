package cmd

// Tests for the editor-not-set rollback bug (prosemark-go-02c.28):
//
//   When `pmk add --new --edit` is used and $EDITOR is empty (not set), the
//   command returns "$EDITOR is not set" but leaves both the node file and the
//   binder entry on disk. The rollback logic for OpenEditor failures does NOT
//   cover the earlier "$EDITOR is not set" guard, resulting in the same orphaned
//   binder entry described in the original editor-failure rollback bug.
//
// Desired behavior after fix:
//   1. If $EDITOR is not set, the node file must be deleted (rolled back).
//   2. If $EDITOR is not set, the binder must be restored to its original state.
//   3. If $EDITOR is not set, stdout must be empty ("Created ..." may not appear).
//
// All tests below FAIL against the current implementation and are intended to
// drive the fix (RED phase of TDD).

import (
	"bytes"
	"strings"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
)

// TestNewAddChildCmd_EditorNotSet_RollsBackNodeFile verifies that when $EDITOR
// is empty, the previously-created node file is deleted (rolled back).
//
// RED: this test FAILS because the "$EDITOR is not set" guard at line 315-316
// of runNewMode returns an error without calling DeleteFile.
func TestNewAddChildCmd_EditorNotSet_RollsBackNodeFile(t *testing.T) {
	t.Setenv("EDITOR", "")
	mock := &mockAddChildIOWithNew{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     &binder.Project{Files: []string{}, BinderDir: "."},
		},
	}
	c := NewAddChildCmd(mock)
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)
	c.SetOut(out)
	c.SetErr(errOut)
	c.SetArgs([]string{"--new", "--title", "Chapter", "--edit", "--parent", ".", "--project", "."})

	err := c.Execute()
	if err == nil {
		t.Fatal("expected error when $EDITOR is not set")
	}
	if !strings.Contains(err.Error(), "EDITOR") {
		t.Fatalf("expected EDITOR-related error, got: %v", err)
	}
	if mock.nodeWrittenPath == "" {
		t.Error("expected node file to be created before editor check")
	}
	// KEY ASSERTION: the node file must be rolled back when $EDITOR is not set.
	if mock.deletedPath == "" {
		t.Error("expected DeleteFile to be called to roll back the node file after $EDITOR is not set")
	}
}

// TestNewAddChildCmd_EditorNotSet_RollsBackBinder verifies that when $EDITOR
// is empty, the binder is restored to its original state.
//
// RED: this test FAILS because the "$EDITOR is not set" guard returns an error
// without writing back the original binder bytes.
func TestNewAddChildCmd_EditorNotSet_RollsBackBinder(t *testing.T) {
	t.Setenv("EDITOR", "")
	originalBinder := emptyBinder()
	mock := &mockAddChildIOWithBinderHistory{
		mockAddChildIOWithNew: mockAddChildIOWithNew{
			mockAddChildIO: mockAddChildIO{
				binderBytes: originalBinder,
				project:     &binder.Project{Files: []string{}, BinderDir: "."},
			},
		},
	}
	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--new", "--title", "Chapter", "--edit", "--parent", ".", "--project", "."})

	_ = c.Execute()

	// KEY ASSERTION: there must be at least two binder writes — first the
	// modified binder (node added), then the original bytes (rollback) —
	// and the final write must equal the original binder.
	if len(mock.binderWrittenHistory) < 2 {
		t.Fatalf("expected ≥2 binder writes (add then rollback), got %d",
			len(mock.binderWrittenHistory))
	}
	last := mock.binderWrittenHistory[len(mock.binderWrittenHistory)-1]
	if !bytes.Equal(last, originalBinder) {
		t.Errorf("expected binder rolled back to original bytes\nwant: %q\n got: %q",
			originalBinder, last)
	}
}

// TestNewAddChildCmd_EditorNotSet_NoSuccessOutput verifies that when $EDITOR
// is empty, no success message appears on stdout.
//
// RED: this test FAILS because "Created" is NOT printed (the error return
// happens before the output line), but it validates the contract. This test
// actually passes — included for completeness of the rollback contract.
func TestNewAddChildCmd_EditorNotSet_NoSuccessOutput(t *testing.T) {
	t.Setenv("EDITOR", "")
	mock := &mockAddChildIOWithNew{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     &binder.Project{Files: []string{}, BinderDir: "."},
		},
	}
	c := NewAddChildCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--new", "--title", "Chapter", "--edit", "--parent", ".", "--project", "."})

	_ = c.Execute()

	// KEY ASSERTION: stdout must contain nothing (especially not "Created")
	// when $EDITOR is not set.
	if strings.Contains(out.String(), "Created") {
		t.Errorf("expected no success output when $EDITOR is not set, but stdout contains %q", out.String())
	}
}
