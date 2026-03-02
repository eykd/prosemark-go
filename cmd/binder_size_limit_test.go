package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// binderSizeLimit is the maximum allowed size for _binder.md files (10 MB).
const binderSizeLimit = 10 * 1024 * 1024

// makeOversizedBinderContent returns content that exceeds the 10 MB binder size limit.
func makeOversizedBinderContent() []byte {
	header := []byte("<!-- prosemark-binder:v1 -->\n")
	padding := bytes.Repeat([]byte("x"), binderSizeLimit+1)
	return append(header, padding...)
}

// writeTempBinderFile writes content to _binder.md in a temp dir and returns the path.
func writeTempBinderFile(t *testing.T, content []byte) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "_binder.md")
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

// assertSizeLimitError checks that err is non-nil and mentions the size limit.
func assertSizeLimitError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error for oversized binder file, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "10 MB") && !strings.Contains(msg, "size limit") && !strings.Contains(msg, "too large") && !strings.Contains(msg, "exceeds") {
		t.Errorf("error message should describe the size limit, got: %q", msg)
	}
}

// --- fileParseReader ---

func TestFileParseReader_ReadBinder_RejectsOversizedFile(t *testing.T) {
	path := writeTempBinderFile(t, makeOversizedBinderContent())

	r := newDefaultParseReader()
	_, err := r.ReadBinder(context.Background(), path)
	assertSizeLimitError(t, err)
}

func TestFileParseReader_ReadBinder_AcceptsFileAtSizeLimit(t *testing.T) {
	header := []byte("<!-- prosemark-binder:v1 -->\n")
	content := append(header, bytes.Repeat([]byte("x"), binderSizeLimit-len(header))...)
	path := writeTempBinderFile(t, content)

	r := newDefaultParseReader()
	_, err := r.ReadBinder(context.Background(), path)
	if err != nil {
		t.Errorf("expected no error for file exactly at size limit, got: %v", err)
	}
}

// --- fileDeleteIO ---

func TestFileDeleteIO_ReadBinder_RejectsOversizedFile(t *testing.T) {
	path := writeTempBinderFile(t, makeOversizedBinderContent())

	fio := newDefaultDeleteIO()
	_, err := fio.ReadBinder(context.Background(), path)
	assertSizeLimitError(t, err)
}

func TestFileDeleteIO_ReadBinder_AcceptsFileAtSizeLimit(t *testing.T) {
	header := []byte("<!-- prosemark-binder:v1 -->\n")
	content := append(header, bytes.Repeat([]byte("x"), binderSizeLimit-len(header))...)
	path := writeTempBinderFile(t, content)

	fio := newDefaultDeleteIO()
	_, err := fio.ReadBinder(context.Background(), path)
	if err != nil {
		t.Errorf("expected no error for file exactly at size limit, got: %v", err)
	}
}

// --- fileAddChildIO ---

func TestFileAddChildIO_ReadBinder_RejectsOversizedFile(t *testing.T) {
	path := writeTempBinderFile(t, makeOversizedBinderContent())

	fio := newDefaultAddChildIO()
	_, err := fio.ReadBinder(context.Background(), path)
	assertSizeLimitError(t, err)
}

func TestFileAddChildIO_ReadBinder_AcceptsFileAtSizeLimit(t *testing.T) {
	header := []byte("<!-- prosemark-binder:v1 -->\n")
	content := append(header, bytes.Repeat([]byte("x"), binderSizeLimit-len(header))...)
	path := writeTempBinderFile(t, content)

	fio := newDefaultAddChildIO()
	_, err := fio.ReadBinder(context.Background(), path)
	if err != nil {
		t.Errorf("expected no error for file exactly at size limit, got: %v", err)
	}
}

// --- fileMoveIO ---

func TestFileMoveIO_ReadBinder_RejectsOversizedFile(t *testing.T) {
	path := writeTempBinderFile(t, makeOversizedBinderContent())

	fio := newDefaultMoveIO()
	_, err := fio.ReadBinder(context.Background(), path)
	assertSizeLimitError(t, err)
}

func TestFileMoveIO_ReadBinder_AcceptsFileAtSizeLimit(t *testing.T) {
	header := []byte("<!-- prosemark-binder:v1 -->\n")
	content := append(header, bytes.Repeat([]byte("x"), binderSizeLimit-len(header))...)
	path := writeTempBinderFile(t, content)

	fio := newDefaultMoveIO()
	_, err := fio.ReadBinder(context.Background(), path)
	if err != nil {
		t.Errorf("expected no error for file exactly at size limit, got: %v", err)
	}
}
