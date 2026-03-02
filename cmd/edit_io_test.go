package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFileEditIO_ReadBinder(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "_binder.md")
	content := []byte("<!-- prosemark-binder:v1 -->\n")
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}

	fio := fileEditIO{}
	got, err := fio.ReadBinder(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("got %q, want %q", got, content)
	}
}

func TestFileEditIO_ReadNodeFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "node.md")
	content := []byte("---\nid: test\n---\n")
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}

	fio := fileEditIO{}
	got, err := fio.ReadNodeFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("got %q, want %q", got, content)
	}
}

func TestFileEditIO_WriteNodeFileAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "node.md")
	content := []byte("---\nid: test\n---\n")

	fio := fileEditIO{}
	if err := fio.WriteNodeFileAtomic(path, content); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("unexpected error reading written file: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("written content = %q, want %q", got, content)
	}
}

func TestFileEditIO_CreateNotesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "node.notes.md")

	fio := fileEditIO{}
	if err := fio.CreateNotesFile(path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file to exist after CreateNotesFile: %v", err)
	}
}

func TestFileEditIO_CreateNotesFile_ExistsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "node.notes.md")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}

	fio := fileEditIO{}
	if err := fio.CreateNotesFile(path); err == nil {
		t.Error("expected error when creating notes file that already exists")
	}
}

func TestFileEditIO_OpenEditor(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	if err := os.WriteFile(path, []byte("draft"), 0600); err != nil {
		t.Fatal(err)
	}

	fio := fileEditIO{}
	// "true" is a POSIX utility that exits 0 — suitable as a no-op editor.
	if err := fio.OpenEditor("true", path); err != nil {
		t.Fatalf("unexpected error with no-op editor: %v", err)
	}
}

// TestFileEditIO_OpenEditor_MultiWordEditor verifies that a multi-word $EDITOR
// value (e.g. "true --extra-arg") is split on whitespace before exec so that
// "true" is the executable and "--extra-arg" is an argument, not part of the
// binary name. The original buggy addchild.go implementation called
// exec.Command(editor, path) which would fail for multi-word editors; this
// test ensures edit.go's shared openEditorImpl path has equivalent behaviour.
func TestFileEditIO_OpenEditor_MultiWordEditor(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	if err := os.WriteFile(path, []byte("draft"), 0600); err != nil {
		t.Fatal(err)
	}

	fio := fileEditIO{}
	// "true --extra-arg" must be split into ["true", "--extra-arg"]; the
	// buggy pattern exec.Command("true --extra-arg", path) fails because no
	// binary named "true --extra-arg" exists on $PATH.
	if err := fio.OpenEditor("true --extra-arg", path); err != nil {
		t.Fatalf("multi-word editor should succeed when split on spaces: %v", err)
	}
}

// TestFileEditIO_OpenEditorImpl_WhitespaceOnlyReturnsError verifies that the
// shared openEditorImpl safety net rejects a whitespace-only editor string.
// This is the inner guard: len(strings.Fields("   ")) == 0 → error.
// Symmetric with TestFileAddChildIO_OpenEditorImpl_WhitespaceOnlyReturnsError.
func TestFileEditIO_OpenEditorImpl_WhitespaceOnlyReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	if err := os.WriteFile(path, []byte("draft"), 0600); err != nil {
		t.Fatal(err)
	}

	fio := fileEditIO{}
	if err := fio.OpenEditorImpl("   ", path); err == nil {
		t.Fatal("expected error when OpenEditorImpl called with whitespace-only editor, got nil")
	}
}

// TestNewEditCmd_BinderParseError covers the error branch after binder.Parse when
// binderBytes contains invalid UTF-8.
func TestNewEditCmd_BinderParseError(t *testing.T) {
	t.Setenv("EDITOR", "vi")
	mock := &mockEditIO{
		binderBytes: []byte{0xff, 0xfe}, // invalid UTF-8 → binder.Parse returns error
	}
	c := NewEditCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{editTestNodeUUID, "--project", "."})

	if err := c.Execute(); err == nil {
		t.Error("expected error when binder contains invalid UTF-8")
	}
}

// TestNewEditCmd_CreateNotesFileError covers the error branch when CreateNotesFile
// returns a non-ErrExist error (notes absent, create fails).
func TestNewEditCmd_CreateNotesFileError(t *testing.T) {
	t.Setenv("EDITOR", "vi")
	mock := &mockEditIO{
		binderBytes: editBinderWithNode(),
		nodeFiles: map[string][]byte{
			editTestNodeUUID + ".md": validEditNodeContent(),
			// notes file NOT present → ReadNodeFile returns ErrNotExist
		},
		createNotesErr: errors.New("permission denied"),
	}
	c := NewEditCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{editTestNodeUUID, "--part", "notes", "--project", "."})

	if err := c.Execute(); err == nil {
		t.Error("expected error when CreateNotesFile fails with non-ErrExist error")
	}
	if len(mock.editorCalls) > 0 {
		t.Error("expected editor not called when CreateNotesFile fails")
	}
}

// TestNewEditCmd_NotesReadNonExistError covers the else branch when ReadNodeFile
// for the notes path returns a non-ErrNotExist error.
func TestNewEditCmd_NotesReadNonExistError(t *testing.T) {
	t.Setenv("EDITOR", "vi")
	mock := &mockEditIO{
		binderBytes: editBinderWithNode(),
		nodeFiles: map[string][]byte{
			editTestNodeUUID + ".md":       validEditNodeContent(),
			editTestNodeUUID + ".notes.md": nil, // nil → returns nodeFileErr
		},
		nodeFileErr: errors.New("permission denied"),
	}
	c := NewEditCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{editTestNodeUUID, "--part", "notes", "--project", "."})

	if err := c.Execute(); err == nil {
		t.Error("expected error when notes ReadNodeFile fails with non-ErrNotExist error")
	}
	if len(mock.editorCalls) > 0 {
		t.Error("expected editor not called when notes read fails")
	}
}

// TestNewEditCmd_PostEditDraftReadError covers the error branch when ReadNodeFile
// for the draft path fails after a successful editor exit.
func TestNewEditCmd_PostEditDraftReadError(t *testing.T) {
	t.Setenv("EDITOR", "vi")
	mock := &mockEditIO{
		binderBytes: editBinderWithNode(),
		nodeFiles: map[string][]byte{
			editTestNodeUUID + ".notes.md": validEditNodeContent(), // notes exist
			// draft NOT present → falls back to nodeFileErr
		},
		nodeFileErr: errors.New("disk read error"),
	}
	c := NewEditCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{editTestNodeUUID, "--part", "notes", "--project", "."})

	if err := c.Execute(); err == nil {
		t.Error("expected error when reading draft after edit fails")
	}
	if len(mock.editorCalls) == 0 {
		t.Error("expected editor to be called before the post-edit read")
	}
	if mock.writtenPath != "" {
		t.Error("expected WriteNodeFileAtomic NOT called when post-edit read fails")
	}
}
