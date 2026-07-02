// Package filtertest - Filter suite: seeds real files/metadata and runs real filter configs
package filtertest

import (
	"fmt"
	"path/filepath"
	"slices"

	"knov/internal/files"
	"knov/internal/filter"
	"knov/internal/logging"
	"knov/internal/test"
)

// Suite runs the filter test scenarios against real filter configs.
type Suite struct{}

func init() {
	test.Register(Suite{})
}

func (Suite) Name() string { return "filter" }

// GetFilterTestMetadata returns the metadata definitions for all filter test files.
func GetFilterTestMetadata() []*files.Metadata {
	return getFilterTestMetadata()
}

// createFilterTestMetadata creates the test files and 12 test metadata objects on disk.
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

// Run executes the filter test scenarios and returns the aggregated suite result.
func (Suite) Run() (*test.SuiteResult, error) {
	debugLogger := logging.LogBuilder("filter-debug")

	if err := createFilterTestMetadata(); err != nil {
		debugLogger.Printf("failed to create filter test metadata: %v", err)
		return nil, fmt.Errorf("failed to create filter test metadata: %v", err)
	}

	result := &test.SuiteResult{Suite: "filter"}

	for _, tc := range testConfigs {
		caseResult := runCase(tc)
		result.Cases = append(result.Cases, caseResult)
		if caseResult.Success {
			result.Passed++
		} else {
			result.Failed++
			debugLogger.Printf("test %s failed: %s", caseResult.Name, caseResult.Error)
		}
	}

	result.Total = len(testConfigs)
	result.Success = result.Failed == 0

	if result.Failed > 0 {
		debugLogger.Printf("filter tests completed with failures: %d passed, %d failed", result.Passed, result.Failed)
	}

	return result, nil
}

func runCase(tc testConfig) test.CaseResult {
	expected := fmt.Sprintf("%d files: %v", tc.expectedCount, tc.expectedFiles)

	filterResult, err := filter.FilterFilesWithConfig(&tc.config)
	if err != nil {
		return test.CaseResult{
			Name:     tc.name,
			Expected: expected,
			Actual:   "error",
			Error:    err.Error(),
			Success:  false,
			Detail:   tc.config,
		}
	}

	actualBasenames := make([]string, len(filterResult.Files))
	for i, file := range filterResult.Files {
		actualBasenames[i] = filepath.Base(file.Path)
	}
	actual := fmt.Sprintf("%d files: %v", len(actualBasenames), actualBasenames)

	var missingFiles []string
	for _, expectedFile := range tc.expectedFiles {
		if !slices.Contains(actualBasenames, expectedFile) {
			missingFiles = append(missingFiles, expectedFile)
		}
	}

	success := len(actualBasenames) == tc.expectedCount && len(missingFiles) == 0

	caseResult := test.CaseResult{
		Name:     tc.name,
		Expected: expected,
		Actual:   actual,
		Success:  success,
		Detail:   tc.config,
	}

	if !success {
		if len(actualBasenames) != tc.expectedCount {
			caseResult.Error = fmt.Sprintf("expected %d files, got %d", tc.expectedCount, len(actualBasenames))
		} else {
			caseResult.Error = fmt.Sprintf("file mismatch — missing: %v", missingFiles)
		}
	}

	return caseResult
}
