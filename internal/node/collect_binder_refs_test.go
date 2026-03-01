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
