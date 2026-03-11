package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
)

func TestFileDeleteIO_ScanProject(t *testing.T) {
	dir := t.TempDir()
	binderPath := filepath.Join(dir, "_binder.md")
	if err := os.WriteFile(binderPath, []byte("<!-- prosemark-binder:v1 -->\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ch1.md"), nil, 0600); err != nil {
		t.Fatal(err)
	}

	fio := newDefaultDeleteIO()
	proj, err := fio.ScanProject(context.Background(), binderPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proj == nil {
		t.Fatal("expected non-nil project")
	}
	if len(proj.Files) != 1 || proj.Files[0] != "ch1.md" {
		t.Errorf("project.Files = %v, want [ch1.md]", proj.Files)
	}
}

func TestFileDeleteIO_ReadBinder(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "_binder.md")
	content := delBinder()
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}

	fio := newDefaultDeleteIO()
	got, err := fio.ReadBinder(context.Background(), path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("got %q, want %q", got, content)
	}
}

func TestFileDeleteIO_WriteBinderAtomicImpl(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "_binder.md")
	content := delBinder()

	fio := newDefaultDeleteIO()
	if err := fio.WriteBinderAtomic(context.Background(), path, content); err != nil {
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

func TestFileDeleteIO_WriteBinderAtomicImpl_RejectsReadOnlyFile(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("permission check bypassed as root")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "_binder.md")
	original := []byte("original content")
	if err := os.WriteFile(path, original, 0444); err != nil {
		t.Fatal(err)
	}
	fio := newDefaultDeleteIO()
	err := fio.WriteBinderAtomic(context.Background(), path, []byte("new content"))
	if err == nil {
		t.Error("expected error writing to read-only file")
	}
	got, _ := os.ReadFile(path)
	if !bytes.Equal(got, original) {
		t.Error("original content must be unchanged after failed write")
	}
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0444 {
		t.Error("file permissions must be unchanged after failed write")
	}
}

func TestFileDeleteIO_RemoveFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chapter.md")
	if err := os.WriteFile(path, []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}

	fio := newDefaultDeleteIO()
	if err := fio.RemoveFile(context.Background(), path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected file to be removed")
	}
}

func TestDeleteCollectTargets_ParseError(t *testing.T) {
	// Non-UTF-8 bytes trigger a parse error; should return nil, not panic.
	got := deleteCollectTargets([]byte{0xff, 0xfe}, nil)
	if got != nil {
		t.Errorf("expected nil for unparseable input, got %v", got)
	}
}

func TestDeleteRemoveNodeFiles_RemoveError(t *testing.T) {
	original := []byte("<!-- prosemark-binder:v1 -->\n- [Ch](ch.md)\n")
	modified := []byte("<!-- prosemark-binder:v1 -->\n")
	proj := &binder.Project{Files: []string{"ch.md"}, BinderDir: "/proj"}
	mock := &mockDeleteIO{removeFileErr: errors.New("perm denied")}

	_, err := deleteRemoveNodeFiles(context.Background(), mock, original, modified, proj)
	if err == nil {
		t.Error("expected error when RemoveFile fails")
	}
}

func TestDeleteRemoveNodeFiles_PartialDelete(t *testing.T) {
	// Modified binder still has one node (covers modSet population loop).
	original := []byte("<!-- prosemark-binder:v1 -->\n- [A](a.md)\n- [B](b.md)\n")
	modified := []byte("<!-- prosemark-binder:v1 -->\n- [A](a.md)\n")
	proj := &binder.Project{Files: []string{"a.md", "b.md"}, BinderDir: "/proj"}
	mock := &mockDeleteIO{}

	removed, err := deleteRemoveNodeFiles(context.Background(), mock, original, modified, proj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(removed) != 1 || removed[0] != "b.md" {
		t.Errorf("removed = %v, want [b.md]", removed)
	}
}

func TestNewDeleteCmd_RmWriteRemovedMessageError(t *testing.T) {
	mock := &mockDeleteIO{
		binderBytes: delBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md"}, BinderDir: "/proj"},
	}
	c := NewDeleteCmd(mock)
	c.SetOut(&errWriterAfterN{n: 1, err: errors.New("write error")})
	c.SetArgs([]string{"--selector", "chapter-one", "--yes", "--rm", "--project", "/proj"})

	if err := c.Execute(); err == nil {
		t.Error("expected error when writing removed message fails")
	}
}

// errWriterAfterN succeeds for the first n writes, then returns err.
type errWriterAfterN struct {
	n     int
	count int
	err   error
}

func (w *errWriterAfterN) Write(p []byte) (int, error) {
	w.count++
	if w.count > w.n {
		return 0, w.err
	}
	return len(p), nil
}

func TestFileDeleteIO_WriteBinderAtomicImpl_LeavesOriginalOnError(t *testing.T) {
	// Write to a path whose directory does not exist → write must fail, original untouched
	path := filepath.Join(t.TempDir(), "nonexistent-dir", "_binder.md")
	content := []byte("new content")

	fio := newDefaultDeleteIO()
	err := fio.WriteBinderAtomic(context.Background(), path, content)
	if err == nil {
		t.Error("expected error writing to nonexistent directory")
	}
	// The file must not have been created
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Error("expected file to not exist after failed atomic write")
	}
}
