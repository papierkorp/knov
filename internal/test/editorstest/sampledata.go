// Package editorstest - sample folder and per-case sample file/metadata helpers
package editorstest

import (
	"os"
	"path/filepath"

	"knov/internal/contentStorage"
	"knov/internal/files"
	"knov/internal/pathutils"
	"knov/internal/test"
)

// testDir is the docs-relative sample folder every case seeds into, wiped at the start
// of each run so cases never see stale state from a previous run.
const testDir = "test/editors-tests"

// resetTestDir clears the sample folder on disk so every run starts from a clean state.
func resetTestDir() error {
	full := pathutils.ToDocsPath(testDir)
	if err := os.RemoveAll(full); err != nil {
		return err
	}
	return os.MkdirAll(full, 0755)
}

// testPath returns a docs-relative path under the sample folder, e.g. "test/editors-tests/toastui.md".
func testPath(name string) string {
	return filepath.Join(testDir, name)
}

func writeFile(relPath, content string) error {
	full := pathutils.ToDocsPath(relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		return err
	}
	return contentStorage.WriteFile(full, []byte(content), 0644)
}

func readFile(relPath string) (string, error) {
	full := pathutils.ToDocsPath(relPath)
	b, err := contentStorage.ReadFile(full)
	return string(b), err
}

func saveMetadata(relPath string, editor files.EditorType) error {
	return files.MetaDataSave(&files.Metadata{
		Path:   pathutils.ToWithPrefix(relPath),
		Editor: editor,
	})
}

func errCase(name string, err error) test.CaseResult {
	return test.CaseResult{Name: name, Success: false, Error: err.Error()}
}
