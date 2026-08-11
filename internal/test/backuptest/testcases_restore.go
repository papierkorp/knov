package backuptest

import (
	"fmt"
	"os"
	"path/filepath"

	"knov/internal/backup"
	"knov/internal/test"
)

// caseRestoreRoundtrip covers backup.Restore (the package function, not job.RunRestore)
// roundtripping: backup -> mutate/delete the seeded probes -> restore -> every probe is back to
// its original value. Goes through restoreAndReinit since this calls backup.Restore directly
// against the real, live sqlite storages - see backuptest.go's package doc. Captures a kanban/chat
// baseline before seeding its own probes - see probeCounts - since earlier cases in this suite's
// fixed run order (caseRunProducesFullSet, caseSelectiveBackup) leave permanent state on those
// shared, live storages that a hardcoded absolute count would otherwise wrongly fail against.
func caseRestoreRoundtrip() test.CaseResult {
	name := "restore-roundtrip"

	baseline, err := countProbes()
	if err != nil {
		return errCase(name, err)
	}

	p, err := seedProbes()
	if err != nil {
		return errCase(name, err)
	}

	target, cleanup, err := scratchTarget()
	if err != nil {
		return errCase(name, err)
	}
	defer cleanup()

	setName, err := backup.Run(target, backup.SourceManual)
	if err != nil {
		return errCase(name, err)
	}

	if err := mutateProbes(p); err != nil {
		return errCase(name, err)
	}

	if err := restoreAndReinit(target, setName); err != nil {
		return errCase(name, err)
	}

	ok, detail, err := verifyProbes(p, baseline)
	if err != nil {
		return errCase(name, err)
	}

	cr := test.CaseResult{
		Name:     name,
		Expected: "every probe's original value is back after backup -> mutate -> restore",
		Actual:   detail,
		Success:  ok,
	}
	if !ok {
		cr.Error = "backup.Restore did not roll back every mutated probe to its backed-up state as expected"
	}
	return cr
}

// caseReadSQLiteBackupRejectsCorrupt is a regression test for the close-before-validate bug
// caught during review: ReadSQLiteBackup must reject a corrupt/truncated/missing db file before
// any live db handle is touched, so a bad backup fails cleanly instead of bricking a storage.
func caseReadSQLiteBackupRejectsCorrupt() test.CaseResult {
	name := "read-sqlite-backup-rejects-corrupt"

	dir, err := os.MkdirTemp("", "knov-backuptest-corrupt-*")
	if err != nil {
		return errCase(name, err)
	}
	defer os.RemoveAll(dir)

	const filename = "corrupt.db"
	if err := os.WriteFile(filepath.Join(dir, filename), []byte("not a sqlite database"), 0644); err != nil {
		return errCase(name, err)
	}

	_, corruptErr := backup.ReadSQLiteBackup(dir, filename)
	rejectedCorrupt := corruptErr != nil

	_, missingErr := backup.ReadSQLiteBackup(dir, "missing.db")
	rejectedMissing := missingErr != nil

	success := rejectedCorrupt && rejectedMissing
	cr := test.CaseResult{
		Name:     name,
		Expected: "ReadSQLiteBackup rejects both a corrupt file and a missing file",
		Actual:   fmt.Sprintf("corruptErr=%v missingErr=%v", corruptErr, missingErr),
		Success:  success,
	}
	if !success {
		cr.Error = "ReadSQLiteBackup did not reject invalid input as expected"
	}
	return cr
}
