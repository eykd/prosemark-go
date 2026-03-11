package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
	"github.com/spf13/cobra"
)

func TestNewRootCmd_RegistersParseSubcommand(t *testing.T) {
	root := NewRootCmd()
	var found bool
	for _, sub := range root.Commands() {
		if sub.Name() == "parse" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected \"parse\" subcommand registered on root command")
	}
}

func TestBuildCommandTree_AllCommandsHandleNilService(t *testing.T) {
	root := NewRootCmd()
	for _, sub := range root.Commands() {
		c := sub
		t.Run(c.Name(), func(t *testing.T) {
			if c.RunE == nil {
				t.Errorf("command %q has nil RunE; must wire RunE for error visibility", c.Name())
			}
		})
	}
}

func TestResolveBinderPath_UsesProjectWhenSet(t *testing.T) {
	got, err := resolveBinderPath("/my/project", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join("/my/project", "_binder.md")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveBinderPath_UsesCWDWhenProjectEmpty(t *testing.T) {
	got, err := resolveBinderPath("", func() (string, error) { return "/cwd", nil })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join("/cwd", "_binder.md")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRootCmd_NoArgs_ShowsHelp(t *testing.T) {
	root := NewRootCmd()
	out := new(bytes.Buffer)
	root.SetOut(out)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "pmk") {
		t.Errorf("expected help output to contain \"pmk\", got: %s", out.String())
	}
}

// TestResolveProjectDirFromCmd_PMKProjectEnvVar verifies that PMK_PROJECT
// env var is used as the project directory when --project flag is not set.
func TestResolveProjectDirFromCmd_PMKProjectEnvVar(t *testing.T) {
	tests := []struct {
		name      string
		envValue  string
		flagValue string
		flagSet   bool
		wantDir   string
		wantGetwd bool // true if getwd should be called
		wantErr   bool
	}{
		{
			name:      "env var used when flag not set",
			envValue:  "/from/env",
			flagSet:   false,
			wantDir:   "/from/env",
			wantGetwd: false,
		},
		{
			name:      "flag takes precedence over env var",
			envValue:  "/from/env",
			flagValue: "/from/flag",
			flagSet:   true,
			wantDir:   "/from/flag",
			wantGetwd: false,
		},
		{
			name:      "falls back to getwd when neither set",
			envValue:  "",
			flagSet:   false,
			wantDir:   "/from/cwd",
			wantGetwd: true,
		},
		{
			name:      "empty env var is ignored, falls back to getwd",
			envValue:  "",
			flagSet:   false,
			wantDir:   "/from/cwd",
			wantGetwd: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(envPMKProject, tt.envValue)

			cmd := &cobra.Command{}
			cmd.Flags().String("project", "", "project directory")
			if tt.flagSet {
				if err := cmd.Flags().Set("project", tt.flagValue); err != nil {
					t.Fatal(err)
				}
			}

			getwdCalled := false
			getwd := func() (string, error) {
				getwdCalled = true
				return "/from/cwd", nil
			}

			got, err := resolveProjectDirFromCmd(cmd, getwd)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.wantDir {
				t.Errorf("got %q, want %q", got, tt.wantDir)
			}
			if tt.wantGetwd != getwdCalled {
				t.Errorf("getwd called = %v, want %v", getwdCalled, tt.wantGetwd)
			}
		})
	}
}

func TestResolveBinderPath_ReturnsErrorWhenGetCWDFails(t *testing.T) {
	_, err := resolveBinderPath("", func() (string, error) { return "", errors.New("getwd failed") })
	if err == nil {
		t.Error("expected error when getwd fails")
	}
}

// TestRootCmd_FileInitIO_ImplementsInitIO is a compile-time assertion that
// fileInitIO (value, not pointer) satisfies the InitIO interface.
// Acceptance: NewInitCmd(fileInitIO{}) registered via rootCmd.AddCommand.
func TestRootCmd_FileInitIO_ImplementsInitIO(t *testing.T) {
	var _ InitIO = fileInitIO{} // fileInitIO value must implement InitIO
	t.Log("fileInitIO value satisfies InitIO")
}

// TestRootCmd_InitHelp_ShowsUsage verifies pmk init --help from the root
// command shows the init command description.
func TestRootCmd_InitHelp_ShowsUsage(t *testing.T) {
	root := NewRootCmd()
	out := new(bytes.Buffer)
	root.SetOut(out)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"init", "--help"})
	_ = root.Execute()

	want := "Initialize a prosemark project"
	if !strings.Contains(out.String(), want) {
		t.Errorf("'pmk init --help' output = %q, want to contain %q", out.String(), want)
	}
}

// TestRootCmd_FileEditIO_ImplementsEditIO is a compile-time assertion that
// fileEditIO (value, not pointer) satisfies the EditIO interface.
// Acceptance: NewEditCmd(fileEditIO{}) registered via rootCmd.AddCommand.
func TestRootCmd_FileEditIO_ImplementsEditIO(t *testing.T) {
	var _ EditIO = fileEditIO{} // fileEditIO value must implement EditIO
	t.Log("fileEditIO value satisfies EditIO")
}

// TestRootCmd_EditHelp_ShowsUsage verifies pmk edit --help from the root
// command shows the edit command description.
func TestRootCmd_EditHelp_ShowsUsage(t *testing.T) {
	root := NewRootCmd()
	out := new(bytes.Buffer)
	root.SetOut(out)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"edit", "--help"})
	_ = root.Execute()

	want := "Open a node file in $EDITOR"
	if !strings.Contains(out.String(), want) {
		t.Errorf("'pmk edit --help' output = %q, want to contain %q", out.String(), want)
	}
}

// TestRootCmd_EditCmd_BinderNotFound_ShowsProjectNotInitialized verifies that
// pmk edit in a directory without _binder.md produces an error message guiding
// the user to run 'pmk init' first, per plan.md §Binder Parse Failure in pmk edit.
func TestRootCmd_EditCmd_BinderNotFound_ShowsProjectNotInitialized(t *testing.T) {
	dir := t.TempDir() // empty dir — no _binder.md
	t.Setenv("EDITOR", "true")

	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	errOut := new(bytes.Buffer)
	root.SetErr(errOut)
	root.SetArgs([]string{"edit", "--project", dir, "01234567-89ab-7def-0123-456789abcdef"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when _binder.md does not exist")
	}
	want := "project not initialized"
	combined := err.Error() + errOut.String()
	if !strings.Contains(combined, want) {
		t.Errorf("expected error to contain %q, got err=%q stderr=%q", want, err.Error(), errOut.String())
	}
}

// TestRootCmd_DoctorHelp_ShowsUsage verifies pmk doctor --help from the root
// command shows the doctor command description.
func TestRootCmd_DoctorHelp_ShowsUsage(t *testing.T) {
	root := NewRootCmd()
	out := new(bytes.Buffer)
	root.SetOut(out)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"doctor", "--help"})
	_ = root.Execute()

	want := "Validate project structural integrity"
	if !strings.Contains(out.String(), want) {
		t.Errorf("'pmk doctor --help' output = %q, want to contain %q", out.String(), want)
	}
}

// TestRootCmd_DoctorCmd_HumanReadable_SeverityColumnAligned verifies that
// pmk doctor human-readable output pads the severity field to 7 characters,
// aligning the message column per contracts/commands.md §Output (default, human-readable).
//
// Contract format:
//
//	AUD001 error   referenced file does not exist: <uuid>.md
//	AUD006 warning node file has no body content: <uuid>.md
//
// "error" (5 chars) is padded to 7, yielding 3 spaces before the message.
// "warning" (7 chars) needs no padding, yielding 1 space before the message.
// This requires format "%-7s" for the severity field (e.g. "%-7s" in fmt.Fprintf).
func TestRootCmd_DoctorCmd_HumanReadable_SeverityColumnAligned(t *testing.T) {
	dir := t.TempDir()
	// Write a valid binder referencing a UUID node file that does not exist on disk.
	// binder.Parse succeeds; doctor then detects the missing file and emits AUD001.
	binderContent := "<!-- prosemark-binder:v1 -->\n- [Node](01234567-89ab-7def-0123-456789abcdef.md)\n"
	if err := os.WriteFile(filepath.Join(dir, "_binder.md"), []byte(binderContent), 0o600); err != nil {
		t.Fatal(err)
	}
	// Node file intentionally absent → AUD001 (error severity) fires.

	root := NewRootCmd()
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)
	root.SetOut(out)
	root.SetErr(errOut)
	root.SetArgs([]string{"doctor", "--project", dir})

	_ = root.Execute()

	// The contract specifies severity padded to 7 chars so messages align:
	//   "AUD001 error   referenced..." ("error" + 2-char pad + 1 separator = 3 spaces)
	//   "AUD006 warning node..."       ("warning" + 0-char pad + 1 separator = 1 space)
	// Diagnostics route to stderr in plain-text mode.
	want := "AUD001 error   " // "error" padded to 7 + separator space = 3 total spaces
	if !strings.Contains(errOut.String(), want) {
		t.Errorf("doctor human-readable output does not use column-aligned severity:\ngot:  %q\nwant to contain: %q\nhint: use \"%%s %%-7s %%s\\n\" format in doctor RunE",
			errOut.String(), want)
	}
}

// TestRootCmd_DryRunFlag_DefaultFalse verifies --dry-run is a persistent bool
// flag on the root command with a default value of false.
func TestRootCmd_DryRunFlag_DefaultFalse(t *testing.T) {
	root := NewRootCmd()
	f := root.PersistentFlags().Lookup("dry-run")
	if f == nil {
		t.Fatal("expected --dry-run persistent flag on root command")
	}
	if f.DefValue != "false" {
		t.Errorf("--dry-run default = %q, want %q", f.DefValue, "false")
	}
}

func TestRootCmd_DryRunFlag_InheritedBySubcommands(t *testing.T) {
	root := NewRootCmd()
	for _, sub := range root.Commands() {
		c := sub
		t.Run(c.Name(), func(t *testing.T) {
			f := c.InheritedFlags().Lookup("dry-run")
			if f == nil {
				t.Errorf("subcommand %q does not inherit --dry-run flag", c.Name())
			}
		})
	}
}

func TestRootCmd_DryRunFlag_SetToTrue(t *testing.T) {
	root := NewRootCmd()
	out := new(bytes.Buffer)
	root.SetOut(out)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"--dry-run"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	f := root.PersistentFlags().Lookup("dry-run")
	if f == nil {
		t.Fatal("expected --dry-run flag")
	}
	if f.Value.String() != "true" {
		t.Errorf("--dry-run value after setting = %q, want %q", f.Value.String(), "true")
	}
}

// rootHelpOutput executes the root command with --help and returns the output.
func rootHelpOutput(t *testing.T) string {
	t.Helper()
	root := NewRootCmd()
	out := new(bytes.Buffer)
	root.SetOut(out)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"--help"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return out.String()
}

func TestRootCmd_LongHelp_ContainsExitCodesTable(t *testing.T) {
	helpOutput := rootHelpOutput(t)

	// Must contain an "Exit Codes" section header
	if !strings.Contains(helpOutput, "Exit Codes") {
		t.Error("help output missing \"Exit Codes\" section")
	}

	// Must document each exit code 0-6 with its meaning
	exitCodes := []struct {
		code    string
		meaning string
	}{
		{"0", "Success"},
		{"1", "Usage error"},
		{"2", "Validation error"},
		{"3", "Not found"},
		{"5", "Conflict"},
		{"6", "Transient"},
	}
	for _, ec := range exitCodes {
		if !strings.Contains(helpOutput, ec.code) || !strings.Contains(helpOutput, ec.meaning) {
			t.Errorf("help output missing exit code %s (%s)", ec.code, ec.meaning)
		}
	}
}

func TestRootCmd_LongHelp_ContainsStateModel(t *testing.T) {
	helpOutput := rootHelpOutput(t)

	// Must contain a "State Model" section
	if !strings.Contains(helpOutput, "State Model") {
		t.Error("help output missing \"State Model\" section")
	}

	// Must mention both state files
	for _, file := range []string{".prosemark.yml", "_binder.md"} {
		if !strings.Contains(helpOutput, file) {
			t.Errorf("help output missing state file %q", file)
		}
	}
}

func TestRootCmd_LongHelp_ContainsEnvironmentVariables(t *testing.T) {
	helpOutput := rootHelpOutput(t)

	for _, envVar := range []string{"EDITOR", "PMK_PROJECT"} {
		if !strings.Contains(helpOutput, envVar) {
			t.Errorf("help output missing environment variable %q", envVar)
		}
	}
}

func TestRootCmd_EditCmd_BinderParseError_ShowsCannotParse(t *testing.T) {
	dir := t.TempDir()
	binderPath := filepath.Join(dir, "_binder.md")
	// Write an invalid binder file (bad UTF-8 → binder.Parse returns error)
	if err := os.WriteFile(binderPath, []byte{0xff, 0xfe}, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EDITOR", "true")

	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	errOut := new(bytes.Buffer)
	root.SetErr(errOut)
	root.SetArgs([]string{"edit", "--project", dir, "01234567-89ab-7def-0123-456789abcdef"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when _binder.md is malformed")
	}
	want := "cannot parse binder"
	combined := err.Error() + errOut.String()
	if !strings.Contains(combined, want) {
		t.Errorf("expected error to contain %q, got err=%q stderr=%q", want, err.Error(), errOut.String())
	}
}

// TestMutationSubcommands_Help_DocumentsDryRun verifies that mutation
// subcommands mention --dry-run in their Long description text (the block
// between the short description and the "Usage:" line in --help output).
func TestMutationSubcommands_Help_DocumentsDryRun(t *testing.T) {
	mutationCmds := []string{"add", "delete", "init", "move"}

	for _, name := range mutationCmds {
		t.Run(name, func(t *testing.T) {
			root := NewRootCmd()
			sub, _, err := root.Find([]string{name})
			if err != nil {
				t.Fatalf("subcommand %q not found: %v", name, err)
			}

			long := sub.Long
			if long == "" {
				t.Fatalf("%s has no Long description", name)
			}

			if !strings.Contains(long, "--dry-run") {
				t.Errorf("%s Long description does not mention --dry-run:\n%s",
					name, long)
			}
		})
	}
}

// TestReadOnlySubcommands_Help_HasLongDescription verifies that read-only
// subcommands have a Long description providing context beyond the Short line.
func TestReadOnlySubcommands_Help_HasLongDescription(t *testing.T) {
	readOnlyCmds := []string{"doctor", "edit", "parse"}

	for _, name := range readOnlyCmds {
		t.Run(name, func(t *testing.T) {
			root := NewRootCmd()
			sub, _, err := root.Find([]string{name})
			if err != nil {
				t.Fatalf("subcommand %q not found: %v", name, err)
			}

			if sub.Long == "" {
				t.Errorf("%s has no Long description", name)
			}
		})
	}
}

// TestReadOnlySubcommands_Help_DocumentsDryRunNoOp verifies that read-only
// subcommands that accept --dry-run document it as a no-op in their Long
// description, so users understand it has no effect on read-only operations.
func TestReadOnlySubcommands_Help_DocumentsDryRunNoOp(t *testing.T) {
	readOnlyCmds := []string{"doctor", "parse"}

	for _, name := range readOnlyCmds {
		t.Run(name, func(t *testing.T) {
			root := NewRootCmd()
			sub, _, err := root.Find([]string{name})
			if err != nil {
				t.Fatalf("subcommand %q not found: %v", name, err)
			}

			long := sub.Long
			if long == "" {
				t.Fatalf("%s has no Long description", name)
			}

			if !strings.Contains(long, "--dry-run") {
				t.Errorf("%s Long description does not mention --dry-run:\n%s",
					name, long)
			}

			if !strings.Contains(long, "no-op") && !strings.Contains(long, "no effect") {
				t.Errorf("%s Long description does not indicate --dry-run is a no-op:\n%s",
					name, long)
			}
		})
	}
}

// TestPrintDiagnostics_SuggestionDisplay verifies that printDiagnostics shows
// suggestions on a separate indented line in human mode, and omits the line
// when Suggestion is empty.
func TestPrintDiagnostics_SuggestionDisplay(t *testing.T) {
	tests := []struct {
		name       string
		suggestion string
		wantLine   string // expected suggestion line in output (empty = none)
	}{
		{
			name:       "with suggestion",
			suggestion: "Run 'pmk init' to create a project",
			wantLine:   "  suggestion: Run 'pmk init' to create a project\n",
		},
		{
			name:       "empty suggestion omits line",
			suggestion: "",
			wantLine:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			errBuf := new(bytes.Buffer)
			cmd.SetErr(errBuf)

			diags := []binder.Diagnostic{
				{
					Severity:   "error",
					Code:       "BNDE001",
					Message:    "something went wrong",
					Suggestion: tt.suggestion,
				},
			}

			printDiagnostics(cmd, diags)
			output := errBuf.String()

			if tt.wantLine != "" {
				if !strings.Contains(output, tt.wantLine) {
					t.Errorf("expected suggestion line %q in output, got:\n%s", tt.wantLine, output)
				}
			} else {
				if strings.Contains(output, "suggestion:") {
					t.Errorf("expected no suggestion line, but got:\n%s", output)
				}
			}
		})
	}
}

// TestPrintDiagnostics_MultipleDiags_SuggestionsOnlyWherePresent verifies that
// in a batch of diagnostics, only those with non-empty Suggestion get a
// suggestion line, and the suggestion follows its parent diagnostic.
func TestPrintDiagnostics_MultipleDiags_SuggestionsOnlyWherePresent(t *testing.T) {
	cmd := &cobra.Command{}
	errBuf := new(bytes.Buffer)
	cmd.SetErr(errBuf)

	diags := []binder.Diagnostic{
		{Severity: "error", Code: "BNDE001", Message: "first error", Suggestion: "fix it"},
		{Severity: "warning", Code: "OPW001", Message: "a warning", Suggestion: ""},
		{Severity: "error", Code: "BNDE002", Message: "second error", Suggestion: "try again"},
	}

	printDiagnostics(cmd, diags)
	output := errBuf.String()
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")

	// Expect 5 lines: diag1, suggestion1, diag2, diag3, suggestion3
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines, got %d:\n%s", len(lines), output)
	}

	// Second line should be the suggestion for the first diagnostic
	wantSug1 := "  suggestion: fix it"
	if lines[1] != wantSug1 {
		t.Errorf("line 2 = %q, want %q", lines[1], wantSug1)
	}

	// Fourth line is the third diagnostic (no suggestion after warning)
	// Fifth line should be the suggestion for the third diagnostic
	wantSug3 := "  suggestion: try again"
	if lines[4] != wantSug3 {
		t.Errorf("line 5 = %q, want %q", lines[4], wantSug3)
	}
}

// TestEmitOPE009AndError_JSONEncodeError verifies that when JSON encoding fails
// (e.g. broken stdout), the encoding error is surfaced in the returned error
// rather than silently discarded.
func TestEmitOPE009AndError_JSONEncodeError(t *testing.T) {
	writeErr := errors.New("write error")
	origErr := errors.New("original I/O failure")

	cmd := &cobra.Command{}
	cmd.SetOut(&errWriter{err: writeErr})

	got := emitOPE009AndError(cmd, true, origErr)

	// The returned error must contain the original error.
	if !strings.Contains(got.Error(), origErr.Error()) {
		t.Errorf("returned error %q does not contain original error %q", got, origErr)
	}

	// The returned error must also surface the encoding failure.
	if !strings.Contains(got.Error(), "encoding") && !strings.Contains(got.Error(), writeErr.Error()) {
		t.Errorf("returned error %q does not surface the JSON encoding error %q", got, writeErr)
	}
}

// TestSubcommands_Help_ContainsExamples verifies that every subcommand has an
// Example field with at least 2 usage examples in standard Cobra format.
func TestSubcommands_Help_ContainsExamples(t *testing.T) {
	subcommands := []string{"add", "delete", "doctor", "edit", "init", "move", "parse"}

	for _, name := range subcommands {
		t.Run(name, func(t *testing.T) {
			root := NewRootCmd()
			out := new(bytes.Buffer)
			root.SetOut(out)
			root.SetErr(new(bytes.Buffer))
			root.SetArgs([]string{name, "--help"})
			_ = root.Execute()

			helpOutput := out.String()

			// Help output must contain an Examples section.
			if !strings.Contains(helpOutput, "Examples:") {
				t.Fatalf("%s --help missing Examples section:\n%s", name, helpOutput)
			}

			// Extract the Examples section and count example lines.
			// Cobra examples are indented lines; count lines containing "pmk".
			examplesIdx := strings.Index(helpOutput, "Examples:")
			afterExamples := helpOutput[examplesIdx:]
			lines := strings.Split(afterExamples, "\n")

			var exampleCount int
			for _, line := range lines[1:] { // skip the "Examples:" header
				trimmed := strings.TrimSpace(line)
				if trimmed == "" {
					continue
				}
				// Stop at the next section header (non-indented, ends with colon).
				if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
					break
				}
				if strings.Contains(trimmed, "pmk") {
					exampleCount++
				}
			}

			if exampleCount < 2 {
				t.Errorf("%s --help has %d usage examples, want at least 2:\n%s",
					name, exampleCount, helpOutput)
			}
		})
	}
}
