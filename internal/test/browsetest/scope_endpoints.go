// Package browsetest - hide-path scope wiring check: /api/files/tree, /api/files/list and
// /api/files/folder each hardcode a HideScope* constant at their call site (see
// internal/server/api_files_cache.go and api_files.go) rather than sharing one function, so a
// future edit could swap two of them with no compile-time signal. This spins up the real
// router on an ephemeral port and hits all three endpoints to prove each one actually applies
// its own scope instead of another endpoint's.
package browsetest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/pathutils"
	"knov/internal/server"
	"knov/internal/test"
)

const (
	scopeTreeFolder     = testDir + "/hide-scope-tree"
	scopeOverviewFolder = testDir + "/hide-scope-overview"
	scopeBrowseFolder   = testDir + "/hide-scope-browse"
)

// seedScopeProbes writes one file into each of three folders, one per endpoint under test,
// and tags each folder so it's hidden only in the scope that folder is named for.
func seedScopeProbes() error {
	probes := []struct {
		folder, scope string
	}{
		{scopeTreeFolder, configmanager.HideScopeTree},
		{scopeOverviewFolder, configmanager.HideScopeOverview},
		{scopeBrowseFolder, configmanager.HideScopeBrowse},
	}

	var entries []string
	for _, p := range probes {
		relPath := p.folder + "/probe.md"
		if err := writeFile(relPath, "# scope probe\n"); err != nil {
			return err
		}
		if err := test.SeedMetadataNoRefresh(&files.Metadata{
			Path:   pathutils.ToWithPrefix(relPath),
			Editor: files.EditorTypeCodeMirror,
		}); err != nil {
			return err
		}
		entries = append(entries, p.folder+"::"+p.scope)
	}

	files.InvalidateFileListCache()
	return configmanager.SetSetting(configmanager.HidePaths, strings.Join(entries, ","))
}

func getJSON(client *http.Client, url string, out interface{}) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: unexpected status %d", url, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// caseHideScopeEndpoints checks that /api/files/tree hides only the tree-scoped probe (while
// showing the other two), /api/files/list hides only the overview-scoped probe, and
// /api/files/folder (browse) hides only the browse-scoped probe - i.e. each handler is wired
// to its own HideScope* constant.
func caseHideScopeEndpoints() test.CaseResult {
	name := "hide-scope-endpoints"

	prevHidePaths := configmanager.HidePaths.Get()
	defer func() {
		configmanager.SetSetting(configmanager.HidePaths, strings.Join(prevHidePaths, ","))
		files.InvalidateFileListCache()
	}()

	if err := seedScopeProbes(); err != nil {
		return errCase(name, err)
	}

	ts := httptest.NewServer(server.NewRouter())
	defer ts.Close()
	client := ts.Client()

	var treeFiles, overviewFiles []files.File
	if err := getJSON(client, ts.URL+"/api/files/tree", &treeFiles); err != nil {
		return errCase(name, err)
	}
	if err := getJSON(client, ts.URL+"/api/files/list", &overviewFiles); err != nil {
		return errCase(name, err)
	}
	var folderResp struct {
		Folders []struct{ Name string }
	}
	if err := getJSON(client, ts.URL+"/api/files/folder?path="+pathutils.ToWithPrefix(testDir), &folderResp); err != nil {
		return errCase(name, err)
	}
	browseFolderShown := func(name string) bool {
		for _, f := range folderResp.Folders {
			if f.Name == name {
				return true
			}
		}
		return false
	}

	treeProbe, overviewProbe, browseProbe := scopeTreeFolder+"/probe.md", scopeOverviewFolder+"/probe.md", scopeBrowseFolder+"/probe.md"

	treeHidesOwn := !containsFilePath(treeFiles, treeProbe)
	treeShowsOthers := containsFilePath(treeFiles, overviewProbe) && containsFilePath(treeFiles, browseProbe)

	overviewHidesOwn := !containsFilePath(overviewFiles, overviewProbe)
	overviewShowsOthers := containsFilePath(overviewFiles, treeProbe) && containsFilePath(overviewFiles, browseProbe)

	browseHidesOwn := !browseFolderShown("hide-scope-browse")
	browseShowsOthers := browseFolderShown("hide-scope-tree") && browseFolderShown("hide-scope-overview")

	success := treeHidesOwn && treeShowsOthers && overviewHidesOwn && overviewShowsOthers && browseHidesOwn && browseShowsOthers
	cr := test.CaseResult{
		Name:     name,
		Expected: "each endpoint hides only its own scope-tagged probe folder and shows the other two",
		Actual: fmt.Sprintf("tree(hidesOwn=%v showsOthers=%v) list(hidesOwn=%v showsOthers=%v) folder(hidesOwn=%v showsOthers=%v)",
			treeHidesOwn, treeShowsOthers, overviewHidesOwn, overviewShowsOthers, browseHidesOwn, browseShowsOthers),
		Success: success,
	}
	if !success {
		cr.Error = "a /api/files/tree, /api/files/list or /api/files/folder handler is not using its documented HideScope* constant"
	}
	return cr
}
