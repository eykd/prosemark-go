package cmd

// Tests for the editor-failure rollback bug (prosemark-go-5o3):
//
//   When `pmk add --new --edit` is used and the editor exits non-zero, the
//   UUID node file AND the binder entry are already committed to disk before
//   the editor is invoked. The command returns an error, but leaves permanent
//   state mutations. Scripts checking $? treat the operation as failed while
//   the project has been silently mutated.
//
// Desired behavior after fix:
//   1. If the editor fails, the node file must be deleted (rolled back).
//   2. If the editor fails, the binder must be restored to its original state.
//   3. If the editor fails, stdout must be empty ("Created ..." may not appear).
//
// All three tests below FAIL against the current implementation and are
// intended to drive the fix (RED phase of TDD).

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
)

// mockAddChildIOWithBinderHistory wraps mockAddChildIOWithNew and records every
// call to WriteBinderAtomic so tests can inspect the full write sequence.
type mockAddChildIOWithBinderHistory struct {
	mockAddChildIOWithNew
	binderWrittenHistory [][]byte
}

func (m *mockAddChildIOWithBinderHistory) WriteBinderAtomic(ctx context.Context, path string, data []byte) error {
	m.binderWrittenHistory = append(m.binderWrittenHistory, append([]byte(nil), data...))
	return m.mockAddChildIOWithNew.mockAddChildIO.WriteBinderAtomic(ctx, path, data)
}

// TestNewAddChildCmd_EditorFailure_RollsBackNodeFile verifies that when the
// editor exits with a non-zero status, the previously-created node file is
// deleted (rolled back).
//
// RED: this test FAILS against the current implementation because
// runNewMode does not call DeleteFile on editor failure.
func TestNewAddChildCmd_EditorFailure_RollsBackNodeFile(t *testing.T) {
	t.Setenv("EDITOR", "vi")
	mock := &mockAddChildIOWithNew{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     &binder.Project{Files: []string{}, BinderDir: "."},
		},
		editorErr: errors.New("exit status 1"),
	}
	c := NewAddChildCmd(mock)
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)
	c.SetOut(out)
	c.SetErr(errOut)
	c.SetArgs([]string{"--new", "--title", "Chapter", "--edit", "--parent", ".", "--project", "."})

	err := c.Execute()
	if err == nil {
		t.Fatal("expected error when editor fails")
	}
	if mock.nodeWrittenPath == "" {
		t.Error("expected node file to be created before editor runs")
	}
	// KEY ASSERTION: the node file must be rolled back when the editor fails.
	if mock.deletedPath == "" {
		t.Error("expected DeleteFile to be called to roll back the node file after editor failure")
	}
}

// TestNewAddChildCmd_EditorFailure_NoSuccessOutput verifies that when the
// editor exits with a non-zero status, no success message appears on stdout.
//
// RED: this test FAILS against the current implementation because
// runNewMode prints "Created ..." before invoking the editor.
func TestNewAddChildCmd_EditorFailure_NoSuccessOutput(t *testing.T) {
	t.Setenv("EDITOR", "vi")
	mock := &mockAddChildIOWithNew{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     &binder.Project{Files: []string{}, BinderDir: "."},
		},
		editorErr: errors.New("exit status 1"),
	}
	c := NewAddChildCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--new", "--title", "Chapter", "--edit", "--parent", ".", "--project", "."})

	_ = c.Execute()

	// KEY ASSERTION: stdout must contain nothing (especially not "Created")
	// when the editor fails.  Printing "Created" before the editor exits
	// misleads callers into thinking the operation succeeded.
	if strings.Contains(out.String(), "Created") {
		t.Errorf("expected no success output on editor failure, but stdout contains %q", out.String())
	}
}

// TestNewAddChildCmd_EditorFailure_RollsBackBinder verifies that when the
// editor exits with a non-zero status, the binder is restored to its original
// state (i.e., the new node entry is removed).
//
// RED: this test FAILS against the current implementation because
// runNewMode does not write the original binder bytes back on editor failure.
func TestNewAddChildCmd_EditorFailure_RollsBackBinder(t *testing.T) {
	t.Setenv("EDITOR", "vi")
	originalBinder := emptyBinder()
	mock := &mockAddChildIOWithBinderHistory{
		mockAddChildIOWithNew: mockAddChildIOWithNew{
			mockAddChildIO: mockAddChildIO{
				binderBytes: originalBinder,
				project:     &binder.Project{Files: []string{}, BinderDir: "."},
			},
			editorErr: errors.New("exit status 1"),
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

// mockAddChildIOWithRollbackWriteFail wraps mockAddChildIOWithNew and fails the
// second (and later) WriteBinderAtomic call, simulating a disk failure during
// binder rollback after an editor failure.
type mockAddChildIOWithRollbackWriteFail struct {
	mockAddChildIOWithNew
	writeCallCount   int
	rollbackWriteErr error
}

func (m *mockAddChildIOWithRollbackWriteFail) WriteBinderAtomic(ctx context.Context, path string, data []byte) error {
	m.writeCallCount++
	if m.writeCallCount > 1 && m.rollbackWriteErr != nil {
		return m.rollbackWriteErr
	}
	return m.mockAddChildIOWithNew.mockAddChildIO.WriteBinderAtomic(ctx, path, data)
}

// TestNewAddChildCmd_EditorFailure_BinderRollbackFails verifies that when both
// the editor exits non-zero AND the subsequent binder rollback write also
// fails, the error message reports both failures.
func TestNewAddChildCmd_EditorFailure_BinderRollbackFails(t *testing.T) {
	t.Setenv("EDITOR", "vi")
	mock := &mockAddChildIOWithRollbackWriteFail{
		mockAddChildIOWithNew: mockAddChildIOWithNew{
			mockAddChildIO: mockAddChildIO{
				binderBytes: emptyBinder(),
				project:     &binder.Project{Files: []string{}, BinderDir: "."},
			},
			editorErr: errors.New("exit status 1"),
		},
		rollbackWriteErr: errors.New("disk full on rollback"),
	}
	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--new", "--title", "Chapter", "--edit", "--parent", ".", "--project", "."})

	err := c.Execute()
	if err == nil {
		t.Fatal("expected error when editor fails and binder rollback fails")
	}
	if !strings.Contains(err.Error(), "binder rollback also failed") {
		t.Errorf("expected 'binder rollback also failed' in error, got: %v", err)
	}
}
