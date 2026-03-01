package node_test

import (
	"strings"
	"testing"

	node "github.com/eykd/prosemark-go/internal/node"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// TestUUIDDependency verifies that github.com/google/uuid v1.6.0 is available.
// This test fails (compile error) until the dependency is added to go.mod.
func TestUUIDDependency(t *testing.T) {
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("uuid.NewV7() error = %v", err)
	}
	if id == (uuid.UUID{}) {
		t.Error("uuid.NewV7() returned zero UUID")
	}
}

// TestYAMLDependency verifies that gopkg.in/yaml.v3 is available.
// This test fails (compile error) until the dependency is added to go.mod.
func TestYAMLDependency(t *testing.T) {
	type stub struct {
		ID string `yaml:"id"`
	}

	src := stub{ID: "test-id"}
	data, err := yaml.Marshal(src)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}

	var dst stub
	if err := yaml.Unmarshal(data, &dst); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}
	if dst.ID != src.ID {
		t.Errorf("round-trip mismatch: got %q, want %q", dst.ID, src.ID)
	}
}

// TestIsUUIDFilename verifies that IsUUIDFilename recognises valid lowercase
// UUIDv7-named files and rejects everything else.
func TestIsUUIDFilename(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     bool
	}{
		{
			name:     "valid UUIDv7 .md filename",
			filename: "0192f0c1-3e7a-7000-8000-5a4b3c2d1e0f.md",
			want:     true,
		},
		{
			name:     "valid UUIDv7 .md filename 2",
			filename: "01932b4a-deaf-7b00-a000-000000000001.md",
			want:     true,
		},
		{
			name:     "uppercase UUID is rejected",
			filename: "0192F0C1-3E7A-7000-8000-5A4B3C2D1E0F.md",
			want:     false,
		},
		{
			name:     "prose filename is rejected",
			filename: "chapter-one.md",
			want:     false,
		},
		{
			name:     "UUID v4 (version digit 4) is rejected",
			filename: "550e8400-e29b-41d4-a716-446655440000.md",
			want:     false,
		},
		{
			name:     "UUID without extension is rejected",
			filename: "0192f0c1-3e7a-7000-8000-5a4b3c2d1e0f",
			want:     false,
		},
		{
			name:     "empty string is rejected",
			filename: "",
			want:     false,
		},
		{
			name:     "README.md is rejected",
			filename: "README.md",
			want:     false,
		},
		{
			name:     "notes file (.notes.md) is rejected",
			filename: "0192f0c1-3e7a-7000-8000-5a4b3c2d1e0f.notes.md",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := node.IsUUIDFilename(tt.filename)
			if got != tt.want {
				t.Errorf("IsUUIDFilename(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

// TestParseFrontmatter verifies that ParseFrontmatter splits a node file into
// its Frontmatter struct and body bytes using the yaml.v3 decoder (not a naive
// line scan), so that "---" inside YAML values is handled correctly.
func TestParseFrontmatter(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantFM   node.Frontmatter
		wantBody string
		wantErr  bool
	}{
		{
			name: "valid full file with all fields",
			content: "---\n" +
				"id: 0192f0c1-3e7a-7000-8000-5a4b3c2d1e0f\n" +
				"title: Chapter One\n" +
				"synopsis: The world before the war.\n" +
				"created: 2026-02-28T15:04:05Z\n" +
				"updated: 2026-02-28T15:04:05Z\n" +
				"---\n" +
				"\nBody content begins here.\n",
			wantFM: node.Frontmatter{
				ID:       "0192f0c1-3e7a-7000-8000-5a4b3c2d1e0f",
				Title:    "Chapter One",
				Synopsis: "The world before the war.",
				Created:  "2026-02-28T15:04:05Z",
				Updated:  "2026-02-28T15:04:05Z",
			},
			wantBody: "\nBody content begins here.\n",
		},
		{
			name: "valid minimal file with required fields only",
			content: "---\n" +
				"id: 0192f0c1-3e7a-7000-8000-5a4b3c2d1e0f\n" +
				"created: 2026-02-28T15:04:05Z\n" +
				"updated: 2026-02-28T15:04:05Z\n" +
				"---\n" +
				"\n",
			wantFM: node.Frontmatter{
				ID:      "0192f0c1-3e7a-7000-8000-5a4b3c2d1e0f",
				Created: "2026-02-28T15:04:05Z",
				Updated: "2026-02-28T15:04:05Z",
			},
			wantBody: "\n",
		},
		{
			name: "triple-dash inside quoted synopsis must not split naively",
			content: "---\n" +
				"id: 0192f0c1-3e7a-7000-8000-5a4b3c2d1e0f\n" +
				"synopsis: \"before --- after\"\n" +
				"created: 2026-02-28T15:04:05Z\n" +
				"updated: 2026-02-28T15:04:05Z\n" +
				"---\n" +
				"\nBody here.\n",
			wantFM: node.Frontmatter{
				ID:       "0192f0c1-3e7a-7000-8000-5a4b3c2d1e0f",
				Synopsis: "before --- after",
				Created:  "2026-02-28T15:04:05Z",
				Updated:  "2026-02-28T15:04:05Z",
			},
			wantBody: "\nBody here.\n",
		},
		{
			name: "triple-dash inside block scalar synopsis must not split naively",
			content: "---\n" +
				"id: 0192f0c1-3e7a-7000-8000-5a4b3c2d1e0f\n" +
				"synopsis: |\n" +
				"  first line\n" +
				"  ---\n" +
				"  second line\n" +
				"created: 2026-02-28T15:04:05Z\n" +
				"updated: 2026-02-28T15:04:05Z\n" +
				"---\n" +
				"\nBody here.\n",
			wantFM: node.Frontmatter{
				ID:       "0192f0c1-3e7a-7000-8000-5a4b3c2d1e0f",
				Synopsis: "first line\n---\nsecond line\n",
				Created:  "2026-02-28T15:04:05Z",
				Updated:  "2026-02-28T15:04:05Z",
			},
			wantBody: "\nBody here.\n",
		},
		{
			name:    "unparseable YAML returns error",
			content: "---\nid: [unclosed bracket\ncreated: 2026-02-28T15:04:05Z\n---\n\nBody.\n",
			wantErr: true,
		},
		{
			name:    "no frontmatter delimiter returns error",
			content: "just plain content\nno frontmatter here\n",
			wantErr: true,
		},
		{
			name:    "empty file returns error",
			content: "",
			wantErr: true,
		},
		{
			// YAML escape sequence \x01 is syntactically valid YAML (yaml.v3 processes it
			// without error) but produces an SOH control character (0x01) in the decoded
			// field value. ParseFrontmatter must reject these decoded control characters.
			name: "SOH control char via YAML escape (\\x01) in synopsis returns error",
			content: "---\n" +
				"id: 0192f0c1-3e7a-7000-8000-5a4b3c2d1e0f\n" +
				"synopsis: \"\\x01embedded control char\"\n" +
				"created: 2026-02-28T15:04:05Z\n" +
				"updated: 2026-02-28T15:04:05Z\n" +
				"---\n" +
				"\nBody here.\n",
			wantErr: true,
		},
		{
			// YAML escape sequence \0 is syntactically valid YAML (yaml.v3 processes it
			// without error) but produces a null byte (0x00) in the decoded field value.
			// ParseFrontmatter must reject decoded null bytes in field values.
			name: "null byte via YAML escape (\\0) in title returns error",
			content: "---\n" +
				"id: 0192f0c1-3e7a-7000-8000-5a4b3c2d1e0f\n" +
				"title: \"\\0null byte in title\"\n" +
				"created: 2026-02-28T15:04:05Z\n" +
				"updated: 2026-02-28T15:04:05Z\n" +
				"---\n" +
				"\nBody here.\n",
			wantErr: true,
		},
		{
			name: "body with only whitespace is returned as-is (AUD006 is caller concern)",
			content: "---\n" +
				"id: 0192f0c1-3e7a-7000-8000-5a4b3c2d1e0f\n" +
				"created: 2026-02-28T15:04:05Z\n" +
				"updated: 2026-02-28T15:04:05Z\n" +
				"---\n" +
				"   \n",
			wantFM: node.Frontmatter{
				ID:      "0192f0c1-3e7a-7000-8000-5a4b3c2d1e0f",
				Created: "2026-02-28T15:04:05Z",
				Updated: "2026-02-28T15:04:05Z",
			},
			wantBody: "   \n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, body, err := node.ParseFrontmatter([]byte(tt.content))
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseFrontmatter() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if fm != tt.wantFM {
				t.Errorf("ParseFrontmatter() fm = %+v, want %+v", fm, tt.wantFM)
			}
			if string(body) != tt.wantBody {
				t.Errorf("ParseFrontmatter() body = %q, want %q", body, tt.wantBody)
			}
		})
	}
}

// TestSerializeFrontmatter verifies that SerializeFrontmatter produces a
// canonical frontmatter block with the specified field order:
// id → title → synopsis → created → updated, omitting optional empty fields.
func TestSerializeFrontmatter(t *testing.T) {
	tests := []struct {
		name       string
		fm         node.Frontmatter
		wantFields []string // expected field lines in order (not exhaustive key check)
		wantAbsent []string // fields that must NOT appear in output
		wantPrefix string   // output must begin with this
		wantSuffix string   // output must end with this
	}{
		{
			name: "all fields serialized in canonical order",
			fm: node.Frontmatter{
				ID:       "0192f0c1-3e7a-7000-8000-5a4b3c2d1e0f",
				Title:    "Chapter One",
				Synopsis: "The world before the war.",
				Created:  "2026-02-28T15:04:05Z",
				Updated:  "2026-02-28T15:04:05Z",
			},
			wantFields: []string{
				"id: 0192f0c1-3e7a-7000-8000-5a4b3c2d1e0f",
				"title: Chapter One",
				"synopsis: The world before the war.",
				"created: 2026-02-28T15:04:05Z",
				"updated: 2026-02-28T15:04:05Z",
			},
			wantPrefix: "---\n",
			wantSuffix: "\n---\n",
		},
		{
			name: "minimal fields omit empty title and synopsis",
			fm: node.Frontmatter{
				ID:      "0192f0c1-3e7a-7000-8000-5a4b3c2d1e0f",
				Created: "2026-02-28T15:04:05Z",
				Updated: "2026-02-28T15:04:05Z",
			},
			wantFields: []string{
				"id: 0192f0c1-3e7a-7000-8000-5a4b3c2d1e0f",
				"created: 2026-02-28T15:04:05Z",
				"updated: 2026-02-28T15:04:05Z",
			},
			wantAbsent: []string{"title:", "synopsis:"},
			wantPrefix: "---\n",
			wantSuffix: "\n---\n",
		},
		{
			name: "canonical field ordering: id before created before updated",
			fm: node.Frontmatter{
				ID:      "0192f0c1-3e7a-7000-8000-5a4b3c2d1e0f",
				Created: "2026-02-28T15:04:05Z",
				Updated: "2026-02-28T16:00:00Z",
			},
			wantFields: []string{
				"id:",
				"created:",
				"updated:",
			},
			wantPrefix: "---\n",
			wantSuffix: "\n---\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := string(node.SerializeFrontmatter(tt.fm))

			if !strings.HasPrefix(out, tt.wantPrefix) {
				t.Errorf("output does not start with %q:\n%s", tt.wantPrefix, out)
			}
			if !strings.HasSuffix(out, tt.wantSuffix) {
				t.Errorf("output does not end with %q:\n%s", tt.wantSuffix, out)
			}

			// Check field presence and relative order.
			prevIdx := -1
			for _, field := range tt.wantFields {
				idx := strings.Index(out, field)
				if idx < 0 {
					t.Errorf("output missing field %q:\n%s", field, out)
					continue
				}
				if idx <= prevIdx {
					t.Errorf("field %q appears out of order (at %d, previous was %d):\n%s", field, idx, prevIdx, out)
				}
				prevIdx = idx
			}

			for _, absent := range tt.wantAbsent {
				if strings.Contains(out, absent) {
					t.Errorf("output contains unexpected field %q:\n%s", absent, out)
				}
			}
		})
	}
}

// TestValidateNode verifies that ValidateNode returns the correct AuditDiagnostics
// for AUD004 (id≠filename stem), AUD005 (missing/invalid RFC3339Z fields),
// and AUD006 (empty/whitespace-only body, warning).
func TestValidateNode(t *testing.T) {
	const (
		validID   = "0192f0c1-3e7a-7000-8000-5a4b3c2d1e0f"
		validTS   = "2026-02-28T15:04:05Z"
		validBody = "\nBody content here.\n"
	)

	validFM := node.Frontmatter{
		ID:      validID,
		Created: validTS,
		Updated: validTS,
	}

	tests := []struct {
		name         string
		filenameStem string
		fm           node.Frontmatter
		body         []byte
		wantCodes    []node.AuditCode
		wantNoCodes  []node.AuditCode
	}{
		{
			name:         "valid node produces no diagnostics",
			filenameStem: validID,
			fm:           validFM,
			body:         []byte(validBody),
			wantCodes:    nil,
			wantNoCodes:  []node.AuditCode{node.AUD004, node.AUD005, node.AUD006},
		},
		{
			name:         "AUD004: id does not match filename stem",
			filenameStem: "deadbeef-0000-7000-8000-000000000001",
			fm:           validFM,
			body:         []byte(validBody),
			wantCodes:    []node.AuditCode{node.AUD004},
			wantNoCodes:  []node.AuditCode{node.AUD005, node.AUD006},
		},
		{
			name:         "AUD005: missing id",
			filenameStem: validID,
			fm: node.Frontmatter{
				ID:      "",
				Created: validTS,
				Updated: validTS,
			},
			body:        []byte(validBody),
			wantCodes:   []node.AuditCode{node.AUD005},
			wantNoCodes: []node.AuditCode{node.AUD006},
		},
		{
			name:         "AUD005: missing created",
			filenameStem: validID,
			fm: node.Frontmatter{
				ID:      validID,
				Created: "",
				Updated: validTS,
			},
			body:        []byte(validBody),
			wantCodes:   []node.AuditCode{node.AUD005},
			wantNoCodes: []node.AuditCode{node.AUD006},
		},
		{
			name:         "AUD005: missing updated",
			filenameStem: validID,
			fm: node.Frontmatter{
				ID:      validID,
				Created: validTS,
				Updated: "",
			},
			body:        []byte(validBody),
			wantCodes:   []node.AuditCode{node.AUD005},
			wantNoCodes: []node.AuditCode{node.AUD006},
		},
		{
			name:         "AUD005: created without Z suffix is invalid RFC3339Z",
			filenameStem: validID,
			fm: node.Frontmatter{
				ID:      validID,
				Created: "2026-02-28T15:04:05+00:00",
				Updated: validTS,
			},
			body:        []byte(validBody),
			wantCodes:   []node.AuditCode{node.AUD005},
			wantNoCodes: []node.AuditCode{node.AUD006},
		},
		{
			name:         "AUD005: updated without Z suffix is invalid RFC3339Z",
			filenameStem: validID,
			fm: node.Frontmatter{
				ID:      validID,
				Created: validTS,
				Updated: "2026-02-28T15:04:05-05:00",
			},
			body:        []byte(validBody),
			wantCodes:   []node.AuditCode{node.AUD005},
			wantNoCodes: []node.AuditCode{node.AUD006},
		},
		{
			name:         "AUD005: invalid timestamp format entirely",
			filenameStem: validID,
			fm: node.Frontmatter{
				ID:      validID,
				Created: "not-a-date",
				Updated: validTS,
			},
			body:        []byte(validBody),
			wantCodes:   []node.AuditCode{node.AUD005},
			wantNoCodes: []node.AuditCode{node.AUD006},
		},
		{
			name:         "AUD006 warning: empty body",
			filenameStem: validID,
			fm:           validFM,
			body:         []byte(""),
			wantCodes:    []node.AuditCode{node.AUD006},
			wantNoCodes:  []node.AuditCode{node.AUD004, node.AUD005},
		},
		{
			name:         "AUD006 warning: whitespace-only body",
			filenameStem: validID,
			fm:           validFM,
			body:         []byte("   \n\t  \n"),
			wantCodes:    []node.AuditCode{node.AUD006},
			wantNoCodes:  []node.AuditCode{node.AUD004, node.AUD005},
		},
		{
			name:         "AUD006 is a warning not an error",
			filenameStem: validID,
			fm:           validFM,
			body:         []byte(""),
			wantCodes:    []node.AuditCode{node.AUD006},
			wantNoCodes:  nil,
		},
		{
			name:         "multiple violations: AUD004 and AUD005 together",
			filenameStem: "wrong-stem",
			fm: node.Frontmatter{
				ID:      validID,
				Created: "",
				Updated: validTS,
			},
			body:        []byte(validBody),
			wantCodes:   []node.AuditCode{node.AUD004, node.AUD005},
			wantNoCodes: []node.AuditCode{node.AUD006},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := node.ValidateNode(tt.filenameStem, tt.fm, tt.body)

			codeSet := make(map[node.AuditCode]node.AuditSeverity)
			for _, d := range diags {
				codeSet[d.Code] = d.Severity
			}

			for _, wantCode := range tt.wantCodes {
				if _, ok := codeSet[wantCode]; !ok {
					t.Errorf("ValidateNode() missing expected diagnostic %q; got %v", wantCode, diags)
				}
			}

			for _, noCode := range tt.wantNoCodes {
				if _, ok := codeSet[noCode]; ok {
					t.Errorf("ValidateNode() produced unexpected diagnostic %q; got %v", noCode, diags)
				}
			}

			// AUD006 must be a warning, not an error.
			if sev, ok := codeSet[node.AUD006]; ok {
				if sev != node.SeverityWarning {
					t.Errorf("AUD006 severity = %q, want %q", sev, node.SeverityWarning)
				}
			}
		})
	}
}
