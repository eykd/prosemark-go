package cmd

import (
	"bytes"
	"testing"
)

// --- edit dry-run tests ---
//
// The edit command currently has no dry-run support. It opens the editor and
// writes files unconditionally. These tests verify the expected dry-run behavior:
// skip editor, skip writes, skip notes creation.

func TestEditCmd_DryRun_SkipsEditorOpen(t *testing.T) {
	t.Setenv("EDITOR", "vi")
	mock := &mockEditIO{
		binderBytes:   editBinderWithNode(),
		nodeFileBytes: validEditNodeContent(),
	}
	sub := NewEditCmd(mock)
	root := withDryRunFlag(sub)
	root.SetOut(new(bytes.Buffer))
	root.SetArgs([]string{"edit", editTestNodeUUID, "--project", ".", "--dry-run"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.editorCalls) > 0 {
		t.Errorf("dry-run must not open editor, but OpenEditor was called %d time(s)", len(mock.editorCalls))
	}
}

func TestEditCmd_DryRun_SkipsNodeFileWrite(t *testing.T) {
	t.Setenv("EDITOR", "vi")
	mock := &mockEditIO{
		binderBytes:   editBinderWithNode(),
		nodeFileBytes: validEditNodeContent(),
	}
	sub := NewEditCmd(mock)
	root := withDryRunFlag(sub)
	root.SetOut(new(bytes.Buffer))
	root.SetArgs([]string{"edit", editTestNodeUUID, "--project", ".", "--dry-run"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mock.writtenPath != "" {
		t.Errorf("dry-run must not write node file, but WriteNodeFileAtomic was called with %q", mock.writtenPath)
	}
}

func TestEditCmd_DryRun_SkipsNotesCreation(t *testing.T) {
	t.Setenv("EDITOR", "vi")
	mock := &mockEditIO{
		binderBytes: editBinderWithNode(),
		nodeFiles: map[string][]byte{
			editTestNodeUUID + ".md": validEditNodeContent(),
			// notes file NOT present → would normally trigger CreateNotesFile
		},
	}
	sub := NewEditCmd(mock)
	root := withDryRunFlag(sub)
	root.SetOut(new(bytes.Buffer))
	root.SetArgs([]string{"edit", editTestNodeUUID, "--part", "notes", "--project", ".", "--dry-run"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mock.notesCreated != "" {
		t.Errorf("dry-run must not create notes file, but CreateNotesFile was called with %q", mock.notesCreated)
	}
	if len(mock.editorCalls) > 0 {
		t.Errorf("dry-run must not open editor, but OpenEditor was called %d time(s)", len(mock.editorCalls))
	}
}

func TestEditCmd_DryRun_HumanOutputPrefix(t *testing.T) {
	t.Setenv("EDITOR", "vi")
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
		t.Fatalf("unexpected error: %v", err)
	}

	got := out.String()
	if !bytes.HasPrefix([]byte(got), []byte("dry-run:")) {
		t.Errorf("expected human output prefixed with 'dry-run:', got: %q", got)
	}
}
