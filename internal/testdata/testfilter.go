// Package testdata - Filter testing functionality
package testdata

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
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
	Errors      []string           `json:"errors,omitempty"`
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
		Errors:  make([]string, 0),
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
			name: "test1and",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "collection",
						Operator: "equals",
						Value:    "filter-tests",
						Action:   "include",
					},
					{
						Metadata: "folders",
						Operator: "equals",
						Value:    "filtertestfolder",
						Action:   "include",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 2,
			expectedFiles: []string{"filterTestA.md", "filterTestB.md"},
			description:   "",
		},
		{
			name: "test2or",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "tags",
						Operator: "contains",
						Value:    "group2",
						Action:   "include",
					},
					{
						Metadata: "tags",
						Operator: "regex",
						Value:    ".*-unique",
						Action:   "include",
					},
				},
				Logic: "or",
				Limit: 0,
			},
			expectedCount: 3,
			expectedFiles: []string{"filterTestA.md", "filterTestC.md", "filterTestD.md"},
			description:   "",
		},
		{
			name: "test3or_exclude",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "tags",
						Operator: "contains",
						Value:    "group2",
						Action:   "include",
					},
					{
						Metadata: "tags",
						Operator: "regex",
						Value:    ".*-unique",
						Action:   "include",
					},
					{
						Metadata: "tags",
						Operator: "equals",
						Value:    "filtertest-group",
						Action:   "exclude",
					},
				},
				Logic: "or",
				Limit: 0,
			},
			expectedCount: 2,
			expectedFiles: []string{"filterTestA.md", "filterTestD.md"},
			description:   "",
		},
		{
			name: "test4exclude_single",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "collection",
						Operator: "equals",
						Value:    "filter-tests",
						Action:   "include",
					},
					{
						Metadata: "title",
						Operator: "equals",
						Value:    "filterTestC",
						Action:   "exclude",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 5,
			expectedFiles: []string{"filterTestA.md", "filterTestB.md", "filterTestD.md", "filterTestE.md", "filterTestF.md"},
			description:   "",
		},
		{
			name: "test5exclude_folder",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "collection",
						Operator: "equals",
						Value:    "filter-tests",
						Action:   "include",
					},
					{
						Metadata: "folders",
						Operator: "equals",
						Value:    "filtertestfolder",
						Action:   "exclude",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 4,
			expectedFiles: []string{"filterTestC.md", "filterTestD.md", "filterTestE.md", "filterTestF.md"},
			description:   "",
		},
		{
			name: "test6regex",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "collection",
						Operator: "equals",
						Value:    "filter-tests",
						Action:   "include",
					},
					{
						Metadata: "title",
						Operator: "regex",
						Value:    "^filterTest[A-C]",
						Action:   "include",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 3,
			expectedFiles: []string{"filterTestA.md", "filterTestB.md", "filterTestC.md"},
			description:   "",
		},
		{
			name: "test7greaterthan",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "collection",
						Operator: "equals",
						Value:    "filter-tests",
						Action:   "include",
					},
					{
						Metadata: "createdAt",
						Operator: "greater",
						Value:    "2025-10-02",
						Action:   "include",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 4,
			expectedFiles: []string{"filterTestC.md", "filterTestD.md", "filterTestE.md", "filterTestF.md"},
			description:   "",
		},
		{
			name: "test8lessthan",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "collection",
						Operator: "equals",
						Value:    "filter-tests",
						Action:   "include",
					},
					{
						Metadata: "createdAt",
						Operator: "less",
						Value:    "2025-10-05",
						Action:   "include",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 4,
			expectedFiles: []string{"filterTestA.md", "filterTestB.md", "filterTestC.md", "filterTestD.md"},
			description:   "",
		},
		{
			name: "test9inarray_tags",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "collection",
						Operator: "equals",
						Value:    "filter-tests",
						Action:   "include",
					},
					{
						Metadata: "tags",
						Operator: "in",
						Value:    "filtertest-group,filtertest-group2",
						Action:   "include",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 3,
			expectedFiles: []string{"filterTestB.md", "filterTestC.md", "filterTestD.md"},
			description:   "",
		},
		{
			name: "test10childof",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "collection",
						Operator: "equals",
						Value:    "filter-tests",
						Action:   "include",
					},
					{
						Metadata: "child-of",
						Operator: "equals",
						Value:    "filter-tests/filterTestD.md",
						Action:   "include",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 1,
			expectedFiles: []string{"filterTestE.md"},
			description:   "",
		},
		{
			name: "test11parentof",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "collection",
						Operator: "equals",
						Value:    "filter-tests",
						Action:   "include",
					},
					{
						Metadata: "parent-of",
						Operator: "equals",
						Value:    "filter-tests/filterTestE.md",
						Action:   "include",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 1,
			expectedFiles: []string{"filterTestD.md"},
			description:   "",
		},
		{
			name: "test12ancestorof",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "collection",
						Operator: "equals",
						Value:    "filter-tests",
						Action:   "include",
					},
					{
						Metadata: "ancestor-of",
						Operator: "equals",
						Value:    "filter-tests/filterTestD.md",
						Action:   "include",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 2,
			expectedFiles: []string{"filterTestE.md", "filterTestF.md"},
			description:   "",
		},
		{
			name: "test13multiple_filters_1",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "collection",
						Operator: "equals",
						Value:    "filter-tests",
						Action:   "include",
					},
					{
						Metadata: "tags",
						Operator: "in",
						Value:    "filtertest-group",
						Action:   "include",
					},
					{
						Metadata: "createdAt",
						Operator: "equals",
						Value:    "2025-10-02",
						Action:   "include",
					},
					{
						Metadata: "editor",
						Operator: "equals",
						Value:    "toastui-editor",
						Action:   "include",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 1,
			expectedFiles: []string{"filterTestB.md"},
			description:   "",
		},
		{
			name: "test14multiple_filters_2",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "collection",
						Operator: "equals",
						Value:    "filter-tests",
						Action:   "include",
					},
					{
						Metadata: "title",
						Operator: "contains",
						Value:    "D",
						Action:   "exclude",
					},
					{
						Metadata: "tags",
						Operator: "contains",
						Value:    "group",
						Action:   "exclude",
					},
					{
						Metadata: "folders",
						Operator: "equals",
						Value:    "filtertestfolder",
						Action:   "exclude",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 2,
			expectedFiles: []string{"filterTestE.md", "filterTestF.md"},
			description:   "",
		},
		{
			name: "test15multiple_filters_3",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "collection",
						Operator: "equals",
						Value:    "filter-tests",
						Action:   "include",
					},
					{
						Metadata: "child-of",
						Operator: "equals",
						Value:    "filter-tests/filterTestD.md",
						Action:   "include",
					},
					{
						Metadata: "parent-of",
						Operator: "equals",
						Value:    "filter-tests/filterTestF.md",
						Action:   "include",
					},
					{
						Metadata: "ancestor-of",
						Operator: "equals",
						Value:    "filter-tests/filterTestD.md",
						Action:   "include",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 1,
			expectedFiles: []string{"filterTestE.md"},
			description:   "",
		},
		{
			name: "test16datecontains",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "collection",
						Operator: "equals",
						Value:    "filter-tests",
						Action:   "include",
					},
					{
						Metadata: "createdAt",
						Operator: "contains",
						Value:    "2025-10-03",
						Action:   "include",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 1,
			expectedFiles: []string{"filterTestC.md"},
			description:   "",
		},
		{
			name: "test17dateregex",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "collection",
						Operator: "equals",
						Value:    "filter-tests",
						Action:   "include",
					},
					{
						Metadata: "createdAt",
						Operator: "regex",
						Value:    "2025-10-0[1-3]",
						Action:   "include",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 3,
			expectedFiles: []string{"filterTestA.md", "filterTestB.md", "filterTestC.md"},
			description:   "",
		},
		{
			name: "test18references",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "collection",
						Operator: "equals",
						Value:    "filter-tests",
						Action:   "include",
					},
					{
						Metadata: "references",
						Operator: "contains",
						Value:    "example reference",
						Action:   "include",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 1,
			expectedFiles: []string{"filterTestE.md"},
			description:   "",
		},
	}

	// run each test
	for _, test := range testConfigs {
		result, err := filter.FilterFilesWithConfig(&test.config)
		if err != nil {
			errMsg := fmt.Sprintf("test %s failed: %v", test.name, err)
			debugLogger.Printf("%s", errMsg)
			results.Errors = append(results.Errors, errMsg)
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

		// extract actual file basenames for comparison
		actualFiles := make([]string, len(result.Files))
		actualBasenames := make([]string, len(result.Files))
		for i, file := range result.Files {
			actualFiles[i] = file.Path
			actualBasenames[i] = filepath.Base(file.Path)
		}

		// compare by basename: check every expected file is present
		filesMismatch := false
		var missingFiles []string
		for _, expected := range test.expectedFiles {
			if !slices.Contains(actualBasenames, expected) {
				filesMismatch = true
				missingFiles = append(missingFiles, expected)
			}
		}

		success := actualCount == test.expectedCount && !filesMismatch

		testResult := FilterTestResult{
			ConfigName:    test.name,
			Success:       success,
			ExpectedCount: test.expectedCount,
			ActualCount:   actualCount,
			Config:        test.config,
			ActualFiles:   actualBasenames,
			ExpectedFiles: test.expectedFiles,
			Description:   test.description,
		}

		if !success {
			if actualCount != test.expectedCount {
				testResult.Error = fmt.Sprintf("expected %d files, got %d", test.expectedCount, actualCount)
			} else {
				testResult.Error = fmt.Sprintf("file mismatch — missing: %v", missingFiles)
			}
			results.FailedTests++
			errMsg := fmt.Sprintf("test %s failed: %s", test.name, testResult.Error)
			debugLogger.Printf("%s", errMsg)
			debugLogger.Printf("expected: %v", test.expectedFiles)
			debugLogger.Printf("found: %v", actualBasenames)
			results.Errors = append(results.Errors, errMsg)
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
		errMsg := fmt.Sprintf("filter tests completed with failures: %d passed, %d failed", results.PassedTests, results.FailedTests)
		debugLogger.Printf("%s", errMsg)
		results.Errors = append(results.Errors, errMsg)
	}
	return results, nil
}
