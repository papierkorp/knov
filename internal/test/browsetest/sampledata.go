// Package browsetest - sample file/folder helpers
package browsetest

import (
	"os"
	"path/filepath"

	"knov/internal/contentStorage"
	"knov/internal/files"
	"knov/internal/pathutils"
	"knov/internal/test"
)

// testDir is the docs-relative sample folder every case seeds into, wiped and reseeded at
// the start of each run so cases never see stale state from a previous run.
const testDir = "test/browse-tests"
const subDir = testDir + "/sub"

const (
	alphaFile  = "browse-alpha.md"    // root of testDir, tagged for browse-by-tag
	betaFile   = "sub/browse-beta.md" // nested, for file-tree/folder-contents
	tocFile    = "browse-toc.md"      // multi-header file, for header/TOC extraction
	hiddenFile = "browse-hidden.md"   // todo-editor, for the hidden-file-type filter case
)

const marker = "browsetest-marker"

func testPath(name string) string {
	return pathutils.ToSlash(filepath.Join(testDir, name))
}

func writeFile(relPath, content string) error {
	full := pathutils.ToDocsPath(relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		return err
	}
	return contentStorage.WriteFile(full, []byte(content), 0644)
}

// resetAndSeed wipes the sample folder and reseeds a fixed set of files: one at the test
// folder root (tagged, for browse-by-tag/folder), one nested a level deeper (for file-tree/
// folder-contents), one with several headers (for TOC extraction) and one todo-editor file
// (for the hidden-file-type filter case).
func resetAndSeed() error {
	full := pathutils.ToDocsPath(testDir)
	if err := os.RemoveAll(full); err != nil {
		return err
	}
	if err := os.MkdirAll(full, 0755); err != nil {
		return err
	}

	if err := writeFile(testPath(alphaFile), "# Browse Alpha\n\ngeneric content\n"); err != nil {
		return err
	}
	if err := test.SeedMetadata(&files.Metadata{
		Path:   pathutils.ToWithPrefix(testPath(alphaFile)),
		Editor: files.EditorTypeCodeMirror,
		Tags:   []string{marker},
	}); err != nil {
		return err
	}

	if err := writeFile(testPath(betaFile), "# Browse Beta\n\nnested content\n"); err != nil {
		return err
	}
	if err := test.SeedMetadata(&files.Metadata{
		Path:   pathutils.ToWithPrefix(testPath(betaFile)),
		Editor: files.EditorTypeCodeMirror,
	}); err != nil {
		return err
	}

	if err := writeFile(testPath(tocFile), "# Heading One\n\ntext\n\n## Heading Two\n\nmore text\n\n### Heading Three\n\nend\n"); err != nil {
		return err
	}
	if err := test.SeedMetadata(&files.Metadata{
		Path:   pathutils.ToWithPrefix(testPath(tocFile)),
		Editor: files.EditorTypeCodeMirror,
	}); err != nil {
		return err
	}

	if err := writeFile(testPath(hiddenFile), "# Browse Hidden\n\ncontent\n"); err != nil {
		return err
	}
	if err := test.SeedMetadata(&files.Metadata{
		Path:   pathutils.ToWithPrefix(testPath(hiddenFile)),
		Editor: files.EditorTypeTodo,
	}); err != nil {
		return err
	}

	// GetAllFolderPathsFromCache/GetAllFilePathsFromCache read the aggregate caches directly
	// (no lazy-rebuild-on-miss like GetAllFilesCached has) and SeedMetadata/Sync only refresh
	// them in a background goroutine - rebuild synchronously here so caseFolderSuggestions
	// doesn't race that goroutine.
	return files.RebuildAllCaches()
}

func errCase(name string, err error) test.CaseResult {
	return test.CaseResult{Name: name, Success: false, Error: err.Error()}
}
