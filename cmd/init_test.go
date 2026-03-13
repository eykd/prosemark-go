package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
)

// mockInitIO is a test double for InitIO.
type mockInitIO struct {
	binderExists  bool
	binderStatErr error
	binderContent string
	readFileErr   error
	configExists  bool
	configStatErr error
	writeErrFor   map[string]error  // keyed by filepath.Base
	written       map[string]string // keyed by filepath.Base
}

func newMockInitIO() *mockInitIO {
	return &mockInitIO{
		writeErrFor: make(map[string]error),
		written:     make(map[string]string),
	}
}

func (m *mockInitIO) StatFile(path string) (bool, error) {
	switch filepath.Base(path) {
	case "_binder.md":
		return m.binderExists, m.binderStatErr
	case ".prosemark.yml":
		return m.configExists, m.configStatErr
	default:
		return false, nil
	}
}

func (m *mockInitIO) ReadFile(path string) (string, error) {
	if m.readFileErr != nil {
		return "", m.readFileErr
	}
	if filepath.Base(path) == "_binder.md" {
		return m.binderContent, nil
	}
	return "", nil
}

func (m *mockInitIO) WriteFileAtomic(path, content string) error {
	base := filepath.Base(path)
	if err, ok := m.writeErrFor[base]; ok {
		return err
	}
	m.written[base] = content
	return nil
}

func TestNewInitCmd_HasRequiredFlags(t *testing.T) {
	c := NewInitCmd(nil)
	required := []string{"project", "force"}
	for _, name := range required {
		t.Run(name, func(t *testing.T) {
			if c.Flags().Lookup(name) == nil {
				t.Errorf("expected --%s flag on init command", name)
			}
		})
	}
}

func TestNewInitCmd_DefaultsToCWD(t *testing.T) {
	mock := newMockInitIO()
	c := NewInitCmd(mock)
	out := new(bytes.Buffer)
	c.SetOut(out)
	c.SetErr(new(bytes.Buffer))

	if err := c.Execute(); err != nil {
		t.Fatalf("expected success with no --project (CWD default): %v", err)
	}
}

func TestNewInitCmd_GetCWDError(t *testing.T) {
	mock := newMockInitIO()
	c := newInitCmdWithGetCWD(mock, func() (string, error) {
		return "", errors.New("getwd failed")
	})
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))

	if err := c.Execute(); err == nil {
		t.Error("expected error when getwd fails")
	}
}

func TestNewInitCmd_Scenarios(t *testing.T) {
	tests := []struct {
		name          string
		binderExists  bool
		binderStatErr error
		configExists  bool
		configStatErr error
		writeErrFor   map[string]error
		force         bool
		wantErr       bool
		wantBinder    bool
		wantConfig    bool
		wantStdout    string
		wantStderrHas string
	}{
		{
			name:       "no files creates both",
			wantBinder: true,
			wantConfig: true,
			wantStdout: "Initialized",
		},
		{
			name:         "binder exists no force errors",
			binderExists: true,
			wantErr:      true,
			wantBinder:   false,
			wantConfig:   false,
		},
		{
			name:         "config exists only creates binder",
			configExists: true,
			wantBinder:   true,
			wantConfig:   false,
			wantStdout:   "Initialized",
		},
		{
			name:          "force overwrites both with warning",
			binderExists:  true,
			configExists:  true,
			force:         true,
			wantBinder:    true,
			wantConfig:    true,
			wantStdout:    "Initialized",
			wantStderrHas: "warning",
		},
		{
			name:        "binder write error",
			writeErrFor: map[string]error{"_binder.md": errors.New("permission denied")},
			wantErr:     true,
			wantBinder:  false,
		},
		{
			name:          "binder stat error",
			binderStatErr: errors.New("stat failed"),
			wantErr:       true,
		},
		{
			name:          "config stat error",
			configStatErr: errors.New("stat failed"),
			wantErr:       true,
			wantBinder:    true, // binder is written before config stat is checked
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockInitIO()
			mock.binderExists = tt.binderExists
			mock.binderStatErr = tt.binderStatErr
			mock.configExists = tt.configExists
			mock.configStatErr = tt.configStatErr
			if tt.writeErrFor != nil {
				mock.writeErrFor = tt.writeErrFor
			}

			c := newInitCmdWithGetCWD(mock, func() (string, error) { return ".", nil })
			out := new(bytes.Buffer)
			errOut := new(bytes.Buffer)
			c.SetOut(out)
			c.SetErr(errOut)

			args := []string{"--project", "."}
			if tt.force {
				args = append(args, "--force")
			}
			c.SetArgs(args)

			err := c.Execute()
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantBinder {
				if _, ok := mock.written["_binder.md"]; !ok {
					t.Error("expected _binder.md to be written")
				}
			} else {
				if _, ok := mock.written["_binder.md"]; ok {
					t.Error("expected _binder.md NOT to be written")
				}
			}

			if tt.wantConfig {
				if _, ok := mock.written[".prosemark.yml"]; !ok {
					t.Error("expected .prosemark.yml to be written")
				}
			} else {
				if _, ok := mock.written[".prosemark.yml"]; ok {
					t.Error("expected .prosemark.yml NOT to be written")
				}
			}

			if tt.wantStdout != "" && !strings.Contains(out.String(), tt.wantStdout) {
				t.Errorf("stdout = %q, want to contain %q", out.String(), tt.wantStdout)
			}
			if tt.wantStderrHas != "" && !strings.Contains(strings.ToLower(errOut.String()), tt.wantStderrHas) {
				t.Errorf("stderr = %q, want to contain %q", errOut.String(), tt.wantStderrHas)
			}
		})
	}
}

func TestNewInitCmd_PartialInitErrorIncludesRecoveryHint(t *testing.T) {
	// Simulate config write failure after binder write succeeds (partial init state)
	mock := newMockInitIO()
	mock.writeErrFor = map[string]error{".prosemark.yml": errors.New("permission denied")}

	c := newInitCmdWithGetCWD(mock, func() (string, error) { return ".", nil })
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)
	c.SetOut(out)
	c.SetErr(errOut)
	c.SetArgs([]string{"--project", "."})

	err := c.Execute()
	if err == nil {
		t.Fatal("expected error on config write failure")
	}

	combined := errOut.String() + err.Error()
	if !strings.Contains(combined, "--force") {
		t.Errorf("expected partial-init error to include --force recovery hint, got: %q", combined)
	}
}

func TestNewInitCmd_ConfigFileContent(t *testing.T) {
	mock := newMockInitIO()
	c := newInitCmdWithGetCWD(mock, func() (string, error) { return ".", nil })
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--project", "."})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	const wantConfigContent = "version: \"1\"\n"
	got := mock.written[".prosemark.yml"]
	if got != wantConfigContent {
		t.Errorf(".prosemark.yml content = %q, want %q", got, wantConfigContent)
	}
}

// --- init OpResult and dry-run tests ---

func TestInitCmd_JSONOutput_ReturnsOpResult(t *testing.T) {
	mock := newMockInitIO()
	sub := newInitCmdWithGetCWD(mock, func() (string, error) { return ".", nil })
	root := withDryRunFlag(sub)
	out := new(bytes.Buffer)
	sub.SetOut(out)
	sub.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"init", "--project", ".", "--json"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result binder.OpResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON OpResult: %v\noutput: %s", err, out.String())
	}
	if result.Version != "1" {
		t.Errorf("Version = %q, want %q", result.Version, "1")
	}
	if !result.Changed {
		t.Error("expected Changed=true when files are created")
	}
	if result.DryRun {
		t.Error("expected DryRun=false when not in dry-run mode")
	}
}

func TestInitCmd_JSONOutput_IncludesInfoDiagnostics(t *testing.T) {
	mock := newMockInitIO()
	sub := newInitCmdWithGetCWD(mock, func() (string, error) { return ".", nil })
	root := withDryRunFlag(sub)
	out := new(bytes.Buffer)
	sub.SetOut(out)
	sub.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"init", "--project", ".", "--json"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result binder.OpResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out.String())
	}

	// Created files should be reported as info-level diagnostics.
	var infoDiags []binder.Diagnostic
	for _, d := range result.Diagnostics {
		if d.Severity == "info" {
			infoDiags = append(infoDiags, d)
		}
	}
	if len(infoDiags) < 2 {
		t.Errorf("expected at least 2 info diagnostics for created files, got %d: %v",
			len(infoDiags), result.Diagnostics)
	}

	// Verify diagnostics mention the created files.
	combined := ""
	for _, d := range infoDiags {
		combined += d.Message + " "
	}
	if !strings.Contains(combined, "_binder.md") {
		t.Errorf("info diagnostics should mention _binder.md, got: %q", combined)
	}
	if !strings.Contains(combined, ".prosemark.yml") {
		t.Errorf("info diagnostics should mention .prosemark.yml, got: %q", combined)
	}
}

func TestInitCmd_DryRun_SkipsWrite(t *testing.T) {
	mock := newMockInitIO()
	sub := newInitCmdWithGetCWD(mock, func() (string, error) { return ".", nil })
	root := withDryRunFlag(sub)
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"init", "--project", ".", "--dry-run"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.written) != 0 {
		t.Errorf("dry-run must not write any files, but wrote: %v", mock.written)
	}
}

func TestInitCmd_DryRun_JSONOutput(t *testing.T) {
	mock := newMockInitIO()
	sub := newInitCmdWithGetCWD(mock, func() (string, error) { return ".", nil })
	root := withDryRunFlag(sub)
	out := new(bytes.Buffer)
	sub.SetOut(out)
	sub.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"init", "--project", ".", "--json", "--dry-run"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result binder.OpResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out.String())
	}
	if !result.DryRun {
		t.Error("expected DryRun=true in OpResult")
	}
	if result.Changed {
		t.Error("expected Changed=false in dry-run mode")
	}
}

func TestInitCmd_DryRun_InfoDiagnosticsListWouldBeCreated(t *testing.T) {
	mock := newMockInitIO()
	sub := newInitCmdWithGetCWD(mock, func() (string, error) { return ".", nil })
	root := withDryRunFlag(sub)
	out := new(bytes.Buffer)
	sub.SetOut(out)
	sub.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"init", "--project", ".", "--json", "--dry-run"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result binder.OpResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out.String())
	}

	// In dry-run mode, info diagnostics should list what would be created.
	var infoDiags []binder.Diagnostic
	for _, d := range result.Diagnostics {
		if d.Severity == "info" {
			infoDiags = append(infoDiags, d)
		}
	}
	if len(infoDiags) < 2 {
		t.Errorf("expected at least 2 info diagnostics for would-be-created files, got %d: %v",
			len(infoDiags), result.Diagnostics)
	}
}

func TestInitCmd_DryRun_HumanOutputPrefix(t *testing.T) {
	mock := newMockInitIO()
	sub := newInitCmdWithGetCWD(mock, func() (string, error) { return ".", nil })
	root := withDryRunFlag(sub)
	out := new(bytes.Buffer)
	sub.SetOut(out)
	sub.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"init", "--project", ".", "--dry-run"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasPrefix(out.String(), "dry-run:") {
		t.Errorf("expected human output prefixed with 'dry-run:', got: %q", out.String())
	}
}

func TestInitCmd_HumanOutput_PrintsInfoDiagnostics(t *testing.T) {
	mock := newMockInitIO()
	sub := newInitCmdWithGetCWD(mock, func() (string, error) { return ".", nil })
	root := withDryRunFlag(sub)
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)
	sub.SetOut(out)
	sub.SetErr(errOut)
	root.SetArgs([]string{"init", "--project", "."})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Info diagnostics about created files should appear in stderr.
	stderr := errOut.String()
	if !strings.Contains(stderr, "_binder.md") {
		t.Errorf("stderr should mention _binder.md, got: %q", stderr)
	}
	if !strings.Contains(stderr, ".prosemark.yml") {
		t.Errorf("stderr should mention .prosemark.yml, got: %q", stderr)
	}
}

func TestInitCmd_StdoutWriteError(t *testing.T) {
	mock := newMockInitIO()
	c := newInitCmdWithGetCWD(mock, func() (string, error) { return ".", nil })
	c.SetOut(&errWriter{err: errors.New("write error")})
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--project", "."})

	err := c.Execute()
	if err == nil {
		t.Fatal("expected error when stdout write fails")
	}
	if !strings.Contains(err.Error(), "writing output") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "writing output")
	}
}

func TestInitCmd_StderrWriteError_Warning(t *testing.T) {
	mock := newMockInitIO()
	mock.binderExists = true
	mock.configExists = true
	c := newInitCmdWithGetCWD(mock, func() (string, error) { return ".", nil })
	c.SetOut(new(bytes.Buffer))
	c.SetErr(&errWriter{err: errors.New("write error")})
	c.SetArgs([]string{"--project", ".", "--force"})

	err := c.Execute()
	if err == nil {
		t.Fatal("expected error when stderr write fails for warning")
	}
	if !strings.Contains(err.Error(), "writing output") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "writing output")
	}
}

// TestInitCmd_JSONEncodeError verifies that a write failure during --json
// output is NOT silently discarded. The command must return an error when
// json.Encode fails (e.g., stdout is closed or a pipe breaks).
func TestInitCmd_JSONEncodeError(t *testing.T) {
	mock := newMockInitIO()
	sub := newInitCmdWithGetCWD(mock, func() (string, error) { return ".", nil })
	root := withDryRunFlag(sub)
	sub.SetOut(&errWriter{err: errors.New("stdout closed")})
	sub.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"init", "--project", ".", "--json"})

	err := root.Execute()
	if err == nil {
		t.Error("expected error when JSON encoding fails in init --json mode, got nil")
	}
}

func TestInitCmd_ForceReadFileError(t *testing.T) {
	mock := newMockInitIO()
	mock.binderExists = true
	mock.readFileErr = errors.New("permission denied")

	c := newInitCmdWithGetCWD(mock, func() (string, error) { return ".", nil })
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--project", ".", "--force"})

	err := c.Execute()
	if err == nil {
		t.Fatal("expected error when ReadFile fails during force init")
	}
	if !strings.Contains(err.Error(), "reading") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "reading")
	}
}

func TestInitCmd_ForceDataLossWarningStderrError(t *testing.T) {
	mock := newMockInitIO()
	mock.binderExists = true
	mock.binderContent = "<!-- prosemark-binder:v1 -->\n\n- [Chapter](ch.md)\n"

	c := newInitCmdWithGetCWD(mock, func() (string, error) { return ".", nil })
	c.SetOut(new(bytes.Buffer))
	c.SetErr(&errWriter{err: errors.New("write error")})
	c.SetArgs([]string{"--project", ".", "--force"})

	err := c.Execute()
	if err == nil {
		t.Fatal("expected error when stderr write fails for data loss warning")
	}
	if !strings.Contains(err.Error(), "writing output") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "writing output")
	}
}

func TestCountBinderNodes_InvalidContent(t *testing.T) {
	// Invalid UTF-8 triggers a parse error → countBinderNodes returns 0.
	count := countBinderNodes(string([]byte{0xff, 0xfe}))
	if count != 0 {
		t.Errorf("countBinderNodes(invalid) = %d, want 0", count)
	}
}

func TestNewRootCmd_RegistersInitSubcommand(t *testing.T) {
	root := NewRootCmd()
	var found bool
	for _, sub := range root.Commands() {
		if sub.Name() == "init" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected \"init\" subcommand registered on root command")
	}
}
