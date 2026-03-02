package node_test

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/eykd/prosemark-go/internal/node"
)

// TestNodeId_IsStringAlias verifies NodeId is a type alias for string,
// allowing direct assignment without explicit conversion.
func TestNodeId_IsStringAlias(t *testing.T) {
	const raw = "0192f0c1-3e7a-7000-8000-5a4b3c2d1e0f"
	var id node.NodeId = raw
	// For a type alias (= string), s = id must compile without conversion.
	var s string = id
	if s != raw {
		t.Errorf("NodeId alias value = %q, want %q", s, raw)
	}
}

// TestFrontmatter_YAMLRoundTrip verifies YAML field names and omitempty behaviour.
func TestFrontmatter_YAMLRoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		fm       node.Frontmatter
		wantKeys []string // keys that must appear in marshalled YAML
		noKeys   []string // keys that must NOT appear when omitempty fires
	}{
		{
			name: "all fields populated",
			fm: node.Frontmatter{
				ID:       "0192f0c1-3e7a-7000-8000-5a4b3c2d1e0f",
				Title:    "Chapter One",
				Synopsis: "The world before the war.",
				Created:  "2026-02-28T15:04:05Z",
				Updated:  "2026-02-28T15:04:05Z",
			},
			wantKeys: []string{"id", "title", "synopsis", "created", "updated"},
		},
		{
			name: "optional fields omitted when empty",
			fm: node.Frontmatter{
				ID:      "0192f0c1-3e7a-7000-8000-5a4b3c2d1e0f",
				Created: "2026-02-28T15:04:05Z",
				Updated: "2026-02-28T15:04:05Z",
			},
			wantKeys: []string{"id", "created", "updated"},
			noKeys:   []string{"title", "synopsis"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := yaml.Marshal(tt.fm)
			if err != nil {
				t.Fatalf("yaml.Marshal error = %v", err)
			}

			var roundTripped node.Frontmatter
			if err := yaml.Unmarshal(data, &roundTripped); err != nil {
				t.Fatalf("yaml.Unmarshal error = %v", err)
			}

			if roundTripped.ID != tt.fm.ID {
				t.Errorf("ID = %q, want %q", roundTripped.ID, tt.fm.ID)
			}
			if roundTripped.Title != tt.fm.Title {
				t.Errorf("Title = %q, want %q", roundTripped.Title, tt.fm.Title)
			}
			if roundTripped.Synopsis != tt.fm.Synopsis {
				t.Errorf("Synopsis = %q, want %q", roundTripped.Synopsis, tt.fm.Synopsis)
			}
			if roundTripped.Created != tt.fm.Created {
				t.Errorf("Created = %q, want %q", roundTripped.Created, tt.fm.Created)
			}
			if roundTripped.Updated != tt.fm.Updated {
				t.Errorf("Updated = %q, want %q", roundTripped.Updated, tt.fm.Updated)
			}

			yamlStr := string(data)
			for _, k := range tt.wantKeys {
				if !containsKey(yamlStr, k) {
					t.Errorf("marshalled YAML missing key %q\nYAML:\n%s", k, yamlStr)
				}
			}
			for _, k := range tt.noKeys {
				if containsKey(yamlStr, k) {
					t.Errorf("marshalled YAML should omit key %q when empty\nYAML:\n%s", k, yamlStr)
				}
			}
		})
	}
}

// containsKey checks whether a YAML key (e.g. "id:") appears in the output.
func containsKey(s, key string) bool {
	return strings.Contains(s, key+":")
}

// TestNodePart_Values verifies NodePart constants have the expected string values.
func TestNodePart_Values(t *testing.T) {
	tests := []struct {
		name string
		part node.NodePart
		want string
	}{
		{"draft", node.NodePartDraft, "draft"},
		{"notes", node.NodePartNotes, "notes"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.part) != tt.want {
				t.Errorf("NodePart %s = %q, want %q", tt.name, string(tt.part), tt.want)
			}
		})
	}
}

// TestAuditCode_Values verifies all AuditCode constants are defined with the correct string values.
func TestAuditCode_Values(t *testing.T) {
	tests := []struct {
		name string
		code node.AuditCode
		want string
	}{
		{"AUD001", node.AUD001, "AUD001"},
		{"AUD002", node.AUD002, "AUD002"},
		{"AUD003", node.AUD003, "AUD003"},
		{"AUD004", node.AUD004, "AUD004"},
		{"AUD005", node.AUD005, "AUD005"},
		{"AUD006", node.AUD006, "AUD006"},
		{"AUD007", node.AUD007, "AUD007"},
		{"AUDW001", node.AUDW001, "AUDW001"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.code) != tt.want {
				t.Errorf("AuditCode %s = %q, want %q", tt.name, string(tt.code), tt.want)
			}
		})
	}
}

// TestAuditSeverity_Values verifies AuditSeverity constants are defined with the correct string values.
func TestAuditSeverity_Values(t *testing.T) {
	tests := []struct {
		name     string
		severity node.AuditSeverity
		want     string
	}{
		{"error", node.SeverityError, "error"},
		{"warning", node.SeverityWarning, "warning"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.severity) != tt.want {
				t.Errorf("AuditSeverity %s = %q, want %q", tt.name, string(tt.severity), tt.want)
			}
		})
	}
}

// TestAuditDiagnostic_Fields verifies AuditDiagnostic can be constructed with all required fields.
func TestAuditDiagnostic_Fields(t *testing.T) {
	d := node.AuditDiagnostic{
		Code:     node.AUD001,
		Severity: node.SeverityError,
		Message:  "referenced file does not exist",
		Path:     "missing.md",
	}

	if d.Code != node.AUD001 {
		t.Errorf("Code = %q, want %q", d.Code, node.AUD001)
	}
	if d.Severity != node.SeverityError {
		t.Errorf("Severity = %q, want %q", d.Severity, node.SeverityError)
	}
	if d.Message != "referenced file does not exist" {
		t.Errorf("Message = %q, want %q", d.Message, "referenced file does not exist")
	}
	if d.Path != "missing.md" {
		t.Errorf("Path = %q, want %q", d.Path, "missing.md")
	}
}
