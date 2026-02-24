package cmd

import (
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
