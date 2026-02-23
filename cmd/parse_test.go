package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

// mockParseReader is a test double for ParseReader.
type mockParseReader struct {
	binderBytes  []byte
	projectBytes []byte
	binderErr    error
	projectErr   error
}

func (m *mockParseReader) ReadBinder(_ context.Context, _ string) ([]byte, error) {
	return m.binderBytes, m.binderErr
}

func (m *mockParseReader) ReadProject(_ context.Context, _ string) ([]byte, error) {
	return m.projectBytes, m.projectErr
}

func TestNewParseCmd_RejectsNon_binderMdFilename(t *testing.T) {
	tests := []struct {
		name       string
		binderPath string
		wantErr    bool
	}{
		{"valid _binder.md", "_binder.md", false},
		{"valid nested _binder.md", "project/_binder.md", false},
		{"invalid notes.md", "notes.md", true},
		{"invalid binder.md without underscore", "binder.md", true},
		{"invalid README.md", "README.md", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := &mockParseReader{
				binderBytes:  []byte("<!-- prosemark-binder:v1 -->\n"),
				projectBytes: []byte(`{"version":"1","files":[]}`),
			}
			c := NewParseCmd(reader)
			out := new(bytes.Buffer)
			errOut := new(bytes.Buffer)
			c.SetOut(out)
			c.SetErr(errOut)
			c.SetArgs([]string{"--json", "--project", "project.json", tt.binderPath})

			err := c.Execute()
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
			// On validation error no JSON must appear on stdout (it is a CLI usage error)
			if tt.wantErr && out.Len() > 0 {
				t.Errorf("expected no stdout output on validation error, got: %s", out.String())
			}
		})
	}
}

func TestNewParseCmd_OutputsJSONOnSuccess(t *testing.T) {
	reader := &mockParseReader{
		binderBytes:  []byte("<!-- prosemark-binder:v1 -->\n- [Chapter One](ch1.md)\n"),
		projectBytes: []byte(`{"version":"1","files":["ch1.md"]}`),
	}
	c := NewParseCmd(reader)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--json", "--project", "project.json", "_binder.md"})

	err := c.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out.String())
	}
	if result["version"] != "1" {
		t.Errorf("version = %v, want \"1\"", result["version"])
	}
	if _, ok := result["root"]; !ok {
		t.Error("expected \"root\" field in JSON output")
	}
	if _, ok := result["diagnostics"]; !ok {
		t.Error("expected \"diagnostics\" field in JSON output")
	}
}

func TestNewParseCmd_ExitsZeroOnWarningsOnly(t *testing.T) {
	// Binder without pragma → BNDW001 (warning severity only) → exit 0
	reader := &mockParseReader{
		binderBytes:  []byte("- [Chapter One](ch1.md)\n"),
		projectBytes: []byte(`{"version":"1","files":["ch1.md"]}`),
	}
	c := NewParseCmd(reader)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--json", "--project", "project.json", "_binder.md"})

	err := c.Execute()
	if err != nil {
		t.Errorf("expected exit 0 for warnings-only parse, got: %v", err)
	}
}

func TestNewParseCmd_ExitsNonZeroOnBNDErrors(t *testing.T) {
	// Path escaping root → BNDE002 (error severity) → exit non-zero
	reader := &mockParseReader{
		binderBytes:  []byte("<!-- prosemark-binder:v1 -->\n- [Escape](../secret.md)\n"),
		projectBytes: []byte(`{"version":"1","files":[]}`),
	}
	c := NewParseCmd(reader)
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)
	c.SetOut(out)
	c.SetErr(errOut)
	c.SetArgs([]string{"--json", "--project", "project.json", "_binder.md"})

	err := c.Execute()
	if err == nil {
		t.Error("expected non-zero exit when diagnostics include error-severity codes")
	}
	// JSON must still be written to stdout so callers can inspect diagnostics
	if out.Len() == 0 {
		t.Error("expected JSON written to stdout even when diagnostics include errors")
	}
	var result map[string]interface{}
	if jsonErr := json.Unmarshal(out.Bytes(), &result); jsonErr != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", jsonErr, out.String())
	}
	diags, ok := result["diagnostics"].([]interface{})
	if !ok || len(diags) == 0 {
		t.Error("expected non-empty diagnostics array for error case")
	}
}

func TestNewParseCmd_HasRequiredFlags(t *testing.T) {
	c := NewParseCmd(nil)
	if c.Flags().Lookup("json") == nil {
		t.Error("expected --json flag on parse command")
	}
	if c.Flags().Lookup("project") == nil {
		t.Error("expected --project flag on parse command")
	}
}
