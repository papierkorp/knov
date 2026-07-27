package browsetest

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/filter"
	"knov/internal/mapping"
	"knov/internal/pathutils"
	"knov/internal/test"
)

// findTreeNode walks a slash-separated relative path down from root, mirroring how
// files.BuildFileTree nests directories/files.
func findTreeNode(root *files.TreeNode, relPath string) *files.TreeNode {
	node := root
	for _, part := range strings.Split(relPath, "/") {
		var next *files.TreeNode
		for _, child := range node.Children {
			if child.Name == part {
				next = child
				break
			}
		}
		if next == nil {
			return nil
		}
		node = next
	}
	return node
}

func caseFileTree() test.CaseResult {
	name := "file-tree"

	allFiles, err := files.GetAllFilesCached()
	if err != nil {
		return errCase(name, err)
	}
	tree := files.BuildFileTree(files.FilterByVisibility(allFiles))

	alphaNode := findTreeNode(tree, testPath(alphaFile))
	subNode := findTreeNode(tree, subDir)
	betaNode := findTreeNode(tree, testPath(betaFile))

	success := alphaNode != nil && !alphaNode.IsDir &&
		subNode != nil && subNode.IsDir &&
		betaNode != nil && !betaNode.IsDir

	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("%s is a file, %s is a directory containing %s", testPath(alphaFile), subDir, testPath(betaFile)),
		Actual:   fmt.Sprintf("alpha=%v sub=%v beta=%v", alphaNode != nil, subNode != nil, betaNode != nil),
		Success:  success,
	}
	if !success {
		cr.Error = "BuildFileTree did not nest the sample files/folders as expected"
	}
	return cr
}

// caseFolderContents replicates handleAPIGetFolder's directory-listing logic (internal/server/
// api_files.go) directly, since the handler builds render.FolderEntry values inline with no
// exported equivalent to call.
func caseFolderContents() test.CaseResult {
	name := "folder-contents"

	fullPath := pathutils.ToDocsPath(testDir)
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return errCase(name, err)
	}

	var folders, filesInDir []string
	for _, entry := range entries {
		entryPath := pathutils.ToSlash(filepath.Join(testDir, entry.Name()))
		if entry.IsDir() {
			folders = append(folders, entryPath)
			continue
		}
		metadata, _ := files.MetaDataGet(pathutils.ToWithPrefix(entryPath))
		if metadata != nil && configmanager.IsFileTypeHidden(string(metadata.Editor)) {
			continue
		}
		filesInDir = append(filesInDir, entryPath)
	}

	success := slices.Contains(folders, subDir) &&
		slices.Contains(filesInDir, testPath(alphaFile)) &&
		slices.Contains(filesInDir, testPath(tocFile)) &&
		slices.Contains(filesInDir, testPath(hiddenFile))

	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("folders=[%s], files include alpha/toc/hidden", subDir),
		Actual:   fmt.Sprintf("folders=%v files=%v", folders, filesInDir),
		Success:  success,
	}
	if !success {
		cr.Error = "folder read did not split directories/files as expected"
	}
	return cr
}

// browseByField replicates handleAPIBrowseFiles' mapping+filter logic directly.
func browseByField(urlField, value string) ([]files.File, error) {
	criteria := []filter.Criteria{{
		Metadata: mapping.URLToDatabase(urlField),
		Operator: "equals",
		Value:    value,
		Action:   "include",
	}}
	if mapping.IsArrayField(urlField) {
		criteria[0].Operator = "contains"
	}
	return filter.FilterFiles(criteria, "and")
}

func containsFilePath(list []files.File, path string) bool {
	for _, f := range list {
		if pathutils.ToRelative(f.Path) == path {
			return true
		}
	}
	return false
}

func caseBrowseByTag() test.CaseResult {
	name := "browse-by-tag"

	result, err := browseByField("tag", marker)
	if err != nil {
		return errCase(name, err)
	}

	success := containsFilePath(result, testPath(alphaFile)) && !containsFilePath(result, testPath(betaFile))
	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("only %s carries tag %q", testPath(alphaFile), marker),
		Actual:   fmt.Sprintf("%d files matched", len(result)),
		Success:  success,
	}
	if !success {
		cr.Error = "browse-by-tag did not return the expected file set"
	}
	return cr
}

// caseBrowseByFolder browses by "sub" - metadata.Folders stores each path *segment*
// separately (see files.metaDataUpdate: currentMetadata.Folders = strings.Split(folderPath,
// "/")), not the joined folder path, so a segment unique to betaFile's folder is what
// actually discriminates it from alphaFile here.
func caseBrowseByFolder() test.CaseResult {
	name := "browse-by-folder"

	result, err := browseByField("folder", "sub")
	if err != nil {
		return errCase(name, err)
	}

	success := containsFilePath(result, testPath(betaFile)) && !containsFilePath(result, testPath(alphaFile))
	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("only %s has folder segment %q", testPath(betaFile), "sub"),
		Actual:   fmt.Sprintf("%d files matched", len(result)),
		Success:  success,
	}
	if !success {
		cr.Error = "browse-by-folder did not return the expected file set"
	}
	return cr
}

// caseAutocomplete replicates handleAPIFilesAutocomplete's substring-match logic directly -
// the handler builds its JSON result inline with no exported equivalent to call.
func caseAutocomplete() test.CaseResult {
	name := "autocomplete"

	allFiles, err := files.GetAllFilesCached()
	if err != nil {
		return errCase(name, err)
	}

	q := strings.ToLower("browse-alpha")
	found := false
	for _, f := range allFiles {
		if strings.Contains(strings.ToLower(pathutils.ToRelative(f.Path)), q) {
			found = true
			break
		}
	}

	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("query %q matches %s", q, testPath(alphaFile)),
		Actual:   fmt.Sprintf("found=%v", found),
		Success:  found,
	}
	if !found {
		cr.Error = "autocomplete substring match did not find the sample file"
	}
	return cr
}

func caseFolderSuggestions() test.CaseResult {
	name := "folder-suggestions"

	folders, err := files.GetAllFolderPathsFromCache()
	if err != nil {
		return errCase(name, err)
	}

	// folder paths are stored with a trailing slash (see files.ancestorFolderPaths)
	success := slices.Contains(folders, testDir+"/") && slices.Contains(folders, subDir+"/")
	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("folder list includes %s/ and %s/", testDir, subDir),
		Actual:   fmt.Sprintf("%d folders", len(folders)),
		Success:  success,
	}
	if !success {
		cr.Error = "GetAllFolderPathsFromCache did not include the sample folders"
	}
	return cr
}

// caseHeadersTOC covers the same GenerateTOC pass handleAPIFilesHeaders uses for wiki-link
// anchor autocomplete, via files.GetFileContent (the shared render+TOC pipeline).
func caseHeadersTOC() test.CaseResult {
	name := "headers-toc"

	content, err := files.GetFileContent(pathutils.ToDocsPath(testPath(tocFile)))
	if err != nil {
		return errCase(name, err)
	}

	success := len(content.TOC) == 3 &&
		content.TOC[0].Level == 1 && content.TOC[0].Text == "Heading One" &&
		content.TOC[1].Level == 2 && content.TOC[1].Text == "Heading Two" &&
		content.TOC[2].Level == 3 && content.TOC[2].Text == "Heading Three"

	cr := test.CaseResult{
		Name:     name,
		Expected: `3 TOC items: h1 "Heading One", h2 "Heading Two", h3 "Heading Three"`,
		Actual:   fmt.Sprintf("%d items: %v", len(content.TOC), content.TOC),
		Success:  success,
	}
	if !success {
		cr.Error = "GenerateTOC did not extract the sample headers as expected"
	}
	return cr
}

// caseHiddenFileTypeFilter toggles the todo-editor hide setting to exercise
// FilterByVisibility's editor-type branch, restoring the prior value afterward.
func caseHiddenFileTypeFilter() test.CaseResult {
	name := "hidden-file-type-filter"

	prev := configmanager.HideTodo.Get()
	defer configmanager.HideTodo.SetFromString(fmt.Sprintf("%v", prev))

	sample := []files.File{{Path: testPath(hiddenFile), Metadata: &files.Metadata{Editor: files.EditorTypeTodo}}}

	configmanager.HideTodo.SetFromString("false")
	shown := files.FilterByVisibility(sample)

	configmanager.HideTodo.SetFromString("true")
	hidden := files.FilterByVisibility(sample)

	success := len(shown) == 1 && len(hidden) == 0
	cr := test.CaseResult{
		Name:     name,
		Expected: "todo file shown when hideTodo=false, filtered out when hideTodo=true",
		Actual:   fmt.Sprintf("shown=%d hidden=%d", len(shown), len(hidden)),
		Success:  success,
	}
	if !success {
		cr.Error = "FilterByVisibility did not respect the hideTodo setting toggle"
	}
	return cr
}
