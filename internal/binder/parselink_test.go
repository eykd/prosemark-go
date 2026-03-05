package binder

import (
	"context"
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

// TestParseLink_PlaceholderProducesNodeInTree verifies Phase 1.3 isPlaceholder guards.
// Each placeholder form must produce a real node in the parse tree with no spurious
// diagnostics. Guards for BNDW007, validateTarget, percentDecodeTarget, BNDW003,
// and BNDW004 are all conditioned on `!isPlaceholder`.
func TestParseLink_PlaceholderProducesNodeInTree(t *testing.T) {
	tests := []struct {
		name          string
		binder        string
		wantNodeCount int
		wantDiagCount int
	}{
		{
			name:          "[Title]() produces one node",
			binder:        "<!-- prosemark-binder:v1 -->\n\n- [Chapter 3]()\n",
			wantNodeCount: 1,
			wantDiagCount: 0,
		},
		{
			name:          "[]() produces one node",
			binder:        "<!-- prosemark-binder:v1 -->\n\n- []()\n",
			wantNodeCount: 1,
			wantDiagCount: 0,
		},
		{
			name:          "[ ]() produces one node with empty title",
			binder:        "<!-- prosemark-binder:v1 -->\n\n- [ ]()\n",
			wantNodeCount: 1,
			wantDiagCount: 0,
		},
		{
			name:          "[Title]( ) produces one node",
			binder:        "<!-- prosemark-binder:v1 -->\n\n- [Title]( )\n",
			wantNodeCount: 1,
			wantDiagCount: 0,
		},
		{
			name:          "duplicate placeholder titles produce two nodes without BNDW003",
			binder:        "<!-- prosemark-binder:v1 -->\n\n- [Chapter]()\n- [Chapter]()\n",
			wantNodeCount: 2,
			wantDiagCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, diags, err := Parse(context.Background(), []byte(tt.binder), nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result.Root.Children) != tt.wantNodeCount {
				t.Errorf("root children = %d, want %d (diags: %v)",
					len(result.Root.Children), tt.wantNodeCount, diags)
			}
			if len(diags) != tt.wantDiagCount {
				t.Errorf("diags = %v, want %d diagnostics", diags, tt.wantDiagCount)
			}
		})
	}
}

// TestParseLink_PlaceholderOnContinuationLine verifies Phase 1.2 Step C tFound promotion.
// When the primary list-item line has no link but the indented continuation line contains
// [Title](), the parser promotes it as a placeholder node by capturing tFound from
// the continuation parseLink call and promoting when tFound && t == "".
func TestParseLink_PlaceholderOnContinuationLine(t *testing.T) {
	// "- placeholder" has no link; "  [Chapter 3]()" is a continuation line.
	binder := "<!-- prosemark-binder:v1 -->\n\n- placeholder\n  [Chapter 3]()\n"
	result, diags, err := Parse(context.Background(), []byte(binder), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Root.Children) != 1 {
		t.Errorf("root children = %d, want 1 (diags: %v)", len(result.Root.Children), diags)
		return
	}
	got := result.Root.Children[0]
	if got.Title != "Chapter 3" {
		t.Errorf("title = %q, want %q", got.Title, "Chapter 3")
	}
	if got.Target != "" {
		t.Errorf("target = %q, want empty (placeholder)", got.Target)
	}
}
