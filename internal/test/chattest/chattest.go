// Package chattest - Chat suite: exercises internal/chat's exported single-message API
// directly (Add/Delete/GetByID/GetPage/MoveFilePath/DeleteForFile), and replicates the
// bulk-move/bulk-delete/formatForEditor logic from internal/server/api_chat.go's handlers
// since that logic is unexported there (mirroring editorstest's bulk-metadata-patch case).
package chattest

import "knov/internal/test"

// Suite runs the chat test cases against the real chat storage backend.
type Suite struct{}

func init() {
	test.Register(Suite{})
}

func (Suite) Name() string { return "chat" }

func (Suite) Run() (*test.SuiteResult, error) {
	if err := resetAndSeed(); err != nil {
		return nil, err
	}

	cases := []func() test.CaseResult{
		caseAddGlobalMessage,
		caseAddFileScopedMessage,
		caseDeleteMessage,
		caseGetPagePagination,
		caseSingleMoveAppend,
		caseSingleMoveNewFile,
		caseBulkMoveNewFile,
		caseBulkDelete,
		caseMoveFilePath,
		caseDeleteForFile,
	}

	result := &test.SuiteResult{Suite: "chat"}
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
