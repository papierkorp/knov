// Package job - test-data setup/cleanup and in-app test suite jobs, triggered from the
// admin UI or API. Every new test suite (internal/test/<group>test) gets its own Job here.
package job

import (
	"fmt"

	"knov/internal/test"
	"knov/internal/test/chattest"
	"knov/internal/test/dashboardtest"
	"knov/internal/test/editorstest"
	"knov/internal/test/filtertest"
	"knov/internal/test/githistorytest"
	"knov/internal/test/kanbantest"
	"knov/internal/test/searchtest"
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

type searchTestJob struct {
	results *test.SuiteResult
}

func (j *searchTestJob) Name() string { return "search-test" }

func (j *searchTestJob) Run() error {
	results, err := (searchtest.Suite{}).Run()
	j.results = results
	if err != nil {
		return fmt.Errorf("search tests failed: %w", err)
	}
	return nil
}

func (j *searchTestJob) Output() any { return j.results }

func (j *searchTestJob) Message() string {
	if j.results == nil {
		return ""
	}
	return fmt.Sprintf("%d passed, %d failed", j.results.Passed, j.results.Failed)
}

type gitHistoryTestJob struct {
	results *test.SuiteResult
}

func (j *gitHistoryTestJob) Name() string { return "git-history-test" }

func (j *gitHistoryTestJob) Run() error {
	results, err := (githistorytest.Suite{}).Run()
	j.results = results
	if err != nil {
		return fmt.Errorf("git history tests failed: %w", err)
	}
	return nil
}

func (j *gitHistoryTestJob) Output() any { return j.results }

func (j *gitHistoryTestJob) Message() string {
	if j.results == nil {
		return ""
	}
	return fmt.Sprintf("%d passed, %d failed", j.results.Passed, j.results.Failed)
}

type chatTestJob struct {
	results *test.SuiteResult
}

func (j *chatTestJob) Name() string { return "chat-test" }

func (j *chatTestJob) Run() error {
	results, err := (chattest.Suite{}).Run()
	j.results = results
	if err != nil {
		return fmt.Errorf("chat tests failed: %w", err)
	}
	return nil
}

func (j *chatTestJob) Output() any { return j.results }

func (j *chatTestJob) Message() string {
	if j.results == nil {
		return ""
	}
	return fmt.Sprintf("%d passed, %d failed", j.results.Passed, j.results.Failed)
}

type dashboardTestJob struct {
	results *test.SuiteResult
}

func (j *dashboardTestJob) Name() string { return "dashboard-test" }

func (j *dashboardTestJob) Run() error {
	results, err := (dashboardtest.Suite{}).Run()
	j.results = results
	if err != nil {
		return fmt.Errorf("dashboard tests failed: %w", err)
	}
	return nil
}

func (j *dashboardTestJob) Output() any { return j.results }

func (j *dashboardTestJob) Message() string {
	if j.results == nil {
		return ""
	}
	return fmt.Sprintf("%d passed, %d failed", j.results.Passed, j.results.Failed)
}

type kanbanTestJob struct {
	results *test.SuiteResult
}

func (j *kanbanTestJob) Name() string { return "kanban-test" }

func (j *kanbanTestJob) Run() error {
	results, err := (kanbantest.Suite{}).Run()
	j.results = results
	if err != nil {
		return fmt.Errorf("kanban tests failed: %w", err)
	}
	return nil
}

func (j *kanbanTestJob) Output() any { return j.results }

func (j *kanbanTestJob) Message() string {
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
