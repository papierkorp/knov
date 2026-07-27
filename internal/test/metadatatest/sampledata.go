// Package metadatatest - sample file helpers
package metadatatest

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
const testDir = "test/metadata-tests"

const (
	fieldsFile     = "metadata-fields.md"
	deleteFile     = "metadata-delete.md"
	exportFile     = "metadata-export.md"
	referencesFile = "metadata-references.md"
)

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

// resetAndSeed wipes the sample folder and writes one plain file per case - each case saves
// its own metadata directly, since the fields under test differ per case. Metadata is deleted
// explicitly (not just the physical file) before reseeding: metaDataUpdate only overwrites
// list fields like References when the new value is non-nil, so a leftover References list
// from a previous run would otherwise silently survive the physical-file wipe below.
func resetAndSeed() error {
	full := pathutils.ToDocsPath(testDir)
	if err := os.RemoveAll(full); err != nil {
		return err
	}
	if err := os.MkdirAll(full, 0755); err != nil {
		return err
	}

	for _, name := range []string{fieldsFile, deleteFile, exportFile, referencesFile} {
		if err := files.MetaDataDelete(pathutils.ToWithPrefix(testPath(name))); err != nil {
			return err
		}
		if err := writeFile(testPath(name), "# "+name+"\n\ncontent\n"); err != nil {
			return err
		}
	}

	files.InvalidateFileListCache()
	return nil
}

func errCase(name string, err error) test.CaseResult {
	return test.CaseResult{Name: name, Success: false, Error: err.Error()}
}
