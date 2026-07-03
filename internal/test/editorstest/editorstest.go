// Package editorstest - editors suite: seeds real files/metadata and exercises the same
// internal functions the editor HTTP handlers call, without going through HTTP.
package editorstest

import "knov/internal/test"

// Suite runs the editors test cases against real files, metadata and content handlers.
type Suite struct{}

func init() {
	test.Register(Suite{})
}

func (Suite) Name() string { return "editors" }

func (Suite) Run() (*test.SuiteResult, error) {
	if err := resetTestDir(); err != nil {
		return nil, err
	}

	cases := []func() test.CaseResult{
		caseToastUICreateEditSave,
		caseTextareaCreateEditSave,
		caseCodeMirrorCreateEditSave,
		caseFilterCreateEditSave,
		caseListCreateEditSave,
		caseTodoCreateEditSave,
		caseIndexCreateEditSave,
		caseTableCreateEditSave,
		caseSectionSave,
		caseTodoToggle,
		caseConvertToMarkdown,
		caseFileRename,
		caseFileMove,
		caseBulkDeleteFiles,
		caseBulkMetadataPatch,
		caseBulkChatMoveDelete,
	}

	result := &test.SuiteResult{Suite: "editors"}
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
