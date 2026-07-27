// Package metadatatest - Metadata suite: exercises files.MetaDataGet/Save/Delete/ExportAll
// for every settable field, the references add/remove/list flow, and the pure kanban-tag
// sanitizer (see docs/temp_todo.md step 5). Inline-display/inline-edit rendering
// (render.RenderSidebarFieldDisplay/Edit) lives in internal/server/render - those functions
// only switch on the same four fields exercised below (tags, parents, editor, path) and read
// straight off *files.Metadata, so covering the data here plus connectionstest's parents/kids
// coverage exercises the same ground without needing to assert on rendered HTML.
package metadatatest

import (
	"knov/internal/job"
	"knov/internal/test"
)

// Suite runs the metadata test cases against real files and metadata storage.
type Suite struct{}

func init() {
	test.Register(Suite{})
	job.RegisterSuiteRunner("metadata-test", func() (*test.SuiteResult, error) { return (Suite{}).Run() })
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
		caseAllEditorTypes,
		caseSanitizeKanbanTags,
	}

	result := &test.SuiteResult{Suite: "metadata"}
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
