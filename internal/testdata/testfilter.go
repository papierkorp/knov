// Package testdata - Filter testing functionality
package testdata

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"knov/internal/files"
	"knov/internal/filter"
	"knov/internal/logging"
)

// FilterTestResult represents the result of a filter test
type FilterTestResult struct {
	ConfigName    string        `json:"config_name"`
	Success       bool          `json:"success"`
	ExpectedCount int           `json:"expected_count"`
	ActualCount   int           `json:"actual_count"`
	Error         string        `json:"error,omitempty"`
	Config        filter.Config `json:"config"`
	ActualFiles   []string      `json:"actual_files"`
	ExpectedFiles []string      `json:"expected_files"`
	Description   string        `json:"description"`
}

// FilterTestResults represents the overall results of filter testing
type FilterTestResults struct {
	TotalTests  int                `json:"total_tests"`
	PassedTests int                `json:"passed_tests"`
	FailedTests int                `json:"failed_tests"`
	Success     bool               `json:"success"`
	Results     []FilterTestResult `json:"results"`
	LogFile     string             `json:"log_file,omitempty"`
}

// CreateFilterTestMetadata creates 12 test metadata objects
func CreateFilterTestMetadata() error {
	debugLogger := logging.LogBuilder("filter-debug")

	// first, create the actual test files on disk (clears existing filter-tests folder)
	if err := createFilterTestFiles(); err != nil {
		debugLogger.Printf("failed to create filter test files: %v", err)
		return fmt.Errorf("failed to create filter test files: %v", err)
	}

	// get metadata definitions from separate file
	testMetadataList := getFilterTestMetadata()

	// save each metadata object
	for _, metadata := range testMetadataList {
		if err := files.MetaDataSave(metadata); err != nil {
			logging.LogError("failed to save metadata for %s: %v", metadata.Path, err)
			debugLogger.Printf("error saving metadata for %s: %v", metadata.Path, err)
			return fmt.Errorf("failed to save metadata for %s: %v", metadata.Path, err)
		}
	}

	// update all metadata caches
	if err := files.SaveAllCollectionsToSystemData(); err != nil {
		logging.LogWarning("failed to update collections cache: %v", err)
	}
	if err := files.SaveAllFoldersToSystemData(); err != nil {
		logging.LogWarning("failed to update folders cache: %v", err)
	}
	if err := files.SaveAllTagsToSystemData(); err != nil {
		logging.LogWarning("failed to update tags cache: %v", err)
	}

	return nil
}

// GetFilterTestMetadata returns the metadata definitions for all filter test files
func GetFilterTestMetadata() []*files.Metadata {
	return getFilterTestMetadata()
}

// RunFilterTests executes various filter test scenarios
func RunFilterTests() (*FilterTestResults, error) {
	debugLogger := logging.LogBuilder("filter-debug")

	// ensure test data exists
	if err := CreateFilterTestMetadata(); err != nil {
		debugLogger.Printf("failed to create filter test metadata: %v", err)
		return nil, fmt.Errorf("failed to create filter test metadata: %v", err)
	}

	results := &FilterTestResults{
		Results: make([]FilterTestResult, 0),
	}

	// define test scenarios with unique metadata values
	testConfigs := []struct {
		name          string
		config        filter.Config
		expectedCount int
		expectedFiles []string
		description   string
	}{
		{
			name: "single_tag_unique_experimental",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "folders",
						Operator: "contains",
						Value:    "filter-tests",
						Action:   "include",
					},
					{
						Metadata: "tags",
						Operator: "contains",
						Value:    "unique-experimental",
						Action:   "include",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 2,
			expectedFiles: []string{"filter-tests/filterTestA.md", "filter-tests/advanced/filterTestD.md"},
			description:   "filter by single unique tag 'unique-experimental'",
		},
	}

	// run each test
	for _, test := range testConfigs {
		result, err := filter.FilterFilesWithConfig(&test.config)
		if err != nil {
			debugLogger.Printf("test %s failed: %v", test.name, err)
			testResult := FilterTestResult{
				ConfigName:    test.name,
				Success:       false,
				ExpectedCount: test.expectedCount,
				ActualCount:   0,
				Error:         err.Error(),
				Config:        test.config,
				ActualFiles:   []string{},
				ExpectedFiles: test.expectedFiles,
				Description:   test.description,
			}
			results.Results = append(results.Results, testResult)
			results.FailedTests++
			continue
		}

		actualCount := len(result.Files)
		success := actualCount == test.expectedCount

		// extract actual file paths
		actualFiles := make([]string, len(result.Files))
		for i, file := range result.Files {
			actualFiles[i] = file.Path
		}

		testResult := FilterTestResult{
			ConfigName:    test.name,
			Success:       success,
			ExpectedCount: test.expectedCount,
			ActualCount:   actualCount,
			Config:        test.config,
			ActualFiles:   actualFiles,
			ExpectedFiles: test.expectedFiles,
			Description:   test.description,
		}

		if !success {
			testResult.Error = fmt.Sprintf("expected %d files, got %d", test.expectedCount, actualCount)
			results.FailedTests++
			debugLogger.Printf("test %s failed: expected %d files, got %d", test.name, test.expectedCount, actualCount)
			debugLogger.Printf("expected: %v", test.expectedFiles)
			debugLogger.Printf("found: %v", actualFiles)
		} else {
			results.PassedTests++
		}

		results.Results = append(results.Results, testResult)
	}

	results.TotalTests = len(testConfigs)
	results.Success = results.FailedTests == 0

	// set log file path using same base directory logic as configmanager
	baseDir := "."
	execPath, err := os.Executable()
	if err == nil {
		execDir := filepath.Dir(execPath)
		// check if running from go build cache (go run)
		if !strings.Contains(execDir, "go-build") {
			baseDir = execDir
		}
	}
	logFile := filepath.Join(baseDir, "logs", "filter-debug.log")
	results.LogFile = logFile

	if results.FailedTests > 0 {
		debugLogger.Printf("filter tests completed with failures: %d passed, %d failed", results.PassedTests, results.FailedTests)
	}
	return results, nil
}
