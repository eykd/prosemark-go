package cmd

import (
	"bytes"
	"testing"
)

// --- addchild --new mode dry-run tests ---
//
// runNewMode currently ignores --dry-run: it unconditionally creates node files
// and writes the binder. These tests verify the expected dry-run behavior.

func TestAddChildCmd_NewMode_DryRun_SkipsNodeFileWrite(t *testing.T) {
	mock := &mockAddChildIOWithNew{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     nil, // default empty project
		},
	}
	sub := NewAddChildCmd(mock)
	root := withDryRunFlag(sub)
	root.SetOut(new(bytes.Buffer))
	root.SetArgs([]string{"add", "--new", "--title", "DryChapter", "--parent", ".", "--project", ".", "--dry-run"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mock.nodeWrittenPath != "" {
		t.Errorf("dry-run must not create node file, but WriteNodeFileAtomic was called with %q", mock.nodeWrittenPath)
	}
}

func TestAddChildCmd_NewMode_DryRun_SkipsBinderWrite(t *testing.T) {
	mock := &mockAddChildIOWithNew{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     nil,
		},
	}
	sub := NewAddChildCmd(mock)
	root := withDryRunFlag(sub)
	root.SetOut(new(bytes.Buffer))
	root.SetArgs([]string{"add", "--new", "--title", "DryChapter", "--parent", ".", "--project", ".", "--dry-run"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mock.writtenPath != "" {
		t.Errorf("dry-run must not write binder, but WriteBinderAtomic was called with %q", mock.writtenPath)
	}
}

func TestAddChildCmd_NewMode_DryRun_SkipsEditorOpen(t *testing.T) {
	t.Setenv("EDITOR", "vi")
	mock := &mockAddChildIOWithNew{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     nil,
		},
	}
	sub := NewAddChildCmd(mock)
	root := withDryRunFlag(sub)
	root.SetOut(new(bytes.Buffer))
	root.SetArgs([]string{"add", "--new", "--title", "DryChapter", "--edit", "--parent", ".", "--project", ".", "--dry-run"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.editorCalls) > 0 {
		t.Errorf("dry-run must not open editor, but OpenEditor was called %d time(s)", len(mock.editorCalls))
	}
}

func TestAddChildCmd_NewMode_DryRun_HumanOutputPrefix(t *testing.T) {
	mock := &mockAddChildIOWithNew{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     nil,
		},
	}
	sub := NewAddChildCmd(mock)
	root := withDryRunFlag(sub)
	out := new(bytes.Buffer)
	sub.SetOut(out)
	root.SetArgs([]string{"add", "--new", "--title", "DryChapter", "--parent", ".", "--project", ".", "--dry-run"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := out.String()
	if !bytes.HasPrefix([]byte(got), []byte("dry-run:")) {
		t.Errorf("expected human output prefixed with 'dry-run:', got: %q", got)
	}
}
