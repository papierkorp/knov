// Package browsetest - Browse suite: exercises the file-tree/folder-listing/browse-by-metadata/
// autocomplete/TOC pieces behind /browse (see docs/temp_todo.md step 5). The page routes
// themselves are template shells with no logic (data comes from the /api/files/* endpoints via
// htmx), so cases call the same exported functions/inline logic those handlers use directly.
package browsetest

import (
	"knov/internal/job"
	"knov/internal/test"
)

// Suite runs the browse test cases against real files and metadata.
type Suite struct{}

func init() {
	test.Register(Suite{})
	job.RegisterSuiteRunner("browse-test", func() (*test.SuiteResult, error) { return (Suite{}).Run() })
}

func (Suite) Name() string { return "browse" }

func (Suite) Run() (*test.SuiteResult, error) {
	if err := resetAndSeed(); err != nil {
		return nil, err
	}

	cases := []func() test.CaseResult{
		caseFileTree,
		caseFolderContents,
		caseBrowseByTag,
		caseBrowseByFolder,
		caseAutocomplete,
		caseFolderSuggestions,
		caseHeadersTOC,
		caseHiddenFileTypeFilter,
	}

	result := &test.SuiteResult{Suite: "browse"}
	for _, c := range cases {
		cr := c()
		result.Cases = append(result.Cases, cr)
		if cr.Success {
			result.Passed++
		} else {
			result.Failed++
		}
	}
	result.Total = len(cases)
	result.Success = result.Failed == 0
	return result, nil
}
