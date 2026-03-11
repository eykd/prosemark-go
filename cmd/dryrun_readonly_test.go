package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
)

// --- parse: --dry-run is a no-op (FR-018) ---

func TestParseCmd_DryRun_AcceptedWithoutError(t *testing.T) {
	reader := &mockParseReader{
		binderBytes: []byte("<!-- prosemark-binder:v1 -->\n- [Chapter One](ch1.md)\n"),
		project:     &binder.Project{Files: []string{"ch1.md"}, BinderDir: "."},
	}
	sub := NewParseCmd(reader)
	root := withDryRunFlag(sub)
	sub.SetOut(new(bytes.Buffer))
	root.SetArgs([]string{"parse", "--project", ".", "--dry-run"})

	if err := root.Execute(); err != nil {
		t.Fatalf("parse with --dry-run should not error: %v", err)
	}
}

func TestParseCmd_DryRun_IdenticalOutput(t *testing.T) {
	makeReader := func() *mockParseReader {
		return &mockParseReader{
			binderBytes: []byte("<!-- prosemark-binder:v1 -->\n- [Chapter One](ch1.md)\n"),
			project:     &binder.Project{Files: []string{"ch1.md"}, BinderDir: "."},
		}
	}

	// Run without --dry-run.
	cmdNormal := NewParseCmd(makeReader())
	outNormal := new(bytes.Buffer)
	cmdNormal.SetOut(outNormal)
	cmdNormal.SetArgs([]string{"--project", "."})
	if err := cmdNormal.Execute(); err != nil {
		t.Fatalf("parse without --dry-run: %v", err)
	}

	// Run with --dry-run.
	cmdDry := NewParseCmd(makeReader())
	root := withDryRunFlag(cmdDry)
	outDry := new(bytes.Buffer)
	cmdDry.SetOut(outDry)
	root.SetArgs([]string{"parse", "--project", ".", "--dry-run"})
	if err := root.Execute(); err != nil {
		t.Fatalf("parse with --dry-run: %v", err)
	}

	if outNormal.String() != outDry.String() {
		t.Errorf("output differs with --dry-run:\nwithout: %q\nwith:    %q", outNormal.String(), outDry.String())
	}
}

func TestParseCmd_DryRun_IdenticalExitCodeOnError(t *testing.T) {
	makeReader := func() *mockParseReader {
		return &mockParseReader{
			binderBytes: []byte("<!-- prosemark-binder:v1 -->\n- [Escape](../secret.md)\n"),
		}
	}

	// Without --dry-run: error exit.
	cmdNormal := NewParseCmd(makeReader())
	cmdNormal.SetOut(new(bytes.Buffer))
	cmdNormal.SetErr(new(bytes.Buffer))
	cmdNormal.SetArgs([]string{"--project", "."})
	errNormal := cmdNormal.Execute()

	// With --dry-run: same error exit.
	cmdDry := NewParseCmd(makeReader())
	root := withDryRunFlag(cmdDry)
	cmdDry.SetOut(new(bytes.Buffer))
	cmdDry.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"parse", "--project", ".", "--dry-run"})
	errDry := root.Execute()

	if (errNormal == nil) != (errDry == nil) {
		t.Errorf("exit code mismatch: without=%v, with=%v", errNormal, errDry)
	}
}

// --- doctor: --dry-run is a no-op (FR-018) ---

func TestDoctorCmd_DryRun_AcceptedWithoutError(t *testing.T) {
	mock := &mockDoctorIO{
		binderBytes: doctorBinderWithNode(doctorTestNodeUUID),
		nodeFiles: map[string]nodeFileEntry{
			doctorTestNodeUUID + ".md": {content: validDoctorNodeContent(doctorTestNodeUUID), exists: true},
			".prosemark.yml":           {content: []byte("version: \"1\"\n"), exists: true},
		},
	}
	sub := NewDoctorCmd(mock)
	root := withDryRunFlag(sub)
	sub.SetOut(new(bytes.Buffer))
	sub.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"doctor", "--project", ".", "--dry-run"})

	if err := root.Execute(); err != nil {
		t.Fatalf("doctor with --dry-run should not error: %v", err)
	}
}

func TestDoctorCmd_DryRun_IdenticalOutput_JSON(t *testing.T) {
	makeMock := func() *mockDoctorIO {
		return &mockDoctorIO{
			binderBytes: doctorBinderWithNode(doctorTestNodeUUID),
			nodeFiles: map[string]nodeFileEntry{
				doctorTestNodeUUID + ".md": {content: validDoctorNodeContent(doctorTestNodeUUID), exists: true},
				".prosemark.yml":           {content: []byte("version: \"1\"\n"), exists: true},
			},
		}
	}

	// Without --dry-run.
	cmdNormal := NewDoctorCmd(makeMock())
	outNormal := new(bytes.Buffer)
	cmdNormal.SetOut(outNormal)
	cmdNormal.SetErr(new(bytes.Buffer))
	cmdNormal.SetArgs([]string{"--project", ".", "--json"})
	if err := cmdNormal.Execute(); err != nil {
		t.Fatalf("doctor --json without --dry-run: %v", err)
	}

	// With --dry-run.
	cmdDry := NewDoctorCmd(makeMock())
	root := withDryRunFlag(cmdDry)
	outDry := new(bytes.Buffer)
	cmdDry.SetOut(outDry)
	cmdDry.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"doctor", "--project", ".", "--json", "--dry-run"})
	if err := root.Execute(); err != nil {
		t.Fatalf("doctor --json with --dry-run: %v", err)
	}

	// Parse both and compare structure (not raw bytes, to avoid ordering issues).
	var normal, dry doctorOutput
	if err := json.Unmarshal(outNormal.Bytes(), &normal); err != nil {
		t.Fatalf("parse normal JSON: %v\noutput: %s", err, outNormal.String())
	}
	if err := json.Unmarshal(outDry.Bytes(), &dry); err != nil {
		t.Fatalf("parse dry-run JSON: %v\noutput: %s", err, outDry.String())
	}

	if normal.Version != dry.Version {
		t.Errorf("version mismatch: normal=%q dry=%q", normal.Version, dry.Version)
	}
	if len(normal.Diagnostics) != len(dry.Diagnostics) {
		t.Errorf("diagnostics count mismatch: normal=%d dry=%d", len(normal.Diagnostics), len(dry.Diagnostics))
	}
}

func TestDoctorCmd_DryRun_IdenticalOutput_Human(t *testing.T) {
	makeMock := func() *mockDoctorIO {
		return &mockDoctorIO{
			binderBytes: doctorBinderWithNode(doctorTestNodeUUID),
			nodeFiles: map[string]nodeFileEntry{
				doctorTestNodeUUID + ".md": {content: nil, exists: false}, // AUD001 triggers
			},
		}
	}

	// Without --dry-run.
	cmdNormal := NewDoctorCmd(makeMock())
	rootNormal := withDryRunFlag(cmdNormal)
	outNormal := new(bytes.Buffer)
	errBufNormal := new(bytes.Buffer)
	cmdNormal.SetOut(outNormal)
	cmdNormal.SetErr(errBufNormal)
	rootNormal.SetArgs([]string{"doctor", "--project", "."})
	execErrNormal := rootNormal.Execute()

	// With --dry-run.
	cmdDry := NewDoctorCmd(makeMock())
	rootDry := withDryRunFlag(cmdDry)
	outDry := new(bytes.Buffer)
	errBufDry := new(bytes.Buffer)
	cmdDry.SetOut(outDry)
	cmdDry.SetErr(errBufDry)
	rootDry.SetArgs([]string{"doctor", "--project", ".", "--dry-run"})
	execErrDry := rootDry.Execute()

	// Both must return matching *ExitError (AUD001 triggers error exit).
	assertMatchingExitErrors(t, execErrNormal, execErrDry)

	if errBufNormal.String() != errBufDry.String() {
		t.Errorf("stderr differs with --dry-run:\nwithout: %q\nwith:    %q", errBufNormal.String(), errBufDry.String())
	}
	if outNormal.String() != outDry.String() {
		t.Errorf("stdout differs with --dry-run:\nwithout: %q\nwith:    %q", outNormal.String(), outDry.String())
	}
}

func TestDoctorCmd_DryRun_IdenticalExitCodeOnError(t *testing.T) {
	makeMock := func() *mockDoctorIO {
		return &mockDoctorIO{
			binderBytes: doctorBinderWithNode(doctorTestNodeUUID),
			nodeFiles: map[string]nodeFileEntry{
				doctorTestNodeUUID + ".md": {content: nil, exists: false}, // AUD001 → error exit
			},
		}
	}

	// Without --dry-run.
	cmdNormal := NewDoctorCmd(makeMock())
	cmdNormal.SetOut(new(bytes.Buffer))
	cmdNormal.SetErr(new(bytes.Buffer))
	cmdNormal.SetArgs([]string{"--project", "."})
	errNormal := cmdNormal.Execute()

	// With --dry-run.
	cmdDry := NewDoctorCmd(makeMock())
	root := withDryRunFlag(cmdDry)
	cmdDry.SetOut(new(bytes.Buffer))
	cmdDry.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"doctor", "--project", ".", "--dry-run"})
	errDry := root.Execute()

	if (errNormal == nil) != (errDry == nil) {
		t.Errorf("exit code mismatch: without=%v, with=%v", errNormal, errDry)
	}
}

// --- FR-018 contract annotation: read-only commands declare dry-run policy ---

func TestParseCmd_DryRun_AnnotatedAsNoOp(t *testing.T) {
	c := NewParseCmd(nil)
	if c.Annotations == nil || c.Annotations[dryRunAnnotationKey] != dryRunNoOp {
		t.Error("parse command must be annotated with dry-run=no-op (FR-018)")
	}
}

func TestDoctorCmd_DryRun_AnnotatedAsNoOp(t *testing.T) {
	c := NewDoctorCmd(nil)
	if c.Annotations == nil || c.Annotations[dryRunAnnotationKey] != dryRunNoOp {
		t.Error("doctor command must be annotated with dry-run=no-op (FR-018)")
	}
}

// assertMatchingExitErrors asserts both errors are non-nil *ExitError with the same exit code.
func assertMatchingExitErrors(t *testing.T, errNormal, errDry error) {
	t.Helper()

	var exitNormal *ExitError
	if !errors.As(errNormal, &exitNormal) {
		t.Fatalf("normal execution: expected *ExitError, got %T: %v", errNormal, errNormal)
	}

	var exitDry *ExitError
	if !errors.As(errDry, &exitDry) {
		t.Fatalf("dry-run execution: expected *ExitError, got %T: %v", errDry, errDry)
	}

	if exitNormal.Code != exitDry.Code {
		t.Errorf("exit code mismatch: normal=%d, dry-run=%d", exitNormal.Code, exitDry.Code)
	}
}

// --- Integration: --dry-run through NewRootCmd ---

func TestReadOnlyCommands_DryRun_AcceptedViaRootCmd(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"parse --dry-run shows help without error", []string{"parse", "--dry-run", "--help"}},
		{"doctor --dry-run shows help without error", []string{"doctor", "--dry-run", "--help"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := NewRootCmd()
			root.SetOut(new(bytes.Buffer))
			root.SetErr(new(bytes.Buffer))
			root.SetArgs(tt.args)

			if err := root.Execute(); err != nil {
				t.Errorf("expected no error for %v, got: %v", tt.args, err)
			}
		})
	}
}
