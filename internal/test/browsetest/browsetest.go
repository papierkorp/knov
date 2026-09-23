// Package browsetest - Browse suite: exercises the file-tree/folder-listing/browse-by-metadata/
// autocomplete/TOC pieces behind /browse (see docs/temp_todo.md step 5). The page routes
// themselves are template shells with no logic (data comes from the /api/files/* endpoints via
// htmx), so cases call the same exported functions/inline logic those handlers use directly.
// caseHideScopeEndpoints (scope_endpoints.go) is the one exception - it hits the real
// /api/files/tree, /api/files/list and /api/files/folder handlers over HTTP (an ephemeral
// httptest server, not the app's own port) because the thing under test is which HideScope*
// constant each handler hardcodes at its call site, which no shared function exposes.
package browsetest

import (
	"knov/internal/test"
)

// Suite runs the browse test cases against real files and metadata.
type Suite struct{}

func init() {
	test.Register(Suite{})
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
		caseHideScopeEndpoints,
	}

	return test.RunCases("browse", cases), nil
}
