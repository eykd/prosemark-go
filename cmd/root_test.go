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
