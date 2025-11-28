// Package testdata - Filter testing functionality
package testdata

import (
	"fmt"

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
}

// CreateFilterTestMetadata creates 12 test metadata objects
func CreateFilterTestMetadata() error {
	logging.LogInfo("creating filter test metadata")

	// first, create the actual test files on disk (clears existing filter-tests folder)
	if err := createFilterTestFiles(); err != nil {
		return fmt.Errorf("failed to create filter test files: %v", err)
	}

	// get metadata definitions from separate file
	testMetadataList := getFilterTestMetadata()

	// save each metadata object
	for _, metadata := range testMetadataList {
		if err := files.MetaDataSave(metadata); err != nil {
			logging.LogError("failed to save metadata for %s: %v", metadata.Path, err)
			return fmt.Errorf("failed to save metadata for %s: %v", metadata.Path, err)
		}
		logging.LogDebug("saved metadata for %s", metadata.Path)
	}

	logging.LogInfo("filter test metadata created successfully")
	return nil
}

// GetFilterTestMetadata returns the metadata definitions for all filter test files
func GetFilterTestMetadata() []*files.Metadata {
	return getFilterTestMetadata()
}

// RunFilterTests executes various filter test scenarios
func RunFilterTests() (*FilterTestResults, error) {
	logging.LogInfo("running filter tests")

	// ensure test data exists
	if err := CreateFilterTestMetadata(); err != nil {
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
			name: "para_projects_contains_unique_filter_system",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "para_projects",
						Operator: "contains",
						Value:    "unique_filter_system",
						Action:   "include",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 3,
			expectedFiles: []string{"filter-tests/filterTestA.md", "filter-tests/filterTestB.md", "filter-tests/advanced/filterTestD.md"},
			description:   "filter by PARA projects containing 'unique_filter_system'",
		},
		{
			name: "para_areas_contains_filter_development",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "para_areas",
						Operator: "contains",
						Value:    "filter_development",
						Action:   "include",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 3,
			expectedFiles: []string{"filter-tests/filterTestA.md", "filter-tests/filterTestB.md", "filter-tests/advanced/filterTestD.md"},
			description:   "filter by PARA areas containing 'filter_development'",
		},
		{
			name: "exclude_archived_status",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "collection",
						Operator: "contains",
						Value:    "filter-testing",
						Action:   "include",
					},
					{
						Metadata: "status",
						Operator: "equals",
						Value:    "archived",
						Action:   "exclude",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 10,
			expectedFiles: []string{
				"filter-tests/filterTestA.md", "filter-tests/filterTestB.md", "filter-tests/filterTestC.md",
				"filter-tests/advanced/filterTestD.md", "filter-tests/basic/filterTestF.md", "filter-tests/basic/filterTestG.md",
				"filter-tests/integration/filterTestH.md", "filter-tests/integration/filterTestI.md",
				"filter-tests/performance/filterTestK.md", "filter-tests/special/filterTestL.md",
			},
			description: "filter by collection containing 'filter-testing' AND exclude archived status",
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
			name: "complex_unique_filter",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "collection",
						Operator: "contains",
						Value:    "filter-testing",
						Action:   "include",
					},
					{
						Metadata: "priority",
						Operator: "equals",
						Value:    "medium",
						Action:   "include",
					},
					{
						Metadata: "status",
						Operator: "equals",
						Value:    "archived",
						Action:   "exclude",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 2,
			expectedFiles: []string{"filter-tests/filterTestB.md", "filter-tests/basic/filterTestG.md"},
			description:   "complex filter: collection contains 'filter-testing' AND priority medium AND exclude archived",
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
			name: "date_edited_before_november_6",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "lastEdited",
						Operator: "less",
						Value:    "2025-11-06",
						Action:   "include",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 5,
			expectedFiles: []string{"filter-tests/filterTestA.md", "filter-tests/filterTestB.md", "filter-tests/filterTestC.md", "filter-tests/advanced/filterTestD.md", "filter-tests/advanced/filterTestE.md"},
			description:   "filter by last edited date before November 6, 2025",
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
			expectedCount: 4,
			expectedFiles: []string{"filter-tests/advanced/filterTestD.md", "filter-tests/advanced/filterTestE.md", "filter-tests/basic/filterTestF.md", "filter-tests/basic/filterTestG.md"},
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
			name: "multiple_filetypes_in_array",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "type",
						Operator: "in",
						Value:    "fleeting,literature",
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
			expectedCount: 6,
			expectedFiles: []string{"filter-tests/filterTestA.md", "filter-tests/filterTestB.md", "filter-tests/advanced/filterTestD.md", "filter-tests/advanced/filterTestE.md", "filter-tests/integration/filterTestH.md", "filter-tests/performance/filterTestJ.md"},
			description:   "filter by multiple file types using 'in' array: fleeting, literature",
		},
		{
			name: "para_projects_array_filtering",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "para_projects",
						Operator: "contains",
						Value:    "unique_filter_system",
						Action:   "include",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 3,
			expectedFiles: []string{"filter-tests/filterTestA.md", "filter-tests/filterTestB.md", "filter-tests/advanced/filterTestD.md"},
			description:   "filter by PARA projects containing 'unique_filter_system'",
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
			name: "complex_multi_criteria_with_dates",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "createdAt",
						Operator: "greater",
						Value:    "2025-10-05",
						Action:   "include",
					},
					{
						Metadata: "status",
						Operator: "equals",
						Value:    "published",
						Action:   "include",
					},
					{
						Metadata: "priority",
						Operator: "in",
						Value:    "medium,low",
						Action:   "include",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 2,
			expectedFiles: []string{"filter-tests/basic/filterTestG.md", "filter-tests/performance/filterTestK.md"},
			description:   "complex multi-criteria: created after Oct 5 AND published status AND medium/low priority",
		},
		{
			name: "or_logic_with_exclusions",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "tags",
						Operator: "contains",
						Value:    "unique-experimental",
						Action:   "include",
					},
					{
						Metadata: "tags",
						Operator: "contains",
						Value:    "unique-stable",
						Action:   "include",
					},
					{
						Metadata: "status",
						Operator: "equals",
						Value:    "archived",
						Action:   "exclude",
					},
				},
				Logic: "or",
				Limit: 0,
			},
			expectedCount: 3,
			expectedFiles: []string{"filter-tests/filterTestA.md", "filter-tests/filterTestB.md", "filter-tests/advanced/filterTestD.md"},
			description:   "OR logic with exclusion: (experimental OR stable tags) AND NOT archived",
		},
		{
			name: "complex_exclude_multiple_criteria",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "type",
						Operator: "in",
						Value:    "fleeting,literature,permanent",
						Action:   "exclude",
					},
					{
						Metadata: "priority",
						Operator: "equals",
						Value:    "high",
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
			expectedCount: 2,
			expectedFiles: []string{"filter-tests/basic/filterTestG.md", "filter-tests/performance/filterTestK.md"},
			description:   "exclude multiple file types AND exclude high priority (only journaling/MOC with medium/low priority)",
		},
		{
			name: "para_areas_and_resources_filtering",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "para_areas",
						Operator: "contains",
						Value:    "unique_quality_assurance",
						Action:   "include",
					},
					{
						Metadata: "para_resources",
						Operator: "contains",
						Value:    "docs",
						Action:   "include",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 1,
			expectedFiles: []string{"filter-tests/basic/filterTestF.md"},
			description:   "filter by PARA areas containing 'unique_quality_assurance' AND resources containing 'docs'",
		},
	}

	// run each test
	for _, test := range testConfigs {
		logging.LogDebug("running filter test: %s", test.name)

		result, err := filter.FilterFilesWithConfig(&test.config)
		if err != nil {
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
		} else {
			results.PassedTests++
		}

		results.Results = append(results.Results, testResult)
	}

	results.TotalTests = len(testConfigs)
	results.Success = results.FailedTests == 0

	logging.LogInfo("filter tests completed: %d passed, %d failed", results.PassedTests, results.FailedTests)
	return results, nil
}
