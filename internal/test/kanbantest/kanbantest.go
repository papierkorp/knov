// Package kanbantest - Kanban suite: exercises internal/kanban's exported board-build,
// card-move, order-persistence and helper functions directly (BuildBoard, MoveCard,
// SaveOrder/GetOrder/ApplyOrder), plus one testkit-driven case covering the native HTML5
// drag-and-drop wiring itself in a real headless browser (see testcases_browser.go).
package kanbantest

import (
	"knov/internal/test"
)

// Suite runs the kanban test cases against real files, metadata and kanban storage.
type Suite struct{}

func init() {
	test.Register(Suite{})
}

func (Suite) Name() string { return "kanban" }

func (Suite) Run() (*test.SuiteResult, error) {
	if err := resetAndSeed(); err != nil {
		return nil, err
	}

	cases := []func() test.CaseResult{
		caseBoardLoadColumns,
		caseHideScopeKanban,
		caseBoardSearchQuery,
		caseBoardSorting,
		caseMoveCard,
		caseMoveCardEventLog,
		caseColumnOrderPersists,
		caseApplyOrderPure,
		caseTagsAndFilesForFolder,
		caseExcerpt,
		caseKanbanHelpers,
		caseDragCardBetweenColumns,
	}

	return test.RunCases("kanban", cases), nil
}
