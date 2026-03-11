package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
)

// TestNewAddChildCmd_NewMode_JSONOutputIncludesTarget verifies that when --new
// and --json are both set, the JSON output includes a "target" field containing
// the filename of the newly created node (e.g. "019cdde6-2daf-7219-af27-faf65b48a757.md").
func TestNewAddChildCmd_NewMode_JSONOutputIncludesTarget(t *testing.T) {
	mock := &mockAddChildIOWithNew{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     &binder.Project{Files: []string{}, BinderDir: "."},
		},
	}
	c := NewAddChildCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{"--new", "--title", "Chapter One", "--parent", ".", "--json", "--project", "."})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Parse into a generic map to check for the "target" key.
	var raw map[string]any
	if err := json.Unmarshal(out.Bytes(), &raw); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out.String())
	}

	targetVal, ok := raw["target"]
	if !ok {
		t.Fatalf("JSON output missing \"target\" field\noutput: %s", out.String())
	}

	target, ok := targetVal.(string)
	if !ok {
		t.Fatalf("\"target\" field is not a string: %v", targetVal)
	}

	if !strings.HasSuffix(target, ".md") {
		t.Errorf("target = %q, want a .md filename", target)
	}

	// The target in JSON should match the path that was written to disk.
	if mock.nodeWrittenPath == "" {
		t.Fatal("expected WriteNodeFileAtomic to have been called")
	}
	// nodeWrittenPath is a full path; target should be just the filename.
	if !strings.HasSuffix(mock.nodeWrittenPath, target) {
		t.Errorf("target %q does not match written path %q", target, mock.nodeWrittenPath)
	}
}

// TestNewAddChildCmd_NewMode_JSONOutputTargetWithExplicitTarget verifies that
// when --new --json --target <uuid>.md is used, the JSON "target" field matches
// the explicitly provided target.
func TestNewAddChildCmd_NewMode_JSONOutputTargetWithExplicitTarget(t *testing.T) {
	const explicitTarget = "01234567-89ab-7def-0123-456789abcdef.md"
	mock := &mockAddChildIOWithNew{
		mockAddChildIO: mockAddChildIO{
			binderBytes: emptyBinder(),
			project:     &binder.Project{Files: []string{}, BinderDir: "."},
		},
	}
	c := NewAddChildCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetArgs([]string{
		"--new", "--target", explicitTarget, "--title", "Named",
		"--parent", ".", "--json", "--project", ".",
	})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(out.Bytes(), &raw); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out.String())
	}

	target, ok := raw["target"].(string)
	if !ok {
		t.Fatalf("JSON output missing or non-string \"target\" field\noutput: %s", out.String())
	}
	if target != explicitTarget {
		t.Errorf("target = %q, want %q", target, explicitTarget)
	}
}
