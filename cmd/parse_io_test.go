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
	c.SetArgs([]string{"--json", "--project", "project.json", "_binder.md"})

	err := c.Execute()
	if err == nil {
		t.Error("expected error when ReadBinder fails")
	}
	if out.Len() > 0 {
		t.Errorf("expected no stdout on ReadBinder error, got: %s", out.String())
	}
}

func TestNewParseCmd_ReadProjectError(t *testing.T) {
	reader := &mockParseReader{
		binderBytes: []byte("<!-- prosemark-binder:v1 -->\n"),
		projectErr:  errors.New("disk error"),
	}
	c := NewParseCmd(reader)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--json", "--project", "project.json", "_binder.md"})

	err := c.Execute()
	if err == nil {
		t.Error("expected error when ReadProject fails")
	}
}

func TestNewParseCmd_InvalidProjectJSON(t *testing.T) {
	reader := &mockParseReader{
		binderBytes:  []byte("<!-- prosemark-binder:v1 -->\n"),
		projectBytes: []byte("not valid json"),
	}
	c := NewParseCmd(reader)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--json", "--project", "project.json", "_binder.md"})

	err := c.Execute()
	if err == nil {
		t.Error("expected error when project JSON is invalid")
	}
}

func TestNewParseCmd_EncodeError(t *testing.T) {
	reader := &mockParseReader{
		binderBytes:  []byte("<!-- prosemark-binder:v1 -->\n"),
		projectBytes: []byte(`{"version":"1","files":[]}`),
	}
	c := NewParseCmd(reader)
	c.SetOut(&errWriter{err: errors.New("write error")})
	c.SetArgs([]string{"--json", "--project", "project.json", "_binder.md"})

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

func TestFileParseReader_ReadProject(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "project.json")
	content := []byte(`{"version":"1","files":[]}`)
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}

	r := newDefaultParseReader()
	got, err := r.ReadProject(context.Background(), path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("got %q, want %q", got, content)
	}
}
