// Package dashboardtest - Dashboard suite: exercises internal/dashboard's exported CRUD
// directly, replicates the export/import JSON round-trip from internal/server/api_dashboards.go
// (trivial one-liners there, no need to import server), and covers each widget type's
// underlying data resolution (the render dispatch itself lives in internal/server/render,
// which isn't imported here since none of these cases need HTML output).
package dashboardtest

import (
	"knov/internal/job"
	"knov/internal/test"
)

// Suite runs the dashboard test cases against the real config-backed dashboard storage.
type Suite struct{}

func init() {
	test.Register(Suite{})
	job.RegisterSuiteRunner("dashboard-test", func() (*test.SuiteResult, error) { return (Suite{}).Run() })
}

func (Suite) Name() string { return "dashboard" }

func (Suite) Run() (*test.SuiteResult, error) {
	if err := resetAndSeed(); err != nil {
		return nil, err
	}

	cases := []func() test.CaseResult{
		caseCreateDashboard,
		caseGetAllDashboards,
		caseUpdateDashboard,
		caseRenameDashboard,
		caseDeleteDashboard,
		caseExportImportDashboard,
		caseWidgetFilterData,
		caseWidgetFileContentData,
		caseWidgetAggregateData,
	}

	result := &test.SuiteResult{Suite: "dashboard"}
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
