package cmd

import (
	"bufio"
	"os"
	"strings"
	"testing"
)

// TestNoStaleREDComments verifies that // RED: comments have been removed from
// test functions whose GREEN implementation is complete.  Each sub-test checks
// one specific line that previously carried a stale RED annotation.
func TestNoStaleREDComments(t *testing.T) {
	checks := []struct {
		file    string
		lineNum int
		context string
	}{
		{
			file:    "addchild_new_test.go",
			lineNum: 290,
			context: "TestNewAddChildCmd_NewMode_ErrorDiagnosticRollsBackNode",
		},
		{
			file:    "addchild_node_construction_test.go",
			lineNum: 75,
			context: "TestNewAddChildCmd_NewMode_EmptyTitleAbsentFromNodeContent",
		},
		{
			file:    "addchild_node_construction_test.go",
			lineNum: 107,
			context: "TestNewAddChildCmd_NewMode_RefreshPreservesEditorBodyContent",
		},
	}

	for _, tc := range checks {
		t.Run(tc.context, func(t *testing.T) {
			f, err := os.Open(tc.file)
			if err != nil {
				t.Fatalf("could not open %s: %v", tc.file, err)
			}
			defer f.Close() //nolint:errcheck

			scanner := bufio.NewScanner(f)
			lineNum := 0
			for scanner.Scan() {
				lineNum++
				if lineNum == tc.lineNum {
					line := scanner.Text()
					if strings.Contains(line, "// RED:") {
						t.Errorf("%s:%d still contains stale // RED: comment: %q",
							tc.file, tc.lineNum, strings.TrimSpace(line))
					}
					break
				}
			}
			if err := scanner.Err(); err != nil {
				t.Fatalf("error reading %s: %v", tc.file, err)
			}
		})
	}
}
