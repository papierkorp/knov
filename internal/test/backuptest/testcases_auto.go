package backuptest

import (
	"fmt"
	"os"
	"strings"
	"time"

	"knov/internal/backup"
	"knov/internal/configmanager"
	"knov/internal/job"
	"knov/internal/test"
)

// caseCheckAutoBackup covers job.checkAutoBackup (via job.CheckAutoBackupNow): disabled is a
// no-op; enabled with no existing sets creates one; enabled with a fresh existing set is a
// no-op; enabled with only a synthetic old-dated set creates a new one. Redirects
// KNOV_BACKUPS_PATH to a scratch directory for the duration (configmanager.SetBackupsPath is
// in-memory only), since checkAutoBackup always targets the default backup target.
func caseCheckAutoBackup() test.CaseResult {
	name := "check-auto-backup"

	origPath := configmanager.GetBackupsPath()
	origEnabled := configmanager.GetBackupAutoEnabled()
	origCron := configmanager.GetBackupAutoCron()
	defer func() {
		configmanager.SetBackupsPath(origPath)
		configmanager.SetBackupAutoEnabled(origEnabled, origCron)
	}()

	scratchDir, err := os.MkdirTemp("", "knov-backuptest-auto-*")
	if err != nil {
		return errCase(name, err)
	}
	defer os.RemoveAll(scratchDir)
	configmanager.SetBackupsPath(scratchDir)

	target, err := backup.NewLocalTarget(scratchDir)
	if err != nil {
		return errCase(name, err)
	}

	const dailyAtMidnight = "0 0 * * *"

	configmanager.SetBackupAutoEnabled(false, dailyAtMidnight)
	job.CheckAutoBackupNow()
	afterDisabled, err := target.List()
	if err != nil {
		return errCase(name, err)
	}
	disabledNoop := len(afterDisabled) == 0

	configmanager.SetBackupAutoEnabled(true, dailyAtMidnight)
	job.CheckAutoBackupNow()
	afterFirst, err := target.List()
	if err != nil {
		return errCase(name, err)
	}
	createdWhenMissing := len(afterFirst) == 1

	job.CheckAutoBackupNow()
	afterSecondCall, err := target.List()
	if err != nil {
		return errCase(name, err)
	}
	noopWhenFresh := len(afterSecondCall) == len(afterFirst)

	for _, n := range afterSecondCall {
		if err := target.Delete(n); err != nil {
			return errCase(name, err)
		}
	}
	oldName := setNameAt(time.Now().Add(-48 * time.Hour))
	if err := target.Write(oldName, strings.NewReader("")); err != nil {
		return errCase(name, err)
	}
	job.CheckAutoBackupNow()
	afterOld, err := target.List()
	if err != nil {
		return errCase(name, err)
	}
	createdWhenDue := len(afterOld) == 2

	success := disabledNoop && createdWhenMissing && noopWhenFresh && createdWhenDue
	cr := test.CaseResult{
		Name:     name,
		Expected: "disabled=noop, enabled+missing=creates, enabled+fresh=noop, enabled+old-dated=creates",
		Actual: fmt.Sprintf("disabledNoop=%v createdWhenMissing=%v noopWhenFresh=%v createdWhenDue=%v",
			disabledNoop, createdWhenMissing, noopWhenFresh, createdWhenDue),
		Success: success,
	}
	if !success {
		cr.Error = "checkAutoBackup did not gate on enabled/due state as expected"
	}
	return cr
}

// caseCheckAutoBackupCronCatchesUp covers job.checkAutoBackup's cron-driven catch-up behavior
// (backup.AutoBackupDue's schedule.Next(lastDefaultBackup) check), at minute precision rather than
// caseCheckAutoBackup's whole-day one: given a default backup from 10 minutes ago, a cron target 5
// minutes in the future is not yet due; the same backup against a cron target 5 minutes in the
// past is due (catching up on a slot that was missed, e.g. because the app wasn't running); once
// caught up, the same past target is a no-op again. Wall-clock-relative fixtures, so this can
// flake within a few minutes of local midnight - same known limitation as caseCheckAutoBackup's.
func caseCheckAutoBackupCronCatchesUp() test.CaseResult {
	name := "check-auto-backup-cron-catches-up"

	origPath := configmanager.GetBackupsPath()
	origEnabled := configmanager.GetBackupAutoEnabled()
	origCron := configmanager.GetBackupAutoCron()
	defer func() {
		configmanager.SetBackupsPath(origPath)
		configmanager.SetBackupAutoEnabled(origEnabled, origCron)
	}()

	scratchDir, err := os.MkdirTemp("", "knov-backuptest-auto-cron-*")
	if err != nil {
		return errCase(name, err)
	}
	defer os.RemoveAll(scratchDir)
	configmanager.SetBackupsPath(scratchDir)

	target, err := backup.NewLocalTarget(scratchDir)
	if err != nil {
		return errCase(name, err)
	}

	seedName := setNameAt(time.Now().Add(-10 * time.Minute))
	if err := target.Write(seedName, strings.NewReader("")); err != nil {
		return errCase(name, err)
	}

	cronAt := func(t time.Time) string {
		return fmt.Sprintf("%d %d * * *", t.Minute(), t.Hour())
	}

	configmanager.SetBackupAutoEnabled(true, cronAt(time.Now().Add(5*time.Minute)))
	job.CheckAutoBackupNow()
	afterFuture, err := target.List()
	if err != nil {
		return errCase(name, err)
	}
	noopBeforeTarget := len(afterFuture) == 1

	configmanager.SetBackupAutoEnabled(true, cronAt(time.Now().Add(-5*time.Minute)))
	job.CheckAutoBackupNow()
	afterPast, err := target.List()
	if err != nil {
		return errCase(name, err)
	}
	createdAfterTarget := len(afterPast) == 2

	job.CheckAutoBackupNow()
	afterSecondCall, err := target.List()
	if err != nil {
		return errCase(name, err)
	}
	noopOnceCaughtUp := len(afterSecondCall) == len(afterPast)

	success := noopBeforeTarget && createdAfterTarget && noopOnceCaughtUp
	cr := test.CaseResult{
		Name:     name,
		Expected: "future cron target=noop, past cron target+missed slot=creates, same past target again=noop",
		Actual: fmt.Sprintf("noopBeforeTarget=%v createdAfterTarget=%v noopOnceCaughtUp=%v",
			noopBeforeTarget, createdAfterTarget, noopOnceCaughtUp),
		Success: success,
	}
	if !success {
		cr.Error = "checkAutoBackup did not catch up on a missed cron slot as expected"
	}
	return cr
}

// caseListBackupLog covers job.ListBackupLog: an event referencing a set that was never
// actually written comes back with Available=false and zeroed Default/Locked; a set on the target
// with no matching event still gets a synthetic row from its own name; entries sort newest
// first (a backup event and its own restore event, logged moments apart). Redirects
// KNOV_BACKUPS_PATH to a scratch directory, since ListBackupLog always reads the default target.
func caseListBackupLog() test.CaseResult {
	name := "list-backup-log"

	origPath := configmanager.GetBackupsPath()
	defer configmanager.SetBackupsPath(origPath)

	scratchDir, err := os.MkdirTemp("", "knov-backuptest-log-*")
	if err != nil {
		return errCase(name, err)
	}
	defer os.RemoveAll(scratchDir)
	configmanager.SetBackupsPath(scratchDir)

	target, err := backup.NewLocalTarget(scratchDir)
	if err != nil {
		return errCase(name, err)
	}

	// a) event referencing a set that was never written -> Available=false, Default/Locked zeroed
	if err := target.LogEvent(backup.EventBackup, "ghost-set", backup.SourceManual); err != nil {
		return errCase(name, err)
	}

	// b) a set on disk with no matching event -> gets a synthetic row from its own name
	orphanName := setNameAt(time.Now().Add(-1 * time.Hour))
	if err := target.Write(orphanName, strings.NewReader("")); err != nil {
		return errCase(name, err)
	}

	// c) a backup event immediately followed by its own restore event -> sorted newest first
	realName := setNameAt(time.Now())
	if err := target.Write(realName, strings.NewReader("")); err != nil {
		return errCase(name, err)
	}
	if err := target.LogEvent(backup.EventBackup, realName, backup.SourceManual); err != nil {
		return errCase(name, err)
	}
	if err := target.LogEvent(backup.EventRestore, realName, backup.SourceManual); err != nil {
		return errCase(name, err)
	}

	entries, err := job.ListBackupLog()
	if err != nil {
		return errCase(name, err)
	}

	var ghost, orphan *backup.LogEntry
	restoreIdx, backupIdx := -1, -1
	for i := range entries {
		e := entries[i]
		switch {
		case e.Set == "ghost-set":
			ghost = &entries[i]
		case e.Set == orphanName:
			orphan = &entries[i]
		case e.Set == realName && e.Kind == backup.EventRestore:
			restoreIdx = i
		case e.Set == realName && e.Kind == backup.EventBackup:
			backupIdx = i
		}
	}

	ghostOK := ghost != nil && !ghost.Available && !ghost.Default && !ghost.Locked
	orphanOK := orphan != nil && orphan.Available && orphan.Kind == backup.EventBackup
	orderOK := restoreIdx != -1 && backupIdx != -1 && restoreIdx < backupIdx

	success := ghostOK && orphanOK && orderOK
	cr := test.CaseResult{
		Name:     name,
		Expected: "a deleted-set event is Available=false with Default/Locked zeroed, an event-less set gets a synthetic row, and the later restore event sorts before its own backup event",
		Actual:   fmt.Sprintf("ghostOK=%v orphanOK=%v orderOK=%v entries=%d", ghostOK, orphanOK, orderOK, len(entries)),
		Success:  success,
	}
	if !success {
		cr.Error = "ListBackupLog did not merge the event log with target state / sort as expected"
	}
	return cr
}
