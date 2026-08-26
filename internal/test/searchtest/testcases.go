package searchtest

import (
	"fmt"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/search"
	"knov/internal/test"
)

func caseSearchTitleOnly() test.CaseResult {
	name := "search-title-only"

	results, err := search.SearchFilesByTitle("AlphaUniqueTitle", 10)
	if err != nil {
		return errCase(name, err)
	}

	found := false
	for _, f := range results {
		if f.Name == alphaFile {
			found = true
		}
	}

	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("results contain %s", alphaFile),
		Actual:   fmt.Sprintf("%d results", len(results)),
		Success:  found,
	}
	if !found {
		cr.Error = fmt.Sprintf("%s not found in title-only search results", alphaFile)
	}
	return cr
}

func caseSearchFullContent() test.CaseResult {
	name := "search-full-content"

	results, err := search.SearchFiles(betaContentMarker, 10)
	if err != nil {
		return errCase(name, err)
	}

	found := false
	for _, f := range results {
		if f.Name == betaFile {
			found = true
		}
	}

	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("results contain %s (matched by content, not filename)", betaFile),
		Actual:   fmt.Sprintf("%d results", len(results)),
		Success:  found,
	}
	if !found {
		cr.Error = fmt.Sprintf("%s not found in full-content search results", betaFile)
	}
	return cr
}

func caseSearchEmptyQuery() test.CaseResult {
	name := "search-empty-query"

	results, err := search.SearchFiles("", 10)
	if err != nil {
		return errCase(name, err)
	}

	success := len(results) == 0
	cr := test.CaseResult{
		Name:     name,
		Expected: "0 results",
		Actual:   fmt.Sprintf("%d results", len(results)),
		Success:  success,
	}
	if !success {
		cr.Error = "empty query should return no results"
	}
	return cr
}

// caseSearchLimit exercises the limit parameter every response format (dropdown/list/
// cards/json) plugs into handleAPISearch with a different value (6/50/20/100) - the
// render functions themselves live in internal/server/render and aren't exercised here,
// since none of these cases need HTML output - this covers the shared truncation behavior
// that backs every format instead.
func caseSearchLimit() test.CaseResult {
	name := "search-limit"

	unlimited, err := search.SearchFilesByTitle("", 0)
	if err != nil {
		return errCase(name, err)
	}
	limited, err := search.SearchFilesByTitle("", 1)
	if err != nil {
		return errCase(name, err)
	}

	success := len(unlimited) >= 2 && len(limited) == 1
	cr := test.CaseResult{
		Name:     name,
		Expected: "limit=0 returns all matches, limit=1 truncates to 1",
		Actual:   fmt.Sprintf("unlimited=%d limited=%d", len(unlimited), len(limited)),
		Success:  success,
	}
	if !success {
		cr.Error = "SearchFilesByTitle did not truncate to the requested limit"
	}
	return cr
}

func caseSearchDeletedFileByTitle() test.CaseResult {
	name := "search-deleted-file-by-title"

	results, err := search.SearchDeletedFilesByTitle("DeltaDeletedUniqueMarker", 10)
	if err != nil {
		return errCase(name, err)
	}

	found := false
	for _, f := range results {
		if f.Name == deltaFile {
			found = true
		}
	}

	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("deleted-file history contains %s", deltaFile),
		Actual:   fmt.Sprintf("%d results", len(results)),
		Success:  found,
	}
	if !found {
		cr.Error = fmt.Sprintf("%s not found via deleted-file title search", deltaFile)
	}
	return cr
}

// caseSearchScopedHidePath checks that SearchFilesByTitle/SearchFiles pass
// configmanager.HideScopeSearch when filtering by visibility, by tagging the sample folder
// so it's hidden only in the search scope - if either call site regressed to passing "" or a
// different scope constant, the tag would never match and the sample files would still show
// up in every result here instead of dropping out.
func caseSearchScopedHidePath() test.CaseResult {
	name := "search-scoped-hide-path"

	prev := configmanager.HidePaths.Get()
	defer configmanager.HidePaths.SetFromString(strings.Join(prev, ","))
	configmanager.HidePaths.SetFromString(testDir + "::" + configmanager.HideScopeSearch)

	byTitle, err := search.SearchFilesByTitle("AlphaUniqueTitle", 10)
	if err != nil {
		return errCase(name, err)
	}
	byContent, err := search.SearchFiles(betaContentMarker, 10)
	if err != nil {
		return errCase(name, err)
	}

	foundTitle, foundContent := false, false
	for _, f := range byTitle {
		if f.Name == alphaFile {
			foundTitle = true
		}
	}
	for _, f := range byContent {
		if f.Name == betaFile {
			foundContent = true
		}
	}

	success := !foundTitle && !foundContent
	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("%s tagged ::search hides %s and %s from search", testDir, alphaFile, betaFile),
		Actual:   fmt.Sprintf("foundTitle=%v foundContent=%v", foundTitle, foundContent),
		Success:  success,
	}
	if !success {
		cr.Error = "SearchFilesByTitle/SearchFiles did not apply configmanager.HideScopeSearch"
	}
	return cr
}

func caseSearchDeletedFileByContent() test.CaseResult {
	name := "search-deleted-file-by-content"

	results, err := search.SearchDeletedFilesByContent(deltaContentMarker, 10)
	if err != nil {
		return errCase(name, err)
	}

	found := false
	for _, f := range results {
		if f.Name == deltaFile {
			found = true
		}
	}

	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("deleted-file history contains %s (matched by content)", deltaFile),
		Actual:   fmt.Sprintf("%d results", len(results)),
		Success:  found,
	}
	if !found {
		cr.Error = fmt.Sprintf("%s not found via deleted-file content search", deltaFile)
	}
	return cr
}
