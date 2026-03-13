package node_test

import (
	"context"
	"testing"

	"github.com/eykd/prosemark-go/internal/node"
)

// TestCollectBinderRefs covers the new domain-layer function that replaces both
// the cmd-layer binder.Parse call and scanEscapingBinderLinks.
func TestCollectBinderRefs(t *testing.T) {
	tests := []struct {
		name      string
		binderSrc []byte
		wantRefs  []string
		wantCodes []node.AuditCode
		wantNone  []node.AuditCode
	}{
		{
			name:      "empty binder returns no refs and no diagnostics",
			binderSrc: []byte{},
			wantRefs:  []string{},
			wantNone:  []node.AuditCode{node.AUDW001},
		},
		{
			name:      "single valid ref is returned",
			binderSrc: binderWithRefs(testDoctorUUID1 + ".md"),
			wantRefs:  []string{testDoctorUUID1 + ".md"},
			wantNone:  []node.AuditCode{node.AUDW001},
		},
		{
			name:      "multiple valid refs are all returned",
			binderSrc: binderWithRefs(testDoctorUUID1+".md", testDoctorUUID2+".md"),
			wantRefs:  []string{testDoctorUUID1 + ".md", testDoctorUUID2 + ".md"},
			wantNone:  []node.AuditCode{node.AUDW001},
		},
		{
			name:      "escaping link with ../ prefix emits AUDW001 and is not included in refs",
			binderSrc: []byte("- [Secret](../../etc/passwd)\n"),
			wantRefs:  []string{},
			wantCodes: []node.AuditCode{node.AUDW001},
		},
		{
			name:      "escaping link equal to .. emits AUDW001 and is not included in refs",
			binderSrc: []byte("- [Parent](..)\n"),
			wantRefs:  []string{},
			wantCodes: []node.AuditCode{node.AUDW001},
		},
		{
			name: "mixed: valid refs and escaping link",
			binderSrc: append(
				binderWithRefs(testDoctorUUID1+".md"),
				[]byte("- [Secret](../../secret)\n")...,
			),
			wantRefs:  []string{testDoctorUUID1 + ".md"},
			wantCodes: []node.AuditCode{node.AUDW001},
		},
		{
			name:      "duplicate ref emits AUD003 and appears only once in refs",
			binderSrc: binderWithRefs(testDoctorUUID1+".md", testDoctorUUID1+".md"),
			wantRefs:  []string{testDoctorUUID1 + ".md"},
			wantCodes: []node.AuditCode{node.AUD003},
			wantNone:  []node.AuditCode{node.AUDW001},
		},
		{
			name:      "triplicate ref emits one AUD003",
			binderSrc: binderWithRefs(testDoctorUUID1+".md", testDoctorUUID1+".md", testDoctorUUID1+".md"),
			wantRefs:  []string{testDoctorUUID1 + ".md"},
			wantCodes: []node.AuditCode{node.AUD003},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refs, diags := node.CollectBinderRefs(context.Background(), tt.binderSrc)

			// Check refs.
			got := make(map[string]struct{}, len(refs))
			for _, r := range refs {
				got[r] = struct{}{}
			}
			for _, want := range tt.wantRefs {
				if _, ok := got[want]; !ok {
					t.Errorf("CollectBinderRefs() missing expected ref %q; got %v", want, refs)
				}
			}
			// Ensure no extra refs beyond wantRefs.
			if len(refs) != len(tt.wantRefs) {
				t.Errorf("CollectBinderRefs() = %d refs, want %d; refs=%v", len(refs), len(tt.wantRefs), refs)
			}

			// Check diagnostics.
			codes := diagCodesOf(diags)
			for _, want := range tt.wantCodes {
				if _, ok := codes[want]; !ok {
					t.Errorf("CollectBinderRefs() missing expected code %q; got %v", want, diags)
				}
			}
			for _, none := range tt.wantNone {
				if _, ok := codes[none]; ok {
					t.Errorf("CollectBinderRefs() produced unexpected code %q; got %v", none, diags)
				}
			}
		})
	}
}

// TestCollectBinderRefs_EscapingLink_IsWarning verifies the AUDW001 severity.
func TestCollectBinderRefs_EscapingLink_IsWarning(t *testing.T) {
	_, diags := node.CollectBinderRefs(context.Background(), []byte("- [X](../../etc/passwd)\n"))
	for _, d := range diags {
		if d.Code == node.AUDW001 && d.Severity != node.SeverityWarning {
			t.Errorf("AUDW001 severity = %q, want %q", d.Severity, node.SeverityWarning)
		}
	}
	if !hasDiagCode(diags, node.AUDW001) {
		t.Errorf("expected AUDW001 for escaping link; got %v", diags)
	}
}

// TestCollectBinderRefs_EscapingLink_PathIsTarget verifies the AUDW001 Path matches the target.
func TestCollectBinderRefs_EscapingLink_PathIsTarget(t *testing.T) {
	target := "../../etc/passwd"
	_, diags := node.CollectBinderRefs(context.Background(), []byte("- [X]("+target+")\n"))
	for _, d := range diags {
		if d.Code == node.AUDW001 {
			if d.Path != target {
				t.Errorf("AUDW001 Path = %q, want %q", d.Path, target)
			}
			return
		}
	}
	t.Errorf("no AUDW001 diagnostic found; got %v", diags)
}

// TestCollectBinderRefs_NoPanic_CancelledContext verifies no panic on cancelled context.
func TestCollectBinderRefs_NoPanic_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _ = node.CollectBinderRefs(ctx, binderWithRefs(testDoctorUUID1+".md"))
}

// TestCollectBinderRefs_BinderParseDiagnosticsIncluded verifies that parse-level
// binder diagnostics (BNDW*) are propagated through CollectBinderRefs so the
// doctor command can surface them to users.
func TestCollectBinderRefs_BinderParseDiagnosticsIncluded(t *testing.T) {
	binderWithPragma := []byte("<!-- prosemark-binder:v1 -->\n- [Title](" + testDoctorUUID1 + ".md)\n")

	tests := []struct {
		name      string
		binderSrc []byte
		wantCodes []node.AuditCode
		wantNone  []node.AuditCode
	}{
		{
			name:      "binder without pragma emits BNDW001",
			binderSrc: binderWithRefs(testDoctorUUID1 + ".md"), // no pragma
			wantCodes: []node.AuditCode{node.BNDW001},
		},
		{
			name:      "binder with pragma does not emit BNDW001",
			binderSrc: binderWithPragma,
			wantNone:  []node.AuditCode{node.BNDW001},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, diags := node.CollectBinderRefs(context.Background(), tt.binderSrc)
			codes := diagCodesOf(diags)
			for _, want := range tt.wantCodes {
				if _, ok := codes[want]; !ok {
					t.Errorf("CollectBinderRefs() missing expected code %q; got %v", want, diags)
				}
			}
			for _, none := range tt.wantNone {
				if _, ok := codes[none]; ok {
					t.Errorf("CollectBinderRefs() produced unexpected code %q; got %v", none, diags)
				}
			}
		})
	}
}

// TestCollectBinderRefs_InvalidUTF8_EmitsAUD009 verifies that binary/invalid-UTF-8
// binder content produces an AUD009 error diagnostic.
func TestCollectBinderRefs_InvalidUTF8_EmitsAUD009(t *testing.T) {
	// \xff\xfe is invalid UTF-8 (not a valid start sequence).
	invalidUTF8 := []byte("- [Title](file.md)\xff\xfe\n")

	refs, diags := node.CollectBinderRefs(context.Background(), invalidUTF8)

	// Must emit AUD009 for invalid UTF-8 content.
	if !hasDiagCode(diags, node.AUD009) {
		t.Errorf("CollectBinderRefs() missing AUD009 for invalid UTF-8; got diags=%v, refs=%v", diags, refs)
	}

	// AUD009 must be error severity.
	for _, d := range diags {
		if d.Code == node.AUD009 {
			if d.Severity != node.SeverityError {
				t.Errorf("AUD009 severity = %q, want %q", d.Severity, node.SeverityError)
			}
			if d.Message == "" {
				t.Errorf("AUD009 has empty Message")
			}
		}
	}
}

// TestCollectBinderRefs_PureBinary_EmitsAUD009 verifies that pure binary content
// (no valid UTF-8 at all) produces an AUD009 error diagnostic.
func TestCollectBinderRefs_PureBinary_EmitsAUD009(t *testing.T) {
	pureBinary := []byte{0x00, 0x80, 0xff, 0xfe, 0x89, 0x50, 0x4e, 0x47}

	refs, diags := node.CollectBinderRefs(context.Background(), pureBinary)

	if !hasDiagCode(diags, node.AUD009) {
		t.Errorf("CollectBinderRefs() missing AUD009 for pure binary; got diags=%v, refs=%v", diags, refs)
	}
}

// TestCollectBinderRefs_BNDW001_IsWarning verifies that BNDW001 (missing pragma)
// is surfaced with warning severity so doctor exits 0 for pragma-only issues.
func TestCollectBinderRefs_BNDW001_IsWarning(t *testing.T) {
	// Binder without pragma: binder.Parse will emit BNDW001.
	_, diags := node.CollectBinderRefs(context.Background(), binderWithRefs(testDoctorUUID1+".md"))
	for _, d := range diags {
		if d.Code == node.BNDW001 {
			if d.Severity != node.SeverityWarning {
				t.Errorf("BNDW001 severity = %q, want %q", d.Severity, node.SeverityWarning)
			}
			return
		}
	}
	t.Errorf("expected BNDW001 diagnostic in output; got %v", diags)
}
