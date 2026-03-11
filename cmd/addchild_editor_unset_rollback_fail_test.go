package cmd

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
)

// TestNewAddChildCmd_EditorNotSet_BinderRollbackFails verifies that when
// $EDITOR is not set AND the binder rollback write also fails, the error
// message reports both failures.
func TestNewAddChildCmd_EditorNotSet_BinderRollbackFails(t *testing.T) {
	t.Setenv("EDITOR", "")
	mock := &mockAddChildIOWithRollbackWriteFail{
		mockAddChildIOWithNew: mockAddChildIOWithNew{
			mockAddChildIO: mockAddChildIO{
				binderBytes: emptyBinder(),
				project:     &binder.Project{Files: []string{}, BinderDir: "."},
			},
		},
		rollbackWriteErr: errors.New("disk full on rollback"),
	}
	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--new", "--title", "Chapter", "--edit", "--parent", ".", "--project", "."})

	err := c.Execute()
	if err == nil {
		t.Fatal("expected error when $EDITOR is not set and binder rollback fails")
	}
	if !strings.Contains(err.Error(), "binder rollback also failed") {
		t.Errorf("expected 'binder rollback also failed' in error, got: %v", err)
	}
}
