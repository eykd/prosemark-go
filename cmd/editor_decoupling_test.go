package cmd

import (
	"bytes"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
)

// TestEditCmd_DryRun_DoesNotRequireEditor verifies that the edit command
// in --dry-run mode does NOT require $EDITOR to be set. Currently fails
// because os.Getenv("EDITOR") is checked before the dry-run early return.
func TestEditCmd_DryRun_DoesNotRequireEditor(t *testing.T) {
	t.Setenv("EDITOR", "") // explicitly unset

	mock := &mockEditIO{
		binderBytes:   editBinderWithNode(),
		nodeFileBytes: validEditNodeContent(),
	}
	sub := NewEditCmd(mock)
	root := withDryRunFlag(sub)
	out := new(bytes.Buffer)
	sub.SetOut(out)
	root.SetArgs([]string{"edit", editTestNodeUUID, "--project", ".", "--dry-run"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("dry-run should not require $EDITOR, but got error: %v", err)
	}

	got := out.String()
	if !bytes.HasPrefix([]byte(got), []byte("dry-run:")) {
		t.Errorf("expected dry-run output, got: %q", got)
	}
}

// TestAddChildCmd_NewMode_EditorNotFromEnv verifies that runNewMode does not
// call os.Getenv("EDITOR") directly. Instead, the editor value should be
// read once in the RunE closure and passed to runNewMode as a parameter.
//
// Strategy: We inject a getwd callback that changes EDITOR in the env after
// RunE has started. Since getwd is called before runNewMode in the RunE flow,
// if runNewMode reads os.Getenv("EDITOR") directly it will see the changed
// value. If editor is passed as a parameter (read before getwd), it will see
// the original value.
func TestAddChildCmd_NewMode_EditorNotFromEnv(t *testing.T) {
	originalEditor := "test-editor-param"
	changedEditor := "changed-after-closure-read"

	t.Setenv("EDITOR", originalEditor)

	mock := &mockAddChildIOWithNew{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     &binder.Project{Files: []string{}, BinderDir: "."},
		},
	}

	// Don't pass --project so that getwd IS called by resolveProjectDirFromCmd.
	// Our getwd callback swaps EDITOR in the env, then returns ".".
	c := newAddChildCmdWithGetCWD(mock, func() (string, error) {
		// This runs AFTER RunE starts but BEFORE runNewMode is called.
		// If runNewMode reads os.Getenv("EDITOR") directly, it sees changedEditor.
		// If editor is passed as a parameter (read before this point), it sees originalEditor.
		t.Setenv("EDITOR", changedEditor)
		return ".", nil
	})
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--new", "--edit", "--title", "Node", "--parent", "."})

	err := c.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.editorCalls) == 0 {
		t.Fatal("expected OpenEditor to be called")
	}

	// If runNewMode received editor as a parameter (read by RunE before
	// getwd changed the env), the mock will see originalEditor.
	// If runNewMode called os.Getenv directly, the mock will see changedEditor.
	gotEditor := mock.editorCalls[0][0]
	if gotEditor != originalEditor {
		t.Errorf("runNewMode should receive editor from parameter, not os.Getenv;\n"+
			"expected %q but got %q (indicates direct os.Getenv call in runNewMode)",
			originalEditor, gotEditor)
	}
}
