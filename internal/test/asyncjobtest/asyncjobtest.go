// Package asyncjobtest - Async job suite: exercises the async job runner's non-obvious
// invariants (docs/temp_todo.md step 10) - StartAsync/runAsync's dedup mutex + panic recovery,
// RecoverInterrupted's resumable/non-resumable startup recovery paths, and
// files.RemoveEmptyDirTree's refusal to remove a non-empty directory tree.
package asyncjobtest

import (
	"knov/internal/test"
)

// Suite runs the async job test cases against the real job runner, jobStorage and
// notificationStorage backends.
type Suite struct{}

func init() {
	test.Register(Suite{})
}

func (Suite) Name() string { return "async-job" }

func (Suite) Run() (*test.SuiteResult, error) {
	if err := resetAndSeed(); err != nil {
		return nil, err
	}

	cases := []func() test.CaseResult{
		caseDedupMutexHeldUntilPersisted,
		casePanicRecoveryBridgesToJobStorage,
		caseCancelAsync,
		caseCancelAsyncNotRunning,
		caseRecoverInterruptedResumable,
		caseRecoverInterruptedNonResumable,
		caseRemoveEmptyDirTreeSuccess,
		caseRemoveEmptyDirTreeRefusesNonEmpty,
	}

	result := &test.SuiteResult{Suite: "async-job"}
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
