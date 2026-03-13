package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
)

// TestNewAddChildCmd_NewMode_JSON_BinderWriteFailure verifies that when
// --new --json is used and the binder write fails, the JSON output on stdout
// does NOT report changed:true. An agent parsing stdout JSON must not be
// misled into thinking the operation succeeded.
func TestNewAddChildCmd_NewMode_JSON_BinderWriteFailure(t *testing.T) {
	tests := []struct {
		name       string
		writeErr   error
		wantErr    bool
		wantNoJSON bool // stdout should contain no JSON (or JSON with changed:false/error)
	}{
		{
			name:       "binder write fails: JSON must not report changed:true",
			writeErr:   errors.New("read-only filesystem"),
			wantErr:    true,
			wantNoJSON: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockAddChildIOWithNew{
				mockAddChildIO: mockAddChildIO{
					binderBytes: emptyBinder(),
					project:     &binder.Project{Files: []string{}, BinderDir: "."},
					writeErr:    tt.writeErr,
				},
			}

			c := NewAddChildCmd(mock)
			out := new(bytes.Buffer)
			errOut := new(bytes.Buffer)
			c.SetOut(out)
			c.SetErr(errOut)
			c.SetArgs([]string{"--new", "--title", "Doomed", "--parent", ".", "--json", "--project", "."})

			err := c.Execute()
			if !tt.wantErr {
				t.Fatalf("test setup expects an error")
			}
			if err == nil {
				t.Fatal("expected error when binder write fails, got nil")
			}

			// The critical assertion: stdout must NOT contain JSON claiming
			// the operation succeeded (changed:true) when the write actually failed.
			if tt.wantNoJSON && out.Len() > 0 {
				var result binder.OpResult
				if jsonErr := json.Unmarshal(out.Bytes(), &result); jsonErr == nil {
					if result.Changed {
						t.Errorf("stdout JSON reports changed:true but binder write failed;\n"+
							"an agent parsing this output would think the operation succeeded.\n"+
							"stdout: %s", out.String())
					}
				}
			}
		})
	}
}
