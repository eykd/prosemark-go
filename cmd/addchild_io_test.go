package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFileAddChildIO_ScanProject(t *testing.T) {
	dir := t.TempDir()
	binderPath := filepath.Join(dir, "_binder.md")
	if err := os.WriteFile(binderPath, []byte("<!-- prosemark-binder:v1 -->\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ch1.md"), nil, 0600); err != nil {
		t.Fatal(err)
	}

	fio := newDefaultAddChildIO()
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

func TestFileAddChildIO_ReadBinder(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "_binder.md")
	content := acBinder()
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}

	fio := newDefaultAddChildIO()
	got, err := fio.ReadBinder(context.Background(), path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("got %q, want %q", got, content)
	}
}

func TestFileAddChildIO_WriteBinderAtomicImpl(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "_binder.md")
	content := acBinder()

	fio := newDefaultAddChildIO()
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

func TestFileAddChildIO_WriteBinderAtomicImpl_RejectsReadOnlyFile(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("permission check bypassed as root")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "_binder.md")
	original := []byte("original content")
	if err := os.WriteFile(path, original, 0444); err != nil {
		t.Fatal(err)
	}
	fio := newDefaultAddChildIO()
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

func TestFileAddChildIO_WriteBinderAtomicImpl_LeavesOriginalOnError(t *testing.T) {
	// Write to a path whose directory does not exist → write must fail, original untouched
	path := filepath.Join(t.TempDir(), "nonexistent-dir", "_binder.md")
	content := []byte("new content")

	fio := newDefaultAddChildIO()
	err := fio.WriteBinderAtomic(context.Background(), path, content)
	if err == nil {
		t.Error("expected error writing to nonexistent directory")
	}
	// The file must not have been created
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Error("expected file to not exist after failed atomic write")
	}
}
