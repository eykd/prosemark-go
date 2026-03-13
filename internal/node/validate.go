package node

import (
	"fmt"
	"os"
	"strings"
)

// ValidateFieldValue returns an error if s contains any control character not
// permitted in frontmatter field values. The range 0x09–0x0D (TAB, LF, VT,
// FF, CR) is allowed; all other characters below U+0020 and DEL (0x7F) are
// rejected.
func ValidateFieldValue(s string) error {
	if containsControlChars(s) {
		return fmt.Errorf("value contains invalid control character")
	}
	return nil
}

// containsNewline reports whether s contains LF (0x0A) or CR (0x0D).
func containsNewline(s string) bool {
	return strings.ContainsAny(s, "\n\r")
}

// ValidateNewNodeInput validates the --target, --title, and --synopsis inputs
// for --new mode. target may be empty (caller will generate one); at least one
// of title or synopsis must be non-empty.
func ValidateNewNodeInput(target, title, synopsis string) error {
	if target != "" {
		if strings.ContainsRune(target, os.PathSeparator) {
			return fmt.Errorf("target must not contain path separators")
		}
		if !IsUUIDFilename(target) {
			return fmt.Errorf("target must be a valid UUID filename when --new is set")
		}
	}
	if title == "" && synopsis == "" {
		return fmt.Errorf("--title or --synopsis is required when --new is set")
	}
	if len(title) > 500 {
		return fmt.Errorf("--title must be 500 characters or fewer")
	}
	if strings.ContainsRune(title, '\t') {
		return fmt.Errorf("--title must not contain tab characters")
	}
	if containsNewline(title) {
		return fmt.Errorf("--title must not contain newline characters")
	}
	if err := ValidateFieldValue(title); err != nil {
		return fmt.Errorf("--title must not contain control characters")
	}
	if len(synopsis) > 2000 {
		return fmt.Errorf("--synopsis must be 2000 characters or fewer")
	}
	if err := ValidateFieldValue(synopsis); err != nil {
		return fmt.Errorf("--synopsis must not contain control characters")
	}
	return nil
}
