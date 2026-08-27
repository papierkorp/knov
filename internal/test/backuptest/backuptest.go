// Package backuptest - Backup suite: exercises internal/backup's Run/Restore/Rotate/event log
// directly against the real, live storages (seeded via test-only probe keys, never real data),
// plus job.checkAutoBackup/job.ListBackupLog - see docs/temp_todo.md's backup-solution "test
// suite" section.
//
// Every archive this suite creates lands in a throwaway scratch backup.NewLocalTarget (or a
// KNOV_BACKUPS_PATH temporarily redirected to one via sampledata.go's scratchTarget/
// withScratchBackupsPath) - never the real default target, so nothing here ever shows up on
// /system/backup or gets mixed into real rotation. backup.Restore is still called directly
// against the real, live storages though - every case that does this goes through
// restoreAndReinit (sampledata.go), which supplies backup.Restore's required afterRestore
// callback: the sqlite storages' own Restore closes their live db handle and, unlike
// job.RunRestore, this suite never restarts the process, so restoreAndReinit's follow-up Init
// calls (mirroring main.go's own startup sequence) are what reopen it in-process instead.
package backuptest

import (
	"knov/internal/job"
	"knov/internal/test"
)

// Suite runs the backup/restore/rotate/event-log test cases against the real backup package
// and live storages.
type Suite struct{}

func init() {
	test.Register(Suite{})
	job.RegisterSuiteRunner("backup-test", func() (*test.SuiteResult, error) { return (Suite{}).Run() })
}

func (Suite) Name() string { return "backup" }

func (Suite) Run() (*test.SuiteResult, error) {
	cases := []func() test.CaseResult{
		caseRunProducesFullSet,
		caseRunAbortsOnPartialFailure,
		caseSelectiveBackup,
		caseRestoreRoundtrip,
		caseReadSQLiteBackupRejectsCorrupt,
		caseRotate,
		caseCheckAutoBackup,
		caseCheckAutoBackupCronCatchesUp,
		caseListBackupLog,
		caseEventRoundtripAcrossRestart,
		caseWriteFileAtomicNoTornReads,
		caseCrossBackendRestoreMetadata,
		caseCrossBackendRestoreKanban,
		caseNonMigratableBackendMismatchFails,
		caseOldFormatBackupNoBackendsRestoresFine,
		caseAfterRestoreOnlyFiresWhenTouched,
		caseKanbanNoopRestoreIsNoop,
		caseYAMLTaggedBackupRestoresViaMigrate,
		caseRestoreRefreshesCache,
	}

	result := &test.SuiteResult{Suite: "backup"}
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
