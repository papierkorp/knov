// Package job - test-data setup/cleanup jobs, triggered from the admin UI or API. Every
// in-app test suite (internal/test/<group>test) registers its own admin-button job directly
// via job.RegisterSuiteRunner in its own init() (see externalsuite.go) instead of getting a
// dedicated wrapper type here - this file no longer imports any suite package.
package job

import (
	"fmt"

	"knov/internal/test"
)

func init() {
	RegisterSuiteRunner("run-all-tests", test.RunAllTests)
}

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
