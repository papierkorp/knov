// Package jobstest - Jobs suite: exercises the admin-triggered background jobs (metadata
// rebuild, search reindex, cache invalidate, media cleanup, the manual "run all jobs now"
// trigger, and the job history/status tracking behind them - see docs/temp_todo.md step 6).
// Cases assert on the resulting filesystem/DB state (recomputed metadata, search hits,
// cache freshness, deleted orphaned media), not just that the job returned no error, per
// this step's explicit instruction in the todo doc.
package jobstest

import (
	"knov/internal/job"
	"knov/internal/test"
)

// Suite runs the jobs test cases against the real job scheduler and storage backends.
type Suite struct{}

func init() {
	test.Register(Suite{})
	job.RegisterSuiteRunner("jobs-test", func() (*test.SuiteResult, error) { return (Suite{}).Run() })
}

func (Suite) Name() string { return "jobs" }

func (Suite) Run() (*test.SuiteResult, error) {
	if err := resetAndSeed(); err != nil {
		return nil, err
	}

	cases := []func() test.CaseResult{
		caseMetadataFullRebuild,
		caseSearchReindex,
		caseCacheInvalidate,
		caseMediaCleanup,
		caseManualTrigger,
		caseJobHistory,
	}

	result := &test.SuiteResult{Suite: "jobs"}
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
