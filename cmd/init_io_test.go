package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileInitIO_StatFile_ExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "_binder.md")
	if err := os.WriteFile(path, []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}

	fio := fileInitIO{}
	exists, err := fio.StatFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected StatFile to return true for existing file")
	}
}

func TestFileInitIO_StatFile_NonExistentFile(t *testing.T) {
	fio := fileInitIO{}
	exists, err := fio.StatFile(filepath.Join(t.TempDir(), "nonexistent.md"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected StatFile to return false for non-existent file")
	}
}

func TestFileInitIO_WriteFileAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "_binder.md")
	content := "<!-- prosemark-binder:v1 -->\n"

	fio := fileInitIO{}
	if err := fio.WriteFileAtomic(path, content); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("unexpected error reading written file: %v", err)
	}
	if string(got) != content {
		t.Errorf("written content = %q, want %q", got, content)
	}
}

func TestFileInitIO_WriteFileAtomic_CreatesWithCorrectPermissions(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("permission check bypassed as root")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "_binder.md")

	fio := fileInitIO{}
	if err := fio.WriteFileAtomic(path, "content"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("unexpected error stat'ing file: %v", err)
	}
	if fi.Mode().Perm() != 0600 {
		t.Errorf("file permissions = %o, want 0600", fi.Mode().Perm())
	}
}

func TestFileInitIO_WriteFileAtomic_LeavesNothingOnError(t *testing.T) {
	// Write to a path whose directory does not exist → must fail
	path := filepath.Join(t.TempDir(), "nonexistent-dir", "_binder.md")

	fio := fileInitIO{}
	err := fio.WriteFileAtomic(path, "content")
	if err == nil {
		t.Error("expected error writing to nonexistent directory")
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Error("expected file to not exist after failed atomic write")
	}
}
