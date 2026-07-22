// Package testdata - copied (embedded) docs files and their metadata
package test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"knov/internal/contentStorage"
	"knov/internal/files"
	"knov/internal/logging"
)

// copyTestFiles copies the embedded docs/testfiles/ tree into the runtime docs/test/
// and the docs root markdown files into docs/test/docs/
func copyTestFiles() error {
	logging.LogInfo(logging.KeyApp, "copying test files")

	if err := copyEmbeddedDir("docs/testfiles", filepath.Join(contentStorage.GetDocsPath(), "test")); err != nil {
		return fmt.Errorf("failed to copy testfiles: %w", err)
	}

	if err := copyEmbeddedDir("docs", filepath.Join(contentStorage.GetDocsPath(), "test", "docs")); err != nil {
		return fmt.Errorf("failed to copy docs: %w", err)
	}

	return nil
}

// copyEmbeddedDir copies files from an embedded FS directory into a runtime destination.
// Only copies files directly in srcDir (non-recursive for docs root to avoid copying testfiles again).
func copyEmbeddedDir(srcDir, destBase string) error {
	if _, err := docsFS.Open(srcDir); err != nil {
		return fmt.Errorf("%s not found in embedded FS: %w", srcDir, err)
	}

	if err := os.MkdirAll(destBase, 0755); err != nil {
		return err
	}

	return fs.WalkDir(docsFS, srcDir, func(path string, d fs.DirEntry, err error) error {
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

		// for docs root: skip subdirectories entirely (testfiles, old, temp folders)
		if srcDir == "docs" && d.IsDir() {
			return fs.SkipDir
		}

		destPath := filepath.Join(destBase, relPath)

		if d.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}

		data, err := docsFS.ReadFile(path)
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
			Editor: files.EditorTypeToastUI,
		},
		{
			Path:   "docs/test/sample Markdown.md",
			Tags:   []string{"test-markdown", "test-files", "kb-status-inbox"},
			Editor: files.EditorTypeToastUI,
			References: []files.Reference{
				{URL: "https://example.com", Description: "example reference for testing"},
				{URL: "https://www.google.com", Description: "another reference"},
			},
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
			Path:   "docs/test/example_index.md",
			Tags:   []string{"test-index", "test-files", "kb-status-inbox"},
			Editor: files.EditorTypeIndex,
		},
	}
}
