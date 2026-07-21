package githistorytest

import (
	"fmt"
	"strings"

	"knov/internal/files"
	"knov/internal/git"
	"knov/internal/pathutils"
	"knov/internal/test"
)

func caseGitLatestChangesPagination(_ *sampleState) test.CaseResult {
	name := "git-latestchanges-pagination"

	page1, err := git.GetRecentlyChangedFiles(1, 0)
	if err != nil {
		return errCase(name, err)
	}
	page2, err := git.GetRecentlyChangedFiles(1, 1)
	if err != nil {
		return errCase(name, err)
	}

	wantPage1 := pathutils.ToWithPrefix(testPath(etaFile))
	wantPage2 := pathutils.ToWithPrefix(testPath(gammaFile))
	success := len(page1) == 1 && len(page2) == 1 &&
		page1[0].Path == wantPage1 &&
		page2[0].Path == wantPage2

	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("offset 0 -> %s (most recent, the delete commit), offset 1 -> %s (gamma v2 commit)", wantPage1, wantPage2),
		Actual:   fmt.Sprintf("page1=%v page2=%v", pathsOf(page1), pathsOf(page2)),
		Success:  success,
	}
	if !success {
		cr.Error = "pagination did not skip/return the expected unique files in commit order"
	}
	return cr
}

func pathsOf(fs []git.GitHistoryFile) []string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = f.Path
	}
	return out
}

// caseGitLatestChangesCollectionFilter replicates handleAPIGetRecentlyChanged's inline
// collection filter (it's not extracted into internal/git, so this reproduces the same
// loop+files.MetaDataGet comparison directly, same approach editorstest uses for its
// unexported-handler bulk-op cases). Collection is derived from a file's top-level folder
// (files.CollectionFromPath), and every sample file here lives under "test/" (so the admin
// "Clean Test Data" button can remove it), meaning they all share the "test" collection -
// checks that collection=testCollection includes gamma while a bogus collection excludes it.
func caseGitLatestChangesCollectionFilter(_ *sampleState) test.CaseResult {
	name := "git-latestchanges-collection-filter"

	wantPath := pathutils.ToWithPrefix(testPath(gammaFile))

	filterByCollection := func(collection string) []string {
		all, err := git.GetRecentlyChangedFiles(20, 0)
		if err != nil {
			return nil
		}
		var filtered []string
		for _, f := range all {
			meta, err := files.MetaDataGet(f.Path)
			if err != nil || meta == nil {
				continue
			}
			if meta.Collection == collection {
				filtered = append(filtered, f.Path)
			}
		}
		return filtered
	}

	matching := filterByCollection(testCollection)
	nonMatching := filterByCollection("nonexistent-collection-zzz")

	hasGamma := containsPath(matching, wantPath)
	success := hasGamma && len(nonMatching) == 0

	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("collection=%s contains %s, a nonexistent collection matches nothing", testCollection, gammaFile),
		Actual:   fmt.Sprintf("matching=%v nonMatching=%v", matching, nonMatching),
		Success:  success,
	}
	if !success {
		cr.Error = "collection filter over latest-changes did not match expected files"
	}
	return cr
}

func containsPath(paths []string, want string) bool {
	for _, p := range paths {
		if p == want {
			return true
		}
	}
	return false
}

func caseGitSearchByFilename(_ *sampleState) test.CaseResult {
	name := "git-search-by-filename"

	results, err := git.SearchGitByTitle("gamma-versioned", 10, false)
	if err != nil {
		return errCase(name, err)
	}

	wantPath := pathutils.ToWithPrefix(testPath(gammaFile))
	found := false
	for _, f := range results {
		if f.Path == wantPath {
			found = true
		}
	}

	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("history search finds %s", gammaFile),
		Actual:   fmt.Sprintf("%d results", len(results)),
		Success:  found,
	}
	if !found {
		cr.Error = fmt.Sprintf("%s not found via SearchGitByTitle", gammaFile)
	}
	return cr
}

func caseGitFileHistoryVersions(_ *sampleState) test.CaseResult {
	name := "git-file-history-versions"

	versions, err := git.GetFileHistory(pathutils.ToDocsPath(testPath(gammaFile)))
	if err != nil {
		return errCase(name, err)
	}

	success := len(versions) >= 2 && versions[0].IsCurrent
	cr := test.CaseResult{
		Name:     name,
		Expected: "at least 2 versions, most recent marked current",
		Actual:   fmt.Sprintf("%d versions, first.IsCurrent=%v", len(versions), len(versions) > 0 && versions[0].IsCurrent),
		Success:  success,
	}
	if !success {
		cr.Error = "gamma file history missing expected version count/current flag"
	}
	return cr
}

func caseGitFileViewVersion(state *sampleState) test.CaseResult {
	name := "git-file-view-version"

	content, err := git.GetFileAtCommit(pathutils.ToDocsPath(testPath(gammaFile)), state.gammaCommit1)
	if err != nil {
		return errCase(name, err)
	}

	success := content == gammaContentV1
	cr := test.CaseResult{
		Name:     name,
		Expected: gammaContentV1,
		Actual:   content,
		Success:  success,
	}
	if !success {
		cr.Error = "content at gammaCommit1 did not match the v1 content it was committed with"
	}
	return cr
}

func caseGitFileDiff(state *sampleState) test.CaseResult {
	name := "git-file-diff"

	diff, _, _, err := git.GetFileDiff(pathutils.ToDocsPath(testPath(gammaFile)), state.gammaCommit1, state.gammaCommit2)
	if err != nil {
		return errCase(name, err)
	}

	success := strings.Contains(diff, "gamma version")
	cr := test.CaseResult{
		Name:     name,
		Expected: "unified diff mentioning the gamma content change",
		Actual:   diff,
		Success:  success,
	}
	if !success {
		cr.Error = "diff between gammaCommit1 and gammaCommit2 missing expected content"
	}
	return cr
}

func caseGitFileRestore(state *sampleState) test.CaseResult {
	name := "git-file-restore"

	if err := git.RestoreFileToCommit(pathutils.ToDocsPath(testPath(gammaFile)), state.gammaCommit1); err != nil {
		return errCase(name, err)
	}

	got, err := readFile(testPath(gammaFile))
	if err != nil {
		return errCase(name, err)
	}

	versions, err := git.GetFileHistory(pathutils.ToDocsPath(testPath(gammaFile)))
	if err != nil {
		return errCase(name, err)
	}

	success := got == gammaContentV1 && len(versions) >= 3
	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("content restored to v1, history grows to >=3 versions (got %q, %d versions)", gammaContentV1, len(versions)),
		Actual:   fmt.Sprintf("content=%q versions=%d", got, len(versions)),
		Success:  success,
	}
	if !success {
		cr.Error = "restore did not produce v1 content and/or a new history entry"
	}
	return cr
}
