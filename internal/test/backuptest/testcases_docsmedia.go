package backuptest

import (
	"fmt"
	"os"
	"path/filepath"

	"knov/internal/backup"
	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/pathutils"
	"knov/internal/test"
)

const (
	probeDocsFile    = "backuptest-probe.md"
	probeMediaFile   = "backuptest-probe.txt"
	probeDocsExtra   = "backuptest-extra.md"
	probeMediaExtra  = "backuptest-extra.txt"
	probeDocsOrig    = "backuptest-docs-value"
	probeMediaOrig   = "backuptest-media-value"
	probeDocsMutated = "backuptest-docs-mutated"
)

// caseDocsMediaRestoreRoundtrip covers backup/restore of the optional docs/media storages (see
// files.docsBackup/mediaBackup, backup.RegisterOptional) - added in the same change that
// introduced them, but deferred until now (see caseRestoreRoundtrip's doc) since it needs
// RestoreFile's clear-then-copy to run against a real directory without touching the live
// DataPath. Redirects DataPath (not StoragePath, so every other storage stays on the isolated
// copy PrepareIsolatedStorage already set up) to a scratch directory for the duration -
// configmanager.SetDataAndStoragePaths is in-memory only, same pattern caseCheckAutoBackup uses
// for KNOV_BACKUPS_PATH.
func caseDocsMediaRestoreRoundtrip() test.CaseResult {
	name := "docs-media-restore-roundtrip"

	scratchData, err := os.MkdirTemp("", "knov-backuptest-data-*")
	if err != nil {
		return errCase(name, err)
	}
	defer os.RemoveAll(scratchData)

	origData := configmanager.GetAppConfig().DataPath
	origStorage := configmanager.GetAppConfig().StoragePath
	configmanager.SetDataAndStoragePaths(scratchData, origStorage)
	defer configmanager.SetDataAndStoragePaths(origData, origStorage)

	docsProbe := filepath.Join(pathutils.DocsRoot(), probeDocsFile)
	mediaProbe := filepath.Join(pathutils.MediaRoot(), probeMediaFile)
	if err := writeUnder(docsProbe, probeDocsOrig); err != nil {
		return errCase(name, err)
	}
	if err := writeUnder(mediaProbe, probeMediaOrig); err != nil {
		return errCase(name, err)
	}

	target, cleanup, err := scratchTarget()
	if err != nil {
		return errCase(name, err)
	}
	defer cleanup()

	setName, err := backup.Run(target, backup.SourceManual, files.DocsStorageName, files.MediaStorageName)
	if err != nil {
		return errCase(name, err)
	}

	// mutate: overwrite both probes, and add a file that didn't exist at backup time - restore
	// must both revert the overwrite and remove the extra file (RestoreFile clears destDir first).
	if err := writeUnder(docsProbe, probeDocsMutated); err != nil {
		return errCase(name, err)
	}
	if err := os.Remove(mediaProbe); err != nil {
		return errCase(name, err)
	}
	docsExtra := filepath.Join(pathutils.DocsRoot(), probeDocsExtra)
	mediaExtra := filepath.Join(pathutils.MediaRoot(), probeMediaExtra)
	if err := writeUnder(docsExtra, "extra"); err != nil {
		return errCase(name, err)
	}
	if err := writeUnder(mediaExtra, "extra"); err != nil {
		return errCase(name, err)
	}

	if err := restoreAndReinit(target, setName); err != nil {
		return errCase(name, err)
	}

	docsContent, docsErr := os.ReadFile(docsProbe)
	mediaContent, mediaErr := os.ReadFile(mediaProbe)
	_, docsExtraErr := os.Stat(docsExtra)
	_, mediaExtraErr := os.Stat(mediaExtra)

	docsOK := docsErr == nil && string(docsContent) == probeDocsOrig
	mediaOK := mediaErr == nil && string(mediaContent) == probeMediaOrig
	docsExtraGone := os.IsNotExist(docsExtraErr)
	mediaExtraGone := os.IsNotExist(mediaExtraErr)

	success := docsOK && mediaOK && docsExtraGone && mediaExtraGone
	cr := test.CaseResult{
		Name:     name,
		Expected: "restore reverts both probes to their backed-up content and removes files added after the backup",
		Actual: fmt.Sprintf("docsOK=%v mediaOK=%v docsExtraGone=%v mediaExtraGone=%v",
			docsOK, mediaOK, docsExtraGone, mediaExtraGone),
		Success: success,
	}
	if !success {
		cr.Error = "docs/media backup.Restore did not roll back to the backed-up snapshot as expected"
	}
	return cr
}

// writeUnder writes content to path, creating any missing parent directories first.
func writeUnder(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}
