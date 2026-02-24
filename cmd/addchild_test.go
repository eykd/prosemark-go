package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
)

// mockAddChildIO is a test double for AddChildIO.
type mockAddChildIO struct {
	binderBytes  []byte
	projectBytes []byte
	binderErr    error
	projectErr   error
	writeErr     error
	writtenBytes []byte
	writtenPath  string
}

func (m *mockAddChildIO) ReadBinder(_ context.Context, _ string) ([]byte, error) {
	return m.binderBytes, m.binderErr
}

func (m *mockAddChildIO) ReadProject(_ context.Context, _ string) ([]byte, error) {
	return m.projectBytes, m.projectErr
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
	required := []string{"json", "project", "parent", "target", "title", "first", "at", "before", "after", "force"}
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
				binderBytes:  acBinder(),
				projectBytes: []byte(`{"version":"1","files":["chapter-two.md"]}`),
			}
			c := NewAddChildCmd(mock)
			out := new(bytes.Buffer)
			errOut := new(bytes.Buffer)
			c.SetOut(out)
			c.SetErr(errOut)
			c.SetArgs([]string{"--project", "p.json", "--parent", ".", "--target", "chapter-two.md", tt.binderPath})

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

func TestNewAddChildCmd_OutputsOpResultJSONOnSuccess(t *testing.T) {
	mock := &mockAddChildIO{
		binderBytes:  acBinder(),
		projectBytes: []byte(`{"version":"1","files":["chapter-two.md"]}`),
	}
	c := NewAddChildCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--json", "--project", "p.json", "--parent", ".", "--target", "chapter-two.md", "_binder.md"})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out.String())
	}
	if result["version"] != "1" {
		t.Errorf("version = %v, want \"1\"", result["version"])
	}
	if _, ok := result["changed"]; !ok {
		t.Error("expected \"changed\" field in JSON output")
	}
	if _, ok := result["diagnostics"]; !ok {
		t.Error("expected \"diagnostics\" field in JSON output")
	}
}

func TestNewAddChildCmd_WritesModifiedBinderOnChange(t *testing.T) {
	mock := &mockAddChildIO{
		binderBytes:  acBinder(),
		projectBytes: []byte(`{"version":"1","files":["chapter-two.md"]}`),
	}
	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetArgs([]string{"--json", "--project", "p.json", "--parent", ".", "--target", "chapter-two.md", "_binder.md"})

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
		binderBytes:  acBinder(),
		projectBytes: []byte(`{"version":"1","files":["chapter-one.md"]}`),
	}
	c := NewAddChildCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	// chapter-one.md already exists in binder → OPW002 (duplicate skipped, changed=false)
	c.SetArgs([]string{"--json", "--project", "p.json", "--parent", ".", "--target", "chapter-one.md", "_binder.md"})

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
	c.SetArgs([]string{"--json", "--project", "p.json", "--parent", ".", "--target", "chapter-two.md", "_binder.md"})

	if err := c.Execute(); err == nil {
		t.Error("expected error when ReadBinder fails")
	}
	if out.Len() > 0 {
		t.Errorf("expected no stdout on ReadBinder error, got: %s", out.String())
	}
}

func TestNewAddChildCmd_ReadProjectError(t *testing.T) {
	mock := &mockAddChildIO{
		binderBytes: acBinder(),
		projectErr:  errors.New("disk error"),
	}
	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetArgs([]string{"--json", "--project", "p.json", "--parent", ".", "--target", "chapter-two.md", "_binder.md"})

	if err := c.Execute(); err == nil {
		t.Error("expected error when ReadProject fails")
	}
}

func TestNewAddChildCmd_InvalidProjectJSON(t *testing.T) {
	mock := &mockAddChildIO{
		binderBytes:  acBinder(),
		projectBytes: []byte("not valid json"),
	}
	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetArgs([]string{"--json", "--project", "p.json", "--parent", ".", "--target", "chapter-two.md", "_binder.md"})

	if err := c.Execute(); err == nil {
		t.Error("expected error when project JSON is invalid")
	}
}

func TestNewAddChildCmd_WriteError(t *testing.T) {
	mock := &mockAddChildIO{
		binderBytes:  acBinder(),
		projectBytes: []byte(`{"version":"1","files":["chapter-two.md"]}`),
		writeErr:     errors.New("write failed"),
	}
	c := NewAddChildCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetArgs([]string{"--json", "--project", "p.json", "--parent", ".", "--target", "chapter-two.md", "_binder.md"})

	if err := c.Execute(); err == nil {
		t.Error("expected error when WriteBinderAtomic fails")
	}
}

func TestNewAddChildCmd_ExitsNonZeroOnOpErrors(t *testing.T) {
	// Selector that matches no node → OPE001 (error-severity diagnostic)
	mock := &mockAddChildIO{
		binderBytes:  acBinder(),
		projectBytes: []byte(`{"version":"1","files":[]}`),
	}
	c := NewAddChildCmd(mock)
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)
	c.SetOut(out)
	c.SetErr(errOut)
	c.SetArgs([]string{"--json", "--project", "p.json", "--parent", "nonexistent.md", "--target", "ch.md", "_binder.md"})

	err := c.Execute()
	if err == nil {
		t.Error("expected non-zero exit when op has error diagnostics")
	}
	// JSON must still be written so callers can inspect diagnostics
	if out.Len() == 0 {
		t.Error("expected JSON written to stdout even when op has error diagnostics")
	}
	var result map[string]interface{}
	if jsonErr := json.Unmarshal(out.Bytes(), &result); jsonErr != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", jsonErr, out.String())
	}
	diags, ok := result["diagnostics"].([]interface{})
	if !ok || len(diags) == 0 {
		t.Error("expected non-empty diagnostics array for op error case")
	}
	// Binder must NOT be written when op has error diagnostics
	if mock.writtenPath != "" {
		t.Errorf("binder must not be written when op has error diagnostics, was written to %q", mock.writtenPath)
	}
}

func TestNewAddChildCmd_ExitsZeroOnWarningOnly(t *testing.T) {
	// OPW002: adding existing target without --force → warning severity, exit 0
	mock := &mockAddChildIO{
		binderBytes:  acBinder(),
		projectBytes: []byte(`{"version":"1","files":["chapter-one.md"]}`),
	}
	c := NewAddChildCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--json", "--project", "p.json", "--parent", ".", "--target", "chapter-one.md", "_binder.md"})

	if err := c.Execute(); err != nil {
		t.Errorf("expected exit 0 for warning-only (OPW002), got: %v", err)
	}
}

func TestNewAddChildCmd_ForceFlag(t *testing.T) {
	// --force allows adding a duplicate target; binder IS changed
	mock := &mockAddChildIO{
		binderBytes:  acBinder(),
		projectBytes: []byte(`{"version":"1","files":["chapter-one.md"]}`),
	}
	c := NewAddChildCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--json", "--project", "p.json", "--parent", ".", "--target", "chapter-one.md", "--force", "_binder.md"})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error with --force on duplicate: %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if result["changed"] != true {
		t.Errorf("expected changed=true with --force on duplicate, got: %v", result["changed"])
	}
	if mock.writtenPath == "" {
		t.Error("expected binder to be written when --force produces a change")
	}
}

func TestNewAddChildCmd_EncodeError(t *testing.T) {
	mock := &mockAddChildIO{
		binderBytes:  acBinder(),
		projectBytes: []byte(`{"version":"1","files":[]}`),
	}
	c := NewAddChildCmd(mock)
	c.SetOut(&errWriter{err: errors.New("write error")})
	c.SetArgs([]string{"--json", "--project", "p.json", "--parent", ".", "--target", "ch.md", "_binder.md"})

	if err := c.Execute(); err == nil {
		t.Error("expected error when JSON encoding fails")
	}
}

func TestNewAddChildCmd_FirstFlag(t *testing.T) {
	// --first inserts the child as the first child; command must succeed
	mock := &mockAddChildIO{
		binderBytes:  acBinder(),
		projectBytes: []byte(`{"version":"1","files":["chapter-two.md"]}`),
	}
	c := NewAddChildCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--json", "--project", "p.json", "--parent", ".", "--target", "chapter-two.md", "--first", "_binder.md"})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error with --first: %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if result["changed"] != true {
		t.Errorf("expected changed=true with --first inserting new child, got: %v", result["changed"])
	}
}

func TestNewAddChildCmd_AtFlag(t *testing.T) {
	// --at 0 inserts the new child at index 0 (before all existing children).
	mock := &mockAddChildIO{
		binderBytes:  acBinder(),
		projectBytes: []byte(`{"version":"1","files":["chapter-two.md"]}`),
	}
	c := NewAddChildCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--project", "p.json", "--parent", ".", "--target", "chapter-two.md", "--at", "0", "_binder.md"})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error with --at flag: %v", err)
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
