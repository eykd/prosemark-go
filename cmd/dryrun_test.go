package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
	"github.com/spf13/cobra"
)

// withDryRunFlag wraps a subcommand under a parent that provides the --dry-run
// persistent flag, matching how NewRootCmd wires it.
func withDryRunFlag(sub *cobra.Command) *cobra.Command {
	root := &cobra.Command{Use: "test-root", SilenceErrors: true}
	root.PersistentFlags().Bool("dry-run", false, "preview changes without writing to disk")
	root.AddCommand(sub)
	return root
}

// --- addchild dry-run tests ---

func TestAddChildCmd_DryRun_SkipsWrite(t *testing.T) {
	mock := &mockAddChildIO{
		binderBytes: acBinder(),
		project:     &binder.Project{Files: []string{"chapter-two.md"}, BinderDir: "."},
	}
	sub := NewAddChildCmd(mock)
	root := withDryRunFlag(sub)
	root.SetOut(new(bytes.Buffer))
	root.SetArgs([]string{"add", "--parent", ".", "--target", "chapter-two.md", "--project", ".", "--dry-run"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mock.writtenPath != "" {
		t.Errorf("dry-run must not write binder, but was written to %q", mock.writtenPath)
	}
	if len(mock.writtenBytes) != 0 {
		t.Error("dry-run must not write any bytes")
	}
}

func TestAddChildCmd_DryRun_JSONOutput(t *testing.T) {
	mock := &mockAddChildIO{
		binderBytes: acBinder(),
		project:     &binder.Project{Files: []string{"chapter-two.md"}, BinderDir: "."},
	}
	sub := NewAddChildCmd(mock)
	root := withDryRunFlag(sub)
	out := new(bytes.Buffer)
	sub.SetOut(out)
	root.SetArgs([]string{"add", "--parent", ".", "--target", "chapter-two.md", "--project", ".", "--json", "--dry-run"})

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

func TestAddChildCmd_DryRun_HumanOutputPrefix(t *testing.T) {
	mock := &mockAddChildIO{
		binderBytes: acBinder(),
		project:     &binder.Project{Files: []string{"chapter-two.md"}, BinderDir: "."},
	}
	sub := NewAddChildCmd(mock)
	root := withDryRunFlag(sub)
	out := new(bytes.Buffer)
	sub.SetOut(out)
	root.SetArgs([]string{"add", "--parent", ".", "--target", "chapter-two.md", "--project", ".", "--dry-run"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasPrefix(out.String(), "dry-run:") {
		t.Errorf("expected human output prefixed with 'dry-run:', got: %q", out.String())
	}
}

func TestAddChildCmd_DryRun_ExitCodeOnError(t *testing.T) {
	// Selector that matches no parent → error diagnostic → non-zero exit
	mock := &mockAddChildIO{
		binderBytes: acBinder(),
	}
	sub := NewAddChildCmd(mock)
	root := withDryRunFlag(sub)
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"add", "--parent", "nonexistent.md", "--target", "chapter-two.md", "--project", ".", "--dry-run"})

	err := root.Execute()
	if err == nil {
		t.Error("expected non-zero exit when op has error diagnostics in dry-run mode")
	}
}

func TestAddChildCmd_DryRun_ExistingTarget_ShowsSkipped(t *testing.T) {
	// When --dry-run is set and the target already exists in the binder,
	// the output should show "already in ... (skipped)", not "dry-run: Added ...".
	mock := &mockAddChildIO{
		binderBytes: acBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md"}, BinderDir: "."},
	}
	sub := NewAddChildCmd(mock)
	root := withDryRunFlag(sub)
	out := new(bytes.Buffer)
	sub.SetOut(out)
	// chapter-one.md is already in acBinder(), so this is a no-op.
	root.SetArgs([]string{"add", "--parent", ".", "--target", "chapter-one.md", "--project", ".", "--dry-run"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := out.String()
	if strings.HasPrefix(got, "dry-run:") {
		t.Errorf("target already in binder: should not show dry-run prefix, got: %q", got)
	}
	if !strings.Contains(got, "already in") {
		t.Errorf("expected 'already in' message for existing target, got: %q", got)
	}
	if !strings.Contains(got, "(skipped)") {
		t.Errorf("expected '(skipped)' suffix for existing target, got: %q", got)
	}
}

// --- delete dry-run tests ---

func TestDeleteCmd_DryRun_SkipsWrite(t *testing.T) {
	mock := &mockDeleteIO{
		binderBytes: delBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md"}, BinderDir: "."},
	}
	sub := NewDeleteCmd(mock)
	root := withDryRunFlag(sub)
	root.SetOut(new(bytes.Buffer))
	root.SetArgs([]string{"delete", "--selector", "chapter-one.md", "--yes", "--project", ".", "--dry-run"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mock.writtenPath != "" {
		t.Errorf("dry-run must not write binder, but was written to %q", mock.writtenPath)
	}
	if len(mock.writtenBytes) != 0 {
		t.Error("dry-run must not write any bytes")
	}
}

func TestDeleteCmd_DryRun_JSONOutput(t *testing.T) {
	mock := &mockDeleteIO{
		binderBytes: delBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md"}, BinderDir: "."},
	}
	sub := NewDeleteCmd(mock)
	root := withDryRunFlag(sub)
	out := new(bytes.Buffer)
	sub.SetOut(out)
	root.SetArgs([]string{"delete", "--selector", "chapter-one.md", "--yes", "--json", "--project", ".", "--dry-run"})

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

func TestDeleteCmd_DryRun_HumanOutputPrefix(t *testing.T) {
	mock := &mockDeleteIO{
		binderBytes: delBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md"}, BinderDir: "."},
	}
	sub := NewDeleteCmd(mock)
	root := withDryRunFlag(sub)
	out := new(bytes.Buffer)
	sub.SetOut(out)
	root.SetArgs([]string{"delete", "--selector", "chapter-one.md", "--yes", "--project", ".", "--dry-run"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasPrefix(out.String(), "dry-run:") {
		t.Errorf("expected human output prefixed with 'dry-run:', got: %q", out.String())
	}
}

func TestDeleteCmd_DryRun_BypassesYesFlag(t *testing.T) {
	// When --dry-run is set, --yes should NOT be required.
	mock := &mockDeleteIO{
		binderBytes: delBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md"}, BinderDir: "."},
	}
	sub := NewDeleteCmd(mock)
	root := withDryRunFlag(sub)
	out := new(bytes.Buffer)
	sub.SetOut(out)
	root.SetArgs([]string{"delete", "--selector", "chapter-one.md", "--project", ".", "--dry-run"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("dry-run should bypass --yes requirement, got error: %v", err)
	}
}

func TestDeleteCmd_DryRun_ExitCodeOnError(t *testing.T) {
	// Selector that matches no node → OPE001 → non-zero exit, even in dry-run
	mock := &mockDeleteIO{
		binderBytes: delBinder(),
	}
	sub := NewDeleteCmd(mock)
	root := withDryRunFlag(sub)
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"delete", "--selector", "nonexistent.md", "--yes", "--project", ".", "--dry-run"})

	err := root.Execute()
	if err == nil {
		t.Error("expected non-zero exit when op has error diagnostics in dry-run mode")
	}
}

// --- move dry-run tests ---

func TestMoveCmd_DryRun_SkipsWrite(t *testing.T) {
	mock := &mockMoveIO{
		binderBytes: moveBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md", "chapter-two.md"}, BinderDir: "."},
	}
	sub := NewMoveCmd(mock)
	root := withDryRunFlag(sub)
	root.SetOut(new(bytes.Buffer))
	root.SetArgs([]string{"move", "--source", "chapter-two.md", "--dest", "chapter-one.md", "--yes", "--project", ".", "--dry-run"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mock.writtenPath != "" {
		t.Errorf("dry-run must not write binder, but was written to %q", mock.writtenPath)
	}
	if len(mock.writtenBytes) != 0 {
		t.Error("dry-run must not write any bytes")
	}
}

func TestMoveCmd_DryRun_JSONOutput(t *testing.T) {
	mock := &mockMoveIO{
		binderBytes: moveBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md", "chapter-two.md"}, BinderDir: "."},
	}
	sub := NewMoveCmd(mock)
	root := withDryRunFlag(sub)
	out := new(bytes.Buffer)
	sub.SetOut(out)
	root.SetArgs([]string{"move", "--source", "chapter-two.md", "--dest", "chapter-one.md", "--yes", "--json", "--project", ".", "--dry-run"})

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

func TestMoveCmd_DryRun_HumanOutputPrefix(t *testing.T) {
	mock := &mockMoveIO{
		binderBytes: moveBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md", "chapter-two.md"}, BinderDir: "."},
	}
	sub := NewMoveCmd(mock)
	root := withDryRunFlag(sub)
	out := new(bytes.Buffer)
	sub.SetOut(out)
	root.SetArgs([]string{"move", "--source", "chapter-two.md", "--dest", "chapter-one.md", "--yes", "--project", ".", "--dry-run"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasPrefix(out.String(), "dry-run:") {
		t.Errorf("expected human output prefixed with 'dry-run:', got: %q", out.String())
	}
}

func TestMoveCmd_DryRun_BypassesYesFlag(t *testing.T) {
	// When --dry-run is set, --yes should NOT be required.
	mock := &mockMoveIO{
		binderBytes: moveBinder(),
		project:     &binder.Project{Files: []string{"chapter-one.md", "chapter-two.md"}, BinderDir: "."},
	}
	sub := NewMoveCmd(mock)
	root := withDryRunFlag(sub)
	out := new(bytes.Buffer)
	sub.SetOut(out)
	root.SetArgs([]string{"move", "--source", "chapter-two.md", "--dest", "chapter-one.md", "--project", ".", "--dry-run"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("dry-run should bypass --yes requirement, got error: %v", err)
	}
}

func TestMoveCmd_DryRun_ExitCodeOnError(t *testing.T) {
	// Selector that matches no node → OPE001 → non-zero exit, even in dry-run
	mock := &mockMoveIO{
		binderBytes: moveBinder(),
	}
	sub := NewMoveCmd(mock)
	root := withDryRunFlag(sub)
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"move", "--source", "nonexistent.md", "--dest", "chapter-one.md", "--yes", "--project", ".", "--dry-run"})

	err := root.Execute()
	if err == nil {
		t.Error("expected non-zero exit when op has error diagnostics in dry-run mode")
	}
}
