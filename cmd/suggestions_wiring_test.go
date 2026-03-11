package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
)

// ─── Helpers ────────────────────────────────────────────────────────────────

// binderWithNode returns a minimal valid binder referencing a single node.
func sugBinderWithNode() []byte {
	return []byte("<!-- prosemark-binder:v1 -->\n- [Chapter One](chapter-one.md)\n")
}

// sugProject returns a Project that knows about the single file.
func sugProject() *binder.Project {
	return &binder.Project{Files: []string{"chapter-one.md"}, BinderDir: "."}
}

// decodeDiags extracts the diagnostics array from JSON output that contains a
// "diagnostics" key (used by delete, move, addchild OpResult, and parse output).
func decodeDiags(t *testing.T, buf *bytes.Buffer) []binder.Diagnostic {
	t.Helper()
	var raw struct {
		Diagnostics []binder.Diagnostic `json:"diagnostics"`
	}
	if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
		t.Fatalf("decoding JSON output: %v\nraw: %s", err, buf.String())
	}
	return raw.Diagnostics
}

// decodeDoctorDiags extracts doctor diagnostics from JSON output.
func decodeDoctorDiags(t *testing.T, buf *bytes.Buffer) []DoctorDiagnosticJSON {
	t.Helper()
	var raw struct {
		Diagnostics []DoctorDiagnosticJSON `json:"diagnostics"`
	}
	if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
		t.Fatalf("decoding doctor JSON output: %v\nraw: %s", err, buf.String())
	}
	return raw.Diagnostics
}

// requireSuggestionOnCode asserts that at least one diagnostic with the given code
// has a non-empty Suggestion field.
func requireSuggestionOnCode(t *testing.T, diags []binder.Diagnostic, code string) {
	t.Helper()
	for _, d := range diags {
		if d.Code == code && d.Suggestion != "" {
			return
		}
	}
	t.Errorf("expected suggestion on diagnostic code %q, but none found; diags: %+v", code, diags)
}

// requireDoctorSuggestionOnCode asserts that at least one doctor diagnostic with
// the given code has a non-empty Suggestion field.
func requireDoctorSuggestionOnCode(t *testing.T, diags []DoctorDiagnosticJSON, code string) {
	t.Helper()
	for _, d := range diags {
		if d.Code == code && d.Suggestion != "" {
			return
		}
	}
	t.Errorf("expected suggestion on doctor diagnostic code %q, but none found; diags: %+v", code, diags)
}

// ─── Delete ─────────────────────────────────────────────────────────────────

func TestDeleteCmd_AttachesSuggestions(t *testing.T) {
	// When delete produces a CodeSelectorNoMatch diagnostic (selector doesn't match),
	// the JSON output should include the mapped suggestion.
	mock := &mockDeleteIO{
		binderBytes: sugBinderWithNode(),
		project:     sugProject(),
	}
	cmd := newDeleteCmdWithGetCWD(mock, func() (string, error) { return "/tmp", nil })
	out := new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"--selector", "nonexistent.md", "--yes", "--json"})

	_ = cmd.Execute() // expect exit error from diagnostic

	diags := decodeDiags(t, out)
	requireSuggestionOnCode(t, diags, binder.CodeSelectorNoMatch)
}

// ─── Move ───────────────────────────────────────────────────────────────────

func TestMoveCmd_AttachesSuggestions(t *testing.T) {
	// When move produces a CodeSelectorNoMatch diagnostic (source doesn't match),
	// the JSON output should include the mapped suggestion.
	mock := &mockMoveIO{
		binderBytes: sugBinderWithNode(),
		project:     sugProject(),
	}
	cmd := newMoveCmdWithGetCWD(mock, func() (string, error) { return "/tmp", nil })
	out := new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"--source", "nonexistent.md", "--dest", "chapter-one.md", "--yes", "--json"})

	_ = cmd.Execute()

	diags := decodeDiags(t, out)
	requireSuggestionOnCode(t, diags, binder.CodeSelectorNoMatch)
}

// ─── AddChild ───────────────────────────────────────────────────────────────

func TestAddChildCmd_AttachesSuggestions(t *testing.T) {
	// When add produces a CodeSelectorNoMatch diagnostic (parent doesn't match),
	// the JSON output should include the mapped suggestion.
	mock := &mockAddChildIO{
		binderBytes: sugBinderWithNode(),
		project:     sugProject(),
	}
	cmd := newAddChildCmdWithGetCWD(mock, func() (string, error) { return "/tmp", nil })
	out := new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"--parent", "nonexistent.md", "--target", "new-node.md", "--json"})

	_ = cmd.Execute()

	diags := decodeDiags(t, out)
	requireSuggestionOnCode(t, diags, binder.CodeSelectorNoMatch)
}

// ─── Parse ──────────────────────────────────────────────────────────────────

func TestParseCmd_AttachesSuggestions(t *testing.T) {
	// Parse a binder that references a file not in the project → produces a warning
	// diagnostic. Escape sequences in the binder cause BNDE001 warnings.
	// Use a binder with a reference to a node whose filename contains illegal chars
	// to trigger a diagnostic with a code in the suggestion map.
	binderBytes := []byte("<!-- prosemark-binder:v1 -->\n- [Node](chap\\*ter.md)\n")
	mock := &mockParseReader{
		binderBytes: binderBytes,
		project:     &binder.Project{Files: []string{}, BinderDir: "."},
	}
	cmd := newParseCmdWithGetCWD(mock, func() (string, error) { return "/tmp", nil })
	out := new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{})

	_ = cmd.Execute()

	diags := decodeDiags(t, out)
	// The parse output should have suggestions attached to any diagnostic whose
	// code is in the suggestion map.
	hasSuggestion := false
	for _, d := range diags {
		if _, mapped := suggestionMap[d.Code]; mapped && d.Suggestion != "" {
			hasSuggestion = true
			break
		}
	}
	if len(diags) > 0 && !hasSuggestion {
		// If there are diagnostics with mapped codes but no suggestions, that's a failure.
		for _, d := range diags {
			if _, mapped := suggestionMap[d.Code]; mapped {
				t.Errorf("diagnostic code %q should have a suggestion but has none; diags: %+v", d.Code, diags)
				return
			}
		}
	}
}

// ─── Init ───────────────────────────────────────────────────────────────────

func TestInitCmd_AttachesSuggestions_JSON(t *testing.T) {
	// Init produces OPI001/OPI002 info diagnostics. While these aren't in the
	// suggestion map, attachSuggestions must still be called (no-op for unmapped codes).
	// We verify that the JSON output is well-formed and that calling attachSuggestions
	// doesn't break anything. The real contract: if init ever produces a mapped code,
	// the suggestion would appear.
	mock := newMockInitIO()
	cmd := newInitCmdWithGetCWD(mock, func() (string, error) { return "/tmp", nil })
	out := new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify JSON is valid and contains diagnostics.
	diags := decodeDiags(t, out)
	if len(diags) == 0 {
		t.Error("expected at least one diagnostic from init")
	}
}

// ─── Doctor ─────────────────────────────────────────────────────────────────

func TestDoctorCmd_AttachesAuditSuggestions(t *testing.T) {
	// When doctor produces an AUD001 diagnostic (file not found),
	// the JSON output should include the mapped audit suggestion.
	nodeUUID := doctorTestNodeUUID
	mock := &mockDoctorIO{
		binderBytes: doctorBinderWithNode(nodeUUID),
		uuidFiles:   []string{},
		nodeFiles: map[string]nodeFileEntry{
			".prosemark.yml": {content: []byte("version: \"1\"\n"), exists: true},
			// Node file NOT present → triggers AUD001 (file not found)
		},
	}
	cmd := newDoctorCmdWithGetCWD(mock, func() (string, error) { return "/tmp", nil })
	out := new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"--json"})

	_ = cmd.Execute() // expect exit error from diagnostic

	diags := decodeDoctorDiags(t, out)
	requireDoctorSuggestionOnCode(t, diags, "AUD001")
}

func TestDoctorCmd_AttachesAuditSuggestions_HumanMode(t *testing.T) {
	// In human (non-JSON) mode, doctor should still output suggestions to stderr.
	// Currently doctor human mode prints "CODE SEVERITY message" per line.
	// After wiring attachAuditSuggestions, it should print suggestions too.
	nodeUUID := doctorTestNodeUUID
	mock := &mockDoctorIO{
		binderBytes: doctorBinderWithNode(nodeUUID),
		uuidFiles:   []string{},
		nodeFiles: map[string]nodeFileEntry{
			".prosemark.yml": {content: []byte("version: \"1\"\n"), exists: true},
		},
	}
	cmd := newDoctorCmdWithGetCWD(mock, func() (string, error) { return "/tmp", nil })
	out := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{})

	_ = cmd.Execute()

	// The stderr output should contain the suggestion text for AUD001.
	wantSubstring := auditSuggestionMap["AUD001"]
	if wantSubstring == "" {
		t.Fatal("test setup error: AUD001 not in auditSuggestionMap")
	}
	if got := stderr.String(); !strings.Contains(got, wantSubstring) {
		t.Errorf("expected stderr to contain suggestion %q, got:\n%s", wantSubstring, got)
	}
}
