package node_test

import (
	"context"
	"testing"

	"github.com/eykd/prosemark-go/internal/node"
)

const (
	// testDoctorUUID1–3 are distinct valid UUIDv7 identifiers used across doctor tests.
	testDoctorUUID1 = "0192f0c1-3e7a-7000-8000-5a4b3c2d1e0f"
	testDoctorUUID2 = "01932b4a-deaf-7b00-a000-000000000001"
	testDoctorUUID3 = "01932b4a-deaf-7b00-a000-000000000002"
	testDoctorTS    = "2026-02-28T15:04:05Z"
)

// nodeFileBytes returns valid node file bytes for the given UUID stem.
func nodeFileBytes(uuid string) []byte {
	return []byte("---\n" +
		"id: " + uuid + "\n" +
		"created: " + testDoctorTS + "\n" +
		"updated: " + testDoctorTS + "\n" +
		"---\n" +
		"\nBody content here.\n")
}

// nodeFileBytesNoBody returns node file bytes with valid frontmatter but no body content.
func nodeFileBytesNoBody(uuid string) []byte {
	return []byte("---\n" +
		"id: " + uuid + "\n" +
		"created: " + testDoctorTS + "\n" +
		"updated: " + testDoctorTS + "\n" +
		"---\n")
}

// nodeFileBytesBadYAML returns node file bytes with syntactically invalid YAML frontmatter.
func nodeFileBytesBadYAML() []byte {
	return []byte("---\nid: [unclosed bracket\n---\nBody.\n")
}

// nodeFileBytesWrongID returns node file bytes where frontmatter id does not match the filename.
func nodeFileBytesWrongID(frontmatterID string) []byte {
	return []byte("---\n" +
		"id: " + frontmatterID + "\n" +
		"created: " + testDoctorTS + "\n" +
		"updated: " + testDoctorTS + "\n" +
		"---\n" +
		"\nBody.\n")
}

// nodeFileBytesMissingUpdated returns node file bytes with the required "updated" field absent.
func nodeFileBytesMissingUpdated(uuid string) []byte {
	return []byte("---\n" +
		"id: " + uuid + "\n" +
		"created: " + testDoctorTS + "\n" +
		"---\n" +
		"\nBody.\n")
}

// binderWithRefs constructs a minimal binder document linking the given targets.
func binderWithRefs(targets ...string) []byte {
	var b []byte
	for _, t := range targets {
		b = append(b, []byte("- [Title]("+t+")\n")...)
	}
	return b
}

// diagCodesOf returns the set of AuditCodes present in diags.
func diagCodesOf(diags []node.AuditDiagnostic) map[node.AuditCode]struct{} {
	m := make(map[node.AuditCode]struct{})
	for _, d := range diags {
		m[d.Code] = struct{}{}
	}
	return m
}

// hasDiagCode reports whether diags contains at least one diagnostic with code.
func hasDiagCode(diags []node.AuditDiagnostic, code node.AuditCode) bool {
	for _, d := range diags {
		if d.Code == code {
			return true
		}
	}
	return false
}

// TestRunDoctor covers all eight audit codes with table-driven cases.
func TestRunDoctor(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		data      node.DoctorData
		wantCodes []node.AuditCode // must appear in result
		wantNone  []node.AuditCode // must NOT appear in result
	}{
		{
			name: "clean project produces no diagnostics",
			data: node.DoctorData{
				BinderSrc: binderWithRefs(testDoctorUUID1 + ".md"),
				UUIDFiles: []string{testDoctorUUID1 + ".md"},
				FileContents: map[string][]byte{
					testDoctorUUID1 + ".md": nodeFileBytes(testDoctorUUID1),
				},
			},
			wantNone: []node.AuditCode{
				node.AUD001, node.AUD002, node.AUD003,
				node.AUD004, node.AUD005, node.AUD006,
				node.AUD007, node.AUDW001,
			},
		},
		{
			name: "empty binder with no UUID files produces no diagnostics",
			data: node.DoctorData{
				BinderSrc:    []byte{},
				UUIDFiles:    []string{},
				FileContents: map[string][]byte{},
			},
			wantNone: []node.AuditCode{
				node.AUD001, node.AUD002, node.AUD003,
				node.AUD004, node.AUD005, node.AUD006,
				node.AUD007, node.AUDW001,
			},
		},
		{
			name: "AUD001: referenced UUID file does not exist (nil FileContents value)",
			data: node.DoctorData{
				BinderSrc: binderWithRefs(testDoctorUUID1 + ".md"),
				UUIDFiles: []string{},
				FileContents: map[string][]byte{
					testDoctorUUID1 + ".md": nil,
				},
			},
			wantCodes: []node.AuditCode{node.AUD001},
			wantNone:  []node.AuditCode{node.AUD002, node.AUD003},
		},
		{
			name: "AUD001: referenced UUID file absent from FileContents entirely",
			data: node.DoctorData{
				BinderSrc:    binderWithRefs(testDoctorUUID1 + ".md"),
				UUIDFiles:    []string{},
				FileContents: map[string][]byte{},
			},
			wantCodes: []node.AuditCode{node.AUD001},
			wantNone:  []node.AuditCode{node.AUD002, node.AUD003},
		},
		{
			name: "AUD002: UUID file exists but is not referenced in binder (orphan)",
			data: node.DoctorData{
				BinderSrc: binderWithRefs(testDoctorUUID1 + ".md"),
				UUIDFiles: []string{testDoctorUUID1 + ".md", testDoctorUUID2 + ".md"},
				FileContents: map[string][]byte{
					testDoctorUUID1 + ".md": nodeFileBytes(testDoctorUUID1),
					testDoctorUUID2 + ".md": nodeFileBytes(testDoctorUUID2),
				},
			},
			wantCodes: []node.AuditCode{node.AUD002},
			wantNone:  []node.AuditCode{node.AUD001, node.AUD003},
		},
		{
			name: "AUD002: multiple orphans",
			data: node.DoctorData{
				BinderSrc: []byte{},
				UUIDFiles: []string{testDoctorUUID1 + ".md", testDoctorUUID2 + ".md"},
				FileContents: map[string][]byte{
					testDoctorUUID1 + ".md": nodeFileBytes(testDoctorUUID1),
					testDoctorUUID2 + ".md": nodeFileBytes(testDoctorUUID2),
				},
			},
			wantCodes: []node.AuditCode{node.AUD002},
		},
		{
			name: "AUD003: same file referenced twice in binder",
			data: node.DoctorData{
				BinderSrc: binderWithRefs(testDoctorUUID1+".md", testDoctorUUID1+".md"),
				UUIDFiles: []string{testDoctorUUID1 + ".md"},
				FileContents: map[string][]byte{
					testDoctorUUID1 + ".md": nodeFileBytes(testDoctorUUID1),
				},
			},
			wantCodes: []node.AuditCode{node.AUD003},
		},
		{
			name: "AUDW001: non-UUID filename referenced in binder (file exists)",
			data: node.DoctorData{
				BinderSrc: binderWithRefs("chapter-one.md"),
				UUIDFiles: []string{},
				FileContents: map[string][]byte{
					"chapter-one.md": []byte("# Chapter One\n\nContent here.\n"),
				},
			},
			wantCodes: []node.AuditCode{node.AUDW001},
			wantNone:  []node.AuditCode{node.AUD001},
		},
		{
			name: "AUDW001 and AUD001: non-UUID filename referenced but missing",
			data: node.DoctorData{
				BinderSrc: binderWithRefs("chapter-one.md"),
				UUIDFiles: []string{},
				FileContents: map[string][]byte{
					"chapter-one.md": nil,
				},
			},
			wantCodes: []node.AuditCode{node.AUDW001, node.AUD001},
		},
		{
			name: "AUD007: UUID file has unparseable YAML frontmatter",
			data: node.DoctorData{
				BinderSrc: binderWithRefs(testDoctorUUID1 + ".md"),
				UUIDFiles: []string{testDoctorUUID1 + ".md"},
				FileContents: map[string][]byte{
					testDoctorUUID1 + ".md": nodeFileBytesBadYAML(),
				},
			},
			wantCodes: []node.AuditCode{node.AUD007},
			wantNone:  []node.AuditCode{node.AUD001, node.AUD004, node.AUD005},
		},
		{
			name: "AUD004: frontmatter id does not match filename stem",
			data: node.DoctorData{
				BinderSrc: binderWithRefs(testDoctorUUID1 + ".md"),
				UUIDFiles: []string{testDoctorUUID1 + ".md"},
				FileContents: map[string][]byte{
					// File is named testDoctorUUID1 but frontmatter says testDoctorUUID2.
					testDoctorUUID1 + ".md": nodeFileBytesWrongID(testDoctorUUID2),
				},
			},
			wantCodes: []node.AuditCode{node.AUD004},
			wantNone:  []node.AuditCode{node.AUD001, node.AUD007},
		},
		{
			name: "AUD005: required frontmatter field (updated) absent",
			data: node.DoctorData{
				BinderSrc: binderWithRefs(testDoctorUUID1 + ".md"),
				UUIDFiles: []string{testDoctorUUID1 + ".md"},
				FileContents: map[string][]byte{
					testDoctorUUID1 + ".md": nodeFileBytesMissingUpdated(testDoctorUUID1),
				},
			},
			wantCodes: []node.AuditCode{node.AUD005},
			wantNone:  []node.AuditCode{node.AUD001, node.AUD007},
		},
		{
			name: "AUD006: UUID file has valid frontmatter but empty body",
			data: node.DoctorData{
				BinderSrc: binderWithRefs(testDoctorUUID1 + ".md"),
				UUIDFiles: []string{testDoctorUUID1 + ".md"},
				FileContents: map[string][]byte{
					testDoctorUUID1 + ".md": nodeFileBytesNoBody(testDoctorUUID1),
				},
			},
			wantCodes: []node.AuditCode{node.AUD006},
			wantNone:  []node.AuditCode{node.AUD001, node.AUD004, node.AUD005, node.AUD007},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := node.RunDoctor(ctx, tt.data)
			codes := diagCodesOf(diags)

			for _, want := range tt.wantCodes {
				if _, ok := codes[want]; !ok {
					t.Errorf("RunDoctor() missing expected code %q; got %v", want, diags)
				}
			}
			for _, none := range tt.wantNone {
				if _, ok := codes[none]; ok {
					t.Errorf("RunDoctor() produced unexpected code %q; got %v", none, diags)
				}
			}
		})
	}
}

// TestRunDoctor_Severity verifies that AUD002, AUD006, and AUDW001 are warnings
// and that AUD001, AUD003, AUD004, AUD005, AUD007 are errors.
func TestRunDoctor_Severity(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		data         node.DoctorData
		code         node.AuditCode
		wantSeverity node.AuditSeverity
	}{
		{
			name: "AUD001 is error",
			data: node.DoctorData{
				BinderSrc:    binderWithRefs(testDoctorUUID1 + ".md"),
				UUIDFiles:    []string{},
				FileContents: map[string][]byte{testDoctorUUID1 + ".md": nil},
			},
			code:         node.AUD001,
			wantSeverity: node.SeverityError,
		},
		{
			name: "AUD002 is warning",
			data: node.DoctorData{
				BinderSrc: binderWithRefs(testDoctorUUID1 + ".md"),
				UUIDFiles: []string{testDoctorUUID1 + ".md", testDoctorUUID2 + ".md"},
				FileContents: map[string][]byte{
					testDoctorUUID1 + ".md": nodeFileBytes(testDoctorUUID1),
					testDoctorUUID2 + ".md": nodeFileBytes(testDoctorUUID2),
				},
			},
			code:         node.AUD002,
			wantSeverity: node.SeverityWarning,
		},
		{
			name: "AUD003 is error",
			data: node.DoctorData{
				BinderSrc: binderWithRefs(testDoctorUUID1+".md", testDoctorUUID1+".md"),
				UUIDFiles: []string{testDoctorUUID1 + ".md"},
				FileContents: map[string][]byte{
					testDoctorUUID1 + ".md": nodeFileBytes(testDoctorUUID1),
				},
			},
			code:         node.AUD003,
			wantSeverity: node.SeverityError,
		},
		{
			name: "AUD004 is error",
			data: node.DoctorData{
				BinderSrc: binderWithRefs(testDoctorUUID1 + ".md"),
				UUIDFiles: []string{testDoctorUUID1 + ".md"},
				FileContents: map[string][]byte{
					testDoctorUUID1 + ".md": nodeFileBytesWrongID(testDoctorUUID2),
				},
			},
			code:         node.AUD004,
			wantSeverity: node.SeverityError,
		},
		{
			name: "AUD005 is error",
			data: node.DoctorData{
				BinderSrc: binderWithRefs(testDoctorUUID1 + ".md"),
				UUIDFiles: []string{testDoctorUUID1 + ".md"},
				FileContents: map[string][]byte{
					testDoctorUUID1 + ".md": nodeFileBytesMissingUpdated(testDoctorUUID1),
				},
			},
			code:         node.AUD005,
			wantSeverity: node.SeverityError,
		},
		{
			name: "AUD006 is warning",
			data: node.DoctorData{
				BinderSrc: binderWithRefs(testDoctorUUID1 + ".md"),
				UUIDFiles: []string{testDoctorUUID1 + ".md"},
				FileContents: map[string][]byte{
					testDoctorUUID1 + ".md": nodeFileBytesNoBody(testDoctorUUID1),
				},
			},
			code:         node.AUD006,
			wantSeverity: node.SeverityWarning,
		},
		{
			name: "AUD007 is error",
			data: node.DoctorData{
				BinderSrc: binderWithRefs(testDoctorUUID1 + ".md"),
				UUIDFiles: []string{testDoctorUUID1 + ".md"},
				FileContents: map[string][]byte{
					testDoctorUUID1 + ".md": nodeFileBytesBadYAML(),
				},
			},
			code:         node.AUD007,
			wantSeverity: node.SeverityError,
		},
		{
			name: "AUDW001 is warning",
			data: node.DoctorData{
				BinderSrc: binderWithRefs("chapter-one.md"),
				UUIDFiles: []string{},
				FileContents: map[string][]byte{
					"chapter-one.md": []byte("# Chapter One\n"),
				},
			},
			code:         node.AUDW001,
			wantSeverity: node.SeverityWarning,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := node.RunDoctor(ctx, tt.data)
			var found *node.AuditDiagnostic
			for i := range diags {
				if diags[i].Code == tt.code {
					found = &diags[i]
					break
				}
			}
			if found == nil {
				t.Fatalf("RunDoctor() did not produce code %q; got %v", tt.code, diags)
			}
			if found.Severity != tt.wantSeverity {
				t.Errorf("diagnostic %q Severity = %q, want %q", tt.code, found.Severity, tt.wantSeverity)
			}
		})
	}
}

// TestRunDoctor_DiagnosticPath verifies that each diagnostic carries the correct file path.
func TestRunDoctor_DiagnosticPath(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		data     node.DoctorData
		code     node.AuditCode
		wantPath string
	}{
		{
			name: "AUD001 path is the missing file",
			data: node.DoctorData{
				BinderSrc:    binderWithRefs(testDoctorUUID1 + ".md"),
				UUIDFiles:    []string{},
				FileContents: map[string][]byte{testDoctorUUID1 + ".md": nil},
			},
			code:     node.AUD001,
			wantPath: testDoctorUUID1 + ".md",
		},
		{
			name: "AUD002 path is the orphaned UUID file",
			data: node.DoctorData{
				BinderSrc: []byte{},
				UUIDFiles: []string{testDoctorUUID1 + ".md"},
				FileContents: map[string][]byte{
					testDoctorUUID1 + ".md": nodeFileBytes(testDoctorUUID1),
				},
			},
			code:     node.AUD002,
			wantPath: testDoctorUUID1 + ".md",
		},
		{
			name: "AUD003 path is the duplicated file",
			data: node.DoctorData{
				BinderSrc: binderWithRefs(testDoctorUUID1+".md", testDoctorUUID1+".md"),
				UUIDFiles: []string{testDoctorUUID1 + ".md"},
				FileContents: map[string][]byte{
					testDoctorUUID1 + ".md": nodeFileBytes(testDoctorUUID1),
				},
			},
			code:     node.AUD003,
			wantPath: testDoctorUUID1 + ".md",
		},
		{
			name: "AUDW001 path is the non-UUID file",
			data: node.DoctorData{
				BinderSrc: binderWithRefs("chapter-one.md"),
				UUIDFiles: []string{},
				FileContents: map[string][]byte{
					"chapter-one.md": []byte("# Chapter One\n"),
				},
			},
			code:     node.AUDW001,
			wantPath: "chapter-one.md",
		},
		{
			name: "AUD007 path is the file with bad YAML",
			data: node.DoctorData{
				BinderSrc: binderWithRefs(testDoctorUUID1 + ".md"),
				UUIDFiles: []string{testDoctorUUID1 + ".md"},
				FileContents: map[string][]byte{
					testDoctorUUID1 + ".md": nodeFileBytesBadYAML(),
				},
			},
			code:     node.AUD007,
			wantPath: testDoctorUUID1 + ".md",
		},
		{
			name: "AUD004 path is the file with mismatched id",
			data: node.DoctorData{
				BinderSrc: binderWithRefs(testDoctorUUID1 + ".md"),
				UUIDFiles: []string{testDoctorUUID1 + ".md"},
				FileContents: map[string][]byte{
					testDoctorUUID1 + ".md": nodeFileBytesWrongID(testDoctorUUID2),
				},
			},
			code:     node.AUD004,
			wantPath: testDoctorUUID1 + ".md",
		},
		{
			name: "AUD005 path is the file with missing field",
			data: node.DoctorData{
				BinderSrc: binderWithRefs(testDoctorUUID1 + ".md"),
				UUIDFiles: []string{testDoctorUUID1 + ".md"},
				FileContents: map[string][]byte{
					testDoctorUUID1 + ".md": nodeFileBytesMissingUpdated(testDoctorUUID1),
				},
			},
			code:     node.AUD005,
			wantPath: testDoctorUUID1 + ".md",
		},
		{
			name: "AUD006 path is the file with empty body",
			data: node.DoctorData{
				BinderSrc: binderWithRefs(testDoctorUUID1 + ".md"),
				UUIDFiles: []string{testDoctorUUID1 + ".md"},
				FileContents: map[string][]byte{
					testDoctorUUID1 + ".md": nodeFileBytesNoBody(testDoctorUUID1),
				},
			},
			code:     node.AUD006,
			wantPath: testDoctorUUID1 + ".md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := node.RunDoctor(ctx, tt.data)
			var found *node.AuditDiagnostic
			for i := range diags {
				if diags[i].Code == tt.code {
					found = &diags[i]
					break
				}
			}
			if found == nil {
				t.Fatalf("RunDoctor() did not produce code %q; got %v", tt.code, diags)
			}
			if found.Path != tt.wantPath {
				t.Errorf("diagnostic %q Path = %q, want %q", tt.code, found.Path, tt.wantPath)
			}
		})
	}
}

// TestRunDoctor_SortOrder verifies diagnostics are sorted errors-before-warnings,
// then alphabetically by path within each severity tier.
func TestRunDoctor_SortOrder(t *testing.T) {
	ctx := context.Background()

	// Mix: two AUD001 errors (different paths) + one AUD002 warning (orphan).
	data := node.DoctorData{
		BinderSrc: binderWithRefs(testDoctorUUID1+".md", testDoctorUUID2+".md"),
		UUIDFiles: []string{
			testDoctorUUID1 + ".md",
			testDoctorUUID2 + ".md",
			testDoctorUUID3 + ".md", // orphan → AUD002 warning
		},
		FileContents: map[string][]byte{
			testDoctorUUID1 + ".md": nil,                            // AUD001 error
			testDoctorUUID2 + ".md": nil,                            // AUD001 error
			testDoctorUUID3 + ".md": nodeFileBytes(testDoctorUUID3), // orphan
		},
	}

	diags := node.RunDoctor(ctx, data)

	// All errors must precede all warnings.
	seenWarning := false
	for _, d := range diags {
		if d.Severity == node.SeverityWarning {
			seenWarning = true
		}
		if seenWarning && d.Severity == node.SeverityError {
			t.Errorf("error diagnostic %q appears after a warning; sort order violated: %v", d.Code, diags)
			break
		}
	}

	// Within the error tier, paths must be in ascending order.
	var lastErrorPath string
	for _, d := range diags {
		if d.Severity == node.SeverityError {
			if lastErrorPath != "" && d.Path < lastErrorPath {
				t.Errorf("error diagnostics not sorted by path: %q < %q; diags: %v", d.Path, lastErrorPath, diags)
			}
			lastErrorPath = d.Path
		}
	}

	// Within the warning tier, paths must be in ascending order.
	var lastWarnPath string
	for _, d := range diags {
		if d.Severity == node.SeverityWarning {
			if lastWarnPath != "" && d.Path < lastWarnPath {
				t.Errorf("warning diagnostics not sorted by path: %q < %q; diags: %v", d.Path, lastWarnPath, diags)
			}
			lastWarnPath = d.Path
		}
	}
}

// TestRunDoctor_CycleGuard verifies that the visited-map prevents duplicate AUD001 checks
// and correctly emits exactly one AUD003 per duplicated target regardless of how many
// times it appears in the binder tree.
func TestRunDoctor_CycleGuard(t *testing.T) {
	ctx := context.Background()

	// Binder lists the same file three times.
	data := node.DoctorData{
		BinderSrc: binderWithRefs(
			testDoctorUUID1+".md",
			testDoctorUUID1+".md",
			testDoctorUUID1+".md",
		),
		UUIDFiles: []string{testDoctorUUID1 + ".md"},
		FileContents: map[string][]byte{
			testDoctorUUID1 + ".md": nodeFileBytes(testDoctorUUID1),
		},
	}

	diags := node.RunDoctor(ctx, data)

	// AUD003 must be produced (duplicate reference).
	if !hasDiagCode(diags, node.AUD003) {
		t.Errorf("RunDoctor() did not produce AUD003 for triplicate reference; got %v", diags)
	}

	// AUD001 must not appear — the file exists.
	if hasDiagCode(diags, node.AUD001) {
		t.Errorf("RunDoctor() produced AUD001 but file exists; got %v", diags)
	}

	// Frontmatter checks (AUD004/AUD005/AUD007) must not appear (file is valid).
	for _, badCode := range []node.AuditCode{node.AUD004, node.AUD005, node.AUD007} {
		if hasDiagCode(diags, badCode) {
			t.Errorf("RunDoctor() produced unexpected %q for valid node; got %v", badCode, diags)
		}
	}
}

// TestRunDoctor_DiagnosticsHaveNonEmptyMessage verifies every returned diagnostic
// carries a non-empty human-readable message.
func TestRunDoctor_DiagnosticsHaveNonEmptyMessage(t *testing.T) {
	ctx := context.Background()

	data := node.DoctorData{
		BinderSrc: binderWithRefs(
			testDoctorUUID1+".md", // AUD001: missing
			testDoctorUUID1+".md", // AUD003: duplicate
			"chapter-one.md",      // AUDW001: non-UUID
		),
		UUIDFiles: []string{testDoctorUUID2 + ".md"}, // AUD002: orphan
		FileContents: map[string][]byte{
			testDoctorUUID1 + ".md": nil, // AUD001
			"chapter-one.md":        []byte("# Chapter One\n"),
			testDoctorUUID2 + ".md": nodeFileBytes(testDoctorUUID2),
		},
	}

	diags := node.RunDoctor(ctx, data)

	for _, d := range diags {
		if d.Message == "" {
			t.Errorf("diagnostic %q has empty Message; full diagnostic: %+v", d.Code, d)
		}
	}
}

// TestRunDoctor_WithPrecomputedRefs verifies the fast path: when DoctorData.BinderRefs
// is non-nil, RunDoctor uses the pre-supplied refs and skips re-parsing BinderSrc.
func TestRunDoctor_WithPrecomputedRefs(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		data      node.DoctorData
		wantCodes []node.AuditCode
		wantNone  []node.AuditCode
	}{
		{
			name: "clean project via pre-computed refs produces no diagnostics",
			data: node.DoctorData{
				BinderSrc: binderWithRefs(testDoctorUUID1 + ".md"),
				UUIDFiles: []string{testDoctorUUID1 + ".md"},
				FileContents: map[string][]byte{
					testDoctorUUID1 + ".md": nodeFileBytes(testDoctorUUID1),
				},
				BinderRefs:     []string{testDoctorUUID1 + ".md"},
				BinderRefDiags: nil,
			},
			wantNone: []node.AuditCode{
				node.AUD001, node.AUD002, node.AUD003, node.AUD004,
				node.AUD005, node.AUD006, node.AUD007, node.AUDW001,
			},
		},
		{
			name: "pre-computed refDiags (AUD003) are included in output",
			data: node.DoctorData{
				BinderSrc: binderWithRefs(testDoctorUUID1 + ".md"),
				UUIDFiles: []string{testDoctorUUID1 + ".md"},
				FileContents: map[string][]byte{
					testDoctorUUID1 + ".md": nodeFileBytes(testDoctorUUID1),
				},
				BinderRefs: []string{testDoctorUUID1 + ".md"},
				BinderRefDiags: []node.AuditDiagnostic{
					{Code: node.AUD003, Severity: node.SeverityError, Message: "dup", Path: testDoctorUUID1 + ".md"},
				},
			},
			wantCodes: []node.AuditCode{node.AUD003},
		},
		{
			name: "orphan detection works with pre-computed refs",
			data: node.DoctorData{
				BinderSrc: binderWithRefs(testDoctorUUID1 + ".md"),
				UUIDFiles: []string{testDoctorUUID1 + ".md", testDoctorUUID2 + ".md"},
				FileContents: map[string][]byte{
					testDoctorUUID1 + ".md": nodeFileBytes(testDoctorUUID1),
					testDoctorUUID2 + ".md": nodeFileBytes(testDoctorUUID2),
				},
				BinderRefs:     []string{testDoctorUUID1 + ".md"},
				BinderRefDiags: nil,
			},
			wantCodes: []node.AuditCode{node.AUD002},
		},
		{
			name: "empty BinderRefs produces only orphan diagnostics for existing UUID files",
			data: node.DoctorData{
				BinderSrc:      []byte{},
				UUIDFiles:      []string{testDoctorUUID1 + ".md"},
				FileContents:   map[string][]byte{},
				BinderRefs:     []string{},
				BinderRefDiags: nil,
			},
			wantCodes: []node.AuditCode{node.AUD002},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := node.RunDoctor(ctx, tt.data)
			codes := diagCodesOf(diags)

			for _, want := range tt.wantCodes {
				if _, ok := codes[want]; !ok {
					t.Errorf("RunDoctor() missing expected code %q; got %v", want, diags)
				}
			}
			for _, none := range tt.wantNone {
				if _, ok := codes[none]; ok {
					t.Errorf("RunDoctor() produced unexpected code %q; got %v", none, diags)
				}
			}
		})
	}
}

// TestRunDoctor_NoPanic_CancelledContext verifies RunDoctor does not panic when
// called with an already-cancelled context.
func TestRunDoctor_NoPanic_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before calling RunDoctor

	data := node.DoctorData{
		BinderSrc: binderWithRefs(testDoctorUUID1 + ".md"),
		UUIDFiles: []string{testDoctorUUID1 + ".md"},
		FileContents: map[string][]byte{
			testDoctorUUID1 + ".md": nodeFileBytes(testDoctorUUID1),
		},
	}

	// Must not panic regardless of cancellation state.
	_ = node.RunDoctor(ctx, data)
}
