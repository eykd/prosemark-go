package binder_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/eykd/prosemark-go/internal/binder"
)

// TestSerialize_RoundTrip verifies that Serialize(Parse(src)) == src for a variety of inputs.
func TestSerialize_RoundTrip(t *testing.T) {
	project := &binder.Project{Version: "1", Files: []string{}}

	tests := []struct {
		name string
		src  []byte
	}{
		{
			name: "single line with LF",
			src:  []byte("<!-- prosemark-binder:v1 -->\n"),
		},
		{
			name: "single line with CRLF",
			src:  []byte("<!-- prosemark-binder:v1 -->\r\n"),
		},
		{
			name: "single line with bare CR",
			src:  []byte("<!-- prosemark-binder:v1 -->\r"),
		},
		{
			name: "multiple lines LF",
			src:  []byte("<!-- prosemark-binder:v1 -->\n\n- [Chapter One](chapter-one.md)\n"),
		},
		{
			name: "multiple lines CRLF",
			src:  []byte("<!-- prosemark-binder:v1 -->\r\n\r\n- [Chapter One](chapter-one.md)\r\n"),
		},
		{
			name: "mixed line endings",
			src:  []byte("<!-- prosemark-binder:v1 -->\r\n\n- [Chapter One](chapter-one.md)\r"),
		},
		{
			name: "no trailing newline",
			src:  []byte("<!-- prosemark-binder:v1 -->\n- [Chapter One](chapter-one.md)"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := binder.Parse(context.Background(), tt.src, project)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			got := binder.Serialize(result)

			if !bytes.Equal(got, tt.src) {
				t.Errorf("Serialize(Parse(src)) round-trip mismatch\ngot:  %q\nwant: %q", got, tt.src)
			}
		})
	}
}

// TestSerialize_EmptyInput verifies that an empty (zero-byte) input survives the round-trip.
func TestSerialize_EmptyInput(t *testing.T) {
	project := &binder.Project{Version: "1", Files: []string{}}
	src := []byte{}

	result, _, err := binder.Parse(context.Background(), src, project)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	got := binder.Serialize(result)

	if !bytes.Equal(got, src) {
		t.Errorf("Serialize(Parse(empty)) = %q, want empty slice", got)
	}
}

// TestSerialize_BOM verifies that a file with a UTF-8 BOM survives the round-trip byte-for-byte.
func TestSerialize_BOM(t *testing.T) {
	const bom = "\xEF\xBB\xBF"
	project := &binder.Project{Version: "1", Files: []string{}}

	tests := []struct {
		name string
		src  []byte
	}{
		{
			name: "BOM only (no other content)",
			src:  []byte(bom),
		},
		{
			name: "BOM followed by content",
			src:  []byte(bom + "<!-- prosemark-binder:v1 -->\n- [Doc](doc.md)\n"),
		},
		{
			name: "BOM with CRLF line endings",
			src:  []byte(bom + "<!-- prosemark-binder:v1 -->\r\n- [Doc](doc.md)\r\n"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := binder.Parse(context.Background(), tt.src, project)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			got := binder.Serialize(result)

			if !bytes.Equal(got, tt.src) {
				t.Errorf("Serialize(Parse(src)) BOM round-trip mismatch\ngot:  %q\nwant: %q", got, tt.src)
			}
		})
	}
}

// TestSerialize_UntouchedLinesAreByteIdentical verifies that lines not modified by any
// operation are reproduced exactly, including all whitespace and punctuation.
func TestSerialize_UntouchedLinesAreByteIdentical(t *testing.T) {
	project := &binder.Project{Version: "1", Files: []string{}}

	// Deliberately unusual whitespace and characters that must pass through unchanged.
	src := []byte(
		"<!-- prosemark-binder:v1 -->\n" +
			"  \t  \n" + // line with mixed whitespace
			"- [Chapter](chapter.md)\n" +
			"   - [Sub Chapter](sub/chapter.md)\n" +
			"Some prose paragraph text here.\n" +
			"  - [Nested](nested.md)\n",
	)

	result, _, err := binder.Parse(context.Background(), src, project)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	got := binder.Serialize(result)

	if !bytes.Equal(got, src) {
		t.Errorf("untouched-line round-trip failed\ngot:  %q\nwant: %q", got, src)
	}
}

// TestSerialize_ReturnType verifies that Serialize returns []byte (compilation check).
func TestSerialize_ReturnType(t *testing.T) {
	project := &binder.Project{Version: "1", Files: []string{}}
	src := []byte("<!-- prosemark-binder:v1 -->\n")

	result, _, err := binder.Parse(context.Background(), src, project)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	got := binder.Serialize(result)

	// Verify the return value is usable as []byte.
	if got == nil {
		t.Error("Serialize() returned nil; want non-nil []byte")
	}
}
