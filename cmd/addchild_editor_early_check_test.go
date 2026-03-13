package cmd

// Tests for early EDITOR validation (prosemark-go-02c.50):
//
//   When `pmk add --new --edit` is used and $EDITOR is empty, the command
//   must reject the request before any filesystem mutations. No node file
//   is created, no binder write occurs, and no rollback deletion is needed.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
)

// TestNewAddChildCmd_EditorNotSet_NoMutations verifies that when --edit
// is requested but $EDITOR is empty, the command fails early: no node file
// is written, no binder write occurs, and no rollback deletion is needed.
func TestNewAddChildCmd_EditorNotSet_NoMutations(t *testing.T) {
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
	if mock.nodeWrittenPath != "" {
		t.Errorf("expected no node file to be written, but WriteNodeFileAtomic was called with path %q", mock.nodeWrittenPath)
	}
	if mock.writtenPath != "" {
		t.Error("expected no binder write when $EDITOR is not set, but WriteBinderAtomic was called")
	}
	if mock.deletedPath != "" {
		t.Errorf("expected no DeleteFile call (nothing to roll back), but DeleteFile was called with %q", mock.deletedPath)
	}
}
