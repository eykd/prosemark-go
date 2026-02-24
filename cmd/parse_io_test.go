package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// errWriter is a writer that always returns an error.
type errWriter struct{ err error }

func (e *errWriter) Write(p []byte) (int, error) { return 0, e.err }

func TestNewParseCmd_ReadBinderError(t *testing.T) {
	reader := &mockParseReader{
		binderErr: errors.New("disk error"),
	}
	c := NewParseCmd(reader)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"_binder.md"})

	err := c.Execute()
	if err == nil {
		t.Error("expected error when ReadBinder fails")
	}
	if out.Len() > 0 {
		t.Errorf("expected no stdout on ReadBinder error, got: %s", out.String())
	}
}

func TestNewParseCmd_ScanProjectError(t *testing.T) {
	reader := &mockParseReader{
		binderBytes: []byte("<!-- prosemark-binder:v1 -->\n"),
		projectErr:  errors.New("disk error"),
	}
	c := NewParseCmd(reader)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"_binder.md"})

	err := c.Execute()
	if err == nil {
		t.Error("expected error when ScanProject fails")
	}
}

func TestNewParseCmd_EncodeError(t *testing.T) {
	reader := &mockParseReader{
		binderBytes: []byte("<!-- prosemark-binder:v1 -->\n"),
	}
	c := NewParseCmd(reader)
	c.SetOut(&errWriter{err: errors.New("write error")})
	c.SetArgs([]string{"_binder.md"})

	err := c.Execute()
	if err == nil {
		t.Error("expected error when JSON encoding fails")
	}
}

func TestRootRunE_ReturnsNil(t *testing.T) {
	if err := rootRunE(nil, nil); err != nil {
		t.Errorf("rootRunE() = %v, want nil", err)
	}
}

func TestFileParseReader_ReadBinder(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "_binder.md")
	content := []byte("<!-- prosemark-binder:v1 -->\n")
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}

	r := newDefaultParseReader()
	got, err := r.ReadBinder(context.Background(), path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("got %q, want %q", got, content)
	}
}

func TestFileParseReader_ScanProject(t *testing.T) {
	dir := t.TempDir()
	binderPath := filepath.Join(dir, "_binder.md")
	if err := os.WriteFile(binderPath, []byte("<!-- prosemark-binder:v1 -->\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ch1.md"), nil, 0600); err != nil {
		t.Fatal(err)
	}

	r := newDefaultParseReader()
	proj, err := r.ScanProject(context.Background(), binderPath)
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
