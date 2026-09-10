// Package jobstest - Jobs suite: exercises the admin-triggered background jobs (metadata
// rebuild, search reindex, cache invalidate, media cleanup, the manual "run all jobs now"
// trigger, and the job history/status tracking behind them - see docs/temp_todo.md step 6).
// Cases assert on the resulting filesystem/DB state (recomputed metadata, search hits,
// cache freshness, deleted orphaned media), not just that the job returned no error, per
// this step's explicit instruction in the todo doc.
package jobstest

import (
	"knov/internal/test"
)

// Suite runs the jobs test cases against the real job scheduler and storage backends.
type Suite struct{}

func init() {
	test.Register(Suite{})
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

	return test.RunCases("jobs", cases), nil
}
