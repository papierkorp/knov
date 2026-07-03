package filtertest

import (
	"fmt"
	"path/filepath"
	"slices"

	"knov/internal/filter"
	"knov/internal/test"
)

// testConfig is a single filter test scenario: a filter.Config plus its expected result.
type testConfig struct {
	name          string
	config        filter.Config
	expectedCount int
	expectedFiles []string
}

var testConfigs = []testConfig{
	{
		name: "test1and",
		config: filter.Config{
			Criteria: []filter.Criteria{
				{
					Metadata: "folders",
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
	},
	{
		name: "test4exclude_single",
		config: filter.Config{
			Criteria: []filter.Criteria{
				{
					Metadata: "folders",
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
	},
	{
		name: "test5exclude_folder",
		config: filter.Config{
			Criteria: []filter.Criteria{
				{
					Metadata: "folders",
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
	},
	{
		name: "test6regex",
		config: filter.Config{
			Criteria: []filter.Criteria{
				{
					Metadata: "folders",
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
	},
	{
		name: "test7greaterthan",
		config: filter.Config{
			Criteria: []filter.Criteria{
				{
					Metadata: "folders",
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
	},
	{
		name: "test8lessthan",
		config: filter.Config{
			Criteria: []filter.Criteria{
				{
					Metadata: "folders",
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
	},
	{
		name: "test9inarray_tags",
		config: filter.Config{
			Criteria: []filter.Criteria{
				{
					Metadata: "folders",
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
	},
	{
		name: "test10childof",
		config: filter.Config{
			Criteria: []filter.Criteria{
				{
					Metadata: "folders",
					Operator: "equals",
					Value:    "filter-tests",
					Action:   "include",
				},
				{
					Metadata: "child-of",
					Operator: "equals",
					Value:    "test/filter-tests/filterTestD.md",
					Action:   "include",
				},
			},
			Logic: "and",
			Limit: 0,
		},
		expectedCount: 1,
		expectedFiles: []string{"filterTestE.md"},
	},
	{
		name: "test11parentof",
		config: filter.Config{
			Criteria: []filter.Criteria{
				{
					Metadata: "folders",
					Operator: "equals",
					Value:    "filter-tests",
					Action:   "include",
				},
				{
					Metadata: "parent-of",
					Operator: "equals",
					Value:    "test/filter-tests/filterTestE.md",
					Action:   "include",
				},
			},
			Logic: "and",
			Limit: 0,
		},
		expectedCount: 1,
		expectedFiles: []string{"filterTestD.md"},
	},
	{
		name: "test12ancestorof",
		config: filter.Config{
			Criteria: []filter.Criteria{
				{
					Metadata: "folders",
					Operator: "equals",
					Value:    "filter-tests",
					Action:   "include",
				},
				{
					Metadata: "ancestor-of",
					Operator: "equals",
					Value:    "test/filter-tests/filterTestD.md",
					Action:   "include",
				},
			},
			Logic: "and",
			Limit: 0,
		},
		expectedCount: 2,
		expectedFiles: []string{"filterTestE.md", "filterTestF.md"},
	},
	{
		name: "test13multiple_filters_1",
		config: filter.Config{
			Criteria: []filter.Criteria{
				{
					Metadata: "folders",
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
	},
	{
		name: "test14multiple_filters_2",
		config: filter.Config{
			Criteria: []filter.Criteria{
				{
					Metadata: "folders",
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
	},
	{
		name: "test15multiple_filters_3",
		config: filter.Config{
			Criteria: []filter.Criteria{
				{
					Metadata: "folders",
					Operator: "equals",
					Value:    "filter-tests",
					Action:   "include",
				},
				{
					Metadata: "child-of",
					Operator: "equals",
					Value:    "test/filter-tests/filterTestD.md",
					Action:   "include",
				},
				{
					Metadata: "parent-of",
					Operator: "equals",
					Value:    "test/filter-tests/filterTestF.md",
					Action:   "include",
				},
				{
					Metadata: "ancestor-of",
					Operator: "equals",
					Value:    "test/filter-tests/filterTestD.md",
					Action:   "include",
				},
			},
			Logic: "and",
			Limit: 0,
		},
		expectedCount: 1,
		expectedFiles: []string{"filterTestE.md"},
	},
	{
		name: "test16datecontains",
		config: filter.Config{
			Criteria: []filter.Criteria{
				{
					Metadata: "folders",
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
	},
	{
		name: "test17dateregex",
		config: filter.Config{
			Criteria: []filter.Criteria{
				{
					Metadata: "folders",
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
	},
	{
		name: "test18references",
		config: filter.Config{
			Criteria: []filter.Criteria{
				{
					Metadata: "folders",
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
	},
}

// runCase executes a single scenario against the real filter engine and compares the
// matched files to what the scenario expects.
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
