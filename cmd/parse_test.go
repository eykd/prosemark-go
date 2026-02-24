package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
)

// mockParseReader is a test double for ParseReader.
type mockParseReader struct {
	binderBytes []byte
	project     *binder.Project
	binderErr   error
	projectErr  error
}

func (m *mockParseReader) ReadBinder(_ context.Context, _ string) ([]byte, error) {
	return m.binderBytes, m.binderErr
}

func (m *mockParseReader) ScanProject(_ context.Context, _ string) (*binder.Project, error) {
	if m.project != nil {
		return m.project, m.projectErr
	}
	return &binder.Project{Files: []string{}, BinderDir: "."}, m.projectErr
}

func TestNewParseCmd_HasProjectFlag(t *testing.T) {
	c := NewParseCmd(nil)
	if c.Flags().Lookup("project") == nil {
		t.Error("expected --project flag on parse command")
	}
}

func TestNewParseCmd_DefaultsToCWD(t *testing.T) {
	// When no --project flag is given, command resolves path from CWD.
	// The mock ignores path, so this verifies the command succeeds with no args.
	reader := &mockParseReader{
		binderBytes: []byte("<!-- prosemark-binder:v1 -->\n"),
		project:     &binder.Project{Files: []string{}, BinderDir: "."},
	}
	c := NewParseCmd(reader)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{})

	if err := c.Execute(); err != nil {
		t.Fatalf("expected success with no args (CWD default): %v", err)
	}
	if out.Len() == 0 {
		t.Error("expected JSON output")
	}
}

func TestNewParseCmd_AcceptsProjectFlag(t *testing.T) {
	reader := &mockParseReader{
		binderBytes: []byte("<!-- prosemark-binder:v1 -->\n"),
		project:     &binder.Project{Files: []string{}, BinderDir: "."},
	}
	c := NewParseCmd(reader)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--project", "/some/dir"})

	if err := c.Execute(); err != nil {
		t.Fatalf("expected success with --project flag: %v", err)
	}
}

func TestNewParseCmd_GetCWDError(t *testing.T) {
	reader := &mockParseReader{
		binderBytes: []byte("<!-- prosemark-binder:v1 -->\n"),
	}
	c := newParseCmdWithGetCWD(reader, func() (string, error) {
		return "", errors.New("getwd failed")
	})
	c.SetOut(new(bytes.Buffer))
	c.SetArgs([]string{}) // no --project, triggers getwd

	if err := c.Execute(); err == nil {
		t.Error("expected error when getwd fails")
	}
}

func TestNewParseCmd_OutputsJSONOnSuccess(t *testing.T) {
	reader := &mockParseReader{
		binderBytes: []byte("<!-- prosemark-binder:v1 -->\n- [Chapter One](ch1.md)\n"),
		project:     &binder.Project{Files: []string{"ch1.md"}, BinderDir: "."},
	}
	c := NewParseCmd(reader)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--project", "."})

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
		binderBytes: []byte("- [Chapter One](ch1.md)\n"),
		project:     &binder.Project{Files: []string{"ch1.md"}, BinderDir: "."},
	}
	c := NewParseCmd(reader)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--project", "."})

	err := c.Execute()
	if err != nil {
		t.Errorf("expected exit 0 for warnings-only parse, got: %v", err)
	}
}

func TestNewParseCmd_ExitsNonZeroOnBNDErrors(t *testing.T) {
	// Path escaping root → BNDE002 (error severity) → exit non-zero
	reader := &mockParseReader{
		binderBytes: []byte("<!-- prosemark-binder:v1 -->\n- [Escape](../secret.md)\n"),
	}
	c := NewParseCmd(reader)
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)
	c.SetOut(out)
	c.SetErr(errOut)
	c.SetArgs([]string{"--project", "."})

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

func TestNewParseCmd_AcceptsJSONFlag(t *testing.T) {
	reader := &mockParseReader{
		binderBytes: []byte("<!-- prosemark-binder:v1 -->\n"),
		project:     &binder.Project{Files: []string{}, BinderDir: "."},
	}
	c := NewParseCmd(reader)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--json", "--project", "."})

	if err := c.Execute(); err != nil {
		t.Fatalf("expected --json flag to be accepted without error: %v", err)
	}
	if out.Len() == 0 {
		t.Error("expected JSON output even with --json flag")
	}
}

func TestNewParseCmd_ReturnsOPE009OnInvalidUTF8(t *testing.T) {
	reader := &mockParseReader{
		binderBytes: []byte("<!-- prosemark-binder:v1 -->\n\xff\xfe"),
	}
	c := NewParseCmd(reader)
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)
	c.SetOut(out)
	c.SetErr(errOut)
	c.SetArgs([]string{"--project", "."})

	err := c.Execute()
	if err == nil {
		t.Error("expected non-zero exit for invalid UTF-8 content")
	}
	if out.Len() == 0 {
		t.Error("expected JSON written to stdout even when UTF-8 invalid")
	}
	var result map[string]interface{}
	if jsonErr := json.Unmarshal(out.Bytes(), &result); jsonErr != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", jsonErr, out.String())
	}
	diags, ok := result["diagnostics"].([]interface{})
	if !ok || len(diags) == 0 {
		t.Fatal("expected non-empty diagnostics array for invalid UTF-8")
	}
	found := false
	for _, d := range diags {
		dm, ok := d.(map[string]interface{})
		if !ok {
			continue
		}
		if dm["severity"] == "error" && dm["code"] == "OPE009" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected OPE009 error diagnostic for invalid UTF-8, got: %v", diags)
	}
}
