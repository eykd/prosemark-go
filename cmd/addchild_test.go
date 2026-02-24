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

// mockAddChildIO is a test double for AddChildIO.
type mockAddChildIO struct {
	binderBytes  []byte
	project      *binder.Project
	binderErr    error
	projectErr   error
	writeErr     error
	writtenBytes []byte
	writtenPath  string
}

func (m *mockAddChildIO) ReadBinder(_ context.Context, _ string) ([]byte, error) {
	return m.binderBytes, m.binderErr
}

func (m *mockAddChildIO) ScanProject(_ context.Context, _ string) (*binder.Project, error) {
	if m.project != nil {
		return m.project, m.projectErr
	}
	return &binder.Project{Files: []string{}, BinderDir: "."}, m.projectErr
}

func (m *mockAddChildIO) WriteBinderAtomic(_ context.Context, path string, data []byte) error {
	m.writtenPath = path
	m.writtenBytes = data
	return m.writeErr
}

// acBinder returns a minimal valid binder with one child node.
func acBinder() []byte {
	return []byte("<!-- prosemark-binder:v1 -->\n- [Chapter One](chapter-one.md)\n")
}

func TestNewAddChildCmd_HasRequiredFlags(t *testing.T) {
	c := NewAddChildCmd(nil)
	required := []string{"parent", "target", "title", "first", "at", "before", "after", "force", "json"}
	for _, name := range required {
		name := name
		t.Run(name, func(t *testing.T) {
			if c.Flags().Lookup(name) == nil {
				t.Errorf("expected --%s flag on add-child command", name)
			}
		})
	}
}

func TestNewAddChildCmd_RejectsNon_binderMdFilename(t *testing.T) {
	tests := []struct {
		name       string
		binderPath string
		wantErr    bool
	}{
		{"valid _binder.md", "_binder.md", false},
		{"valid nested path", "project/_binder.md", false},
		{"invalid notes.md", "notes.md", true},
		{"invalid binder.md without underscore", "binder.md", true},
		{"invalid README.md", "README.md", true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockAddChildIO{
				binderBytes: acBinder(),
				project:     &binder.Project{Files: []string{"chapter-two.md"}, BinderDir: "."},
			}
			c := NewAddChildCmd(mock)
			out := new(bytes.Buffer)
			errOut := new(bytes.Buffer)
			c.SetOut(out)
			c.SetErr(errOut)
			c.SetArgs([]string{"--parent", ".", "--target", "chapter-two.md", tt.binderPath})

			err := c.Execute()
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && out.Len() > 0 {
				t.Errorf("expected no stdout output on filename validation error, got: %s", out.String())
			}
		})
	}
}

func TestNewAddChildCmd_PrintsConfirmationOnSuccess(t *testing.T) {
	mock := &mockAddChildIO{
		binderBytes: acBinder(),
		project:     &binder.Project{Files: []string{"chapter-two.md"}, BinderDir: "."},
	}
	c := NewAddChildCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--parent", ".", "--target", "chapter-two.md", "_binder.md"})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "Added chapter-two.md to _binder.md") {
		t.Errorf("expected stdout to contain confirmation, got: %s", out.String())
	}
}

func TestNewAddChildCmd_WritesModifiedBinderOnChange(t *testing.T) {
	mock := &mockAddChildIO{
		binderBytes: acBinder(),
		project:     &binder.Project{Files: []string{"chapter-two.md"}, BinderDir: "."},
	}
	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetArgs([]string{"--parent", ".", "--target", "chapter-two.md", "_binder.md"})

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

func TestNewAddChildCmd_DoesNotWriteBinderWhenUnchanged(t *testing.T) {
	// Adding an existing target without --force → OPW002 (duplicate skipped, no bytes change)
	mock := &mockAddChildIO{
		binderBytes: acBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md"}, BinderDir: "."},
	}
	c := NewAddChildCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	// chapter-one.md already exists in binder → OPW002 (duplicate skipped, changed=false)
	c.SetArgs([]string{"--parent", ".", "--target", "chapter-one.md", "_binder.md"})

	if err := c.Execute(); err != nil {
		t.Fatalf("expected exit 0 for OPW002, got: %v", err)
	}
	if mock.writtenPath != "" {
		t.Errorf("binder must not be written when no change occurred, but was written to %q", mock.writtenPath)
	}
}

func TestNewAddChildCmd_ReadBinderError(t *testing.T) {
	mock := &mockAddChildIO{binderErr: errors.New("disk error")}
	c := NewAddChildCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--parent", ".", "--target", "chapter-two.md", "_binder.md"})

	if err := c.Execute(); err == nil {
		t.Error("expected error when ReadBinder fails")
	}
	if out.Len() > 0 {
		t.Errorf("expected no stdout on ReadBinder error, got: %s", out.String())
	}
}

func TestNewAddChildCmd_ScanProjectError(t *testing.T) {
	mock := &mockAddChildIO{
		binderBytes: acBinder(),
		projectErr:  errors.New("disk error"),
	}
	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetArgs([]string{"--parent", ".", "--target", "chapter-two.md", "_binder.md"})

	if err := c.Execute(); err == nil {
		t.Error("expected error when ScanProject fails")
	}
}

func TestNewAddChildCmd_WriteError(t *testing.T) {
	mock := &mockAddChildIO{
		binderBytes: acBinder(),
		project:     &binder.Project{Files: []string{"chapter-two.md"}, BinderDir: "."},
		writeErr:    errors.New("write failed"),
	}
	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetArgs([]string{"--parent", ".", "--target", "chapter-two.md", "_binder.md"})

	if err := c.Execute(); err == nil {
		t.Error("expected error when WriteBinderAtomic fails")
	}
}

func TestNewAddChildCmd_ExitsNonZeroOnOpErrors(t *testing.T) {
	// Selector that matches no node → OPE001 (error-severity diagnostic)
	mock := &mockAddChildIO{
		binderBytes: acBinder(),
	}
	c := NewAddChildCmd(mock)
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)
	c.SetOut(out)
	c.SetErr(errOut)
	c.SetArgs([]string{"--parent", "nonexistent.md", "--target", "ch.md", "_binder.md"})

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

func TestNewAddChildCmd_ExitsZeroOnWarningOnly(t *testing.T) {
	// OPW002: adding existing target without --force → warning severity, exit 0
	mock := &mockAddChildIO{
		binderBytes: acBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md"}, BinderDir: "."},
	}
	c := NewAddChildCmd(mock)
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)
	c.SetOut(out)
	c.SetErr(errOut)
	c.SetArgs([]string{"--parent", ".", "--target", "chapter-one.md", "_binder.md"})

	if err := c.Execute(); err != nil {
		t.Errorf("expected exit 0 for warning-only (OPW002), got: %v", err)
	}
	if !strings.Contains(out.String(), "skipped") {
		t.Errorf("expected stdout to contain \"skipped\", got: %s", out.String())
	}
	if !strings.Contains(errOut.String(), "OPW002") {
		t.Errorf("expected errOut to contain \"OPW002\", got: %s", errOut.String())
	}
}

func TestNewAddChildCmd_ForceFlag(t *testing.T) {
	// --force allows adding a duplicate target; binder IS changed
	mock := &mockAddChildIO{
		binderBytes: acBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md"}, BinderDir: "."},
	}
	c := NewAddChildCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--parent", ".", "--target", "chapter-one.md", "--force", "_binder.md"})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error with --force on duplicate: %v", err)
	}
	if mock.writtenPath == "" {
		t.Error("expected binder to be written when --force produces a change")
	}
}

func TestNewAddChildCmd_WriteSuccessMessageError(t *testing.T) {
	mock := &mockAddChildIO{
		binderBytes: acBinder(),
	}
	c := NewAddChildCmd(mock)
	c.SetOut(&errWriter{err: errors.New("write error")})
	c.SetArgs([]string{"--parent", ".", "--target", "ch.md", "_binder.md"})

	if err := c.Execute(); err == nil {
		t.Error("expected error when writing success message fails")
	}
}

func TestNewAddChildCmd_WriteSkippedMessageError(t *testing.T) {
	// Adding an existing target (OPW002, changed=false) with a failing stdout writer.
	mock := &mockAddChildIO{
		binderBytes: acBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md"}, BinderDir: "."},
	}
	c := NewAddChildCmd(mock)
	c.SetOut(&errWriter{err: errors.New("write error")})
	c.SetArgs([]string{"--parent", ".", "--target", "chapter-one.md", "_binder.md"})

	if err := c.Execute(); err == nil {
		t.Error("expected error when writing skipped message fails")
	}
}

func TestNewAddChildCmd_FirstFlag(t *testing.T) {
	// --first inserts the child as the first child; command must succeed
	mock := &mockAddChildIO{
		binderBytes: acBinder(),
		project:     &binder.Project{Files: []string{"chapter-two.md"}, BinderDir: "."},
	}
	c := NewAddChildCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--parent", ".", "--target", "chapter-two.md", "--first", "_binder.md"})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error with --first: %v", err)
	}
	if mock.writtenPath == "" {
		t.Error("expected binder to be written when --first inserts new child")
	}
}

func TestNewAddChildCmd_AtFlag(t *testing.T) {
	// --at 0 inserts the new child at index 0 (before all existing children).
	mock := &mockAddChildIO{
		binderBytes: acBinder(),
		project:     &binder.Project{Files: []string{"chapter-two.md"}, BinderDir: "."},
	}
	c := NewAddChildCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--parent", ".", "--target", "chapter-two.md", "--at", "0", "_binder.md"})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error with --at flag: %v", err)
	}
}

func TestNewAddChildCmd_OutputsOpResultJSONOnSuccess(t *testing.T) {
	mock := &mockAddChildIO{
		binderBytes: acBinder(),
		project:     &binder.Project{Files: []string{"chapter-two.md"}, BinderDir: "."},
	}
	c := NewAddChildCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--parent", ".", "--target", "chapter-two.md", "--json", "_binder.md"})

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
		t.Error("expected Changed=true when child is added")
	}
}

func TestNewAddChildCmd_JSONEncodeError(t *testing.T) {
	mock := &mockAddChildIO{
		binderBytes: acBinder(),
		project:     &binder.Project{Files: []string{"chapter-two.md"}, BinderDir: "."},
	}
	c := NewAddChildCmd(mock)
	c.SetOut(&errWriter{err: errors.New("write error")})
	c.SetArgs([]string{"--parent", ".", "--target", "chapter-two.md", "--json", "_binder.md"})

	if err := c.Execute(); err == nil {
		t.Error("expected error when JSON encoding fails")
	}
}

func TestNewAddChildCmd_ScanProjectErrorWithJSON(t *testing.T) {
	mock := &mockAddChildIO{
		binderBytes: acBinder(),
		projectErr:  errors.New("disk error"),
	}
	c := NewAddChildCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--parent", ".", "--target", "chapter-two.md", "--json", "_binder.md"})

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

func TestNewRootCmd_RegistersAddChildSubcommand(t *testing.T) {
	root := NewRootCmd()
	var found bool
	for _, sub := range root.Commands() {
		if sub.Name() == "add-child" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected \"add-child\" subcommand registered on root command")
	}
}
