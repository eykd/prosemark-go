package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/eykd/prosemark-go/internal/node"
)

func TestFileDoctorIO_ReadBinder(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "_binder.md")
	content := []byte("<!-- prosemark-binder:v1 -->\n")
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}

	fio := fileDoctorIO{}
	got, err := fio.ReadBinder(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("got %q, want %q", got, content)
	}
}

func TestFileDoctorIO_ListUUIDFiles(t *testing.T) {
	dir := t.TempDir()
	uuidName := "01234567-89ab-7def-0123-456789abcdef.md"
	nonUUID := "chapter-one.md"

	for _, name := range []string{uuidName, nonUUID} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}

	fio := fileDoctorIO{}
	got, err := fio.ListUUIDFiles(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != uuidName {
		t.Errorf("ListUUIDFiles = %v, want [%s]", got, uuidName)
	}
}

func TestFileDoctorIO_ReadNodeFile_Exists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "node.md")
	content := []byte("---\nid: test\n---\nbody\n")
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}

	fio := fileDoctorIO{}
	got, exists, err := fio.ReadNodeFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected exists=true for existing file")
	}
	if !bytes.Equal(got, content) {
		t.Errorf("got %q, want %q", got, content)
	}
}

func TestFileDoctorIO_ReadNodeFile_NotExists(t *testing.T) {
	fio := fileDoctorIO{}
	got, exists, err := fio.ReadNodeFile("/nonexistent/path/node.md")
	if err != nil {
		t.Fatalf("unexpected error for missing file: %v", err)
	}
	if exists {
		t.Error("expected exists=false for missing file")
	}
	if got != nil {
		t.Errorf("expected nil content for missing file, got %q", got)
	}
}

func TestScanEscapingBinderLinks_NoEscaping(t *testing.T) {
	binderBytes := []byte("<!-- prosemark-binder:v1 -->\n- [Node](01234567-89ab-7def-0123-456789abcdef.md)\n")
	diags := scanEscapingBinderLinks(binderBytes)
	if len(diags) != 0 {
		t.Errorf("expected no diagnostics for non-escaping binder, got: %v", diags)
	}
}

func TestScanEscapingBinderLinks_WithEscaping(t *testing.T) {
	binderBytes := []byte("<!-- prosemark-binder:v1 -->\n- [Secret](../../etc/passwd)\n")
	diags := scanEscapingBinderLinks(binderBytes)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic for escaping link, got %d: %v", len(diags), diags)
	}
	if diags[0].Code != node.AUDW001 {
		t.Errorf("expected AUDW001, got %s", diags[0].Code)
	}
	if diags[0].Severity != node.SeverityWarning {
		t.Errorf("expected warning severity, got %s", diags[0].Severity)
	}
}

func TestDoctorReadFile_NotExists(t *testing.T) {
	mock := &mockDoctorIO{
		nodeFiles: map[string]nodeFileEntry{
			"missing.md": {content: nil, exists: false},
		},
	}
	got := doctorReadFile(mock, ".", "missing.md")
	if got != nil {
		t.Errorf("expected nil for non-existing file, got %q", got)
	}
}

func TestDoctorReadFile_Oversized(t *testing.T) {
	oversized := bytes.Repeat([]byte("x"), 1024*1024+1)
	mock := &mockDoctorIO{
		nodeFiles: map[string]nodeFileEntry{
			"big.md": {content: oversized, exists: true},
		},
	}
	got := doctorReadFile(mock, ".", "big.md")
	if got == nil {
		t.Fatal("expected non-nil sentinel for oversized file")
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice sentinel for oversized file, got len=%d", len(got))
	}
}

// TestNewDoctorCmd_ListUUIDFilesError verifies that a ListUUIDFiles failure is
// non-fatal: the command proceeds with an empty UUID file list and exits 0.
func TestNewDoctorCmd_ListUUIDFilesError(t *testing.T) {
	mock := &mockDoctorIO{
		binderBytes:  doctorBinderEmpty(),
		uuidFilesErr: errors.New("disk error"),
	}
	c := NewDoctorCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--project", "."})
	if err := c.Execute(); err != nil {
		t.Errorf("expected nil error when ListUUIDFiles fails, got: %v", err)
	}
}

func TestDoctorReadFile_Normal(t *testing.T) {
	content := []byte("---\nid: test\n---\nbody\n")
	mock := &mockDoctorIO{
		nodeFiles: map[string]nodeFileEntry{
			"node.md": {content: content, exists: true},
		},
	}
	got := doctorReadFile(mock, ".", "node.md")
	if !bytes.Equal(got, content) {
		t.Errorf("got %q, want %q", got, content)
	}
}
