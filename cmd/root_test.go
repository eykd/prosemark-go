package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	root.SetOut(out)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"doctor", "--project", dir})

	_ = root.Execute()

	// The contract specifies severity padded to 7 chars so messages align:
	//   "AUD001 error   referenced..." ("error" + 2-char pad + 1 separator = 3 spaces)
	//   "AUD006 warning node..."       ("warning" + 0-char pad + 1 separator = 1 space)
	// Current implementation uses "%s %s %s\n" (single space) which does NOT align.
	want := "AUD001 error   " // "error" padded to 7 + separator space = 3 total spaces
	if !strings.Contains(out.String(), want) {
		t.Errorf("doctor human-readable output does not use column-aligned severity:\ngot:  %q\nwant to contain: %q\nhint: use \"%%s %%-7s %%s\\n\" format in doctor RunE",
			out.String(), want)
	}
}

// TestRootCmd_EditCmd_BinderParseError_ShowsCannotParse verifies that pmk edit
// produces a "cannot parse binder" error message when _binder.md exists but is
// malformed, per plan.md §Binder Parse Failure in pmk edit case (3).
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
