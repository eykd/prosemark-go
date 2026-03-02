package cmd

import "strings"

// sanitizePath replaces control characters (runes < 0x20 or == 0x7F) with '?'
// before including path values in human-readable output, preventing ANSI injection.
func sanitizePath(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7F {
			return '?'
		}
		return r
	}, s)
}
