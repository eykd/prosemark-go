package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

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

func TestFileDeleteIO_ReadProject(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "project.json")
	content := []byte(`{"version":"1","files":["chapter-one.md"]}`)
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}

	fio := newDefaultDeleteIO()
	got, err := fio.ReadProject(context.Background(), path)
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
