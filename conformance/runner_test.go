// Package conformance_test implements the CLI conformance runner for
// docs/conformance/v1. It exercises all parse and ops fixtures by invoking
// the pmk binary as a subprocess, exactly as described in
// docs/conformance/v1/runner-contract.md §3.5.
//
// TestMain builds bin/pmk once into a temporary directory before any test
// runs, then removes the directory on exit.
package conformance_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// pmkBinary is the absolute path to the compiled pmk binary, set by TestMain.
var pmkBinary string

// conformanceRoot is the path to docs/conformance/v1 relative to this package.
const conformanceRoot = "../docs/conformance/v1"

// TestMain builds the pmk binary to a temporary directory (avoiding binary
// path races between parallel test processes) and runs all tests. It removes
// the temporary directory on exit regardless of test outcome.
func TestMain(m *testing.M) {
	repoRoot, err := filepath.Abs("..")
	if err != nil {
		fmt.Fprintf(os.Stderr, "filepath.Abs: %v\n", err)
		os.Exit(1)
	}

	tmpDir, err := os.MkdirTemp("", "conformance-pmk-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "os.MkdirTemp: %v\n", err)
		os.Exit(1)
	}

	pmkBinary = filepath.Join(tmpDir, "pmk")
	build := exec.Command("go", "build", "-o", pmkBinary, ".")
	build.Dir = repoRoot
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "go build failed: %v\n%s\n", err, out)
		os.RemoveAll(tmpDir)
		os.Exit(1)
	}

	code := m.Run()
	os.RemoveAll(tmpDir)
	os.Exit(code)
}

// ---------------------------------------------------------------------------
// Parse fixtures
// ---------------------------------------------------------------------------

// TestConformance_ParseFixtures walks docs/conformance/v1/parse/fixtures/ and
// verifies that `pmk parse --json` produces the expected tree and diagnostics
// for every fixture directory.
func TestConformance_ParseFixtures(t *testing.T) {
	fixturesDir := filepath.Join(conformanceRoot, "parse", "fixtures")
	entries, err := os.ReadDir(fixturesDir)
	if err != nil {
		t.Fatalf("os.ReadDir(%s): %v", fixturesDir, err)
	}

	ran := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		fixturePath := filepath.Join(fixturesDir, name)
		t.Run(name, func(t *testing.T) {
			runParseFixture(t, fixturePath)
		})
		ran++
	}
	if ran == 0 {
		t.Fatal("no parse fixtures found")
	}
}

// runParseFixture runs a single parse fixture against the pmk binary.
// It copies binder.md to a temp directory as _binder.md (the CLI requires
// this filename), invokes pmk parse --json, then asserts the JSON output
// matches expected-parse.json and expected-diagnostics.json.
func runParseFixture(t *testing.T, fixturePath string) {
	t.Helper()

	skipIfMissingFiles(t, fixturePath, []string{"binder.md", "expected-parse.json", "expected-diagnostics.json"})

	// Set up temp working directory with _binder.md.
	tmpDir, err := os.MkdirTemp("", "pmk-parse-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	binderPath := filepath.Join(tmpDir, "_binder.md")
	if err := copyFile(filepath.Join(fixturePath, "binder.md"), binderPath); err != nil {
		t.Fatalf("copyFile binder.md: %v", err)
	}
	copyFixtureStubs(t, fixturePath, tmpDir, "binder.md")

	// Invoke pmk parse --json. Non-zero exit is acceptable for error fixtures;
	// stdout still contains JSON diagnostics per the runner contract.
	cmd := exec.Command(pmkBinary, "parse", "--json", binderPath)
	stdout, runErr := cmd.Output()
	var exitErr *exec.ExitError
	if runErr != nil && !errors.As(runErr, &exitErr) {
		t.Fatalf("pmk parse: %v", runErr)
	}

	var actual parseJSONOutput
	if err := json.Unmarshal(stdout, &actual); err != nil {
		t.Fatalf("unmarshal pmk parse stdout: %v\nstdout: %s", err, stdout)
	}

	// Assert parse tree matches expected-parse.json (subset check).
	expectedParseRaw, err := os.ReadFile(filepath.Join(fixturePath, "expected-parse.json"))
	if err != nil {
		t.Fatalf("read expected-parse.json: %v", err)
	}
	actualParseRaw, err := json.Marshal(map[string]interface{}{
		"version": actual.Version,
		"root":    actual.Root,
	})
	if err != nil {
		t.Fatalf("marshal actual parse output: %v", err)
	}
	checkJSONSubset(t, "parse-tree", expectedParseRaw, actualParseRaw)

	// Assert diagnostics match expected-diagnostics.json (subset check).
	expectedDiags := readExpectedDiagnostics(t, filepath.Join(fixturePath, "expected-diagnostics.json"))
	checkDiagnosticsSubset(t, expectedDiags, actual.Diagnostics)
}

// ---------------------------------------------------------------------------
// Ops fixtures
// ---------------------------------------------------------------------------

// TestConformance_OpsFixtures walks docs/conformance/v1/ops/fixtures/*/ and
// verifies each leaf fixture directory against the pmk binary.
func TestConformance_OpsFixtures(t *testing.T) {
	walkOpsFixtures(t, runOpsFixture)
}

// walkOpsFixtures walks docs/conformance/v1/ops/fixtures/ and calls fn for
// each leaf fixture directory. It fatals if no fixture directories are found.
func walkOpsFixtures(t *testing.T, fn func(t *testing.T, fixturePath string)) {
	t.Helper()
	opsFixturesDir := filepath.Join(conformanceRoot, "ops", "fixtures")
	opDirs, err := os.ReadDir(opsFixturesDir)
	if err != nil {
		t.Fatalf("os.ReadDir(%s): %v", opsFixturesDir, err)
	}
	ran := 0
	for _, opDir := range opDirs {
		if !opDir.IsDir() {
			continue
		}
		opName := opDir.Name()
		opPath := filepath.Join(opsFixturesDir, opName)
		fixtures, err := os.ReadDir(opPath)
		if err != nil {
			t.Fatalf("os.ReadDir(%s): %v", opPath, err)
		}
		for _, fixture := range fixtures {
			if !fixture.IsDir() {
				continue
			}
			fixtureName := fixture.Name()
			fixturePath := filepath.Join(opPath, fixtureName)
			name := opName + "/" + fixtureName
			t.Run(name, func(t *testing.T) {
				fn(t, fixturePath)
			})
			ran++
		}
	}
	if ran == 0 {
		t.Fatal("no ops fixtures found")
	}
}

// runOpsFixture runs a single ops fixture.
//
// If op.json is present, it invokes `pmk <operation> --json` with the
// parameters specified in op.json and asserts the binder output and
// diagnostics match the expected files.
//
// If op.json is absent (stability fixture), it verifies byte-identity between
// input-binder.md and expected-binder.md, and checks that `pmk parse --json`
// does not modify the binder file (runner-contract §3, Step 3).
func runOpsFixture(t *testing.T, fixturePath string) {
	t.Helper()

	skipIfMissingFiles(t, fixturePath, []string{"input-binder.md", "expected-diagnostics.json"})

	inputBinderBytes, err := os.ReadFile(filepath.Join(fixturePath, "input-binder.md"))
	if err != nil {
		t.Fatalf("read input-binder.md: %v", err)
	}

	// Set up temp working directory.
	tmpDir, err := os.MkdirTemp("", "pmk-ops-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	binderPath := filepath.Join(tmpDir, "_binder.md")
	if err := os.WriteFile(binderPath, inputBinderBytes, 0600); err != nil {
		t.Fatalf("write _binder.md: %v", err)
	}
	copyFixtureStubs(t, fixturePath, tmpDir, "input-binder.md", "expected-binder.md")

	// Dispatch to mutation or stability runner.
	opJSONPath := filepath.Join(fixturePath, "op.json")
	if _, err := os.Stat(opJSONPath); os.IsNotExist(err) {
		runStabilityFixture(t, fixturePath, binderPath, inputBinderBytes)
		return
	}

	// Mutation fixture: parse op.json and invoke pmk <operation>.
	opRaw, err := os.ReadFile(opJSONPath)
	if err != nil {
		t.Fatalf("read op.json: %v", err)
	}
	var spec opSpec
	if err := json.Unmarshal(opRaw, &spec); err != nil {
		t.Fatalf("parse op.json: %v", err)
	}

	opArgs, err := buildOpArgs(spec)
	if err != nil {
		t.Fatalf("buildOpArgs: %v", err)
	}

	cmdArgs := append([]string{spec.Operation, "--json", binderPath}, opArgs...)
	cmd := exec.Command(pmkBinary, cmdArgs...)
	stdout, runErr := cmd.Output()
	var exitErr *exec.ExitError
	isErrorExit := runErr != nil && errors.As(runErr, &exitErr)
	if runErr != nil && !isErrorExit {
		t.Fatalf("pmk %s: %v", spec.Operation, runErr)
	}

	var actual opsJSONOutput
	if len(stdout) > 0 {
		if err := json.Unmarshal(stdout, &actual); err != nil {
			t.Fatalf("unmarshal pmk %s stdout: %v\nstdout: %s", spec.Operation, err, stdout)
		}
	}

	// Check diagnostics (subset check, §4.2).
	expectedDiags := readExpectedDiagnostics(t, filepath.Join(fixturePath, "expected-diagnostics.json"))
	checkDiagnosticsSubset(t, expectedDiags, actual.Diagnostics)

	// Determine whether this is an error scenario (OPExx expected, §4.6).
	expectsError := false
	for _, d := range expectedDiags {
		if d.Severity == "error" {
			expectsError = true
			break
		}
	}

	// Check exit code (§4.6).
	if expectsError && !isErrorExit {
		t.Errorf("expected non-zero exit (OPExx error), got exit 0")
	}
	if !expectsError && isErrorExit {
		t.Errorf("unexpected non-zero exit: %v", runErr)
	}

	// Read the binder file after the command ran.
	actualBinderBytes, err := os.ReadFile(binderPath)
	if err != nil {
		t.Fatalf("read _binder.md after operation: %v", err)
	}

	if expectsError {
		// Abort check (§4.6): file must be byte-for-byte unchanged.
		if !bytes.Equal(inputBinderBytes, actualBinderBytes) {
			t.Errorf("error scenario: binder file was modified (must be unchanged on OPExx)")
		}
		return
	}

	// Success scenario: mutation output check (§4.4) or no-change check (§4.5).
	expectedBinderPath := filepath.Join(fixturePath, "expected-binder.md")
	if _, err := os.Stat(expectedBinderPath); err == nil {
		expectedBinderBytes, err := os.ReadFile(expectedBinderPath)
		if err != nil {
			t.Fatalf("read expected-binder.md: %v", err)
		}
		if !bytes.Equal(expectedBinderBytes, actualBinderBytes) {
			t.Errorf("binder output mismatch:\n  expected (%d bytes): %q\n  actual   (%d bytes): %q",
				len(expectedBinderBytes), expectedBinderBytes,
				len(actualBinderBytes), actualBinderBytes)
		}
	} else {
		// No expected-binder.md: file must be unchanged (§4.5).
		if !bytes.Equal(inputBinderBytes, actualBinderBytes) {
			t.Errorf("no-change scenario: binder file was modified")
		}
	}
}

// runStabilityFixture handles ops fixtures without op.json (round-trip tests).
//
// It verifies that expected-binder.md is byte-for-byte identical to
// input-binder.md (the fixture proves the serializer produces the same bytes),
// and checks that pmk parse --json does not modify the binder file.
func runStabilityFixture(t *testing.T, fixturePath, binderPath string, inputBinderBytes []byte) {
	t.Helper()

	// Byte-identity check: expected-binder.md must equal input-binder.md.
	expectedBinderPath := filepath.Join(fixturePath, "expected-binder.md")
	if _, err := os.Stat(expectedBinderPath); err == nil {
		expectedBinderBytes, err := os.ReadFile(expectedBinderPath)
		if err != nil {
			t.Fatalf("read expected-binder.md: %v", err)
		}
		if !bytes.Equal(inputBinderBytes, expectedBinderBytes) {
			t.Errorf("stability: input-binder.md and expected-binder.md differ")
		}
	}

	// Invoke pmk parse --json to verify diagnostics and that parse does not
	// mutate the binder file.
	cmd := exec.Command(pmkBinary, "parse", "--json", binderPath)
	stdout, runErr := cmd.Output()
	var exitErr *exec.ExitError
	if runErr != nil && !errors.As(runErr, &exitErr) {
		t.Fatalf("pmk parse (stability): %v", runErr)
	}

	var actual parseJSONOutput
	if err := json.Unmarshal(stdout, &actual); err != nil {
		t.Fatalf("unmarshal pmk parse stdout: %v\nstdout: %s", err, stdout)
	}

	expectedDiags := readExpectedDiagnostics(t, filepath.Join(fixturePath, "expected-diagnostics.json"))
	checkDiagnosticsSubset(t, expectedDiags, actual.Diagnostics)

	// Parse must not modify the binder file.
	afterParseBytes, err := os.ReadFile(binderPath)
	if err != nil {
		t.Fatalf("read _binder.md after parse: %v", err)
	}
	if !bytes.Equal(inputBinderBytes, afterParseBytes) {
		t.Errorf("stability: pmk parse modified the binder file")
	}
}

// ---------------------------------------------------------------------------
// op.json type definitions and CLI argument builders
// ---------------------------------------------------------------------------

// opSpec is the top-level structure of op.json.
type opSpec struct {
	Version   string          `json:"version"`
	Operation string          `json:"operation"`
	Params    json.RawMessage `json:"params"`
}

type addChildParamsJSON struct {
	ParentSelector   string `json:"parentSelector"`
	Target           string `json:"target"`
	Title            string `json:"title"`
	Position         string `json:"position"`
	At               *int   `json:"at"`
	PositionIndex    *int   `json:"positionIndex"`
	Before           string `json:"before"`
	After            string `json:"after"`
	PositionSelector string `json:"positionSelector"`
	Force            bool   `json:"force"`
}

type deleteParamsJSON struct {
	Selector string `json:"selector"`
	Yes      bool   `json:"yes"`
}

type moveParamsJSON struct {
	SourceSelector            string `json:"sourceSelector"`
	DestinationParentSelector string `json:"destinationParentSelector"`
	Position                  string `json:"position"`
	At                        *int   `json:"at"`
	Before                    string `json:"before"`
	After                     string `json:"after"`
	Yes                       bool   `json:"yes"`
}

// buildOpArgs converts an opSpec into CLI flag strings for the named operation.
func buildOpArgs(spec opSpec) ([]string, error) {
	switch spec.Operation {
	case "add-child":
		var p addChildParamsJSON
		if err := json.Unmarshal(spec.Params, &p); err != nil {
			return nil, fmt.Errorf("parse add-child params: %w", err)
		}
		return buildAddChildArgs(p), nil
	case "delete":
		var p deleteParamsJSON
		if err := json.Unmarshal(spec.Params, &p); err != nil {
			return nil, fmt.Errorf("parse delete params: %w", err)
		}
		return buildDeleteArgs(p), nil
	case "move":
		var p moveParamsJSON
		if err := json.Unmarshal(spec.Params, &p); err != nil {
			return nil, fmt.Errorf("parse move params: %w", err)
		}
		return buildMoveArgs(p), nil
	default:
		return nil, fmt.Errorf("unknown operation %q", spec.Operation)
	}
}

func buildAddChildArgs(p addChildParamsJSON) []string {
	args := []string{"--parent", p.ParentSelector, "--target", p.Target}
	if p.Title != "" {
		args = append(args, "--title", p.Title)
	}
	if p.Position == "first" {
		args = append(args, "--first")
	}
	// Support both "at" and "positionIndex" field names.
	atVal := p.At
	if atVal == nil {
		atVal = p.PositionIndex
	}
	if atVal != nil {
		args = append(args, "--at", strconv.Itoa(*atVal))
	}
	// Support both "before"/"after" and "positionSelector" + "position" field names.
	before := p.Before
	after := p.After
	if before == "" && after == "" && p.PositionSelector != "" {
		switch p.Position {
		case "before":
			before = p.PositionSelector
		case "after":
			after = p.PositionSelector
		}
	}
	if before != "" {
		args = append(args, "--before", before)
	}
	if after != "" {
		args = append(args, "--after", after)
	}
	if p.Force {
		args = append(args, "--force")
	}
	return args
}

func buildDeleteArgs(p deleteParamsJSON) []string {
	args := []string{"--selector", p.Selector}
	if p.Yes {
		args = append(args, "--yes")
	}
	return args
}

func buildMoveArgs(p moveParamsJSON) []string {
	args := []string{"--source", p.SourceSelector, "--dest", p.DestinationParentSelector}
	if p.Position == "first" {
		args = append(args, "--first")
	}
	if p.At != nil {
		args = append(args, "--at", strconv.Itoa(*p.At))
	}
	if p.Before != "" {
		args = append(args, "--before", p.Before)
	}
	if p.After != "" {
		args = append(args, "--after", p.After)
	}
	if p.Yes {
		args = append(args, "--yes")
	}
	return args
}

// ---------------------------------------------------------------------------
// JSON output type definitions
// ---------------------------------------------------------------------------

// parseJSONOutput is the shape emitted by `pmk parse --json`.
type parseJSONOutput struct {
	Version     string           `json:"version"`
	Root        json.RawMessage  `json:"root"`
	Diagnostics []diagnosticItem `json:"diagnostics"`
}

// opsJSONOutput is the shape emitted by `pmk <op> --json`.
type opsJSONOutput struct {
	Version     string           `json:"version"`
	Changed     bool             `json:"changed"`
	Diagnostics []diagnosticItem `json:"diagnostics"`
}

type diagnosticItem struct {
	Severity string        `json:"severity"`
	Code     string        `json:"code"`
	Message  string        `json:"message,omitempty"`
	Location *locationItem `json:"location,omitempty"`
}

type locationItem struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

type expectedDiagnosticsFile struct {
	Version     string           `json:"version"`
	Diagnostics []diagnosticItem `json:"diagnostics"`
}

// readExpectedDiagnostics reads and parses an expected-diagnostics.json file.
func readExpectedDiagnostics(t *testing.T, path string) []diagnosticItem {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read expected-diagnostics.json: %v", err)
	}
	var f expectedDiagnosticsFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("parse expected-diagnostics.json: %v", err)
	}
	return f.Diagnostics
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// skipIfMissingFiles skips the test if any of the named files are absent from dir.
func skipIfMissingFiles(t *testing.T, dir string, files []string) {
	t.Helper()
	for _, f := range files {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Skipf("required file %q missing; skipping", f)
		}
	}
}

// ---------------------------------------------------------------------------
// Assertion helpers
// ---------------------------------------------------------------------------

// checkDiagnosticsSubset implements the diagnostic subset check from
// runner-contract §4.2:
//   - every expected {severity, code} pair must appear in actual
//   - unexpected error-severity diagnostics cause failure
//   - unexpected warning-severity diagnostics are permitted
func checkDiagnosticsSubset(t *testing.T, expected, actual []diagnosticItem) {
	t.Helper()

	for _, exp := range expected {
		if !findDiagnostic(exp, actual) {
			t.Errorf("expected diagnostic {severity: %q, code: %q} not found in actual",
				exp.Severity, exp.Code)
		}
	}
	for _, act := range actual {
		if act.Severity == "error" && !findDiagnosticByCode(act, expected) {
			t.Errorf("unexpected error diagnostic {code: %q, message: %q}",
				act.Code, act.Message)
		}
	}
}

// findDiagnostic reports whether diag appears in list, matching severity and
// code. When diag has a Location, the matching item must also match line and
// column.
func findDiagnostic(diag diagnosticItem, list []diagnosticItem) bool {
	for _, item := range list {
		if item.Severity != diag.Severity || item.Code != diag.Code {
			continue
		}
		if diag.Location != nil {
			if item.Location == nil {
				continue
			}
			if item.Location.Line != diag.Location.Line || item.Location.Column != diag.Location.Column {
				continue
			}
		}
		return true
	}
	return false
}

// findDiagnosticByCode reports whether diag appears in list matching only severity
// and code (ignoring location). Used when checking that actual errors are expected.
func findDiagnosticByCode(diag diagnosticItem, list []diagnosticItem) bool {
	for _, item := range list {
		if item.Severity == diag.Severity && item.Code == diag.Code {
			return true
		}
	}
	return false
}

// checkJSONSubset asserts that every key-value pair in expectedJSON also
// appears in actualJSON (recursive subset, per runner-contract §4.1).
func checkJSONSubset(t *testing.T, label string, expectedJSON, actualJSON []byte) {
	t.Helper()
	var expected, actual interface{}
	if err := json.Unmarshal(expectedJSON, &expected); err != nil {
		t.Errorf("%s: unmarshal expected JSON: %v", label, err)
		return
	}
	if err := json.Unmarshal(actualJSON, &actual); err != nil {
		t.Errorf("%s: unmarshal actual JSON: %v", label, err)
		return
	}
	jsonSubsetEqual(t, label, expected, actual)
}

// jsonSubsetEqual recursively checks that expected is a subset of actual.
func jsonSubsetEqual(t *testing.T, path string, expected, actual interface{}) bool {
	t.Helper()
	switch e := expected.(type) {
	case map[string]interface{}:
		a, ok := actual.(map[string]interface{})
		if !ok {
			t.Errorf("%s: expected JSON object, got %T (%v)", path, actual, actual)
			return false
		}
		allOK := true
		for k, ev := range e {
			av, exists := a[k]
			if !exists {
				t.Errorf("%s.%s: key missing in actual", path, k)
				allOK = false
				continue
			}
			if !jsonSubsetEqual(t, path+"."+k, ev, av) {
				allOK = false
			}
		}
		return allOK
	case []interface{}:
		a, ok := actual.([]interface{})
		if !ok {
			t.Errorf("%s: expected JSON array, got %T (%v)", path, actual, actual)
			return false
		}
		if len(e) != len(a) {
			t.Errorf("%s: array length: expected %d, got %d", path, len(e), len(a))
			return false
		}
		allOK := true
		for i := range e {
			if !jsonSubsetEqual(t, fmt.Sprintf("%s[%d]", path, i), e[i], a[i]) {
				allOK = false
			}
		}
		return allOK
	case nil:
		if actual != nil {
			t.Errorf("%s: expected null, got %v", path, actual)
			return false
		}
		return true
	default:
		if expected != actual {
			t.Errorf("%s: expected %v (%T), got %v (%T)", path, expected, expected, actual, actual)
			return false
		}
		return true
	}
}

// ---------------------------------------------------------------------------
// File utilities
// ---------------------------------------------------------------------------

// copyFile copies the file at src to dst.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0600)
}

// copyFixtureStubs walks fixtureDir recursively and creates zero-byte stub
// files in tmpDir for every .md file found, preserving relative paths, except
// those whose base name appears in skip. This populates the temp working
// directory with stub files that ScanProjectImpl will discover when resolving
// wikilinks.
func copyFixtureStubs(t *testing.T, fixtureDir, tmpDir string, skip ...string) {
	t.Helper()
	skipSet := make(map[string]bool, len(skip))
	for _, s := range skip {
		skipSet[s] = true
	}
	err := filepath.WalkDir(fixtureDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}
		rel, relErr := filepath.Rel(fixtureDir, path)
		if relErr != nil {
			return relErr
		}
		if skipSet[filepath.Base(rel)] {
			return nil
		}
		dst := filepath.Join(tmpDir, rel)
		if mkErr := os.MkdirAll(filepath.Dir(dst), 0o755); mkErr != nil {
			return mkErr
		}
		return os.WriteFile(dst, nil, 0600)
	})
	if err != nil {
		t.Fatalf("copyFixtureStubs WalkDir %s: %v", fixtureDir, err)
	}
}

// ---------------------------------------------------------------------------
// Justfile integration checks (Phase G / runner-contract §6)
// ---------------------------------------------------------------------------

// readJustfile reads ../justfile and fatals the test if the file cannot be read.
func readJustfile(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("../justfile")
	if err != nil {
		t.Fatalf("read justfile: %v", err)
	}
	return data
}

// TestConformance_JustfileConformanceRunHasTimeout verifies that the
// conformance-run recipe includes -timeout=120s, as required by the
// acceptance criteria for Phase G integration (task prosemark-go-48x.6.5.2).
func TestConformance_JustfileConformanceRunHasTimeout(t *testing.T) {
	data := readJustfile(t)
	// Find the conformance-run recipe body and verify it includes -timeout=120s.
	lines := strings.Split(string(data), "\n")
	inConformanceRun := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "conformance-run:") {
			inConformanceRun = true
			continue
		}
		if inConformanceRun {
			// Recipe body lines are indented; a non-indented line ends the recipe.
			if len(line) > 0 && line[0] != ' ' && line[0] != '\t' {
				break
			}
			if strings.Contains(trimmed, "go test") {
				if !strings.Contains(trimmed, "-timeout=120s") {
					t.Errorf("conformance-run recipe 'go test' invocation is missing -timeout=120s flag: %q", trimmed)
				}
				return
			}
		}
	}
	if !inConformanceRun {
		t.Error("justfile: no conformance-run recipe found")
	}
}

// TestConformance_JustfileConformanceRunDependsOnBuild verifies that the
// conformance-run recipe declares build as a dependency, so that bin/pmk is
// built before the conformance tests run (Phase G acceptance criteria).
func TestConformance_JustfileConformanceRunDependsOnBuild(t *testing.T) {
	data := readJustfile(t)
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "conformance-run:") {
			// The dependency list follows the colon on the same line.
			deps := strings.TrimPrefix(trimmed, "conformance-run:")
			if !strings.Contains(deps, "build") {
				t.Errorf("conformance-run recipe does not depend on build: %q (want 'conformance-run: build ...')", trimmed)
			}
			return
		}
	}
	t.Error("justfile: no conformance-run recipe found")
}

// TestConformance_JustfileConformanceRunTarget verifies that the justfile
// contains the conformance-run recipe required by runner-contract.md §6
// and plan.md Phase-G.
func TestConformance_JustfileConformanceRunTarget(t *testing.T) {
	data := readJustfile(t)
	if !bytes.Contains(data, []byte("conformance-run:")) {
		t.Error("justfile is missing 'conformance-run:' target (required by Phase G / runner-contract §6)")
	}
}

// TestConformance_JustfileTestAllIncludesConformance verifies that the
// test-all recipe includes conformance-run, as required by plan.md Phase-G.
func TestConformance_JustfileTestAllIncludesConformance(t *testing.T) {
	data := readJustfile(t)
	for i, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "test-all:") {
			if !strings.Contains(line, "conformance-run") {
				t.Errorf("justfile test-all (line %d) does not include conformance-run: %q", i+1, line)
			}
			return
		}
	}
	t.Error("justfile: no test-all recipe found")
}

// ---------------------------------------------------------------------------
// Ops JSON output contract validation (runner-contract §3.5)
// ---------------------------------------------------------------------------

// TestConformance_OpsChangedField validates the `changed` field in ops JSON
// output per runner-contract §3.5: changed must be true when the binder bytes
// were modified and false when unchanged or aborted with an error.
func TestConformance_OpsChangedField(t *testing.T) {
	walkOpsFixtures(t, validateChangedField)
}

// validateChangedField runs a single ops fixture and asserts the `changed`
// field in the JSON output correctly reflects whether the binder was mutated.
func validateChangedField(t *testing.T, fixturePath string) {
	t.Helper()

	// Stability fixtures (no op.json) do not emit ops JSON; skip them here.
	opJSONPath := filepath.Join(fixturePath, "op.json")
	if _, err := os.Stat(opJSONPath); os.IsNotExist(err) {
		t.Skip("stability fixture (no op.json); changed field not applicable")
		return
	}

	skipIfMissingFiles(t, fixturePath, []string{"input-binder.md", "expected-diagnostics.json"})

	inputBinderBytes, err := os.ReadFile(filepath.Join(fixturePath, "input-binder.md"))
	if err != nil {
		t.Fatalf("read input-binder.md: %v", err)
	}

	tmpDir, err := os.MkdirTemp("", "pmk-ops-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	binderPath := filepath.Join(tmpDir, "_binder.md")
	if err := os.WriteFile(binderPath, inputBinderBytes, 0600); err != nil {
		t.Fatalf("write _binder.md: %v", err)
	}
	copyFixtureStubs(t, fixturePath, tmpDir, "input-binder.md", "expected-binder.md")

	opRaw, err := os.ReadFile(opJSONPath)
	if err != nil {
		t.Fatalf("read op.json: %v", err)
	}
	var spec opSpec
	if err := json.Unmarshal(opRaw, &spec); err != nil {
		t.Fatalf("parse op.json: %v", err)
	}

	opArgs, err := buildOpArgs(spec)
	if err != nil {
		t.Fatalf("buildOpArgs: %v", err)
	}

	cmdArgs := append([]string{spec.Operation, "--json", binderPath}, opArgs...)
	cmd := exec.Command(pmkBinary, cmdArgs...)
	stdout, runErr := cmd.Output()
	var exitErr *exec.ExitError
	isErrorExit := runErr != nil && errors.As(runErr, &exitErr)
	if runErr != nil && !isErrorExit {
		t.Fatalf("pmk %s: %v", spec.Operation, runErr)
	}

	if len(stdout) == 0 {
		t.Errorf("ops command produced no stdout; expected JSON output per runner-contract §3.5")
		return
	}

	var actual opsJSONOutput
	if err := json.Unmarshal(stdout, &actual); err != nil {
		t.Fatalf("unmarshal pmk %s stdout: %v\nstdout: %s", spec.Operation, err, stdout)
	}

	// Verify version field (runner-contract §3.5).
	if actual.Version != "1" {
		t.Errorf("ops output version must be %q, got %q", "1", actual.Version)
	}

	// Determine whether binder bytes actually changed after the operation.
	actualBinderBytes, err := os.ReadFile(binderPath)
	if err != nil {
		t.Fatalf("read _binder.md after operation: %v", err)
	}
	binderWasModified := !bytes.Equal(inputBinderBytes, actualBinderBytes)

	// changed field MUST match whether binder bytes were actually modified
	// (runner-contract §3.5: "The `changed` field indicates whether the binder
	// bytes were modified").
	if actual.Changed != binderWasModified {
		t.Errorf("changed field mismatch: JSON reports changed=%v but binder bytes changed=%v",
			actual.Changed, binderWasModified)
	}

	// Abort check (runner-contract §4.6): error scenarios must not modify binder.
	if isErrorExit && actual.Changed {
		t.Errorf("error scenario (non-zero exit): changed must be false (abort check), got true")
	}
}
