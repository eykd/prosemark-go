package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── Test double ────────────────────────────────────────────────────────────

// mockDoctorIO is a test double for DoctorIO.
type mockDoctorIO struct {
	binderBytes  []byte
	binderErr    error
	uuidFiles    []string
	uuidFilesErr error
	// nodeFiles maps filepath.Base(path) → nodeFileEntry; missing key returns (nil, false, nil).
	nodeFiles     map[string]nodeFileEntry
	readFileCalls []string // tracks every ReadNodeFile call path
}

// nodeFileEntry holds the mock response for a single node file.
type nodeFileEntry struct {
	content []byte
	exists  bool
	err     error
}

func (m *mockDoctorIO) ReadBinder(path string) ([]byte, error) {
	return m.binderBytes, m.binderErr
}

func (m *mockDoctorIO) ListUUIDFiles(dir string) ([]string, error) {
	return m.uuidFiles, m.uuidFilesErr
}

func (m *mockDoctorIO) ReadNodeFile(path string) ([]byte, bool, error) {
	m.readFileCalls = append(m.readFileCalls, path)
	if m.nodeFiles != nil {
		base := filepath.Base(path)
		if entry, ok := m.nodeFiles[base]; ok {
			return entry.content, entry.exists, entry.err
		}
	}
	return nil, false, nil
}

// ─── Test fixtures ──────────────────────────────────────────────────────────

const (
	doctorTestNodeUUID  = "01234567-89ab-7def-0123-456789abcdef"
	doctorTestNodeUUID2 = "01234567-89ab-7def-0123-111111111111"
)

func doctorBinderWithNode(uuid string) []byte {
	return []byte("<!-- prosemark-binder:v1 -->\n- [Node](" + uuid + ".md)\n")
}

func doctorBinderEmpty() []byte {
	return []byte("<!-- prosemark-binder:v1 -->\n")
}

func validDoctorNodeContent(uuid string) []byte {
	return []byte(
		"---\n" +
			"id: " + uuid + "\n" +
			"title: Test Node\n" +
			"created: 2026-01-01T00:00:00Z\n" +
			"updated: 2026-01-01T00:00:00Z\n" +
			"---\n" +
			"Body text here.\n",
	)
}

func emptyBodyDoctorNodeContent(uuid string) []byte {
	return []byte(
		"---\n" +
			"id: " + uuid + "\n" +
			"title: Test Node\n" +
			"created: 2026-01-01T00:00:00Z\n" +
			"updated: 2026-01-01T00:00:00Z\n" +
			"---\n",
	)
}

func mismatchedIDDoctorNodeContent() []byte {
	// Frontmatter id does not match the UUID filename stem.
	return []byte(
		"---\n" +
			"id: wrong-uuid-value\n" +
			"title: Test Node\n" +
			"created: 2026-01-01T00:00:00Z\n" +
			"updated: 2026-01-01T00:00:00Z\n" +
			"---\n" +
			"Body text here.\n",
	)
}

func missingFieldDoctorNodeContent(uuid string) []byte {
	// Missing 'updated' field — triggers AUD005.
	return []byte(
		"---\n" +
			"id: " + uuid + "\n" +
			"title: Test Node\n" +
			"created: 2026-01-01T00:00:00Z\n" +
			"---\n" +
			"Body text here.\n",
	)
}

func invalidYAMLDoctorNodeContent() []byte {
	// Syntactically invalid YAML — triggers AUD007.
	return []byte(
		"---\n" +
			"id: [this: is invalid yaml\n" +
			"---\n" +
			"Body.\n",
	)
}

// ─── Flag / structure tests ──────────────────────────────────────────────────

func TestNewDoctorCmd_HasRequiredFlags(t *testing.T) {
	c := NewDoctorCmd(nil)
	for _, name := range []string{"project", "json"} {
		t.Run(name, func(t *testing.T) {
			if c.Flags().Lookup(name) == nil {
				t.Errorf("expected --%s flag on doctor command", name)
			}
		})
	}
}

func TestNewDoctorCmd_DefaultsToCWD(t *testing.T) {
	mock := &mockDoctorIO{binderBytes: doctorBinderEmpty()}
	c := NewDoctorCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{})
	// CWD may not have a valid binder — the important check is no panic.
	_ = c.Execute()
}

func TestNewDoctorCmd_GetCWDError(t *testing.T) {
	mock := &mockDoctorIO{binderBytes: doctorBinderEmpty()}
	c := newDoctorCmdWithGetCWD(mock, func() (string, error) {
		return "", errors.New("getwd failed")
	})
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Error("expected error when getwd fails")
	}
}

// ─── US4 scenario table ──────────────────────────────────────────────────────

func TestNewDoctorCmd_Scenarios(t *testing.T) {
	tests := []struct {
		name string
		args []string

		// IO mock state
		binderBytes []byte
		binderErr   error
		uuidFiles   []string
		nodeFiles   map[string]nodeFileEntry

		// Expected outcomes
		wantErr   bool   // command returns error (exit 1)
		wantInOut string // substring expected in stdout
		wantInErr string // substring expected in stderr or err.Error()
	}{
		{
			// US4/1: clean project → no diagnostics, exit 0.
			name:        "US4/1 clean project: no diagnostics, exit 0",
			args:        []string{"--project", "."},
			binderBytes: doctorBinderWithNode(doctorTestNodeUUID),
			nodeFiles: map[string]nodeFileEntry{
				doctorTestNodeUUID + ".md": {content: validDoctorNodeContent(doctorTestNodeUUID), exists: true},
				".prosemark.yml":           {content: []byte("version: \"1\"\n"), exists: true},
			},
			wantErr: false,
		},
		{
			// US4/2: AUD001 missing referenced file → exit 1.
			name:        "US4/2 AUD001: missing referenced file",
			args:        []string{"--project", "."},
			binderBytes: doctorBinderWithNode(doctorTestNodeUUID),
			nodeFiles: map[string]nodeFileEntry{
				doctorTestNodeUUID + ".md": {content: nil, exists: false},
			},
			wantErr:   true,
			wantInErr: "AUD001",
		},
		{
			// US4/3: AUD002 orphaned UUID file (warning) → exit 0.
			// The non-UUID filename not in uuidFiles produces no AUD002.
			name:        "US4/3 AUD002: orphaned UUID file, warning only",
			args:        []string{"--project", "."},
			binderBytes: doctorBinderEmpty(),
			uuidFiles:   []string{doctorTestNodeUUID + ".md"},
			nodeFiles: map[string]nodeFileEntry{
				".prosemark.yml": {content: []byte("version: \"1\"\n"), exists: true},
			},
			wantErr:   false,
			wantInErr: "AUD002",
		},
		{
			// US4/4: AUD003 duplicate binder reference → exit 1.
			name: "US4/4 AUD003: duplicate binder reference",
			args: []string{"--project", "."},
			binderBytes: []byte(
				"<!-- prosemark-binder:v1 -->\n" +
					"- [A](" + doctorTestNodeUUID + ".md)\n" +
					"- [B](" + doctorTestNodeUUID + ".md)\n",
			),
			nodeFiles: map[string]nodeFileEntry{
				doctorTestNodeUUID + ".md": {content: validDoctorNodeContent(doctorTestNodeUUID), exists: true},
			},
			wantErr:   true,
			wantInErr: "AUD003",
		},
		{
			// US4/5: AUD004 id/filename mismatch → exit 1.
			name:        "US4/5 AUD004: id/filename mismatch",
			args:        []string{"--project", "."},
			binderBytes: doctorBinderWithNode(doctorTestNodeUUID),
			nodeFiles: map[string]nodeFileEntry{
				doctorTestNodeUUID + ".md": {content: mismatchedIDDoctorNodeContent(), exists: true},
			},
			wantErr:   true,
			wantInErr: "AUD004",
		},
		{
			// US4/6: AUD005 missing required field → exit 1.
			name:        "US4/6 AUD005: missing required field",
			args:        []string{"--project", "."},
			binderBytes: doctorBinderWithNode(doctorTestNodeUUID),
			nodeFiles: map[string]nodeFileEntry{
				doctorTestNodeUUID + ".md": {content: missingFieldDoctorNodeContent(doctorTestNodeUUID), exists: true},
			},
			wantErr:   true,
			wantInErr: "AUD005",
		},
		{
			// US4/7: AUD006 empty body (warning) → exit 0.
			name:        "US4/7 AUD006: empty body, warning only",
			args:        []string{"--project", "."},
			binderBytes: doctorBinderWithNode(doctorTestNodeUUID),
			nodeFiles: map[string]nodeFileEntry{
				doctorTestNodeUUID + ".md": {content: emptyBodyDoctorNodeContent(doctorTestNodeUUID), exists: true},
				".prosemark.yml":           {content: []byte("version: \"1\"\n"), exists: true},
			},
			wantErr:   false,
			wantInErr: "AUD006",
		},
		{
			// US4/8: --json output produces structured diagnostic list with code, message, path.
			name:        "US4/8 --json output: structured diagnostic list",
			args:        []string{"--project", ".", "--json"},
			binderBytes: doctorBinderWithNode(doctorTestNodeUUID),
			nodeFiles: map[string]nodeFileEntry{
				doctorTestNodeUUID + ".md": {content: nil, exists: false},
			},
			wantErr:   true,
			wantInOut: "AUD001", // AUD001 code appears in JSON output
		},
		{
			// US4/9: only AUD006 warnings → exit 0.
			name: "US4/9 only AUD006 warnings: exit 0",
			args: []string{"--project", "."},
			binderBytes: []byte(
				"<!-- prosemark-binder:v1 -->\n" +
					"- [A](" + doctorTestNodeUUID + ".md)\n" +
					"- [B](" + doctorTestNodeUUID2 + ".md)\n",
			),
			nodeFiles: map[string]nodeFileEntry{
				doctorTestNodeUUID + ".md":  {content: emptyBodyDoctorNodeContent(doctorTestNodeUUID), exists: true},
				doctorTestNodeUUID2 + ".md": {content: emptyBodyDoctorNodeContent(doctorTestNodeUUID2), exists: true},
				".prosemark.yml":            {content: []byte("version: \"1\"\n"), exists: true},
			},
			wantErr:   false,
			wantInErr: "AUD006",
		},
		{
			// US4/10: only AUD002 warnings → exit 0.
			name:        "US4/10 only AUD002 warnings: exit 0",
			args:        []string{"--project", "."},
			binderBytes: doctorBinderEmpty(),
			uuidFiles:   []string{doctorTestNodeUUID + ".md", doctorTestNodeUUID2 + ".md"},
			nodeFiles: map[string]nodeFileEntry{
				".prosemark.yml": {content: []byte("version: \"1\"\n"), exists: true},
			},
			wantErr:   false,
			wantInErr: "AUD002",
		},
		{
			// US4/11: AUDW001 non-UUID filename in binder (warning) → exit 0.
			name: "US4/11 AUDW001: non-UUID in binder, warning only",
			args: []string{"--project", "."},
			binderBytes: []byte(
				"<!-- prosemark-binder:v1 -->\n" +
					"- [Chapter](chapter-one.md)\n",
			),
			nodeFiles: map[string]nodeFileEntry{
				"chapter-one.md": {content: []byte("Content here."), exists: true},
				".prosemark.yml": {content: []byte("version: \"1\"\n"), exists: true},
			},
			wantErr:   false,
			wantInErr: "AUDW001",
		},
		{
			// US4/12: AUD007 unparseable YAML frontmatter → exit 1.
			name:        "US4/12 AUD007: unparseable YAML frontmatter",
			args:        []string{"--project", "."},
			binderBytes: doctorBinderWithNode(doctorTestNodeUUID),
			nodeFiles: map[string]nodeFileEntry{
				doctorTestNodeUUID + ".md": {content: invalidYAMLDoctorNodeContent(), exists: true},
			},
			wantErr:   true,
			wantInErr: "AUD007",
		},
		{
			// Uninitialized project: binder ErrNotExist → distinct "project not initialized" message.
			name:      "uninitialized project: binder not found",
			args:      []string{"--project", "."},
			binderErr: os.ErrNotExist,
			wantErr:   true,
			wantInErr: "project not initialized",
		},
		{
			// Unreadable binder: permissions error → distinct "cannot read binder" message.
			name:      "unreadable binder: permissions error",
			args:      []string{"--project", "."},
			binderErr: errors.New("permission denied"),
			wantErr:   true,
			wantInErr: "cannot read binder",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockDoctorIO{
				binderBytes: tt.binderBytes,
				binderErr:   tt.binderErr,
				uuidFiles:   tt.uuidFiles,
				nodeFiles:   tt.nodeFiles,
			}
			c := NewDoctorCmd(mock)
			out := new(bytes.Buffer)
			errOut := new(bytes.Buffer)
			c.SetOut(out)
			c.SetErr(errOut)
			c.SetArgs(tt.args)

			err := c.Execute()

			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v (stdout=%q stderr=%q)",
					err, tt.wantErr, out, errOut)
			}
			if tt.wantInOut != "" && !strings.Contains(out.String(), tt.wantInOut) {
				t.Errorf("stdout = %q, want to contain %q", out.String(), tt.wantInOut)
			}
			if tt.wantInErr != "" {
				combined := errOut.String()
				if err != nil {
					combined += err.Error()
				}
				if !strings.Contains(combined, tt.wantInErr) {
					t.Errorf("stderr+err = %q, want to contain %q", combined, tt.wantInErr)
				}
			}
		})
	}
}

// ─── JSON output schema validation ──────────────────────────────────────────

// TestNewDoctorCmd_JSONOutput_ContainsRequiredFields verifies --json mode outputs
// a wrapped object with version and diagnostics, where each diagnostic has
// severity, code, message, and path.
func TestNewDoctorCmd_JSONOutput_ContainsRequiredFields(t *testing.T) {
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

	// Must be valid JSON wrapped object with version and diagnostics.
	var result doctorOutput
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("--json output is not valid JSON: %v\noutput: %q", err, out.String())
	}
	if len(result.Diagnostics) == 0 {
		t.Fatal("expected at least one diagnostic in JSON output")
	}
	for i, r := range result.Diagnostics {
		if r.Code == "" {
			t.Errorf("results[%d] missing 'code' field", i)
		}
		if r.Message == "" {
			t.Errorf("results[%d] missing 'message' field", i)
		}
		if r.Severity == "" {
			t.Errorf("results[%d] missing 'severity' field", i)
		}
		// r.Path may be empty string — field is present in the struct.
	}
}

// TestNewDoctorCmd_JSONOutput_CleanProject verifies --json on a clean project
// outputs a wrapped object with an empty diagnostics array.
func TestNewDoctorCmd_JSONOutput_CleanProject(t *testing.T) {
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

	var result doctorOutput
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("--json clean project output is not valid JSON: %v\noutput: %q", err, out.String())
	}
	if len(result.Diagnostics) != 0 {
		t.Errorf("expected empty diagnostics for clean project, got %d: %v", len(result.Diagnostics), result.Diagnostics)
	}
}

// ─── File size limit ────────────────────────────────────────────────────────

// TestNewDoctorCmd_FileSizeLimit verifies that node files exceeding 1MB emit
// AUD007 rather than being parsed (protecting against YAML bomb attacks).
func TestNewDoctorCmd_FileSizeLimit(t *testing.T) {
	oversized := bytes.Repeat([]byte("x"), 1024*1024+1) // 1 MB + 1 byte

	mock := &mockDoctorIO{
		binderBytes: doctorBinderWithNode(doctorTestNodeUUID),
		nodeFiles: map[string]nodeFileEntry{
			doctorTestNodeUUID + ".md": {content: oversized, exists: true},
		},
	}
	c := NewDoctorCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--project", ".", "--json"})

	_ = c.Execute()

	var result doctorOutput
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output not valid JSON: %v\noutput: %q", err, out.String())
	}
	found := false
	for _, r := range result.Diagnostics {
		if r.Code == "AUD007" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected AUD007 for file exceeding 1MB size limit, got: %v", result.Diagnostics)
	}
}

// ─── Path containment ───────────────────────────────────────────────────────

// TestNewDoctorCmd_PathContainment verifies that binder links resolving outside
// the project directory emit AUDW001 and ReadNodeFile is never called with an
// out-of-project path (preventing path traversal reads).
func TestNewDoctorCmd_PathContainment(t *testing.T) {
	mock := &mockDoctorIO{
		// Binder contains a traversal path that escapes the project root.
		binderBytes: []byte(
			"<!-- prosemark-binder:v1 -->\n" +
				"- [Secret](../../etc/passwd)\n",
		),
		nodeFiles: map[string]nodeFileEntry{},
	}

	c := newDoctorCmdWithGetCWD(mock, func() (string, error) {
		return "/tmp/myproject", nil
	})
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)
	c.SetOut(out)
	c.SetErr(errOut)
	c.SetArgs([]string{})

	_ = c.Execute()

	// ReadNodeFile must NOT be called with any path outside /tmp/myproject.
	projectPrefix := "/tmp/myproject" + string(filepath.Separator)
	for _, call := range mock.readFileCalls {
		if !strings.HasPrefix(filepath.Clean(call), filepath.Clean("/tmp/myproject")) {
			t.Errorf("ReadNodeFile called with path outside project dir: %q", call)
		}
		_ = projectPrefix // used for clarity
	}

	// AUDW001 must appear in stderr for the traversal attempt.
	if !strings.Contains(errOut.String(), "AUDW001") {
		t.Errorf("expected AUDW001 for path traversal attempt, stderr: %q", errOut.String())
	}
}

// ─── sanitizePath in output ──────────────────────────────────────────────────

// TestNewDoctorCmd_SanitizesControlCharsInOutput verifies that control characters
// in path values are replaced with '?' before appearing in human-readable output
// (preventing ANSI injection).
func TestNewDoctorCmd_SanitizesControlCharsInOutput(t *testing.T) {
	// A binder link containing a control character in the path name.
	controlPath := "chapter\x01evil.md"
	mock := &mockDoctorIO{
		binderBytes: []byte(
			"<!-- prosemark-binder:v1 -->\n" +
				"- [Evil](" + controlPath + ")\n",
		),
		nodeFiles: map[string]nodeFileEntry{
			controlPath: {content: []byte("some content"), exists: true},
		},
	}

	c := NewDoctorCmd(mock)
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)
	c.SetOut(out)
	c.SetErr(errOut)
	c.SetArgs([]string{"--project", "."})

	_ = c.Execute()

	// stdout must be empty — plain-text diagnostics route to stderr.
	if out.String() != "" {
		t.Errorf("stdout = %q, want empty (plain-text diagnostics route to stderr)", out.String())
	}
	// stderr must not contain the raw control character — sanitizePath must be applied.
	if strings.ContainsRune(errOut.String(), '\x01') {
		t.Errorf("stderr contains raw control char \\x01 — sanitizePath not applied: %q", errOut.String())
	}
}

// ─── Exit code semantics ─────────────────────────────────────────────────────

// TestNewDoctorCmd_ErrorsOnlyExitsOne verifies that any error-severity diagnostic
// causes the command to exit with code 1 (return non-nil error).
func TestNewDoctorCmd_ErrorsOnlyExitsOne(t *testing.T) {
	// AUD001 is error-severity: referenced file missing.
	mock := &mockDoctorIO{
		binderBytes: doctorBinderWithNode(doctorTestNodeUUID),
		nodeFiles: map[string]nodeFileEntry{
			doctorTestNodeUUID + ".md": {content: nil, exists: false},
		},
	}
	c := NewDoctorCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--project", "."})

	if err := c.Execute(); err == nil {
		t.Error("expected exit 1 (error) when error-severity diagnostic present")
	}
}

// TestNewDoctorCmd_WarningsOnlyExitsZero verifies that warning-only diagnostics
// produce exit code 0 (nil error from Execute).
func TestNewDoctorCmd_WarningsOnlyExitsZero(t *testing.T) {
	// AUD002 is warning-severity: orphaned UUID file.
	mock := &mockDoctorIO{
		binderBytes: doctorBinderEmpty(),
		uuidFiles:   []string{doctorTestNodeUUID + ".md"},
		nodeFiles: map[string]nodeFileEntry{
			".prosemark.yml": {content: []byte("version: \"1\"\n"), exists: true},
		},
	}
	c := NewDoctorCmd(mock)
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--project", "."})

	if err := c.Execute(); err != nil {
		t.Errorf("expected exit 0 for warnings-only project, got: %v", err)
	}
}

// ─── Root command wiring ─────────────────────────────────────────────────────

// TestNewRootCmd_RegistersDoctorSubcommand verifies that "doctor" is registered
// on the root pmk command.
func TestNewRootCmd_RegistersDoctorSubcommand(t *testing.T) {
	root := NewRootCmd()
	var found bool
	for _, sub := range root.Commands() {
		if sub.Name() == "doctor" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected \"doctor\" subcommand registered on root command")
	}
}

// TestRootCmd_FileDoctorIO_ImplementsDoctorIO is a compile-time assertion that
// fileDoctorIO (value, not pointer) satisfies the DoctorIO interface.
func TestRootCmd_FileDoctorIO_ImplementsDoctorIO(t *testing.T) {
	var _ DoctorIO = fileDoctorIO{}
	t.Log("fileDoctorIO value satisfies DoctorIO")
}

// ─── Compile-time interface check ────────────────────────────────────────────

// Compile-time assertion: *fileDoctorIO satisfies DoctorIO.
var _ DoctorIO = (*fileDoctorIO)(nil)

// ─── JSON encode error handling ───────────────────────────────────────────────

// TestNewDoctorCmd_JSONEncodeError verifies that a write failure during --json
// output is NOT silently discarded. The command must return an error when
// json.Encode fails (e.g., stdout is closed or a pipe breaks).
func TestNewDoctorCmd_JSONEncodeError(t *testing.T) {
	// Clean project so hasAuditDiagnosticError is false — the only error path is
	// the encode failure itself.
	mock := &mockDoctorIO{
		binderBytes: doctorBinderWithNode(doctorTestNodeUUID),
		nodeFiles: map[string]nodeFileEntry{
			doctorTestNodeUUID + ".md": {content: validDoctorNodeContent(doctorTestNodeUUID), exists: true},
		},
	}
	c := NewDoctorCmd(mock)
	c.SetOut(&errWriter{err: errors.New("stdout closed")}) // write always fails
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--project", ".", "--json"})

	err := c.Execute()
	if err == nil {
		t.Error("expected error when JSON encoding fails, got nil")
	}
}

// ─── Binder parse diagnostics (BNDW*) ────────────────────────────────────────

// TestNewDoctorCmd_BinderParseWarnings verifies that binder-level parse diagnostics
// (BNDW*) are surfaced by doctor so users get a complete integrity picture in one
// command rather than having to run both 'pmk parse' and 'pmk doctor'.
func TestNewDoctorCmd_BinderParseWarnings(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		binderBytes []byte
		nodeFiles   map[string]nodeFileEntry
		wantErr     bool
		wantInErr   string
		wantNoneOut string
	}{
		{
			// BNDW001: binder has no pragma → warning reported, exit 0.
			name:        "BNDW001: binder missing pragma is reported as warning",
			args:        []string{"--project", "."},
			binderBytes: []byte("- [Node](" + doctorTestNodeUUID + ".md)\n"), // no pragma
			nodeFiles: map[string]nodeFileEntry{
				doctorTestNodeUUID + ".md": {content: validDoctorNodeContent(doctorTestNodeUUID), exists: true},
				".prosemark.yml":           {content: []byte("version: \"1\"\n"), exists: true},
			},
			wantErr:   false, // BNDW001 is warning severity → exit 0
			wantInErr: "BNDW001",
		},
		{
			// Binder with valid pragma → no BNDW001 in output.
			name:        "binder with pragma produces no BNDW001 in output",
			args:        []string{"--project", "."},
			binderBytes: doctorBinderWithNode(doctorTestNodeUUID), // has pragma
			nodeFiles: map[string]nodeFileEntry{
				doctorTestNodeUUID + ".md": {content: validDoctorNodeContent(doctorTestNodeUUID), exists: true},
				".prosemark.yml":           {content: []byte("version: \"1\"\n"), exists: true},
			},
			wantErr:     false,
			wantNoneOut: "BNDW001",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockDoctorIO{
				binderBytes: tt.binderBytes,
				nodeFiles:   tt.nodeFiles,
			}
			c := NewDoctorCmd(mock)
			out := new(bytes.Buffer)
			errOut := new(bytes.Buffer)
			c.SetOut(out)
			c.SetErr(errOut)
			c.SetArgs(tt.args)

			err := c.Execute()

			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v (stderr=%q)", err, tt.wantErr, errOut)
			}
			if tt.wantInErr != "" && !strings.Contains(errOut.String(), tt.wantInErr) {
				t.Errorf("stderr = %q, want to contain %q", errOut.String(), tt.wantInErr)
			}
			if tt.wantNoneOut != "" && strings.Contains(out.String(), tt.wantNoneOut) {
				t.Errorf("stdout = %q, must NOT contain %q", out.String(), tt.wantNoneOut)
			}
		})
	}
}

// ─── Stderr routing for plain-text diagnostics ───────────────────────────────

// TestNewDoctorCmd_PlainText_DiagnosticsRouteToStderr verifies that AUD*
// diagnostic lines in plain-text mode are written to stderr, not stdout, to
// match the convention used by all other pmk commands (add-child, delete, move).
// Scriptable pipelines rely on stdout being clean for piping and stderr for
// human-readable diagnostics.
func TestNewDoctorCmd_PlainText_DiagnosticsRouteToStderr(t *testing.T) {
	tests := []struct {
		name     string
		mock     *mockDoctorIO
		wantCode string // diagnostic code expected in stderr
	}{
		{
			name: "AUD001 error diagnostic routes to stderr",
			mock: &mockDoctorIO{
				binderBytes: doctorBinderWithNode(doctorTestNodeUUID),
				nodeFiles: map[string]nodeFileEntry{
					doctorTestNodeUUID + ".md": {content: nil, exists: false},
				},
			},
			wantCode: "AUD001",
		},
		{
			name: "AUD002 warning diagnostic routes to stderr",
			mock: &mockDoctorIO{
				binderBytes: doctorBinderEmpty(),
				uuidFiles:   []string{doctorTestNodeUUID + ".md"},
				nodeFiles: map[string]nodeFileEntry{
					".prosemark.yml": {content: []byte("version: \"1\"\n"), exists: true},
				},
			},
			wantCode: "AUD002",
		},
		{
			name: "AUD003 error diagnostic routes to stderr",
			mock: &mockDoctorIO{
				binderBytes: []byte(
					"<!-- prosemark-binder:v1 -->\n" +
						"- [A](" + doctorTestNodeUUID + ".md)\n" +
						"- [B](" + doctorTestNodeUUID + ".md)\n",
				),
				nodeFiles: map[string]nodeFileEntry{
					doctorTestNodeUUID + ".md": {content: validDoctorNodeContent(doctorTestNodeUUID), exists: true},
				},
			},
			wantCode: "AUD003",
		},
		{
			name: "AUD006 warning diagnostic routes to stderr",
			mock: &mockDoctorIO{
				binderBytes: doctorBinderWithNode(doctorTestNodeUUID),
				nodeFiles: map[string]nodeFileEntry{
					doctorTestNodeUUID + ".md": {content: emptyBodyDoctorNodeContent(doctorTestNodeUUID), exists: true},
					".prosemark.yml":           {content: []byte("version: \"1\"\n"), exists: true},
				},
			},
			wantCode: "AUD006",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewDoctorCmd(tt.mock)
			out := new(bytes.Buffer)
			errOut := new(bytes.Buffer)
			c.SetOut(out)
			c.SetErr(errOut)
			c.SetArgs([]string{"--project", "."})

			_ = c.Execute()

			// Diagnostic codes MUST appear in stderr.
			if !strings.Contains(errOut.String(), tt.wantCode) {
				t.Errorf("stderr = %q, want to contain %q (diagnostic should route to stderr)", errOut.String(), tt.wantCode)
			}
			// Diagnostic codes must NOT appear in stdout.
			if strings.Contains(out.String(), tt.wantCode) {
				t.Errorf("stdout = %q, must NOT contain %q (diagnostics should route to stderr, not stdout)", out.String(), tt.wantCode)
			}
		})
	}
}

// TestNewDoctorCmd_PlainText_StdoutEmptyWhenDiagnosticsPresent verifies that
// stdout is empty in plain-text mode regardless of whether diagnostics are
// produced. All diagnostic output belongs on stderr; stdout must remain clean
// for pipeline composition.
func TestNewDoctorCmd_PlainText_StdoutEmptyWhenDiagnosticsPresent(t *testing.T) {
	// AUD001 error: referenced file missing.
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
	c.SetArgs([]string{"--project", "."})

	_ = c.Execute()

	if out.String() != "" {
		t.Errorf("stdout = %q, want empty string (all plain-text doctor output should go to stderr)", out.String())
	}
}
