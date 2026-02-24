package cmd

import (
	"context"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/eykd/prosemark-go/internal/binder"
)

// ScanProjectImpl walks the directory containing binderPath recursively,
// collecting all .md files (excluding _binder.md itself) and returns a
// *binder.Project. It is an Impl function: it performs OS filesystem
// operations and is excluded from unit test coverage calculations.
func ScanProjectImpl(_ context.Context, binderPath string) (*binder.Project, error) {
	dir := filepath.Dir(binderPath)
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if rel == "_binder.md" {
			return nil
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if files == nil {
		files = []string{}
	}
	return &binder.Project{Files: files, BinderDir: "."}, err
}
