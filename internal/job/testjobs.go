// Package job - test-data setup/cleanup and in-app test suite jobs, triggered from the
// admin UI or API. Every new test suite (internal/test/<group>test) gets its own Job here.
package job

import (
	"fmt"

	"knov/internal/test"
	"knov/internal/test/editorstest"
	"knov/internal/test/filtertest"
)

// ----------------------------------------------------------------------------------------
// ----------------------------------- testdata jobs --------------------------------------
// ----------------------------------------------------------------------------------------

type testdataSetupJob struct{}

func (j *testdataSetupJob) Name() string { return "testdata-setup" }

func (j *testdataSetupJob) Run() error {
	if err := test.SetupTestData(); err != nil {
		return fmt.Errorf("failed to setup test data: %w", err)
	}
	return nil
}

type testdataCleanJob struct{}

func (j *testdataCleanJob) Name() string { return "testdata-clean" }

func (j *testdataCleanJob) Run() error {
	if err := test.CleanTestData(); err != nil {
		return fmt.Errorf("failed to clean test data: %w", err)
	}
	return nil
}

// ----------------------------------------------------------------------------------------
// -------------------------------------- suite jobs ---------------------------------------
// ----------------------------------------------------------------------------------------

type filterTestJob struct {
	results *test.SuiteResult
}

func (j *filterTestJob) Name() string { return "filter-test" }

func (j *filterTestJob) Run() error {
	results, err := (filtertest.Suite{}).Run()
	j.results = results
	if err != nil {
		return fmt.Errorf("filter tests failed: %w", err)
	}
	return nil
}

func (j *filterTestJob) Output() any { return j.results }

func (j *filterTestJob) Message() string {
	if j.results == nil {
		return ""
	}
	return fmt.Sprintf("%d passed, %d failed", j.results.Passed, j.results.Failed)
}

type editorsTestJob struct {
	results *test.SuiteResult
}

func (j *editorsTestJob) Name() string { return "editors-test" }

func (j *editorsTestJob) Run() error {
	results, err := (editorstest.Suite{}).Run()
	j.results = results
	if err != nil {
		return fmt.Errorf("editors tests failed: %w", err)
	}
	return nil
}

func (j *editorsTestJob) Output() any { return j.results }

func (j *editorsTestJob) Message() string {
	if j.results == nil {
		return ""
	}
	return fmt.Sprintf("%d passed, %d failed", j.results.Passed, j.results.Failed)
}

type runAllTestsJob struct {
	results *test.SuiteResult
}

func (j *runAllTestsJob) Name() string { return "run-all-tests" }

func (j *runAllTestsJob) Run() error {
	results, err := test.RunAllTests()
	j.results = results
	if err != nil {
		return fmt.Errorf("test suites failed: %w", err)
	}
	return nil
}

func (j *runAllTestsJob) Output() any { return j.results }

func (j *runAllTestsJob) Message() string {
	if j.results == nil {
		return ""
	}
	return fmt.Sprintf("%d passed, %d failed", j.results.Passed, j.results.Failed)
}
