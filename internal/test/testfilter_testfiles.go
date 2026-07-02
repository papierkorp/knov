// Package testdata - Test file creation for filter testing
package test

import (
	"fmt"
	"os"
	"path/filepath"

	"knov/internal/contentStorage"
	"knov/internal/logging"
)

// createFilterTestFiles creates the physical test files on disk
func createFilterTestFiles() error {
	logging.LogInfo("creating filter test files on disk")

	docsPath := contentStorage.GetDocsPath()

	// remove existing filter-tests directory to ensure clean state
	filterTestsPath := filepath.Join(docsPath, "filter-tests")
	if _, err := os.Stat(filterTestsPath); err == nil {
		logging.LogInfo("removing existing filter-tests directory")
		if err := os.RemoveAll(filterTestsPath); err != nil {
			return fmt.Errorf("failed to remove existing filter-tests directory: %v", err)
		}
	}

	// define test files with their paths and unique content
	testFiles := []struct {
		path    string
		content string
	}{
		{
			path:    "filter-tests/filtertestfolder/filterTestA.md",
			content: `# filterTestA`,
		},
		{
			path:    "filter-tests/filtertestfolder/filterTestB.md",
			content: `# filterTestB`,
		},
		{
			path:    "filter-tests/filterTestC.md",
			content: `# filterTestC`,
		},
		{
			path:    "filter-tests/filterTestD.md",
			content: `# filterTestD`,
		},
		{
			path:    "filter-tests/filterTestE.md",
			content: `# filterTestE`,
		},
		{
			path:    "filter-tests/filterTestF.md",
			content: `# filterTestF`,
		},
	}

	// create directories and files
	for _, file := range testFiles {
		fullPath := filepath.Join(docsPath, file.path)

		// create directory if it doesn't exist
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %v", dir, err)
		}

		// create the file
		if err := os.WriteFile(fullPath, []byte(file.content), 0644); err != nil {
			return fmt.Errorf("failed to create file %s: %v", fullPath, err)
		}
	}

	logging.LogInfo("filter test files created successfully")
	return nil
}
