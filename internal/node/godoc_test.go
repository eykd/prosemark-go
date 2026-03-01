package node_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestAuditCode_GoDocComments verifies that the GoDoc comments for AuditCode
// constants match the authoritative descriptions from the data model specification.
// This test parses types.go at the AST level so that incorrect or misleading
// documentation is caught as a test failure.
func TestAuditCode_GoDocComments(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed to resolve current file path")
	}
	typesFile := filepath.Join(filepath.Dir(thisFile), "types.go")

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, typesFile, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse types.go: %v", err)
	}

	docs := extractConstDocs(f)

	tests := []struct {
		constant string
		wantDoc  string
	}{
		{
			"AUD001",
			"referenced file does not exist on disk",
		},
		{
			"AUD002",
			"UUID-pattern file exists in project root but is not referenced in the binder (orphaned node)",
		},
		{
			"AUD003",
			"same file appears more than once in the binder (duplicate reference)",
		},
		{
			"AUD004",
			"node file frontmatter id does not match its filename stem",
		},
		{
			"AUD005",
			"required frontmatter field (id, created, or updated) is absent or malformed",
		},
		{
			"AUD006",
			"node file has valid frontmatter but empty or whitespace-only body (warning)",
		},
		{
			"AUD007",
			"node file YAML frontmatter block is syntactically unparseable",
		},
		{
			"AUDW001",
			"non-UUID filename linked in binder (backward-compatibility warning for Feature 001 projects)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.constant, func(t *testing.T) {
			doc, ok := docs[tt.constant]
			if !ok {
				t.Fatalf("constant %s not found in types.go; was it renamed or removed?", tt.constant)
			}
			if !strings.Contains(doc, tt.wantDoc) {
				t.Errorf(
					"GoDoc for %s does not match specification\ngot:  %q\nwant: %q",
					tt.constant, strings.TrimSpace(doc), tt.wantDoc,
				)
			}
		})
	}
}

// extractConstDocs walks an AST file and returns a map of constant name to its
// GoDoc comment text (trimmed of leading "// " prefixes by ast.CommentGroup.Text).
func extractConstDocs(f *ast.File) map[string]string {
	docs := make(map[string]string)
	for _, decl := range f.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.CONST {
			continue
		}
		for _, spec := range genDecl.Specs {
			valSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			// Prefer the per-spec doc comment; fall back to group-level doc only
			// when there is a single spec in the group.
			var commentText string
			switch {
			case valSpec.Doc != nil:
				commentText = valSpec.Doc.Text()
			case genDecl.Doc != nil && len(genDecl.Specs) == 1:
				commentText = genDecl.Doc.Text()
			}
			for _, name := range valSpec.Names {
				if commentText != "" {
					docs[name.Name] = commentText
				}
			}
		}
	}
	return docs
}
