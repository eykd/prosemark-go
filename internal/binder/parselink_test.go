package binder

import (
	"testing"
)

func TestParseLink(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		wantTarget string
		wantTitle  string
		wantFound  bool
		wantDiags  int
	}{
		{"placeholder with title", "[Title]()", "", "Title", true, 0},
		{"placeholder empty title", "[]()", "", "", true, 0},
		{"placeholder whitespace title", "[ ]()", "", "", true, 0},
		{"placeholder whitespace target", "[Title]( )", "", "Title", true, 0},
		{"regular inline link", "[T](x.md)", "x.md", "T", true, 0},
		{"regular inline link with tooltip", `[T](x.md "Tip")`, "x.md", "T", true, 0},
		{"unrecognised text", "just text", "", "", false, 0},
		{"ref link absent", "[Title][nonexistent-ref]", "", "", false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, title, found, diags := parseLink(tt.content, nil, nil, "", 1, 1)
			if target != tt.wantTarget {
				t.Errorf("target = %q, want %q", target, tt.wantTarget)
			}
			if title != tt.wantTitle {
				t.Errorf("title = %q, want %q", title, tt.wantTitle)
			}
			if found != tt.wantFound {
				t.Errorf("found = %v, want %v", found, tt.wantFound)
			}
			if len(diags) != tt.wantDiags {
				t.Errorf("len(diags) = %d, want %d", len(diags), tt.wantDiags)
			}
		})
	}
}
