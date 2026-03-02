package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
)

// TestFileAddChildIO_OpenEditor_MultiWordEditor verifies that OpenEditorImpl
// splits a multi-word $EDITOR value on spaces, executing only the first token
// as the command name. The original buggy implementation called
// exec.Command(editor, path) which fails when editor contains spaces because no
// binary named e.g. "true --extra-arg" exists on $PATH.
func TestFileAddChildIO_OpenEditor_MultiWordEditor(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	if err := os.WriteFile(path, []byte("draft"), 0600); err != nil {
		t.Fatal(err)
	}

	fio := newDefaultAddChildIO()
	// "true --extra-arg" must be split into ["true", "--extra-arg"]; the buggy
	// code tried exec.Command("true --extra-arg", path) which fails because no
	// binary named "true --extra-arg" exists.
	if err := fio.OpenEditor("true --extra-arg", path); err != nil {
		t.Fatalf("multi-word editor should succeed when split on spaces: %v", err)
	}
}

// TestNewAddChildCmd_NewMode_EditorShellSplit verifies that the add command
// passes the full $EDITOR string to OpenEditor. This mirrors the contract in
// TestNewEditCmd_EditorShellSplit: the command passes the raw value, and
// OpenEditorImpl is responsible for splitting with strings.Fields.
func TestNewAddChildCmd_NewMode_EditorShellSplit(t *testing.T) {
	t.Setenv("EDITOR", "code --wait")
	mock := &mockAddChildIOWithNew{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     &binder.Project{Files: []string{}, BinderDir: "."},
		},
	}
	c := NewAddChildCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--new", "--title", "Chapter", "--edit", "--parent", ".", "--project", "."})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.editorCalls) == 0 {
		t.Fatal("expected OpenEditor to be called")
	}
	// The add command must pass the full $EDITOR value to OpenEditor; the Impl
	// layer performs the shell split with strings.Fields.
	gotEditor := mock.editorCalls[0][0]
	if gotEditor != "code --wait" {
		t.Errorf("OpenEditor called with editor=%q, want %q", gotEditor, "code --wait")
	}
}

// TestNewAddChildCmd_NewMode_WhitespaceOnlyEditorRejected verifies that a
// $EDITOR value consisting entirely of whitespace is treated as "not configured"
// and rejected before OpenEditor is called. This aligns with the strings.Fields
// split behaviour: Fields("   ") returns no tokens, so there is no executable to
// run. The check must happen at the runNewMode level so that OpenEditor is never
// invoked with a meaningless value.
func TestNewAddChildCmd_NewMode_WhitespaceOnlyEditorRejected(t *testing.T) {
	t.Setenv("EDITOR", "   ") // whitespace-only; passes editor == "" but has no tokens
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
		t.Error("expected error when $EDITOR is whitespace-only, got nil")
	}
	// The node must be persisted (same behaviour as unset $EDITOR, US2 scenario 9).
	if len(mock.nodeWrittenContents) == 0 {
		t.Error("expected node file to be written even when EDITOR is whitespace-only")
	}
	// OpenEditor must NOT be called when the editor string has no tokens.
	if len(mock.editorCalls) > 0 {
		t.Errorf("expected OpenEditor NOT to be called for whitespace-only EDITOR, got calls: %v", mock.editorCalls)
	}
}
