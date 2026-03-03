package cmd

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// TestNewDoctorCmd_ProjectConfigValidation verifies that doctor validates
// .prosemark.yml existence and readability, emitting AUD008 for a missing,
// unreadable, or corrupted project config file.
func TestNewDoctorCmd_ProjectConfigValidation(t *testing.T) {
	tests := []struct {
		name       string
		nodeFiles  map[string]nodeFileEntry // ".prosemark.yml" entry controls config state
		wantErr    bool                     // command returns error (exit 1)
		wantCode   string                   // audit code that MUST appear in stdout
		wantNoCode string                   // audit code that must NOT appear in stdout
	}{
		{
			// Missing .prosemark.yml: ReadNodeFile returns (nil, false, nil).
			// Doctor should emit AUD008 and exit 1.
			name:      "missing .prosemark.yml emits AUD008 error",
			nodeFiles: map[string]nodeFileEntry{},
			wantErr:   true,
			wantCode:  "AUD008",
		},
		{
			// Unreadable .prosemark.yml: ReadNodeFile returns a permission error.
			// Doctor should emit AUD008 and exit 1.
			name: "unreadable .prosemark.yml emits AUD008 error",
			nodeFiles: map[string]nodeFileEntry{
				".prosemark.yml": {content: nil, exists: true, err: errors.New("permission denied")},
			},
			wantErr:  true,
			wantCode: "AUD008",
		},
		{
			// Corrupted .prosemark.yml: exists but contains invalid YAML.
			// Doctor should emit AUD008 and exit 1.
			name: "corrupted YAML in .prosemark.yml emits AUD008 error",
			nodeFiles: map[string]nodeFileEntry{
				".prosemark.yml": {content: []byte(": invalid: [yaml\n"), exists: true},
			},
			wantErr:  true,
			wantCode: "AUD008",
		},
		{
			// Valid .prosemark.yml: exists and parses cleanly.
			// Doctor should not emit AUD008 and should exit 0.
			name: "valid .prosemark.yml produces no AUD008",
			nodeFiles: map[string]nodeFileEntry{
				".prosemark.yml": {content: []byte("version: \"1\"\n"), exists: true},
			},
			wantErr:    false,
			wantNoCode: "AUD008",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockDoctorIO{
				binderBytes: doctorBinderEmpty(),
				nodeFiles:   tt.nodeFiles,
			}
			c := NewDoctorCmd(mock)
			out := new(bytes.Buffer)
			c.SetOut(out)
			c.SetErr(new(bytes.Buffer))
			c.SetArgs([]string{"--project", "."})

			err := c.Execute()

			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v (stdout=%q)", err, tt.wantErr, out.String())
			}
			if tt.wantCode != "" && !strings.Contains(out.String(), tt.wantCode) {
				t.Errorf("stdout = %q, expected to contain %q", out.String(), tt.wantCode)
			}
			if tt.wantNoCode != "" && strings.Contains(out.String(), tt.wantNoCode) {
				t.Errorf("stdout = %q, must NOT contain %q", out.String(), tt.wantNoCode)
			}
		})
	}
}

// TestNewDoctorCmd_ProjectConfig_JSONMode verifies that AUD008 appears in
// --json output when .prosemark.yml is absent.
func TestNewDoctorCmd_ProjectConfig_JSONMode(t *testing.T) {
	mock := &mockDoctorIO{
		binderBytes: doctorBinderEmpty(),
		nodeFiles:   map[string]nodeFileEntry{},
	}
	c := NewDoctorCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--project", ".", "--json"})

	_ = c.Execute()

	if !strings.Contains(out.String(), "AUD008") {
		t.Errorf("--json output missing AUD008 for absent .prosemark.yml: %q", out.String())
	}
}
