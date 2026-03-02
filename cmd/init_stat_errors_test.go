package cmd

import (
	"bytes"
	"errors"
	"testing"
)

func TestNewInitCmd_BinderStatError(t *testing.T) {
	mock := newMockInitIO()
	mock.binderStatErr = errors.New("permission denied")

	c := newInitCmdWithGetCWD(mock, func() (string, error) { return ".", nil })
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--project", "."})

	if err := c.Execute(); err == nil {
		t.Error("expected error when StatFile fails for binder")
	}
}

func TestNewInitCmd_ConfigStatError(t *testing.T) {
	mock := newMockInitIO()
	mock.configStatErr = errors.New("permission denied")

	c := newInitCmdWithGetCWD(mock, func() (string, error) { return ".", nil })
	c.SetOut(new(bytes.Buffer))
	c.SetErr(new(bytes.Buffer))
	c.SetArgs([]string{"--project", "."})

	if err := c.Execute(); err == nil {
		t.Error("expected error when StatFile fails for config")
	}
}
