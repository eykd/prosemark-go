package cmd

import (
	"testing"

	"github.com/eykd/prosemark-go/internal/node"
)

func TestToDoctorDiagnosticJSON(t *testing.T) {
	tests := []struct {
		name  string
		input []node.AuditDiagnostic
		want  []DoctorDiagnosticJSON
	}{
		{
			name:  "empty slice",
			input: []node.AuditDiagnostic{},
			want:  []DoctorDiagnosticJSON{},
		},
		{
			name: "single diagnostic",
			input: []node.AuditDiagnostic{
				{
					Code:     node.AUD001,
					Severity: node.SeverityError,
					Message:  "file not found",
					Path:     "abc.md",
				},
			},
			want: []DoctorDiagnosticJSON{
				{
					Severity: string(node.SeverityError),
					Code:     string(node.AUD001),
					Message:  "file not found",
					Path:     "abc.md",
				},
			},
		},
		{
			name: "multiple diagnostics preserve order",
			input: []node.AuditDiagnostic{
				{
					Code:     node.AUD002,
					Severity: node.SeverityWarning,
					Message:  "orphan file",
					Path:     "orphan.md",
				},
				{
					Code:     node.AUD003,
					Severity: node.SeverityError,
					Message:  "duplicate ref",
					Path:     "_binder.md",
				},
			},
			want: []DoctorDiagnosticJSON{
				{
					Severity: string(node.SeverityWarning),
					Code:     string(node.AUD002),
					Message:  "orphan file",
					Path:     "orphan.md",
				},
				{
					Severity: string(node.SeverityError),
					Code:     string(node.AUD003),
					Message:  "duplicate ref",
					Path:     "_binder.md",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toDoctorDiagnosticJSON(tt.input)

			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.want))
			}

			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("index %d: got %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}
