// Package job - test-data setup/cleanup jobs, triggered from the admin UI.
package job

import (
	"fmt"

	"knov/internal/test"
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
