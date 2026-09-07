// Package logstest - Logs suite: exercises the in-memory ring buffer (logging.GetRecentEntries),
// render.ParseLogLines' multi-line-record folding, and the download path-safety guard
// (resolveLogFilePath, replicated directly here - inline handler logic with no exported wrapper,
// same pattern as editorstest/mediatest replicate other unexported handler logic).
//
// Cases write real probe lines to logging.KeyInAppTests' own log file (logs/in-app-tests.log)
// - the same key the job scheduler already logs every suite run's pass/fail summary to, so
// this is real, shared, continuously-written app state, not a sandboxed docs/test/ file.
// Assertions check that probe lines appear in the expected region rather than exact byte/line-
// count equality, tolerating any real interleaved log activity from the running app.
package logstest

import (
	"knov/internal/test"
)

// Suite runs the logs test cases against the real logging subsystem.
type Suite struct{}

func init() {
	test.Register(Suite{})
}

func (Suite) Name() string { return "logs" }

func (Suite) Run() (*test.SuiteResult, error) {
	cases := []func() test.CaseResult{
		caseInMemoryRingBuffer,
		caseParseLogContinuation,
		caseDownloadPathGuard,
	}

	result := &test.SuiteResult{Suite: "logs"}
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
