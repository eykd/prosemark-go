package cmd

import (
	"testing"

	"github.com/eykd/prosemark-go/internal/node"
)

func TestAttachAuditSuggestions_MappedCodes(t *testing.T) {
	tests := []struct {
		name string
		code node.AuditCode
		want string
	}{
		{"AUD001", node.AUD001, "Check that the referenced file exists on disk. Run 'pmk parse --json' to list binder entries."},
		{"AUD002", node.AUD002, "Add the orphaned file to the binder with 'pmk add-child', or remove it from the project directory."},
		{"AUD003", node.AUD003, "Remove the duplicate entry from '_binder.md'. Each file should appear only once."},
		{"AUD004", node.AUD004, "Rename the file to match its frontmatter id, or update the frontmatter id to match the filename."},
		{"AUD005", node.AUD005, "Add the missing frontmatter fields (id, created, updated) to the node file."},
		{"AUD006", node.AUD006, "Add content to the node file body, or remove the empty node from the binder."},
		{"AUD007", node.AUD007, "Fix the YAML syntax in the frontmatter block. Ensure it starts and ends with '---'."},
		{"AUD008", node.AUD008, "Create a '.prosemark.yml' config file in the project root. Run 'pmk init' to generate one."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := []DoctorDiagnosticJSON{
				{Severity: "error", Code: string(tt.code), Message: "some message", Path: "some/path"},
			}

			attachAuditSuggestions(diags)

			if diags[0].Suggestion != tt.want {
				t.Errorf("attachAuditSuggestions() suggestion = %q, want %q", diags[0].Suggestion, tt.want)
			}
		})
	}
}

func TestAttachAuditSuggestions_UnknownCode(t *testing.T) {
	diags := []DoctorDiagnosticJSON{
		{Severity: "error", Code: "UNKNOWN999", Message: "unknown error", Path: "x"},
	}

	attachAuditSuggestions(diags)

	if diags[0].Suggestion != "" {
		t.Errorf("attachAuditSuggestions() suggestion = %q, want empty for unknown code", diags[0].Suggestion)
	}
}

func TestAttachAuditSuggestions_PreservesExistingFields(t *testing.T) {
	diags := []DoctorDiagnosticJSON{
		{
			Severity: "error",
			Code:     string(node.AUD001),
			Message:  "original message",
			Path:     "original/path",
		},
	}

	attachAuditSuggestions(diags)

	if diags[0].Severity != "error" {
		t.Errorf("Severity changed: got %q", diags[0].Severity)
	}
	if diags[0].Code != string(node.AUD001) {
		t.Errorf("Code changed: got %q", diags[0].Code)
	}
	if diags[0].Message != "original message" {
		t.Errorf("Message changed: got %q", diags[0].Message)
	}
	if diags[0].Path != "original/path" {
		t.Errorf("Path changed: got %q", diags[0].Path)
	}
}

func TestAttachAuditSuggestions_MultipleDiagnostics(t *testing.T) {
	diags := []DoctorDiagnosticJSON{
		{Severity: "error", Code: string(node.AUD001), Message: "first", Path: "a"},
		{Severity: "warning", Code: "UNKNOWN", Message: "second", Path: "b"},
		{Severity: "error", Code: string(node.AUD008), Message: "third", Path: "c"},
	}

	attachAuditSuggestions(diags)

	if diags[0].Suggestion == "" {
		t.Error("first diagnostic should have suggestion")
	}
	if diags[1].Suggestion != "" {
		t.Errorf("unknown code should have no suggestion, got %q", diags[1].Suggestion)
	}
	if diags[2].Suggestion == "" {
		t.Error("third diagnostic should have suggestion")
	}
}

func TestAuditSuggestionMap_CoversAllAUDCodes(t *testing.T) {
	allCodes := []node.AuditCode{
		node.AUD001, node.AUD002, node.AUD003, node.AUD004,
		node.AUD005, node.AUD006, node.AUD007, node.AUD008,
	}

	for _, code := range allCodes {
		t.Run(string(code), func(t *testing.T) {
			if _, ok := auditSuggestionMap[string(code)]; !ok {
				t.Errorf("auditSuggestionMap missing entry for %s", code)
			}
		})
	}
}

func TestDoctorDiagnosticJSON_HasSuggestionField(t *testing.T) {
	d := DoctorDiagnosticJSON{
		Severity:   "error",
		Code:       "AUD001",
		Message:    "test",
		Path:       "test/path",
		Suggestion: "do something",
	}

	if d.Suggestion != "do something" {
		t.Errorf("Suggestion field = %q, want %q", d.Suggestion, "do something")
	}
}
