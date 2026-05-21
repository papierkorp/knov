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
		{
			name: "two_tags_and_advanced_stable",
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
						Value:    "unique-advanced",
						Action:   "include",
					},
					{
						Metadata: "tags",
						Operator: "contains",
						Value:    "unique-stable",
						Action:   "include",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 1,
			expectedFiles: []string{"filter-tests/advanced/filterTestE.md"},
			description:   "filter by two unique tags with AND logic",
		},
		{
			name: "two_tags_or_basic_performance",
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
						Value:    "unique-basic",
						Action:   "include",
					},
					{
						Metadata: "tags",
						Operator: "contains",
						Value:    "unique-performance",
						Action:   "include",
					},
				},
				Logic: "or",
				Limit: 0,
			},
			expectedCount: 5,
			expectedFiles: []string{"filter-tests/filterTestC.md", "filter-tests/basic/filterTestF.md", "filter-tests/basic/filterTestG.md", "filter-tests/performance/filterTestJ.md", "filter-tests/performance/filterTestK.md"},
			description:   "filter by two unique tags with OR logic",
		},
		{
			name: "collection_equals_filter_testing_unique",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "folders",
						Operator: "contains",
						Value:    "filter-tests",
						Action:   "include",
					},
					{
						Metadata: "collection",
						Operator: "equals",
						Value:    "filter-testing-unique",
						Action:   "include",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 3,
			expectedFiles: []string{"filter-tests/filterTestA.md", "filter-tests/filterTestB.md", "filter-tests/filterTestC.md"},
			description:   "filter by exact collection 'filter-testing-unique'",
		},
		{
			name: "collection_contains_filter_testing",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "folders",
						Operator: "contains",
						Value:    "filter-tests",
						Action:   "include",
					},
					{
						Metadata: "collection",
						Operator: "contains",
						Value:    "filter-testing",
						Action:   "include",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 12,
			expectedFiles: []string{
				"filter-tests/filterTestA.md", "filter-tests/filterTestB.md", "filter-tests/filterTestC.md",
				"filter-tests/advanced/filterTestD.md", "filter-tests/advanced/filterTestE.md",
				"filter-tests/basic/filterTestF.md", "filter-tests/basic/filterTestG.md",
				"filter-tests/integration/filterTestH.md", "filter-tests/integration/filterTestI.md",
				"filter-tests/performance/filterTestJ.md", "filter-tests/performance/filterTestK.md",
				"filter-tests/special/filterTestL.md",
			},
			description: "filter by collection containing 'filter-testing'",
		},
		{
			name: "folders_contains_advanced",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "folders",
						Operator: "contains",
						Value:    "filter-tests",
						Action:   "include",
					},
					{
						Metadata: "folders",
						Operator: "contains",
						Value:    "advanced",
						Action:   "include",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 2,
			expectedFiles: []string{"filter-tests/advanced/filterTestD.md", "filter-tests/advanced/filterTestE.md"},
			description:   "filter by folder containing 'advanced'",
		},
		{
			name: "specific_tag_pattern",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "tags",
						Operator: "contains",
						Value:    "-specific",
						Action:   "include",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 10,
			expectedFiles: []string{
				"filter-tests/filterTestA.md", "filter-tests/filterTestB.md", "filter-tests/filterTestC.md",
				"filter-tests/basic/filterTestF.md", "filter-tests/basic/filterTestG.md",
				"filter-tests/integration/filterTestH.md", "filter-tests/integration/filterTestI.md",
				"filter-tests/performance/filterTestJ.md", "filter-tests/performance/filterTestK.md",
				"filter-tests/special/filterTestL.md",
			},
			description: "filter by tag pattern containing '-specific'",
		},
		{
			name: "date_created_after_october_5",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "createdAt",
						Operator: "greater",
						Value:    "2025-10-05",
						Action:   "include",
					},
					{
						Metadata: "collection",
						Operator: "contains",
						Value:    "filter-testing",
						Action:   "include",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 7,
			expectedFiles: []string{"filter-tests/basic/filterTestF.md", "filter-tests/basic/filterTestG.md", "filter-tests/integration/filterTestH.md", "filter-tests/integration/filterTestI.md", "filter-tests/performance/filterTestJ.md", "filter-tests/performance/filterTestK.md", "filter-tests/special/filterTestL.md"},
			description:   "filter by creation date after October 5, 2025",
		},
		{
			name: "date_range_october_filter",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "createdAt",
						Operator: "greater",
						Value:    "2025-10-03",
						Action:   "include",
					},
					{
						Metadata: "createdAt",
						Operator: "less",
						Value:    "2025-10-09",
						Action:   "include",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 5,
			expectedFiles: []string{"filter-tests/advanced/filterTestD.md", "filter-tests/advanced/filterTestE.md", "filter-tests/basic/filterTestF.md", "filter-tests/basic/filterTestG.md", "filter-tests/integration/filterTestH.md"},
			description:   "filter by creation date range: October 4-8, 2025",
		},
		{
			name: "multiple_tags_in_array",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "tags",
						Operator: "in",
						Value:    "alpha-test,beta-test,gamma-test",
						Action:   "include",
					},
					{
						Metadata: "collection",
						Operator: "contains",
						Value:    "filter-testing",
						Action:   "include",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 3,
			expectedFiles: []string{"filter-tests/filterTestA.md", "filter-tests/filterTestB.md", "filter-tests/filterTestC.md"},
			description:   "filter by tags using 'in' array: alpha-test, beta-test, gamma-test",
		},
		{
			name: "exclude_multiple_collections",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "collection",
						Operator: "in",
						Value:    "advanced-filter-testing,special-filter-testing",
						Action:   "exclude",
					},
					{
						Metadata: "folders",
						Operator: "contains",
						Value:    "filter-tests",
						Action:   "include",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 9,
			expectedFiles: []string{"filter-tests/filterTestA.md", "filter-tests/filterTestB.md", "filter-tests/filterTestC.md", "filter-tests/basic/filterTestF.md", "filter-tests/basic/filterTestG.md", "filter-tests/integration/filterTestH.md", "filter-tests/integration/filterTestI.md", "filter-tests/performance/filterTestJ.md", "filter-tests/performance/filterTestK.md"},
			description:   "exclude multiple collections using 'in' array",
		},
		{
			name: "name_regex_markdown_files",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "name",
						Operator: "regex",
						Value:    `\.md$`,
						Action:   "include",
					},
					{
						Metadata: "collection",
						Operator: "contains",
						Value:    "filter-testing",
						Action:   "include",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 12,
			expectedFiles: []string{
				"filter-tests/filterTestA.md", "filter-tests/filterTestB.md", "filter-tests/filterTestC.md",
				"filter-tests/advanced/filterTestD.md", "filter-tests/advanced/filterTestE.md",
				"filter-tests/basic/filterTestF.md", "filter-tests/basic/filterTestG.md",
				"filter-tests/integration/filterTestH.md", "filter-tests/integration/filterTestI.md",
				"filter-tests/performance/filterTestJ.md", "filter-tests/performance/filterTestK.md",
				"filter-tests/special/filterTestL.md",
			},
			description: "filter by name using regex to find all .md files in filter-testing collection",
		},
		{
			name: "name_contains_with_folder",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "name",
						Operator: "contains",
						Value:    "filterTest",
						Action:   "include",
					},
					{
						Metadata: "folders",
						Operator: "contains",
						Value:    "advanced",
						Action:   "include",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 2,
			expectedFiles: []string{"filter-tests/advanced/filterTestD.md", "filter-tests/advanced/filterTestE.md"},
			description:   "filter by name containing 'filterTest' AND folder containing 'advanced'",
		},
		{
			name: "boards_contains_filter_board",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "boards",
						Operator: "contains",
						Value:    "filter-board",
						Action:   "include",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 3,
			expectedFiles: []string{"filter-tests/filterTestA.md", "filter-tests/filterTestB.md", "filter-tests/filterTestC.md"},
			description:   "filter by boards field containing 'filter-board'",
		},
		{
			name: "empty_result_set",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "collection",
						Operator: "equals",
						Value:    "unique_nonexistent_value",
						Action:   "include",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 0,
			expectedFiles: []string{},
			description:   "query that should return no results (nonexistent value)",
		},
		{
			name: "limit_functionality",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "collection",
						Operator: "contains",
						Value:    "filter-testing",
						Action:   "include",
					},
				},
				Logic: "and",
				Limit: 5,
			},
			expectedCount: 5,
			expectedFiles: nil,
			description:   "test limit functionality - should return only 5 results",
		},
		{
			name: "boards_in_multiple",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "boards",
						Operator: "in",
						Value:    "advanced-board,performance-board",
						Action:   "include",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 5,
			expectedFiles: []string{
				"filter-tests/filterTestC.md",
				"filter-tests/advanced/filterTestD.md", "filter-tests/advanced/filterTestE.md",
				"filter-tests/performance/filterTestJ.md", "filter-tests/performance/filterTestK.md",
			},
			description: "filter by boards using 'in' operator with multiple board names",
		},
		{
			name: "case_sensitivity_test",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "collection",
						Operator: "contains",
						Value:    "FILTER-TESTING",
						Action:   "include",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 12,
			expectedFiles: []string{
				"filter-tests/filterTestA.md", "filter-tests/filterTestB.md", "filter-tests/filterTestC.md",
				"filter-tests/advanced/filterTestD.md", "filter-tests/advanced/filterTestE.md",
				"filter-tests/basic/filterTestF.md", "filter-tests/basic/filterTestG.md",
				"filter-tests/integration/filterTestH.md", "filter-tests/integration/filterTestI.md",
				"filter-tests/performance/filterTestJ.md", "filter-tests/performance/filterTestK.md",
				"filter-tests/special/filterTestL.md",
			},
			description: "test case insensitivity - sqlite like is case-insensitive by default",
		},
		{
			name: "or_complex_multi_field",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "tags",
						Operator: "contains",
						Value:    "unique-advanced",
						Action:   "include",
					},
					{
						Metadata: "tags",
						Operator: "contains",
						Value:    "unique-basic",
						Action:   "include",
					},
					{
						Metadata: "tags",
						Operator: "contains",
						Value:    "unique-performance",
						Action:   "include",
					},
				},
				Logic: "or",
				Limit: 0,
			},
			expectedCount: 7,
			expectedFiles: []string{
				"filter-tests/filterTestC.md",
				"filter-tests/advanced/filterTestD.md", "filter-tests/advanced/filterTestE.md",
				"filter-tests/basic/filterTestF.md", "filter-tests/basic/filterTestG.md",
				"filter-tests/performance/filterTestJ.md", "filter-tests/performance/filterTestK.md",
			},
			description: "or logic multi-field - tags with unique-advanced OR unique-basic OR unique-performance",
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
