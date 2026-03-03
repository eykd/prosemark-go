package cmd

// Tests for consistent JSON diagnostic schema across all pmk commands.
//
// Issue: doctor --json emits a bare array with {code, message, path} items,
// while parse/add-child/delete/move --json emit a wrapped object with
// diagnostics using {severity, code, message, location}. All commands should
// use a consistent diagnostic schema.
//
// These tests document the DESIRED behavior and currently FAIL.

import (
	"bytes"
	"encoding/json"
	"testing"
)

// ─── Unified schema tests ─────────────────────────────────────────────────────

// TestNewDoctorCmd_JSON_IsWrappedObject verifies that --json outputs a wrapped
// object with "version" and "diagnostics" keys (not a bare array), consistent
// with parse/add-child/delete/move JSON output.
//
// Current behavior: bare JSON array → test FAILS.
// Expected behavior: {"version":"1","diagnostics":[...]}
func TestNewDoctorCmd_JSON_IsWrappedObject(t *testing.T) {
	mock := &mockDoctorIO{
		binderBytes: doctorBinderWithNode(doctorTestNodeUUID),
		nodeFiles: map[string]nodeFileEntry{
			doctorTestNodeUUID + ".md": {content: nil, exists: false},
		},
	}
	c := NewDoctorCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--project", ".", "--json"})

	_ = c.Execute()

	// The top-level JSON value must be an object, not an array.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &raw); err != nil {
		t.Fatalf("--json output is not a JSON object: %v\noutput: %q", err, out.String())
	}

	// Must have "version" key.
	versionRaw, hasVersion := raw["version"]
	if !hasVersion {
		t.Errorf("--json output missing 'version' key; got keys: %v", mapKeys(raw))
	} else {
		var version string
		if err := json.Unmarshal(versionRaw, &version); err != nil {
			t.Errorf("'version' is not a string: %v", err)
		} else if version != "1" {
			t.Errorf("'version' = %q, want \"1\"", version)
		}
	}

	// Must have "diagnostics" key.
	if _, hasDiags := raw["diagnostics"]; !hasDiags {
		t.Errorf("--json output missing 'diagnostics' key; got keys: %v", mapKeys(raw))
	}
}

// TestNewDoctorCmd_JSON_CleanProject_IsWrappedObject verifies that a clean project
// also emits a wrapped object with an empty diagnostics array.
//
// Current behavior: "[]" (bare empty array) → test FAILS.
// Expected behavior: {"version":"1","diagnostics":[]}
func TestNewDoctorCmd_JSON_CleanProject_IsWrappedObject(t *testing.T) {
	mock := &mockDoctorIO{
		binderBytes: doctorBinderWithNode(doctorTestNodeUUID),
		nodeFiles: map[string]nodeFileEntry{
			doctorTestNodeUUID + ".md": {content: validDoctorNodeContent(doctorTestNodeUUID), exists: true},
			".prosemark.yml":           {content: []byte("version: \"1\"\n"), exists: true},
		},
	}
	c := NewDoctorCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--project", ".", "--json"})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &raw); err != nil {
		t.Fatalf("clean project --json output is not a JSON object: %v\noutput: %q", err, out.String())
	}

	diagsRaw, hasDiags := raw["diagnostics"]
	if !hasDiags {
		t.Fatalf("--json output missing 'diagnostics' key")
	}

	var diags []json.RawMessage
	if err := json.Unmarshal(diagsRaw, &diags); err != nil {
		t.Fatalf("'diagnostics' is not a JSON array: %v", err)
	}
	if len(diags) != 0 {
		t.Errorf("expected empty diagnostics array for clean project, got %d items", len(diags))
	}
}

// TestNewDoctorCmd_JSON_DiagnosticsHaveSeverity verifies that each diagnostic in
// --json output includes a "severity" field (consistent with binder diagnostic schema).
//
// Current behavior: severity is excluded → test FAILS.
// Expected behavior: each diagnostic has "severity": "error"|"warning"
func TestNewDoctorCmd_JSON_DiagnosticsHaveSeverity(t *testing.T) {
	mock := &mockDoctorIO{
		binderBytes: doctorBinderWithNode(doctorTestNodeUUID),
		nodeFiles: map[string]nodeFileEntry{
			doctorTestNodeUUID + ".md": {content: nil, exists: false},
		},
	}
	c := NewDoctorCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--project", ".", "--json"})

	_ = c.Execute()

	// Parse as wrapped object.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &raw); err != nil {
		t.Fatalf("--json output is not a JSON object: %v\noutput: %q", err, out.String())
	}

	diagsRaw, ok := raw["diagnostics"]
	if !ok {
		t.Fatal("--json output missing 'diagnostics' key")
	}

	var diags []map[string]json.RawMessage
	if err := json.Unmarshal(diagsRaw, &diags); err != nil {
		t.Fatalf("'diagnostics' is not a valid JSON array: %v", err)
	}
	if len(diags) == 0 {
		t.Fatal("expected at least one diagnostic")
	}

	for i, d := range diags {
		severityRaw, hasSeverity := d["severity"]
		if !hasSeverity {
			t.Errorf("diagnostics[%d] missing 'severity' field; got keys: %v", i, mapKeys(d))
			continue
		}
		var severity string
		if err := json.Unmarshal(severityRaw, &severity); err != nil {
			t.Errorf("diagnostics[%d] 'severity' is not a string: %v", i, err)
			continue
		}
		if severity != "error" && severity != "warning" {
			t.Errorf("diagnostics[%d] 'severity' = %q, want \"error\" or \"warning\"", i, severity)
		}
	}
}

// TestNewDoctorCmd_JSON_DiagnosticsHaveRequiredFields verifies that each diagnostic
// in --json output has the unified schema fields: severity, code, message, and path.
//
// Currently fails because (a) output is a bare array and (b) severity is absent.
func TestNewDoctorCmd_JSON_DiagnosticsHaveRequiredFields(t *testing.T) {
	tests := []struct {
		name         string
		nodeFiles    map[string]nodeFileEntry
		wantCode     string
		wantSeverity string
	}{
		{
			name: "missing file emits AUD001 with error severity",
			nodeFiles: map[string]nodeFileEntry{
				doctorTestNodeUUID + ".md": {content: nil, exists: false},
			},
			wantCode:     "AUD001",
			wantSeverity: "error",
		},
		{
			name: "missing prosemark.yml emits AUD008 with error severity",
			nodeFiles: map[string]nodeFileEntry{
				doctorTestNodeUUID + ".md": {content: validDoctorNodeContent(doctorTestNodeUUID), exists: true},
				// .prosemark.yml not in nodeFiles → treated as missing
			},
			wantCode:     "AUD008",
			wantSeverity: "error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockDoctorIO{
				binderBytes: doctorBinderWithNode(doctorTestNodeUUID),
				nodeFiles:   tt.nodeFiles,
			}
			c := NewDoctorCmd(mock)
			out := new(bytes.Buffer)
			c.SetOut(out)
			c.SetErr(new(bytes.Buffer))
			c.SetArgs([]string{"--project", ".", "--json"})

			_ = c.Execute()

			// Top level must be an object.
			var raw map[string]json.RawMessage
			if err := json.Unmarshal(out.Bytes(), &raw); err != nil {
				t.Fatalf("--json output is not a JSON object: %v\noutput: %q", err, out.String())
			}

			diagsRaw, ok := raw["diagnostics"]
			if !ok {
				t.Fatal("missing 'diagnostics' key in JSON output")
			}

			var diags []map[string]json.RawMessage
			if err := json.Unmarshal(diagsRaw, &diags); err != nil {
				t.Fatalf("'diagnostics' is not a JSON array: %v", err)
			}

			// Find the expected diagnostic.
			found := false
			for _, d := range diags {
				codeRaw, hasCode := d["code"]
				if !hasCode {
					t.Errorf("diagnostic missing 'code' field; got: %v", mapKeys(d))
					continue
				}
				var code string
				_ = json.Unmarshal(codeRaw, &code)
				if code != tt.wantCode {
					continue
				}
				found = true

				// Verify required fields are present.
				requiredFields := []string{"severity", "code", "message", "path"}
				for _, field := range requiredFields {
					if _, has := d[field]; !has {
						t.Errorf("diagnostic with code=%q missing field %q; got keys: %v",
							code, field, mapKeys(d))
					}
				}

				// Verify severity value.
				if severityRaw, ok := d["severity"]; ok {
					var severity string
					_ = json.Unmarshal(severityRaw, &severity)
					if severity != tt.wantSeverity {
						t.Errorf("diagnostic code=%q severity=%q, want %q", code, severity, tt.wantSeverity)
					}
				}
			}

			if !found {
				t.Errorf("expected diagnostic with code=%q not found in output; got: %s",
					tt.wantCode, out.String())
			}
		})
	}
}

// mapKeys returns the keys of a map for use in error messages.
func mapKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
