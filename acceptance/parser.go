package acceptance

import (
	"os"
	"strings"
)

// isSeparatorLine returns true if the line consists only of ;= characters.
func isSeparatorLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if len(trimmed) == 0 {
		return false
	}
	for _, c := range trimmed {
		if c != ';' && c != '=' {
			return false
		}
	}
	return true
}

// isDescriptionLine returns true if the line is a ; comment (not a separator).
func isDescriptionLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, ";") && !isSeparatorLine(trimmed)
}

// parseKeyword extracts a GIVEN/WHEN/THEN keyword and remaining text from a line.
// Returns empty keyword if the line doesn't start with a known keyword.
func parseKeyword(line string) (keyword, text string) {
	trimmed := strings.TrimSpace(line)
	for _, kw := range []string{"GIVEN", "WHEN", "THEN"} {
		if strings.HasPrefix(trimmed, kw) {
			rest := strings.TrimSpace(trimmed[len(kw):])
			return kw, rest
		}
	}
	return "", ""
}

// ParseSpec parses a GWT spec file's content into a Feature.
// It handles ;=== separators, ; comment lines, GIVEN/WHEN/THEN keywords,
// empty lines, and multi-scenario files. This is a pure function with no I/O.
func ParseSpec(content string, sourcePath string) (*Feature, error) {
	// Normalize line endings
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")

	feature := &Feature{
		SourceFile: sourcePath,
	}

	var currentScenario *Scenario
	// Track state: looking for description after opening separator
	expectDescription := false

	for i, line := range lines {
		lineNum := i + 1 // 1-based line numbers

		if isSeparatorLine(line) {
			if expectDescription {
				// Closing separator — scenario header is complete
				expectDescription = false
				continue
			}
			// Opening separator — next ; line is the description
			expectDescription = true
			continue
		}

		if expectDescription && isDescriptionLine(line) {
			// This is the scenario description line
			desc := strings.TrimSpace(line)
			desc = strings.TrimPrefix(desc, ";")
			desc = strings.TrimSpace(desc)

			currentScenario = &Scenario{
				Description: desc,
				Line:        lineNum,
			}
			feature.Scenarios = append(feature.Scenarios, *currentScenario)
			continue
		}

		// Regular comment line (not a description)
		if isDescriptionLine(line) {
			continue
		}

		keyword, text := parseKeyword(line)
		if keyword != "" {
			// If we have steps without a scenario header, create an unnamed scenario
			if len(feature.Scenarios) == 0 {
				feature.Scenarios = append(feature.Scenarios, Scenario{})
			}
			step := Step{
				Keyword: keyword,
				Text:    text,
				Line:    lineNum,
			}
			lastIdx := len(feature.Scenarios) - 1
			feature.Scenarios[lastIdx].Steps = append(feature.Scenarios[lastIdx].Steps, step)
		}
	}

	return feature, nil
}

// ParseSpecFileImpl reads a spec file from disk and parses it.
// This is an Impl function exempt from coverage requirements.
func ParseSpecFileImpl(path string) (*Feature, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseSpec(string(data), path)
}
