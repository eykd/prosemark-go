package node

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// uuidV7FilenameRE matches lowercase UUIDv7 filenames with a .md extension.
var uuidV7FilenameRE = regexp.MustCompile(
	`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[0-9a-f]{4}-[0-9a-f]{12}\.md$`,
)

// IsUUIDFilename reports whether filename is a lowercase UUIDv7 .md filename.
func IsUUIDFilename(filename string) bool {
	return uuidV7FilenameRE.MatchString(filename)
}

// frontmatterRE matches a complete YAML frontmatter block at the start of a
// file. The closing "---" must appear unindented (at column 0); "---" inside
// YAML block scalars is always indented, so this is unambiguous without
// needing a full YAML-aware boundary scanner.
var frontmatterRE = regexp.MustCompile(`(?s)^---\n(.*?)\n---\n`)

// ParseFrontmatter splits a node file's content into its Frontmatter and body.
// yaml.v3 is used to parse the extracted YAML so that field values are decoded
// correctly (handling quoted strings, block scalars, etc.). The boundary between
// frontmatter and body is located by frontmatterRE, which matches the first
// unindented closing "---" — so "---" inside YAML block scalars (which are
// always indented by at least one space) does not cause a false split.
func ParseFrontmatter(content []byte) (Frontmatter, []byte, error) {
	loc := frontmatterRE.FindIndex(content)
	if loc == nil {
		return Frontmatter{}, nil, errors.New("no valid frontmatter block found")
	}

	yamlContent := content[:loc[1]]
	body := content[loc[1]:]

	var fm Frontmatter
	if err := yaml.Unmarshal(yamlContent, &fm); err != nil {
		return Frontmatter{}, nil, fmt.Errorf("parse frontmatter: %w", err)
	}

	return fm, append([]byte(nil), body...), nil
}

// SerializeFrontmatter serializes fm into a canonical frontmatter block.
// Field order: id → title → synopsis → created → updated.
// Empty optional fields (title, synopsis) are omitted.
// The output is wrapped in "---\n" delimiters.
func SerializeFrontmatter(fm Frontmatter) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.WriteString("id: " + fm.ID + "\n")
	if fm.Title != "" {
		buf.WriteString("title: " + fm.Title + "\n")
	}
	if fm.Synopsis != "" {
		buf.WriteString("synopsis: " + fm.Synopsis + "\n")
	}
	buf.WriteString("created: " + fm.Created + "\n")
	buf.WriteString("updated: " + fm.Updated + "\n")
	buf.WriteString("---\n")
	return buf.Bytes(), nil
}

// ValidateNode checks the given node for AUD004, AUD005, and AUD006 violations.
// It returns the set of diagnostics found; nil when the node is valid.
func ValidateNode(filenameStem string, fm Frontmatter, body []byte) []AuditDiagnostic {
	var diags []AuditDiagnostic

	// AUD004: frontmatter id does not match the filename stem.
	if fm.ID != filenameStem {
		diags = append(diags, AuditDiagnostic{
			Code:     AUD004,
			Severity: SeverityError,
			Message:  fmt.Sprintf("frontmatter id %q does not match filename stem %q", fm.ID, filenameStem),
		})
	}

	// AUD005: required field absent or not RFC3339Z.
	if fm.ID == "" || !isRFC3339Z(fm.Created) || !isRFC3339Z(fm.Updated) {
		diags = append(diags, AuditDiagnostic{
			Code:     AUD005,
			Severity: SeverityError,
			Message:  "required frontmatter field (id, created, or updated) is missing or not RFC3339Z",
		})
	}

	// AUD006: empty or whitespace-only body (warning).
	if strings.TrimSpace(string(body)) == "" {
		diags = append(diags, AuditDiagnostic{
			Code:     AUD006,
			Severity: SeverityWarning,
			Message:  "node body is empty or whitespace-only",
		})
	}

	return diags
}

// isRFC3339Z reports whether s is a valid RFC3339 timestamp with a Z suffix.
func isRFC3339Z(s string) bool {
	if !strings.HasSuffix(s, "Z") {
		return false
	}
	_, err := time.Parse(time.RFC3339, s)
	return err == nil
}
