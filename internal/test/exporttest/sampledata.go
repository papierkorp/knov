// Package exporttest - sample file helpers
package exporttest

import (
	"os"
	"path/filepath"

	"knov/internal/contentStorage"
	"knov/internal/pathutils"
	"knov/internal/test"
)

// testDir is the docs-relative sample folder every case seeds into, wiped and reseeded at
// the start of each run so cases never see stale state from a previous run.
const testDir = "test/export-tests"

const (
	sampleFile = "export-sample.md"
	txtFile    = "export-legacy.txt" // converted to markdown by the export-with-conversion handler only
)

const sampleContent = "# export-sample.md\n\ncontent\n"

// dokuwikiContent mirrors editorstest's caseConvertToMarkdown fixture - that suite already
// verifies dokuwikiconverter's own output shape, so this only needs content that reliably
// produces recognizable markdown, not a from-scratch check of the converter itself.
const dokuwikiContent = "====== Heading ======\n\n**bold text**\n"

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

func resetAndSeed() error {
	full := pathutils.ToDocsPath(testDir)
	if err := os.RemoveAll(full); err != nil {
		return err
	}
	if err := os.MkdirAll(full, 0755); err != nil {
		return err
	}

	if err := writeFile(testPath(sampleFile), sampleContent); err != nil {
		return err
	}
	return writeFile(testPath(txtFile), dokuwikiContent)
}

func errCase(name string, err error) test.CaseResult {
	return test.CaseResult{Name: name, Success: false, Error: err.Error()}
}
