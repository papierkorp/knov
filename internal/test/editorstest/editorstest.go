// Package editorstest - editors suite: seeds real files/metadata and exercises the same
// internal functions the editor HTTP handlers call, without going through HTTP.
package editorstest

import (
	"knov/internal/test"
)

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
		caseCodeMirrorCreateEditSave,
		caseFilterCreateEditSave,
		caseListCreateEditSave,
		caseTodoCreateEditSave,
		caseIndexCreateEditSave,
		caseBookCreateEditSave,
		caseBookUnknownEntryRoundTrip,
		caseBookTitleLevelRoundTrip,
		caseBookPathContainment,
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

	return test.RunCases("editors", cases), nil
}
