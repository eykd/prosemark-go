package cmd

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestMissingFlagError_EmitError(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetOut(&errWriter{err: errors.New("write error")})
	cmd.SetErr(new(bytes.Buffer))

	err := missingFlagError(cmd, true, false, "test", "--flag")
	if err == nil {
		t.Fatal("expected error when emitOpResult fails")
	}
	if !strings.Contains(err.Error(), "encoding output") {
		t.Errorf("expected encoding error, got: %v", err)
	}
}

func TestMissingFlagError_TextMode(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))

	err := missingFlagError(cmd, false, false, "test", "--flag")
	if err == nil {
		t.Fatal("expected error")
	}
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected *ExitError, got %T", err)
	}
	if exitErr.Code != ExitUsage {
		t.Errorf("exit code = %d, want %d", exitErr.Code, ExitUsage)
	}
}
