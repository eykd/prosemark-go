package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
)

// nonEmptyBinderContent is a binder with node entries — overwriting this
// silently destroys the user's outline structure.
const nonEmptyBinderContent = `<!-- prosemark-binder:v1 -->

- [Chapter One](ch1.md)
  - [Section A](ch1-a.md)
- [Chapter Two](ch2.md)
`

// emptyBinderContent is a binder with only the pragma — no data loss concern.
const emptyBinderContent = "<!-- prosemark-binder:v1 -->\n"

func TestInitCmd_ForceWithNonEmptyBinder_WarnsAboutDataLoss(t *testing.T) {
	mock := newMockInitIO()
	mock.binderExists = true
	mock.binderContent = nonEmptyBinderContent

	c := newInitCmdWithGetCWD(mock, func() (string, error) { return ".", nil })
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)
	c.SetOut(out)
	c.SetErr(errOut)
	c.SetArgs([]string{"--project", ".", "--force"})

	err := c.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stderr := strings.ToLower(errOut.String())
	if !strings.Contains(stderr, "node") || !strings.Contains(stderr, "lost") {
		t.Errorf("stderr should warn about node entries being lost, got: %q", errOut.String())
	}
}

func TestInitCmd_ForceWithEmptyBinder_NoDataLossWarning(t *testing.T) {
	mock := newMockInitIO()
	mock.binderExists = true
	mock.binderContent = emptyBinderContent

	c := newInitCmdWithGetCWD(mock, func() (string, error) { return ".", nil })
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)
	c.SetOut(out)
	c.SetErr(errOut)
	c.SetArgs([]string{"--project", ".", "--force"})

	err := c.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stderr := strings.ToLower(errOut.String())
	// Should still show generic overwrite warning, but NOT a data-loss warning.
	if strings.Contains(stderr, "node") && strings.Contains(stderr, "lost") {
		t.Errorf("stderr should NOT warn about data loss for empty binder, got: %q", errOut.String())
	}
}

func TestInitCmd_ForceWithNonEmptyBinder_JSONOutput_IncludesWarningDiagnostic(t *testing.T) {
	mock := newMockInitIO()
	mock.binderExists = true
	mock.binderContent = nonEmptyBinderContent

	sub := newInitCmdWithGetCWD(mock, func() (string, error) { return ".", nil })
	root := withDryRunFlag(sub)
	out := new(bytes.Buffer)
	sub.SetOut(out)
	sub.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"init", "--project", ".", "--force", "--json"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result binder.OpResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON OpResult: %v\noutput: %s", err, out.String())
	}

	var warnDiags []binder.Diagnostic
	for _, d := range result.Diagnostics {
		if d.Severity == "warning" {
			warnDiags = append(warnDiags, d)
		}
	}
	if len(warnDiags) == 0 {
		t.Errorf("expected at least 1 warning diagnostic about data loss, got none; diagnostics: %v",
			result.Diagnostics)
	}

	// At least one warning should mention node entries being lost.
	found := false
	for _, d := range warnDiags {
		msg := strings.ToLower(d.Message)
		if strings.Contains(msg, "node") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a warning diagnostic mentioning node entries, got: %v", warnDiags)
	}
}

func TestInitCmd_ForceWithNonEmptyBinder_ReportsNodeCount(t *testing.T) {
	mock := newMockInitIO()
	mock.binderExists = true
	mock.binderContent = nonEmptyBinderContent // has 3 nodes

	c := newInitCmdWithGetCWD(mock, func() (string, error) { return ".", nil })
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)
	c.SetOut(out)
	c.SetErr(errOut)
	c.SetArgs([]string{"--project", ".", "--force"})

	err := c.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stderr := errOut.String()
	// The warning should include the number of nodes that will be lost.
	if !strings.Contains(stderr, "3") {
		t.Errorf("stderr should include count of nodes (3) being lost, got: %q", stderr)
	}
}
