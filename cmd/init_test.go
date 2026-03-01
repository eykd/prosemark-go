package cmd

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// mockInitIO is a test double for InitIO.
type mockInitIO struct {
	binderExists  bool
	binderStatErr error
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
		name := name
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
	}

	for _, tt := range tests {
		tt := tt
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
