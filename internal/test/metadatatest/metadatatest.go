// Package metadatatest - Metadata suite: exercises files.MetaDataGet/Mutate/Sync/Delete/ExportAll
// for every settable field, the references add/remove/list flow, the pure kanban-tag sanitizer,
// and the MoveCard∥Sync race (see docs/temp_todo.md metadata refactor). Inline-display/inline-edit
// rendering (render.RenderSidebarFieldDisplay/Edit) lives in internal/server/render - those
// functions only switch on the same four fields exercised below (tags, parents, editor, path)
// and read straight off *files.Metadata, so covering the data here plus connectionstest's
// parents/kids coverage exercises the same ground without needing to assert on rendered HTML.
package metadatatest

import (
	"knov/internal/test"
)

// Suite runs the metadata test cases against real files and metadata storage.
type Suite struct{}

func init() {
	test.Register(Suite{})
}

func (Suite) Name() string { return "metadata" }

func (Suite) Run() (*test.SuiteResult, error) {
	if err := resetAndSeed(); err != nil {
		return nil, err
	}

	cases := []func() test.CaseResult{
		caseMetadataGetSetFields,
		caseMetadataPartialUpdate,
		caseMetadataDelete,
		caseMetadataExportAll,
		caseReferencesAdd,
		caseReferencesRemove,
		caseMutateVsSyncRace,
		caseMutateMissingPath,
		caseAllEditorTypes,
		caseSanitizeKanbanTags,
		caseAggregatesRespectHiddenPaths,
		caseGetMetadataSpecialCharFilepath,
	}

	return test.RunCases("metadata", cases), nil
}
