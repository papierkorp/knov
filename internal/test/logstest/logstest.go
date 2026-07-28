// Package logstest - Logs suite: exercises the in-memory ring buffer (logging.GetRecentEntries),
// the per-key log file's pagination/chunking arithmetic (handleAPIGetLogsFile's offset/limit
// slicing, internal/server/api_system.go - inline handler logic with no exported wrapper, so
// this suite replicates it directly, same pattern as editorstest/mediatest replicate other
// unexported handler logic), and the download path-safety guard (resolveLogFilePath).
//
// Cases write real probe lines to logging.KeyInAppTests' own log file (logs/in-app-tests.log)
// - the same key the job scheduler already logs every suite run's pass/fail summary to, so
// this is real, shared, continuously-written app state, not a sandboxed docs/test/ file.
// Assertions check that probe lines appear in the expected region rather than exact byte/line-
// count equality, tolerating any real interleaved log activity from the running app.
package logstest

import (
	"knov/internal/job"
	"knov/internal/test"
)

// Suite runs the logs test cases against the real logging subsystem.
type Suite struct{}

func init() {
	test.Register(Suite{})
	job.RegisterSuiteRunner("logs-test", func() (*test.SuiteResult, error) { return (Suite{}).Run() })
}

func (Suite) Name() string { return "logs" }

func (Suite) Run() (*test.SuiteResult, error) {
	cases := []func() test.CaseResult{
		caseInMemoryRingBuffer,
		caseFilePaginationChunking,
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
