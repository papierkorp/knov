package kanbantest

import (
	"fmt"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/filter"
	"knov/internal/kanban"
	"knov/internal/pathutils"
	"knov/internal/server/render"
	"knov/internal/test"
)

// emptyFilterConfig returns a filter config with no criteria - folder scoping is handled
// directly by BuildBoard's folderPath parameter, not injected as a filter criterion.
func emptyFilterConfig() *filter.Config {
	return &filter.Config{Logic: "and"}
}

func columnPaths(cols []kanban.Column, status string) []string {
	for _, c := range cols {
		if c.Status == status {
			paths := make([]string, len(c.Cards))
			for i, card := range c.Cards {
				paths[i] = card.FilePath
			}
			return paths
		}
	}
	return nil
}

func containsPath(paths []string, target string) bool {
	for _, p := range paths {
		if p == target {
			return true
		}
	}
	return false
}

func caseBoardLoadColumns() test.CaseResult {
	name := "board-load-columns"

	cols, err := kanban.BuildBoard(testFolder, emptyFilterConfig(), "", "")
	if err != nil {
		return errCase(name, err)
	}

	inbox := columnPaths(cols, "inbox")
	inprogress := columnPaths(cols, "inprogress")
	blocked := columnPaths(cols, "blocked")

	success := containsPath(inbox, testPath(alphaFile)) && containsPath(inbox, testPath(betaFile)) &&
		containsPath(inprogress, testPath(gammaFile)) && containsPath(blocked, testPath(deltaFile))

	cr := test.CaseResult{
		Name:     name,
		Expected: "alpha+beta in inbox, gamma in inprogress, delta in blocked",
		Actual:   fmt.Sprintf("inbox=%d inprogress=%d blocked=%d", len(inbox), len(inprogress), len(blocked)),
		Success:  success,
	}
	if !success {
		cr.Error = "BuildBoard did not bucket sample cards into the expected columns"
	}
	return cr
}

// caseHideScopeKanban checks that BuildBoard passes configmanager.HideScopeKanban when
// filtering by visibility, by tagging the board's own folder so it's hidden only in the
// kanban scope - if the call site regressed to passing "" or a different scope constant,
// the tag would never match and every card would still show up on the board here instead
// of the board coming back empty.
func caseHideScopeKanban() test.CaseResult {
	name := "hide-scope-kanban"

	prev := configmanager.HidePaths.Get()
	defer configmanager.SetSetting(configmanager.HidePaths, strings.Join(prev, ","))
	configmanager.SetSetting(configmanager.HidePaths, testFolder+"::"+configmanager.HideScopeKanban)

	cols, err := kanban.BuildBoard(testFolder, emptyFilterConfig(), "", "")
	if err != nil {
		return errCase(name, err)
	}

	inbox := columnPaths(cols, "inbox")
	success := !containsPath(inbox, testPath(alphaFile))
	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("%s tagged ::kanban hides %s from the board", testFolder, alphaFile),
		Actual:   fmt.Sprintf("inbox=%d", len(inbox)),
		Success:  success,
	}
	if !success {
		cr.Error = "BuildBoard did not apply configmanager.HideScopeKanban"
	}
	return cr
}

// caseHiddenByTag checks that a HideFilesByTag pattern matching a status tag empties that
// column and that the rendered board shows the hidden warning only on that column.
func caseHiddenByTag() test.CaseResult {
	name := "hidden-by-tag"

	prev := configmanager.HideFilesByTag.Get()
	defer configmanager.SetSetting(configmanager.HideFilesByTag, strings.Join(prev, ","))
	if err := configmanager.SetSetting(configmanager.HideFilesByTag, configmanager.KanbanStatusTag("inbox")+"::"+configmanager.HideScopeKanban); err != nil {
		return errCase(name, err)
	}

	cols, err := kanban.BuildBoard(testFolder, emptyFilterConfig(), "", "")
	if err != nil {
		return errCase(name, err)
	}

	board := configmanager.KanbanBoard{FolderPath: testFolder}
	var inboxHidden, inprogressHidden bool
	for _, c := range cols {
		warned := strings.Contains(render.RenderKanbanBoard([]kanban.Column{c}, board), "kanban-hidden-warning")
		switch c.Status {
		case "inbox":
			inboxHidden = warned
		case "inprogress":
			inprogressHidden = warned
		}
	}
	inbox := columnPaths(cols, "inbox")
	success := inboxHidden && !inprogressHidden && len(inbox) == 0
	cr := test.CaseResult{
		Name:     name,
		Expected: "inbox shows the hidden warning and is empty, inprogress shows no warning",
		Actual:   fmt.Sprintf("inboxHidden=%v inprogressHidden=%v inbox=%d", inboxHidden, inprogressHidden, len(inbox)),
		Success:  success,
	}
	if !success {
		cr.Error = "RenderKanbanBoard did not warn on the column whose status tag is hidden by HideFilesByTag"
	}
	return cr
}

func caseBoardSearchQuery() test.CaseResult {
	name := "board-search-query"

	cols, err := kanban.BuildBoard(testFolder, emptyFilterConfig(), "Gamma", "")
	if err != nil {
		return errCase(name, err)
	}

	inbox := columnPaths(cols, "inbox")
	inprogress := columnPaths(cols, "inprogress")

	success := len(inbox) == 0 && containsPath(inprogress, testPath(gammaFile)) && len(inprogress) == 1
	cr := test.CaseResult{
		Name:     name,
		Expected: "search query \"Gamma\" narrows the board to just the gamma card",
		Actual:   fmt.Sprintf("inbox=%d inprogress=%d", len(inbox), len(inprogress)),
		Success:  success,
	}
	if !success {
		cr.Error = "BuildBoard's search query did not filter down to the matching card"
	}
	return cr
}

// caseBoardSorting exercises SortCreatedAt (oldest first) and SortAlphabetical against the
// inbox column, where alpha's title sorts before beta's but alpha was seeded with the later
// CreatedAt - so the two sort modes disagree on order, proving each is actually applied.
func caseBoardSorting() test.CaseResult {
	name := "board-sorting"

	byCreated, err := kanban.BuildBoard(testFolder, emptyFilterConfig(), "", kanban.SortCreatedAt)
	if err != nil {
		return errCase(name, err)
	}
	byAlpha, err := kanban.BuildBoard(testFolder, emptyFilterConfig(), "", kanban.SortAlphabetical)
	if err != nil {
		return errCase(name, err)
	}

	createdOrder := columnPaths(byCreated, "inbox")
	alphaOrder := columnPaths(byAlpha, "inbox")

	createdOK := len(createdOrder) >= 2 && createdOrder[0] == testPath(betaFile) && createdOrder[1] == testPath(alphaFile)
	alphaOK := len(alphaOrder) >= 2 && alphaOrder[0] == testPath(alphaFile) && alphaOrder[1] == testPath(betaFile)

	success := createdOK && alphaOK
	cr := test.CaseResult{
		Name:     name,
		Expected: "createdAt sort: beta,alpha (oldest first) - alphabetical sort: alpha,beta",
		Actual:   fmt.Sprintf("createdAt=%v alphabetical=%v", createdOrder, alphaOrder),
		Success:  success,
	}
	if !success {
		cr.Error = "SortCreatedAt/SortAlphabetical did not order the inbox column as expected"
	}
	return cr
}

func caseMoveCard() test.CaseResult {
	name := "move-card"

	oldStatus, _, err := kanban.MoveCard(testFolder, testPath(moveFile), "inprogress")
	if err != nil {
		return errCase(name, err)
	}

	cols, err := kanban.BuildBoard(testFolder, emptyFilterConfig(), "", "")
	if err != nil {
		return errCase(name, err)
	}

	success := oldStatus == "" && containsPath(columnPaths(cols, "inprogress"), testPath(moveFile))
	cr := test.CaseResult{
		Name:     name,
		Expected: "oldStatus=\"\" (unstatused before move), card now in inprogress",
		Actual:   fmt.Sprintf("oldStatus=%q inInprogress=%v", oldStatus, containsPath(columnPaths(cols, "inprogress"), testPath(moveFile))),
		Success:  success,
	}
	if !success {
		cr.Error = "MoveCard did not move the card to the target column as expected"
	}
	return cr
}

// caseMoveCardEventLog depends on caseMoveCard having already moved moveFile to inprogress -
// it moves it again to blocked and checks both moves are recorded, newest first.
func caseMoveCardEventLog() test.CaseResult {
	name := "move-card-event-log"

	oldStatus, _, err := kanban.MoveCard(testFolder, testPath(moveFile), "blocked")
	if err != nil {
		return errCase(name, err)
	}

	events, err := kanban.GetEvents(testFolder, testPath(moveFile), nil, nil, 10)
	if err != nil {
		return errCase(name, err)
	}

	success := oldStatus == "inprogress" && len(events) >= 2 &&
		events[0].FromStatus == "inprogress" && events[0].ToStatus == "blocked"

	cr := test.CaseResult{
		Name:     name,
		Expected: "oldStatus=\"inprogress\", newest event inprogress->blocked",
		Actual:   fmt.Sprintf("oldStatus=%q events=%d", oldStatus, len(events)),
		Success:  success,
	}
	if !success {
		cr.Error = "kanbanStorage did not log the move event as expected"
	}
	return cr
}

// caseColumnOrderPersists saves a custom card order for inbox and checks the baseline
// (sortBy="") board build applies it on top of the createdAt ordering.
func caseColumnOrderPersists() test.CaseResult {
	name := "column-order-persists"
	defer kanban.SaveOrder(testFolder, kanban.Order{})

	if err := kanban.SaveOrder(testFolder, kanban.Order{
		"inbox": {testPath(alphaFile), testPath(betaFile)},
	}); err != nil {
		return errCase(name, err)
	}

	cols, err := kanban.BuildBoard(testFolder, emptyFilterConfig(), "", "")
	if err != nil {
		return errCase(name, err)
	}

	order := columnPaths(cols, "inbox")
	success := len(order) >= 2 && order[0] == testPath(alphaFile) && order[1] == testPath(betaFile)

	cr := test.CaseResult{
		Name:     name,
		Expected: "inbox column follows the stored order (alpha, beta) despite beta being newer",
		Actual:   fmt.Sprintf("order=%v", order),
		Success:  success,
	}
	if !success {
		cr.Error = "BuildBoard's baseline sort did not apply the persisted column order"
	}
	return cr
}

func caseApplyOrderPure() test.CaseResult {
	name := "apply-order-pure"

	got := kanban.ApplyOrder([]string{"b", "a"}, []string{"a", "b", "c"})
	success := len(got) == 3 && got[0] == "b" && got[1] == "a" && got[2] == "c"

	cr := test.CaseResult{
		Name:     name,
		Expected: `ApplyOrder(["b","a"], ["a","b","c"]) = ["b","a","c"]`,
		Actual:   fmt.Sprintf("%v", got),
		Success:  success,
	}
	if !success {
		cr.Error = "ApplyOrder did not reorder known entries and append unknown ones as expected"
	}
	return cr
}

func caseTagsAndFilesForFolder() test.CaseResult {
	name := "tags-and-files-for-folder"

	tags, err := kanban.TagsForFolder(testFolder)
	if err != nil {
		return errCase(name, err)
	}
	filePaths, err := kanban.FilesForFolder(testFolder)
	if err != nil {
		return errCase(name, err)
	}

	hasMarker, hasKanbanTag := false, false
	for _, t := range tags {
		if t == gammaExtraTag {
			hasMarker = true
		}
		if configmanager.IsKanbanTag(t) {
			hasKanbanTag = true
		}
	}

	success := hasMarker && !hasKanbanTag && containsPath(filePaths, testPath(alphaFile)) && containsPath(filePaths, testPath(gammaFile))
	cr := test.CaseResult{
		Name:     name,
		Expected: "TagsForFolder includes the marker tag but no kanban status tags; FilesForFolder includes alpha and gamma",
		Actual:   fmt.Sprintf("tags=%v filesIncludeAlpha=%v filesIncludeGamma=%v", tags, containsPath(filePaths, testPath(alphaFile)), containsPath(filePaths, testPath(gammaFile))),
		Success:  success,
	}
	if !success {
		cr.Error = "TagsForFolder/FilesForFolder did not return the expected data"
	}
	return cr
}

func caseExcerpt() test.CaseResult {
	name := "excerpt"

	got := kanban.Excerpt(pathutils.ToDocsPath(testPath(excerptFile)), 200)
	success := got == "This is the excerpt body text."

	cr := test.CaseResult{
		Name:     name,
		Expected: `"This is the excerpt body text." (front matter, heading, and code fence stripped, markdown emphasis removed)`,
		Actual:   fmt.Sprintf("%q", got),
		Success:  success,
	}
	if !success {
		cr.Error = "Excerpt did not strip front matter/markdown as expected"
	}
	return cr
}

func caseKanbanHelpers() test.CaseResult {
	name := "kanban-helpers"

	prefix := configmanager.GetKanbanPrefix()
	tags := []string{"unrelated", kanbanTag("inprogress")}

	status := kanban.StatusFromTags(tags, prefix)
	tag := kanban.TagFromList(tags)
	msg := kanban.TagNotifyMsg("", kanbanTag("inprogress"))

	success := status == "inprogress" && tag == kanbanTag("inprogress") && msg != ""
	cr := test.CaseResult{
		Name:     name,
		Expected: `StatusFromTags="inprogress", TagFromList=kb-status-inprogress, TagNotifyMsg non-empty for a fresh tag`,
		Actual:   fmt.Sprintf("status=%q tag=%q msg=%q", status, tag, msg),
		Success:  success,
	}
	if !success {
		cr.Error = "one or more pure kanban tag helpers did not behave as expected"
	}
	return cr
}
