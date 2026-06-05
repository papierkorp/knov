// Package testdata - copied (embedded) test files and their metadata
package testdata

import (
	"io/fs"
	"os"
	"path/filepath"

	"knov/internal/contentStorage"
	"knov/internal/files"
	"knov/internal/logging"
)

// copyTestFiles copies the embedded testfiles into docs/test/
func copyTestFiles() error {
	logging.LogInfo("copying test files")

	docsPath := contentStorage.GetDocsPath()
	if err := os.MkdirAll(filepath.Join(docsPath, "test"), 0755); err != nil {
		return err
	}

	srcDir := "internal/testdata/testfiles"

	return fs.WalkDir(testFilesFS, srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if path == srcDir {
			return nil
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(docsPath, "test", relPath)

		if d.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}

		data, err := testFilesFS.ReadFile(path)
		if err != nil {
			return err
		}

		return os.WriteFile(destPath, data, 0644)
	})
}

// getCopiedFilesMetadata returns metadata for the embedded test files
func getCopiedFilesMetadata() []*files.Metadata {
	return []*files.Metadata{
		{
			Path:   "docs/test/example_markdown.md",
			Tags:   []string{"test-markdown", "test-files", "kb-status-inbox"},
			Editor: files.EditorTypeMarkdown,
		},
		{
			Path:   "docs/test/sample Markdown.md",
			Tags:   []string{"test-markdown", "test-files", "kb-status-inbox"},
			Editor: files.EditorTypeMarkdown,
		},
		{
			Path:   "docs/test/example_text.md",
			Tags:   []string{"test-text", "test-files", "kb-status-inbox"},
			Editor: files.EditorTypeTextarea,
		},
		{
			Path:   "docs/test/example_list.md",
			Tags:   []string{"test-list", "test-files", "kb-status-inbox"},
			Editor: files.EditorTypeList,
		},
		{
			Path:   "docs/test/example_todo.md",
			Tags:   []string{"test-todo", "test-files", "kb-status-inbox"},
			Editor: files.EditorTypeTodo,
		},
		{
			Path:   "docs/test/example_filter.md",
			Tags:   []string{"test-filter", "test-files", "kb-status-inbox"},
			Editor: files.EditorTypeFilter,
		},
		{
			Path:   "docs/test/example_index.md",
			Tags:   []string{"test-index", "test-files", "kb-status-inbox"},
			Editor: files.EditorTypeIndex,
		},
	}
}
