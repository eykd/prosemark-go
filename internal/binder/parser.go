package binder

import (
	"context"
	"regexp"
	"strings"
)

var (
	pragmaRE = regexp.MustCompile(`<!--\s*prosemark-binder:v1\s*-->`)
	linkRE   = regexp.MustCompile(`\[[^\]]*\]\([^)]*\)`)
)

// Parse parses a binder file and returns a ParseResult, diagnostics, and any fatal error.
// project may be nil.
func Parse(ctx context.Context, src []byte, project *Project) (*ParseResult, []Diagnostic, error) {
	_ = ctx
	_ = project

	var diags []Diagnostic

	result := &ParseResult{
		Version: "1",
		Root: &Node{
			Type:     "root",
			Children: []*Node{},
		},
		RefDefs: make(map[string]RefDef),
	}

	// Strip UTF-8 BOM if present.
	if len(src) >= 3 && src[0] == 0xef && src[1] == 0xbb && src[2] == 0xbf {
		result.HasBOM = true
		src = src[3:]
		diags = append(diags, Diagnostic{
			Severity: "warning",
			Code:     CodeBOMPresence,
			Message:  "UTF-8 BOM detected",
		})
	}

	// Split into lines, recording endings per line.
	result.Lines, result.LineEnds = splitLines(src)

	// Scan lines: track fences, detect pragma, detect links in fences.
	inFence := false
	fenceMarker := ""

	for i, line := range result.Lines {
		lineNum := i + 1

		if !inFence {
			switch {
			case strings.HasPrefix(line, "```"):
				inFence = true
				fenceMarker = "```"
			case strings.HasPrefix(line, "~~~"):
				inFence = true
				fenceMarker = "~~~"
			default:
				if !result.HasPragma && pragmaRE.MatchString(line) {
					result.HasPragma = true
					result.PragmaLine = lineNum
				}
			}
		} else {
			if strings.HasPrefix(line, fenceMarker) {
				inFence = false
				fenceMarker = ""
			} else if linkRE.MatchString(line) {
				diags = append(diags, Diagnostic{
					Severity: "warning",
					Code:     CodeLinkInCodeFence,
					Message:  "link inside fenced code block will be ignored",
					Location: &Location{Line: lineNum},
				})
			}
		}
	}

	// Emit BNDW001 if no effective pragma found.
	if !result.HasPragma {
		diags = append(diags, Diagnostic{
			Severity: "warning",
			Code:     CodeMissingPragma,
			Message:  "prosemark-binder:v1 pragma not found",
		})
	}

	return result, diags, nil
}

// splitLines splits src into lines and their corresponding line endings.
// Lines do not include the ending characters.
// A trailing newline does not produce an extra empty line.
func splitLines(src []byte) ([]string, []string) {
	if len(src) == 0 {
		return []string{}, []string{}
	}

	var lines []string
	var ends []string
	start := 0

	for i := 0; i < len(src); {
		switch src[i] {
		case '\n':
			lines = append(lines, string(src[start:i]))
			ends = append(ends, "\n")
			i++
			start = i
		case '\r':
			end := "\r"
			advance := 1
			if i+1 < len(src) && src[i+1] == '\n' {
				end = "\r\n"
				advance = 2
			}
			lines = append(lines, string(src[start:i]))
			ends = append(ends, end)
			i += advance
			start = i
		default:
			i++
		}
	}

	// Handle remaining content (file does not end with a newline).
	if start < len(src) {
		lines = append(lines, string(src[start:]))
		ends = append(ends, "")
	}

	return lines, ends
}
