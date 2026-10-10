package searchtest

import (
	"fmt"
	"os"
	"strings"
	"time"

	"knov/internal/configmanager"
	"knov/internal/pathutils"
	"knov/internal/search"
	"knov/internal/searchStorage"
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

// caseSearchLoneCharNoFlood guards that a query which sanitizes down to lone
// ascii chars ("C++" -> "c", "+++" -> nothing) returns no results instead of a
// common-token flood - a flood counts as a hit and suppresses the trigram
// fallback the caller would otherwise reach.
func caseSearchLoneCharNoFlood() test.CaseResult {
	name := "search-lone-char-no-flood"

	results, err := searchStorage.SearchContent("C++", 50)
	if err != nil {
		return errCase(name, err)
	}

	cr := test.CaseResult{
		Name:     name,
		Expected: "0 results (query collapses to a lone char)",
		Actual:   fmt.Sprintf("%d results", len(results)),
		Success:  len(results) == 0,
	}
	if len(results) != 0 {
		cr.Error = "lone-char query flooded FTS instead of returning nothing"
	}
	return cr
}

// caseSearchMultiWordPartial guards the FTS5 MATCH query builder: a multi-word
// query where one word doesn't match and the last is only a prefix of an
// indexed token used to fail the old implicit-AND and return nothing.
func caseSearchMultiWordPartial() test.CaseResult {
	name := "search-multiword-partial"

	results, err := searchStorage.SearchContent("zzznomatch "+betaContentMarkerPrefix, 10)
	if err != nil {
		return errCase(name, err)
	}

	found := false
	for _, r := range results {
		if strings.HasSuffix(r.Path, betaFile) {
			found = true
		}
	}

	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("results contain %s despite a non-matching word and a truncated last word", betaFile),
		Actual:   fmt.Sprintf("%d results", len(results)),
		Success:  found,
	}
	if !found {
		cr.Error = fmt.Sprintf("%s not found - FTS5 MATCH query builder regressed to implicit AND", betaFile)
	}
	return cr
}

// caseSearchCommitReindexNoDuplicate guards that indexing a file by its docs-relative path and the
// periodic reindex (search.IndexAllFiles, from the file's metadata path) land on the same
// search_index row. A key mismatch (or a non-idempotent IndexFile) leaves two rows for one file,
// and SearchContent returns one result per row, so a duplicate shows up as two hits for the marker.
func caseSearchCommitReindexNoDuplicate() test.CaseResult {
	name := "search-commit-reindex-no-duplicate"

	const marker = "CommitHookIndexMarker"
	rel := testPath("commit-hook-indexed.md")
	full := pathutils.ToDocsPath(rel)
	content := fmt.Sprintf("# hook\n%s\n", marker)
	if err := writeFile(rel, content); err != nil {
		return errCase(name, err)
	}
	if err := saveMetadata(rel); err != nil {
		return errCase(name, err)
	}

	if err := searchStorage.IndexFile(pathutils.NewDocsRel(pathutils.ToSlash(rel)), []byte(content)); err != nil {
		return errCase(name, err)
	}

	// the reindex skips files whose mtime predates indexed_at, so push the mtime
	// forward to force IndexAllFiles to re-write this file's row too.
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(full, future, future); err != nil {
		return errCase(name, err)
	}
	if err := search.IndexAllFiles(); err != nil {
		return errCase(name, err)
	}

	results, err := searchStorage.SearchContent(marker, 10)
	if err != nil {
		return errCase(name, err)
	}

	cr := test.CaseResult{
		Name:     name,
		Expected: "exactly 1 index row after a docs-relative index + reindex",
		Actual:   fmt.Sprintf("%d results", len(results)),
		Success:  len(results) == 1,
	}
	if len(results) != 1 {
		cr.Error = "a direct index and the reindex disagree on the index key, or IndexFile is not idempotent"
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
	defer configmanager.SetSetting(configmanager.HidePaths, strings.Join(prev, ","))
	configmanager.SetSetting(configmanager.HidePaths, testDir+"::"+configmanager.HideScopeSearch)

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
