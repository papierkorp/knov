// Package filtertest - sample files and metadata seeded before every run
package filtertest

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"knov/internal/contentStorage"
	"knov/internal/files"
	"knov/internal/logging"
)

// createFilterTestFiles creates the physical test files on disk
func createFilterTestFiles() error {
	logging.LogInfo("creating filter test files on disk")

	docsPath := contentStorage.GetDocsPath()

	// remove existing filter-tests directory to ensure clean state
	filterTestsPath := filepath.Join(docsPath, "test/filter-tests")
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
			path:    "test/filter-tests/filtertestfolder/filterTestA.md",
			content: `# filterTestA`,
		},
		{
			path:    "test/filter-tests/filtertestfolder/filterTestB.md",
			content: `# filterTestB`,
		},
		{
			path:    "test/filter-tests/filterTestC.md",
			content: `# filterTestC`,
		},
		{
			path:    "test/filter-tests/filterTestD.md",
			content: `# filterTestD`,
		},
		{
			path:    "test/filter-tests/filterTestE.md",
			content: `# filterTestE`,
		},
		{
			path:    "test/filter-tests/filterTestF.md",
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

// getFilterTestMetadata returns the metadata definitions for all filter test files
func getFilterTestMetadata() []*files.Metadata {
	return []*files.Metadata{
		// filterTestA
		{
			Path:       "docs/test/filter-tests/filtertestfolder/filterTestA.md",
			CreatedAt:  time.Date(2025, 10, 1, 10, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 1, 10, 0, 0, 0, time.UTC),
			Tags:       []string{"filtertest-unique"},
			Editor:     files.EditorTypeToastUI,
		},
		// filterTestB
		{
			Path:       "docs/test/filter-tests/filtertestfolder/filterTestB.md",
			CreatedAt:  time.Date(2025, 10, 2, 11, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 2, 11, 0, 0, 0, time.UTC),
			Tags:       []string{"filtertest-group"},
			Editor:     files.EditorTypeToastUI,
		},
		// filterTestC
		{
			Path:       "docs/test/filter-tests/filterTestC.md",
			CreatedAt:  time.Date(2025, 10, 3, 12, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 3, 12, 0, 0, 0, time.UTC),
			Tags:       []string{"filtertest-group", "filtertest-group2"},
			Editor:     files.EditorTypeToastUI,
		},
		// filterTestD
		{
			Path:       "docs/test/filter-tests/filterTestD.md",
			CreatedAt:  time.Date(2025, 10, 4, 13, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 4, 13, 0, 0, 0, time.UTC),
			Tags:       []string{"filtertest-group2"},
			Editor:     files.EditorTypeToastUI,
		},
		// filterTestE
		{
			Path:       "docs/test/filter-tests/filterTestE.md",
			CreatedAt:  time.Date(2025, 10, 5, 14, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 5, 14, 0, 0, 0, time.UTC),
			Parents:    []string{"docs/test/filter-tests/filterTestD.md"},
			Editor:     files.EditorTypeToastUI,
			References: []files.Reference{{URL: "https://example.com", Description: "example reference for testing"}, {URL: "https://www.google.com", Description: "another reference"}},
		},
		// filterTestF
		{
			Path:       "docs/test/filter-tests/filterTestF.md",
			CreatedAt:  time.Date(2025, 10, 6, 15, 0, 0, 0, time.UTC),
			LastEdited: time.Date(2025, 11, 6, 15, 0, 0, 0, time.UTC),
			Parents:    []string{"docs/test/filter-tests/filterTestE.md"},
			Editor:     files.EditorTypeToastUI,
		},
	}
}

// createFilterTestMetadata creates the test files and metadata objects on disk.
func createFilterTestMetadata() error {
	debugLogger := logging.LogBuilder("filter-debug")

	if err := createFilterTestFiles(); err != nil {
		debugLogger.Printf("failed to create filter test files: %v", err)
		return fmt.Errorf("failed to create filter test files: %v", err)
	}

	for _, metadata := range getFilterTestMetadata() {
		if err := files.MetaDataSave(metadata); err != nil {
			logging.LogError("failed to save metadata for %s: %v", metadata.Path, err)
			debugLogger.Printf("error saving metadata for %s: %v", metadata.Path, err)
			return fmt.Errorf("failed to save metadata for %s: %v", metadata.Path, err)
		}
	}

	if err := files.SaveAllCollectionsToCache(); err != nil {
		logging.LogWarning("failed to update collections cache: %v", err)
	}
	if err := files.SaveAllFoldersToCache(); err != nil {
		logging.LogWarning("failed to update folders cache: %v", err)
	}
	if err := files.SaveAllTagsToCache(); err != nil {
		logging.LogWarning("failed to update tags cache: %v", err)
	}

	return nil
}
