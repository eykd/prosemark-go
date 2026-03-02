package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestFileAddChildIO_WriteNodeFileAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "01234567-89ab-7def-0123-456789abcdef.md")
	content := []byte("---\nid: 01234567-89ab-7def-0123-456789abcdef\n---\n")

	fio := newDefaultAddChildIO()
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

func TestFileAddChildIO_DeleteFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	if err := os.WriteFile(path, []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}

	fio := newDefaultAddChildIO()
	if err := fio.DeleteFile(path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected file to be deleted")
	}
}

func TestFileAddChildIO_ReadNodeFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "01234567-89ab-7def-0123-456789abcdef.md")
	content := []byte("---\nid: 01234567-89ab-7def-0123-456789abcdef\n---\n")
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}

	fio := newDefaultAddChildIO()
	got, err := fio.ReadNodeFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("got %q, want %q", got, content)
	}
}

func TestFileAddChildIO_OpenEditor(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	if err := os.WriteFile(path, []byte("draft"), 0600); err != nil {
		t.Fatal(err)
	}

	fio := newDefaultAddChildIO()
	// "true" is a POSIX utility that exits 0 — suitable as a no-op editor.
	if err := fio.OpenEditor("true", path); err != nil {
		t.Fatalf("unexpected error with no-op editor: %v", err)
	}
}
