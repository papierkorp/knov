// Package exporttest - Export/import suite: exercises the settings export/import round-trip
// (see docs/temp_todo.md step 6). Dashboard export/import is already covered by dashboardtest's caseExportImportDashboard (step 4) and
// files.MetaDataExportAll by metadatatest's export-all case (step 5), the zip exports by the
// unit tests of internal/export and internal/files - not duplicated here.
package exporttest

import (
	"knov/internal/test"
)

// Suite runs the export/import test cases against the real settings.
type Suite struct{}

func init() {
	test.Register(Suite{})
}

func (Suite) Name() string { return "export" }

func (Suite) Run() (*test.SuiteResult, error) {
	cases := []func() test.CaseResult{
		caseSettingsExportImportRoundtrip,
		caseSettingsImportReportsSkipped,
	}

	return test.RunCases("export", cases), nil
}
