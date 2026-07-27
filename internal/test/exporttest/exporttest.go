// Package exporttest - Export/import suite: exercises the two "export everything as zip"
// handlers (raw and dokuwiki->markdown-converted, internal/server/api_files.go) and the
// settings export/import round-trip (see docs/temp_todo.md step 6). Dashboard export/import
// is already covered by dashboardtest's caseExportImportDashboard (step 4) and
// files.MetaDataExportAll by metadatatest's export-all case (step 5) - not duplicated here.
// Both zip-export handlers are pure inline logic (filepath.Walk + archive/zip, no exported
// wrapper function), so cases replicate that walk directly, same as editorstest/chattest
// replicate other unexported handler logic elsewhere.
package exporttest

import (
	"knov/internal/job"
	"knov/internal/test"
)

// Suite runs the export/import test cases against real files and settings.
type Suite struct{}

func init() {
	test.Register(Suite{})
	job.RegisterSuiteRunner("export-test", func() (*test.SuiteResult, error) { return (Suite{}).Run() })
}

func (Suite) Name() string { return "export" }

func (Suite) Run() (*test.SuiteResult, error) {
	if err := resetAndSeed(); err != nil {
		return nil, err
	}

	cases := []func() test.CaseResult{
		caseExportZipRaw,
		caseExportZipMarkdownConverted,
		caseSettingsExportImportRoundtrip,
	}

	result := &test.SuiteResult{Suite: "export"}
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
