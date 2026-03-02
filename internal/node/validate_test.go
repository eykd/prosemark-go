package node_test

import (
	"strings"
	"testing"

	node "github.com/eykd/prosemark-go/internal/node"
)

// TestValidateFieldValue verifies control-character detection with correct
// semantics: horizontal tab and the other C0 whitespace escapes (0x09–0x0D)
// are allowed; all other control characters are rejected.
func TestValidateFieldValue(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "clean ASCII string",
			input:   "Hello, world!",
			wantErr: false,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: false,
		},
		{
			name:    "tab (0x09) is allowed",
			input:   "col1\tcol2",
			wantErr: false,
		},
		{
			name:    "newline (0x0A) is allowed",
			input:   "line1\nline2",
			wantErr: false,
		},
		{
			name:    "vertical tab (0x0B) is allowed",
			input:   "a\x0Bb",
			wantErr: false,
		},
		{
			name:    "form feed (0x0C) is allowed",
			input:   "a\x0Cb",
			wantErr: false,
		},
		{
			name:    "carriage return (0x0D) is allowed",
			input:   "a\rb",
			wantErr: false,
		},
		{
			name:    "null byte (0x00) is rejected",
			input:   "a\x00b",
			wantErr: true,
		},
		{
			name:    "SOH (0x01) is rejected",
			input:   "a\x01b",
			wantErr: true,
		},
		{
			name:    "BEL (0x07) is rejected",
			input:   "a\x07b",
			wantErr: true,
		},
		{
			name:    "BS (0x08) is rejected",
			input:   "a\x08b",
			wantErr: true,
		},
		{
			name:    "SO (0x0E) is rejected",
			input:   "a\x0Eb",
			wantErr: true,
		},
		{
			name:    "US (0x1F) is rejected",
			input:   "a\x1Fb",
			wantErr: true,
		},
		{
			name:    "DEL (0x7F) is rejected",
			input:   "a\x7Fb",
			wantErr: true,
		},
		{
			name:    "unicode non-control char is allowed",
			input:   "café",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := node.ValidateFieldValue(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFieldValue(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

// TestValidateNewNodeInput verifies domain invariants for new node creation:
// target format, title/synopsis presence, length limits, and character constraints.
func TestValidateNewNodeInput(t *testing.T) {
	validUUID := "0192f0c1-3e7a-7000-8000-5a4b3c2d1e0f.md"
	longTitle := strings.Repeat("a", 501)
	longSynopsis := strings.Repeat("a", 2001)

	tests := []struct {
		name     string
		target   string
		title    string
		synopsis string
		wantErr  bool
	}{
		{
			name:     "empty target with title is valid",
			target:   "",
			title:    "A good title",
			synopsis: "",
			wantErr:  false,
		},
		{
			name:     "valid UUID target with title is valid",
			target:   validUUID,
			title:    "A good title",
			synopsis: "",
			wantErr:  false,
		},
		{
			name:     "synopsis alone (no title) is valid",
			target:   "",
			title:    "",
			synopsis: "just a synopsis",
			wantErr:  false,
		},
		{
			name:     "both title and synopsis provided is valid",
			target:   "",
			title:    "Title",
			synopsis: "Synopsis",
			wantErr:  false,
		},
		{
			name:     "empty title and synopsis is rejected",
			target:   "",
			title:    "",
			synopsis: "",
			wantErr:  true,
		},
		{
			name:     "target with path separator is rejected",
			target:   "subdir/file.md",
			title:    "Title",
			synopsis: "",
			wantErr:  true,
		},
		{
			name:     "non-UUID target filename is rejected",
			target:   "chapter-one.md",
			title:    "Title",
			synopsis: "",
			wantErr:  true,
		},
		{
			name:     "uppercase UUID target is rejected",
			target:   "0192F0C1-3E7A-7000-8000-5A4B3C2D1E0F.md",
			title:    "Title",
			synopsis: "",
			wantErr:  true,
		},
		{
			name:     "title exceeding 500 chars is rejected",
			target:   "",
			title:    longTitle,
			synopsis: "",
			wantErr:  true,
		},
		{
			name:     "title of exactly 500 chars is valid",
			target:   "",
			title:    strings.Repeat("a", 500),
			synopsis: "",
			wantErr:  false,
		},
		{
			name:     "synopsis exceeding 2000 chars is rejected",
			target:   "",
			title:    "Title",
			synopsis: longSynopsis,
			wantErr:  true,
		},
		{
			name:     "synopsis of exactly 2000 chars is valid",
			target:   "",
			title:    "Title",
			synopsis: strings.Repeat("a", 2000),
			wantErr:  false,
		},
		{
			name:     "title with null byte is rejected",
			target:   "",
			title:    "bad\x00title",
			synopsis: "",
			wantErr:  true,
		},
		{
			name:     "synopsis with null byte is rejected",
			target:   "",
			title:    "Title",
			synopsis: "bad\x00synopsis",
			wantErr:  true,
		},
		{
			name:     "title with tab is allowed (correct semantics)",
			target:   "",
			title:    "title\twith\ttab",
			synopsis: "",
			wantErr:  false,
		},
		{
			name:     "synopsis with tab is allowed (correct semantics)",
			target:   "",
			title:    "Title",
			synopsis: "synopsis\twith\ttab",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := node.ValidateNewNodeInput(tt.target, tt.title, tt.synopsis)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateNewNodeInput(%q, %q, %q) error = %v, wantErr %v",
					tt.target, tt.title, tt.synopsis, err, tt.wantErr)
			}
		})
	}
}
