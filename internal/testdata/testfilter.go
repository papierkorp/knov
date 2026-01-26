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
	if err := files.SaveAllPARAProjectsToSystemData(); err != nil {
		logging.LogWarning("failed to update para projects cache: %v", err)
	}
	if err := files.SaveAllPARAAreasToSystemData(); err != nil {
		logging.LogWarning("failed to update para areas cache: %v", err)
	}
	if err := files.SaveAllPARAResourcesToSystemData(); err != nil {
		logging.LogWarning("failed to update para resources cache: %v", err)
	}
	if err := files.SaveAllPARAArchiveToSystemData(); err != nil {
		logging.LogWarning("failed to update para archive cache: %v", err)
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
			name: "para_projects_contains_unique_filter_system",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "folders",
						Operator: "contains",
						Value:    "filter-tests",
						Action:   "include",
					},
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
						Metadata: "folders",
						Operator: "contains",
						Value:    "filter-tests",
						Action:   "include",
					},
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
			expectedFiles: []string{"filter-tests/basic/filterTestG.md", "filter-tests/performance/filterTestK.md", "filter-tests/integration/filterTestH.md"},
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
			name: "name_regex_pattern_with_status",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "name",
						Operator: "regex",
						Value:    `^filterTest[A-C]\.md$`,
						Action:   "include",
					},
					{
						Metadata: "status",
						Operator: "equals",
						Value:    "published",
						Action:   "include",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 2,
			expectedFiles: []string{"filter-tests/filterTestB.md", "filter-tests/filterTestC.md"},
			description:   "filter by name using regex pattern (filterTestA-C.md) AND status equals published",
		},
		{
			name: "para_archive_contains_old",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "para_archive",
						Operator: "contains",
						Value:    "old",
						Action:   "include",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 2,
			expectedFiles: []string{"filter-tests/advanced/filterTestE.md", "filter-tests/performance/filterTestJ.md"},
			description:   "filter by PARA archive field containing 'old'",
		},
		{
			name: "para_archive_in_multiple_values",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "para_archive",
						Operator: "in",
						Value:    "unique_old_prototypes,unique_old_benchmarks",
						Action:   "include",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 2,
			expectedFiles: []string{"filter-tests/advanced/filterTestE.md", "filter-tests/performance/filterTestJ.md"},
			description:   "filter by PARA archive using 'in' operator with multiple values",
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
			name: "exclude_status_draft",
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
						Value:    "draft",
						Action:   "exclude",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 7,
			expectedFiles: []string{
				"filter-tests/filterTestB.md", "filter-tests/filterTestC.md",
				"filter-tests/advanced/filterTestE.md",
				"filter-tests/basic/filterTestG.md",
				"filter-tests/integration/filterTestH.md",
				"filter-tests/performance/filterTestJ.md", "filter-tests/performance/filterTestK.md",
			},
			description: "include filter-testing collection but exclude draft status",
		},
		{
			name: "empty_result_set",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "para_archive",
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
			description:   "query that should return no results (nonexistent PARA archive value)",
		},
		{
			name: "priority_in_high_or_low",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "priority",
						Operator: "in",
						Value:    "high,low",
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
			expectedCount: 8,
			expectedFiles: []string{
				"filter-tests/filterTestA.md", "filter-tests/filterTestC.md",
				"filter-tests/advanced/filterTestD.md", "filter-tests/basic/filterTestF.md",
				"filter-tests/integration/filterTestH.md", "filter-tests/integration/filterTestI.md",
				"filter-tests/performance/filterTestK.md", "filter-tests/special/filterTestL.md",
			},
			description: "filter by priority using 'in' operator (high OR low)",
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
			name: "complex_para_and_tags",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "para_areas",
						Operator: "contains",
						Value:    "unique",
						Action:   "include",
					},
					{
						Metadata: "tags",
						Operator: "contains",
						Value:    "specific",
						Action:   "include",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 9,
			expectedFiles: []string{
				"filter-tests/filterTestA.md", "filter-tests/filterTestC.md",
				"filter-tests/basic/filterTestF.md", "filter-tests/basic/filterTestG.md",
				"filter-tests/integration/filterTestH.md", "filter-tests/integration/filterTestI.md",
				"filter-tests/performance/filterTestJ.md", "filter-tests/performance/filterTestK.md",
				"filter-tests/special/filterTestL.md",
			},
			description: "complex filter combining PARA areas and tags",
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
			name: "multiple_exclude_criteria",
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
						Value:    "draft",
						Action:   "exclude",
					},
					{
						Metadata: "priority",
						Operator: "equals",
						Value:    "low",
						Action:   "exclude",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 4,
			expectedFiles: []string{
				"filter-tests/filterTestB.md",
				"filter-tests/advanced/filterTestE.md",
				"filter-tests/basic/filterTestG.md",
				"filter-tests/performance/filterTestJ.md",
			},
			description: "multiple exclude criteria - exclude both draft status AND low priority",
		},
		{
			name: "para_projects_in_operator",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "para_projects",
						Operator: "in",
						Value:    "unique_filter_system,unique_basic_functionality",
						Action:   "include",
					},
				},
				Logic: "and",
				Limit: 0,
			},
			expectedCount: 5,
			expectedFiles: []string{
				"filter-tests/filterTestA.md", "filter-tests/filterTestB.md",
				"filter-tests/advanced/filterTestD.md",
				"filter-tests/basic/filterTestF.md", "filter-tests/basic/filterTestG.md",
			},
			description: "filter PARA projects using 'in' operator - matches multiple project values",
		},
		{
			name: "or_include_multiple_statuses",
			config: filter.Config{
				Criteria: []filter.Criteria{
					{
						Metadata: "status",
						Operator: "equals",
						Value:    "draft",
						Action:   "include",
					},
					{
						Metadata: "status",
						Operator: "equals",
						Value:    "archived",
						Action:   "include",
					},
					{
						Metadata: "collection",
						Operator: "equals",
						Value:    "filter-testing-unique",
						Action:   "include",
					},
				},
				Logic: "or",
				Limit: 0,
			},
			expectedCount: 9,
			expectedFiles: []string{
				"filter-tests/filterTestA.md", "filter-tests/filterTestB.md", "filter-tests/filterTestC.md",
				"filter-tests/advanced/filterTestD.md", "filter-tests/advanced/filterTestE.md",
				"filter-tests/basic/filterTestF.md",
				"filter-tests/integration/filterTestI.md",
				"filter-tests/performance/filterTestJ.md",
				"filter-tests/special/filterTestL.md",
			},
			description: "or logic with include - status draft OR archived OR collection filter-testing-unique",
		},
		{
			name: "or_exclude_multiple_priorities",
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
						Value:    "high",
						Action:   "exclude",
					},
					{
						Metadata: "priority",
						Operator: "equals",
						Value:    "low",
						Action:   "exclude",
					},
				},
				Logic: "or",
				Limit: 0,
			},
			expectedCount: 4,
			expectedFiles: []string{
				"filter-tests/filterTestB.md",
				"filter-tests/advanced/filterTestE.md",
				"filter-tests/basic/filterTestG.md",
				"filter-tests/performance/filterTestJ.md",
			},
			description: "or logic with exclude - exclude high OR low priority (only medium remains)",
		},
		{
			name: "or_mixed_tags_with_status_exclude",
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
			expectedFiles: []string{
				"filter-tests/filterTestA.md", "filter-tests/filterTestB.md",
				"filter-tests/advanced/filterTestD.md",
			},
			description: "or logic mixed - include experimental OR stable tags, exclude archived",
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
