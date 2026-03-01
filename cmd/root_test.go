package cmd

import (
	"bytes"
	"errors"
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
