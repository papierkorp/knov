// Package dashboardtest - sample folder/file and dashboard-name/id helpers
package dashboardtest

import (
	"os"
	"path/filepath"

	"knov/internal/contentStorage"
	"knov/internal/dashboard"
	"knov/internal/files"
	"knov/internal/pathutils"
	"knov/internal/test"
	"knov/internal/utils"
)

// testDir is the docs-relative sample folder every case seeds into, wiped at the start of
// each run so cases never see stale state from a previous run.
const testDir = "test/dashboard-tests"

const sampleFile = "dashboard-sample.md"

const (
	sampleTag           = "dashtest-marker"
	sampleContentMarker = "DashtestContentMarker"
)

// dashboardNames lists every fixed dashboard name a case creates - resetAndSeed deletes
// their derived ids up front so a previous (possibly failed) run never collides with Create.
var dashboardNames = []string{
	"Dashtest Create",
	"Dashtest GetAll A",
	"Dashtest GetAll B",
	"Dashtest Update",
	"Dashtest Rename",
	"Dashtest Delete",
	"Dashtest Export",
	"Dashtest Export Imported",
}

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

func saveMetadata(relPath string, tags []string) error {
	return files.MetaDataSave(&files.Metadata{
		Path:   pathutils.ToWithPrefix(relPath),
		Editor: files.EditorTypeToastUI,
		Tags:   tags,
	})
}

// resetAndSeed wipes the sample folder, writes the shared sample file used by the widget
// data cases, and clears out any dashboards left over from a previous run under the fixed
// names cases use - dashboards live in configStorage keyed by id, not under docs/test/, so
// wiping the folder doesn't touch them.
func resetAndSeed() error {
	full := pathutils.ToDocsPath(testDir)
	if err := os.RemoveAll(full); err != nil {
		return err
	}
	if err := os.MkdirAll(full, 0755); err != nil {
		return err
	}

	if err := writeFile(testPath(sampleFile), "# Dashtest sample\n"+sampleContentMarker+"\n"); err != nil {
		return err
	}
	if err := saveMetadata(testPath(sampleFile), []string{sampleTag}); err != nil {
		return err
	}

	for _, name := range dashboardNames {
		_ = dashboard.Delete(utils.CleanseID(name))
	}

	return nil
}

func errCase(name string, err error) test.CaseResult {
	return test.CaseResult{Name: name, Success: false, Error: err.Error()}
}
