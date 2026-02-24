package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
)

// mockDeleteIO is a test double for DeleteIO.
type mockDeleteIO struct {
	binderBytes  []byte
	project      *binder.Project
	binderErr    error
	projectErr   error
	writeErr     error
	writtenBytes []byte
	writtenPath  string
}

func (m *mockDeleteIO) ReadBinder(_ context.Context, _ string) ([]byte, error) {
	return m.binderBytes, m.binderErr
}

func (m *mockDeleteIO) ScanProject(_ context.Context, _ string) (*binder.Project, error) {
	if m.project != nil {
		return m.project, m.projectErr
	}
	return &binder.Project{Files: []string{}, BinderDir: "."}, m.projectErr
}

func (m *mockDeleteIO) WriteBinderAtomic(_ context.Context, path string, data []byte) error {
	m.writtenPath = path
	m.writtenBytes = data
	return m.writeErr
}

// delBinder returns a minimal binder with one child node for delete tests.
func delBinder() []byte {
	return []byte("<!-- prosemark-binder:v1 -->\n- [Chapter One](chapter-one.md)\n")
}

func TestNewDeleteCmd_HasRequiredFlags(t *testing.T) {
	c := NewDeleteCmd(nil)
	required := []string{"project", "selector", "yes", "json"}
	for _, name := range required {
		name := name
		t.Run(name, func(t *testing.T) {
			if c.Flags().Lookup(name) == nil {
				t.Errorf("expected --%s flag on delete command", name)
			}
		})
	}
}

func TestNewDeleteCmd_DefaultsToCWD(t *testing.T) {
	mock := &mockDeleteIO{
		binderBytes: delBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md"}, BinderDir: "."},
	}
	c := NewDeleteCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--selector", "chapter-one.md", "--yes"})

	if err := c.Execute(); err != nil {
		t.Fatalf("expected success with no --project (CWD default): %v", err)
	}
}

func TestNewDeleteCmd_AcceptsProjectFlag(t *testing.T) {
	mock := &mockDeleteIO{
		binderBytes: delBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md"}, BinderDir: "."},
	}
	c := NewDeleteCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--selector", "chapter-one.md", "--yes", "--project", "/some/dir"})

	if err := c.Execute(); err != nil {
		t.Fatalf("expected success with --project flag: %v", err)
	}
}

func TestNewDeleteCmd_GetCWDError(t *testing.T) {
	mock := &mockDeleteIO{binderBytes: delBinder()}
	c := newDeleteCmdWithGetCWD(mock, func() (string, error) {
		return "", errors.New("getwd failed")
	})
	c.SetOut(new(bytes.Buffer))
	c.SetArgs([]string{"--selector", "chapter-one.md", "--yes"})

	if err := c.Execute(); err == nil {
		t.Error("expected error when getwd fails")
	}
}

func TestNewDeleteCmd_PrintsConfirmationOnSuccess(t *testing.T) {
	mock := &mockDeleteIO{
		binderBytes: delBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md"}, BinderDir: "."},
	}
	c := NewDeleteCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--selector", "chapter-one.md", "--yes", "--project", "."})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "Deleted chapter-one.md from _binder.md") {
		t.Errorf("expected stdout to contain confirmation, got: %s", out.String())
	}
}

func TestNewDeleteCmd_WritesModifiedBinderOnChange(t *testing.T) {
	mock := &mockDeleteIO{
		binderBytes: delBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md"}, BinderDir: "."},
	}
	c := NewDeleteCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetArgs([]string{"--selector", "chapter-one.md", "--yes", "--project", "."})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mock.writtenPath != "_binder.md" {
		t.Errorf("written to %q, want \"_binder.md\"", mock.writtenPath)
	}
	if len(mock.writtenBytes) == 0 {
		t.Error("expected non-empty bytes written to binder")
	}
}

func TestNewDeleteCmd_ReadBinderError(t *testing.T) {
	mock := &mockDeleteIO{binderErr: errors.New("disk error")}
	c := NewDeleteCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--selector", "chapter-one.md", "--yes", "--project", "."})

	if err := c.Execute(); err == nil {
		t.Error("expected error when ReadBinder fails")
	}
	if out.Len() > 0 {
		t.Errorf("expected no stdout on ReadBinder error, got: %s", out.String())
	}
}

func TestNewDeleteCmd_ScanProjectError(t *testing.T) {
	mock := &mockDeleteIO{
		binderBytes: delBinder(),
		projectErr:  errors.New("disk error"),
	}
	c := NewDeleteCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetArgs([]string{"--selector", "chapter-one.md", "--yes", "--project", "."})

	if err := c.Execute(); err == nil {
		t.Error("expected error when ScanProject fails")
	}
}

func TestNewDeleteCmd_WriteError(t *testing.T) {
	mock := &mockDeleteIO{
		binderBytes: delBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md"}, BinderDir: "."},
		writeErr:    errors.New("write failed"),
	}
	c := NewDeleteCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetArgs([]string{"--selector", "chapter-one.md", "--yes", "--project", "."})

	if err := c.Execute(); err == nil {
		t.Error("expected error when WriteBinderAtomic fails")
	}
}

func TestNewDeleteCmd_ExitsNonZeroOnOpErrors(t *testing.T) {
	// Selector that matches no node → OPE001 (error-severity diagnostic)
	mock := &mockDeleteIO{
		binderBytes: delBinder(),
	}
	c := NewDeleteCmd(mock)
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)
	c.SetOut(out)
	c.SetErr(errOut)
	c.SetArgs([]string{"--selector", "nonexistent.md", "--yes", "--project", "."})

	err := c.Execute()
	if err == nil {
		t.Error("expected non-zero exit when op has error diagnostics")
	}
	if out.Len() > 0 {
		t.Errorf("expected no stdout on op error, got: %s", out.String())
	}
	if !strings.Contains(errOut.String(), "OPE001") {
		t.Errorf("expected errOut to contain OPE001, got: %s", errOut.String())
	}
	// Binder must NOT be written when op has error diagnostics
	if mock.writtenPath != "" {
		t.Errorf("binder must not be written when op has error diagnostics, was written to %q", mock.writtenPath)
	}
}

func TestNewDeleteCmd_MissingYesFlagReturnsError(t *testing.T) {
	// Omitting --yes → ops.Delete returns OPE009 (missing confirmation)
	mock := &mockDeleteIO{
		binderBytes: delBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md"}, BinderDir: "."},
	}
	c := NewDeleteCmd(mock)
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)
	c.SetOut(out)
	c.SetErr(errOut)
	c.SetArgs([]string{"--selector", "chapter-one.md", "--project", "."})

	if err := c.Execute(); err == nil {
		t.Error("expected error when --yes flag is missing (OPE009)")
	}
	if out.Len() > 0 {
		t.Errorf("expected no stdout for OPE009 error, got: %s", out.String())
	}
	if !strings.Contains(errOut.String(), "OPE009") {
		t.Errorf("expected errOut to contain OPE009, got: %s", errOut.String())
	}
}

func TestNewDeleteCmd_WriteSuccessMessageError(t *testing.T) {
	mock := &mockDeleteIO{
		binderBytes: delBinder(),
	}
	c := NewDeleteCmd(mock)
	c.SetOut(&errWriter{err: errors.New("write error")})
	c.SetArgs([]string{"--selector", "chapter-one.md", "--yes", "--project", "."})

	if err := c.Execute(); err == nil {
		t.Error("expected error when writing success message fails")
	}
}

func TestNewDeleteCmd_OutputsOpResultJSONOnSuccess(t *testing.T) {
	mock := &mockDeleteIO{
		binderBytes: delBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md"}, BinderDir: "."},
	}
	c := NewDeleteCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--selector", "chapter-one.md", "--yes", "--json", "--project", "."})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result binder.OpResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out.String())
	}
	if result.Version != "1" {
		t.Errorf("version = %q, want \"1\"", result.Version)
	}
	if !result.Changed {
		t.Error("expected Changed=true when node is deleted")
	}
}

func TestNewDeleteCmd_JSONEncodeError(t *testing.T) {
	mock := &mockDeleteIO{
		binderBytes: delBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md"}, BinderDir: "."},
	}
	c := NewDeleteCmd(mock)
	c.SetOut(&errWriter{err: errors.New("write error")})
	c.SetArgs([]string{"--selector", "chapter-one.md", "--yes", "--json", "--project", "."})

	if err := c.Execute(); err == nil {
		t.Error("expected error when JSON encoding fails")
	}
}

func TestNewDeleteCmd_ScanProjectErrorWithJSON(t *testing.T) {
	mock := &mockDeleteIO{
		binderBytes: delBinder(),
		projectErr:  errors.New("disk error"),
	}
	c := NewDeleteCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--selector", "chapter-one.md", "--yes", "--json", "--project", "."})

	if err := c.Execute(); err == nil {
		t.Error("expected error when ScanProject fails")
	}

	var result binder.OpResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("expected OPE009 JSON on stdout, got: %s", out.String())
	}
	if len(result.Diagnostics) == 0 || result.Diagnostics[0].Code != binder.CodeIOOrParseFailure {
		t.Errorf("expected OPE009 diagnostic, got: %v", result.Diagnostics)
	}
}

func TestNewRootCmd_RegistersDeleteSubcommand(t *testing.T) {
	root := NewRootCmd()
	var found bool
	for _, sub := range root.Commands() {
		if sub.Name() == "delete" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected \"delete\" subcommand registered on root command")
	}
}
