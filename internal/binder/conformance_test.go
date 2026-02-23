package binder_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
)

// parseFixturesDir is the path to the parse conformance fixtures, relative to this file.
const parseFixturesDir = "../../docs/conformance/v1/parse/fixtures"

// parseConformanceOutput mirrors the JSON shape emitted by cmd.parseOutput (version + root),
// used to marshal the actual ParseResult for fixture comparison.
type parseConformanceOutput struct {
	Version string       `json:"version"`
	Root    *binder.Node `json:"root"`
}

// diagnosticsConformanceOutput mirrors the expected-diagnostics.json wrapper schema.
type diagnosticsConformanceOutput struct {
	Version     string              `json:"version"`
	Diagnostics []binder.Diagnostic `json:"diagnostics"`
}

// TestConformance_ParseFixtures verifies that binder.Parse produces the expected node
// tree and diagnostics for every fixture in docs/conformance/v1/parse/fixtures/.
//
// This test is intentionally broad: it will fail for any fixture where the parser
// behaviour diverges from the expected output, exposing all known gaps in one run.
func TestConformance_ParseFixtures(t *testing.T) {
	entries, err := os.ReadDir(parseFixturesDir)
	if err != nil {
		t.Fatalf("reading fixtures dir %s: %v", parseFixturesDir, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		t.Run(name, func(t *testing.T) {
			fixturePath := filepath.Join(parseFixturesDir, name)
			runParseFixture(t, fixturePath)
		})
	}
}

// runParseFixture loads one fixture directory, runs the parser, and asserts the output
// matches the expected JSON files.
func runParseFixture(t *testing.T, fixturePath string) {
	t.Helper()

	// Load inputs.
	binderBytes, err := os.ReadFile(filepath.Join(fixturePath, "binder.md"))
	if err != nil {
		t.Fatalf("read binder.md: %v", err)
	}

	projectBytes, err := os.ReadFile(filepath.Join(fixturePath, "project.json"))
	if err != nil {
		t.Fatalf("read project.json: %v", err)
	}

	var proj binder.Project
	if err := json.Unmarshal(projectBytes, &proj); err != nil {
		t.Fatalf("parse project.json: %v", err)
	}

	// Run the parser.
	result, diags, parseErr := binder.Parse(context.Background(), binderBytes, &proj)
	if parseErr != nil {
		t.Fatalf("Parse() returned unexpected error: %v", parseErr)
	}
	if diags == nil {
		diags = []binder.Diagnostic{}
	}

	// --- Assert parse tree matches expected-parse.json ---
	expectedParseBytes, err := os.ReadFile(filepath.Join(fixturePath, "expected-parse.json"))
	if err != nil {
		t.Fatalf("read expected-parse.json: %v", err)
	}

	actualParseOutput := parseConformanceOutput{
		Version: result.Version,
		Root:    result.Root,
	}
	actualParseBytes, err := json.Marshal(actualParseOutput)
	if err != nil {
		t.Fatalf("marshal actual parse output: %v", err)
	}

	if !jsonSubsetContains(t, "parse", expectedParseBytes, actualParseBytes) {
		t.Errorf("parse tree mismatch\n  expected: %s\n  actual:   %s",
			compactJSON(expectedParseBytes), compactJSON(actualParseBytes))
	}

	// --- Assert diagnostics match expected-diagnostics.json ---
	expectedDiagBytes, err := os.ReadFile(filepath.Join(fixturePath, "expected-diagnostics.json"))
	if err != nil {
		t.Fatalf("read expected-diagnostics.json: %v", err)
	}

	var expectedDiagWrapper diagnosticsConformanceOutput
	if err := json.Unmarshal(expectedDiagBytes, &expectedDiagWrapper); err != nil {
		t.Fatalf("parse expected-diagnostics.json: %v", err)
	}

	assertDiagnosticsMatch(t, expectedDiagWrapper.Diagnostics, diags)
}

// TestConformance_ParseStability verifies that for every parse fixture, calling
// binder.Parse then binder.Serialize produces bytes identical to the original input.
// This guards against the serializer losing any source data (including BOM, line endings,
// whitespace-only lines, and non-structural content).
func TestConformance_ParseStability(t *testing.T) {
	entries, err := os.ReadDir(parseFixturesDir)
	if err != nil {
		t.Fatalf("reading fixtures dir %s: %v", parseFixturesDir, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		t.Run(name, func(t *testing.T) {
			fixturePath := filepath.Join(parseFixturesDir, name)

			binderBytes, err := os.ReadFile(filepath.Join(fixturePath, "binder.md"))
			if err != nil {
				t.Fatalf("read binder.md: %v", err)
			}

			projectBytes, err := os.ReadFile(filepath.Join(fixturePath, "project.json"))
			if err != nil {
				t.Fatalf("read project.json: %v", err)
			}

			var proj binder.Project
			if err := json.Unmarshal(projectBytes, &proj); err != nil {
				t.Fatalf("parse project.json: %v", err)
			}

			result, _, parseErr := binder.Parse(context.Background(), binderBytes, &proj)
			if parseErr != nil {
				t.Fatalf("Parse() returned unexpected error: %v", parseErr)
			}

			serialized := binder.Serialize(result)
			if !bytes.Equal(binderBytes, serialized) {
				t.Errorf("stability: serialized output differs from original input\n"+
					"  input  (%d bytes): %q\n"+
					"  output (%d bytes): %q",
					len(binderBytes), binderBytes,
					len(serialized), serialized)
			}
		})
	}
}

// --- JSON comparison helpers ---

// jsonSubsetContains reports whether every key-value pair present in expectedJSON
// is also present in actualJSON with an equal value.  Keys present in actualJSON
// but absent from expectedJSON are not checked.  For JSON arrays the lengths must
// be equal and elements are compared positionally using the same subset rule.
func jsonSubsetContains(t *testing.T, label string, expectedJSON, actualJSON []byte) bool {
	t.Helper()
	var expected, actual interface{}
	if err := json.Unmarshal(expectedJSON, &expected); err != nil {
		t.Errorf("%s: unmarshal expected JSON: %v", label, err)
		return false
	}
	if err := json.Unmarshal(actualJSON, &actual); err != nil {
		t.Errorf("%s: unmarshal actual JSON: %v", label, err)
		return false
	}
	return jsonSubsetEqual(t, label, expected, actual)
}

// jsonSubsetEqual recursively compares two unmarshaled JSON values.
func jsonSubsetEqual(t *testing.T, path string, expected, actual interface{}) bool {
	t.Helper()
	if expected == nil {
		// nil in expected means "don't care" only when it is a JSON null;
		// we still require the actual to be null too.
		if actual != nil {
			t.Errorf("%s: expected null, got %v", path, actual)
			return false
		}
		return true
	}

	switch e := expected.(type) {
	case map[string]interface{}:
		a, ok := actual.(map[string]interface{})
		if !ok {
			t.Errorf("%s: expected JSON object, got %T (%v)", path, actual, actual)
			return false
		}
		ok = true
		for k, ev := range e {
			av, exists := a[k]
			if !exists {
				t.Errorf("%s.%s: key present in expected but missing in actual", path, k)
				ok = false
				continue
			}
			if !jsonSubsetEqual(t, path+"."+k, ev, av) {
				ok = false
			}
		}
		return ok

	case []interface{}:
		a, ok := actual.([]interface{})
		if !ok {
			t.Errorf("%s: expected JSON array, got %T (%v)", path, actual, actual)
			return false
		}
		if len(e) != len(a) {
			t.Errorf("%s: expected array length %d, got %d\n  expected: %v\n  actual:   %v",
				path, len(e), len(a), expected, actual)
			return false
		}
		ok = true
		for i, ev := range e {
			if !jsonSubsetEqual(t, fmt.Sprintf("%s[%d]", path, i), ev, a[i]) {
				ok = false
			}
		}
		return ok

	default:
		// Scalar: must be deeply equal.
		if !reflect.DeepEqual(expected, actual) {
			t.Errorf("%s: expected %v (%T), got %v (%T)", path, expected, expected, actual, actual)
			return false
		}
		return true
	}
}

// assertDiagnosticsMatch checks that the actual diagnostics match the expected
// diagnostics in count, code, and severity.  If the expected diagnostic also
// specifies a message, that field must match too.  Location is compared when
// present in the expected diagnostic (line and column only; byteOffset is not
// required to match).
func assertDiagnosticsMatch(t *testing.T, expected []binder.Diagnostic, actual []binder.Diagnostic) {
	t.Helper()

	if len(expected) != len(actual) {
		expCodes := extractCodes(expected)
		actCodes := extractCodes(actual)
		t.Errorf("diagnostics count: expected %d, got %d\n  expected codes: %v\n  actual codes:   %v",
			len(expected), len(actual), expCodes, actCodes)
		return
	}

	for i, exp := range expected {
		act := actual[i]
		if exp.Severity != act.Severity {
			t.Errorf("diagnostic[%d].severity: expected %q, got %q", i, exp.Severity, act.Severity)
		}
		if exp.Code != act.Code {
			t.Errorf("diagnostic[%d].code: expected %q, got %q", i, exp.Code, act.Code)
		}
		if exp.Message != "" && exp.Message != act.Message {
			t.Errorf("diagnostic[%d].message: expected %q, got %q", i, exp.Message, act.Message)
		}
		if exp.Location != nil {
			if act.Location == nil {
				t.Errorf("diagnostic[%d].location: expected {line:%d, column:%d}, got nil",
					i, exp.Location.Line, exp.Location.Column)
			} else {
				if exp.Location.Line != act.Location.Line {
					t.Errorf("diagnostic[%d].location.line: expected %d, got %d",
						i, exp.Location.Line, act.Location.Line)
				}
				if exp.Location.Column != act.Location.Column {
					t.Errorf("diagnostic[%d].location.column: expected %d, got %d",
						i, exp.Location.Column, act.Location.Column)
				}
			}
		}
	}
}

func extractCodes(diags []binder.Diagnostic) []string {
	codes := make([]string, len(diags))
	for i, d := range diags {
		codes[i] = d.Code
	}
	return codes
}

func compactJSON(b []byte) []byte {
	var buf bytes.Buffer
	if err := json.Compact(&buf, b); err != nil {
		return b
	}
	return buf.Bytes()
}
