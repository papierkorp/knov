// Package kanbantest - Kanban suite: exercises internal/kanban's exported board-build,
// card-move, order-persistence and helper functions directly. The one genuinely
// browser-only piece - native HTML5 drag-and-drop - can't be verified by an in-app runtime
// suite (see docs/temp_todo.md's testing note), so this covers the API/state it drives
// instead: BuildBoard, MoveCard, SaveOrder/GetOrder/ApplyOrder.
package kanbantest

import "knov/internal/test"

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
		caseBoardSearchQuery,
		caseBoardSorting,
		caseMoveCard,
		caseMoveCardEventLog,
		caseColumnOrderPersists,
		caseApplyOrderPure,
		caseTagsAndFilesForCollection,
		caseExcerpt,
		caseKanbanHelpers,
	}

	result := &test.SuiteResult{Suite: "kanban"}
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
