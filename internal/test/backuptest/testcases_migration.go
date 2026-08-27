// Package backuptest - cross-backend restore/migration cases: backup.Migratable/RestoreMigrate,
// the manifest "backends" field's non-Migratable-mismatch and pre-migration-era compatibility
// paths, backup.Restore's afterRestore/touched contract, kanbanStorage's noop short-circuit, and
// the cache refresh a restore is expected to trigger. See docs/temp_todo.md's "backup/restore
// testcases" section this file was written to close out.
package backuptest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"os"

	"knov/internal/backup"
	"knov/internal/cacheStorage"
	"knov/internal/configStorage"
	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/kanbanStorage"
	"knov/internal/logging"
	"knov/internal/metadataStorage"
	"knov/internal/test"
)

// caseCrossBackendRestoreMetadata covers backup.Migratable/RestoreMigrate for metadataStorage: a
// backup taken with the sqlite backend restores/converts cleanly into a live json backend. Both
// legs use throwaway temp storage paths (never the real, configured one) so this never touches
// real metadata - metadataStorage.Init always creates/opens sqlite and json at the exact path it's
// given, so this is safe (unlike "yaml", whose docsPath is fixed to the real DataPath regardless
// of the path passed in - see caseYAMLTaggedBackupRestoresViaMigrate for why yaml is exercised
// differently).
func caseCrossBackendRestoreMetadata() test.CaseResult {
	name := "cross-backend-restore-metadata"

	cfg := configmanager.GetAppConfig()
	defer func() {
		if err := metadataStorage.Init(cfg.MetadataStorageProvider, cfg.StoragePath); err != nil {
			logging.LogWarning(logging.KeyApp, "backuptest: failed to restore live metadata storage after %s: %v", name, err)
		}
	}()

	fromPath, err := os.MkdirTemp("", "knov-backuptest-meta-from-*")
	if err != nil {
		return errCase(name, err)
	}
	defer os.RemoveAll(fromPath)
	if err := metadataStorage.Init("sqlite", fromPath); err != nil {
		return errCase(name, err)
	}
	if err := metadataStorage.Set(probeMetaKey, probeMetaValue); err != nil {
		return errCase(name, err)
	}

	target, cleanup, err := scratchTarget()
	if err != nil {
		return errCase(name, err)
	}
	defer cleanup()

	setName, err := backup.Run(target, backup.SourceManual, "metadata")
	if err != nil {
		return errCase(name, err)
	}

	toPath, err := os.MkdirTemp("", "knov-backuptest-meta-to-*")
	if err != nil {
		return errCase(name, err)
	}
	defer os.RemoveAll(toPath)
	if err := metadataStorage.Init("json", toPath); err != nil {
		return errCase(name, err)
	}

	if err := backup.Restore(target, setName, func() {}); err != nil {
		return errCase(name, err)
	}

	convertedVal, err := metadataStorage.Get(probeMetaKey)
	if err != nil {
		return errCase(name, err)
	}
	convertedType := metadataStorage.GetBackendType()

	success := convertedType == "json" && metaTitle(convertedVal) == probeMetaTitle
	cr := test.CaseResult{
		Name:     name,
		Expected: "a sqlite-tagged metadata backup converts cleanly into a live json backend via RestoreMigrate",
		Actual:   fmt.Sprintf("convertedType=%s metaTitle=%q", convertedType, metaTitle(convertedVal)),
		Success:  success,
	}
	if !success {
		cr.Error = "RestoreMigrate did not convert the sqlite-tagged backup into the json backend as expected"
	}
	return cr
}

// caseCrossBackendRestoreKanban is caseCrossBackendRestoreMetadata's kanbanStorage counterpart:
// a backup taken with the sqlite backend restores/converts cleanly into a live json backend,
// preserving the event's original timestamp/status (kanbanStorage.migrate's insertEvents, unlike
// LogEvent, never re-stamps time.Now()).
func caseCrossBackendRestoreKanban() test.CaseResult {
	name := "cross-backend-restore-kanban"

	cfg := configmanager.GetAppConfig()
	defer func() {
		if err := kanbanStorage.Init(cfg.KanbanEventsEnabled, cfg.KanbanEventsProvider, cfg.StoragePath); err != nil {
			logging.LogWarning(logging.KeyApp, "backuptest: failed to restore live kanban storage after %s: %v", name, err)
		}
	}()

	fromPath, err := os.MkdirTemp("", "knov-backuptest-kanban-from-*")
	if err != nil {
		return errCase(name, err)
	}
	defer os.RemoveAll(fromPath)
	if err := kanbanStorage.Init(true, "sqlite", fromPath); err != nil {
		return errCase(name, err)
	}
	if err := kanbanStorage.LogEvent(probeKanbanFilePath, probeKanbanFolder, "", "inbox"); err != nil {
		return errCase(name, err)
	}

	target, cleanup, err := scratchTarget()
	if err != nil {
		return errCase(name, err)
	}
	defer cleanup()

	setName, err := backup.Run(target, backup.SourceManual, "kanban")
	if err != nil {
		return errCase(name, err)
	}

	toPath, err := os.MkdirTemp("", "knov-backuptest-kanban-to-*")
	if err != nil {
		return errCase(name, err)
	}
	defer os.RemoveAll(toPath)
	if err := kanbanStorage.Init(true, "json", toPath); err != nil {
		return errCase(name, err)
	}

	if err := backup.Restore(target, setName, func() {}); err != nil {
		return errCase(name, err)
	}

	events, err := kanbanStorage.GetEvents(probeKanbanFolder, probeKanbanFilePath, nil, nil, 0)
	if err != nil {
		return errCase(name, err)
	}
	convertedType := kanbanStorage.GetBackendType()

	success := convertedType == "json" && len(events) == 1 && events[0].ToStatus == "inbox"
	cr := test.CaseResult{
		Name:     name,
		Expected: "a sqlite-tagged kanban backup converts cleanly into a live json backend via RestoreMigrate",
		Actual:   fmt.Sprintf("convertedType=%s events=%+v", convertedType, events),
		Success:  success,
	}
	if !success {
		cr.Error = "RestoreMigrate did not convert the sqlite-tagged kanban backup into the json backend as expected"
	}
	return cr
}

// caseNonMigratableBackendMismatchFails covers a storage without Migratable support (config here,
// but the same applies to chat/notification/search): if the manifest's recorded backend differs
// from the currently active one, Restore must fail with an explicit "cannot auto-convert between
// backends" error and leave the live data untouched, rather than silently applying the mismatched
// backup as-is.
func caseNonMigratableBackendMismatchFails() test.CaseResult {
	name := "non-migratable-backend-mismatch-fails"

	if err := configStorage.Set(probeConfigKey, probeConfigValue); err != nil {
		return errCase(name, err)
	}

	target, cleanup, err := scratchTarget()
	if err != nil {
		return errCase(name, err)
	}
	defer cleanup()

	setName, err := backup.Run(target, backup.SourceManual, "config")
	if err != nil {
		return errCase(name, err)
	}

	fakeFromType := configStorage.GetBackendType() + "-legacy" // guaranteed to differ from the current type
	fakeSetName := setName + "-mismatched"
	if err := rewriteBackupSet(target, setName, fakeSetName, func(m *testManifest) {
		m.Backends["config"] = fakeFromType
	}); err != nil {
		return errCase(name, err)
	}

	const survivorValue = "backuptest-should-survive-mismatched-restore"
	if err := configStorage.Set(probeConfigKey, []byte(survivorValue)); err != nil {
		return errCase(name, err)
	}

	var touched bool
	restoreErr := backup.Restore(target, fakeSetName, func() { touched = true })

	errOK := restoreErr != nil && strings.Contains(restoreErr.Error(), "cannot auto-convert between backends")
	afterVal, err := configStorage.Get(probeConfigKey)
	if err != nil {
		return errCase(name, err)
	}
	dataUntouched := bytes.Equal(afterVal, []byte(survivorValue))

	success := errOK && !touched && dataUntouched
	cr := test.CaseResult{
		Name:     name,
		Expected: `a manifest-recorded backend mismatch for a non-Migratable storage fails restore with "cannot auto-convert between backends", fires no afterRestore, and leaves live data untouched`,
		Actual:   fmt.Sprintf("restoreErr=%v touched=%v dataUntouched=%v", restoreErr, touched, dataUntouched),
		Success:  success,
	}
	if !success {
		cr.Error = "a non-Migratable backend mismatch was not rejected/left untouched as expected"
	}
	return cr
}

// caseOldFormatBackupNoBackendsRestoresFine covers a pre-migration-era backup set whose manifest
// has no "backends" entry at all (backup.Run didn't record one yet) - Restore must still apply it
// fine via the existing same-backend path, treating a missing entry as "unknown" rather than a
// mismatch.
func caseOldFormatBackupNoBackendsRestoresFine() test.CaseResult {
	name := "old-format-backup-no-backends-restores-fine"

	if err := metadataStorage.Set(probeMetaKey, probeMetaValue); err != nil {
		return errCase(name, err)
	}

	target, cleanup, err := scratchTarget()
	if err != nil {
		return errCase(name, err)
	}
	defer cleanup()

	setName, err := backup.Run(target, backup.SourceManual, "metadata")
	if err != nil {
		return errCase(name, err)
	}

	legacyName := setName + "-legacy"
	if err := rewriteBackupSet(target, setName, legacyName, func(m *testManifest) {
		m.Backends = nil
	}); err != nil {
		return errCase(name, err)
	}

	if err := metadataStorage.Set(probeMetaKey, probeMetaMutatedValue); err != nil {
		return errCase(name, err)
	}

	if err := restoreAndReinit(target, legacyName); err != nil {
		return errCase(name, err)
	}

	val, err := metadataStorage.Get(probeMetaKey)
	if err != nil {
		return errCase(name, err)
	}

	success := metaTitle(val) == probeMetaTitle
	cr := test.CaseResult{
		Name:     name,
		Expected: `a backup set with no manifest "backends" entry restores fine via the same-backend path`,
		Actual:   fmt.Sprintf("metaTitle=%q", metaTitle(val)),
		Success:  success,
	}
	if !success {
		cr.Error = "a pre-migration-era backup (no backends entry) did not restore via the same-backend path as expected"
	}
	return cr
}

// caseAfterRestoreOnlyFiresWhenTouched covers backup.Restore's afterRestore/touched contract for
// the two ways a restore can fail before touching any live storage: an unknown set name, and a
// corrupt/non-archive set. The other documented untouched case - every attempted storage's
// RestoreMigrate itself reporting untouched - is covered by caseKanbanNoopRestoreIsNoop, since
// metadataStorage's RestoreMigrate has no such short-circuit and always reports touched=true once
// invoked (see its doc), while kanbanStorage's noop path is the one concrete way this happens in
// the current codebase.
func caseAfterRestoreOnlyFiresWhenTouched() test.CaseResult {
	name := "after-restore-only-fires-when-touched"

	target, cleanup, err := scratchTarget()
	if err != nil {
		return errCase(name, err)
	}
	defer cleanup()

	var fired bool
	afterRestore := func() { fired = true }

	badNameErr := backup.Restore(target, "does-not-exist", afterRestore)
	badNameOK := badNameErr != nil && !fired

	fired = false
	const corruptName = "corrupt-archive"
	if err := target.Write(corruptName, strings.NewReader("not a valid tar.gz archive")); err != nil {
		return errCase(name, err)
	}
	corruptErr := backup.Restore(target, corruptName, afterRestore)
	corruptOK := corruptErr != nil && !fired

	success := badNameOK && corruptOK
	cr := test.CaseResult{
		Name:     name,
		Expected: "a restore that fails before touching any storage (unknown set name, corrupt archive) never fires afterRestore",
		Actual:   fmt.Sprintf("badNameErr=%v fired-after-bad-name=%v corruptErr=%v fired-after-corrupt=%v", badNameErr, !badNameOK, corruptErr, !corruptOK),
		Success:  success,
	}
	if !success {
		cr.Error = "afterRestore fired despite Restore failing before touching any live storage"
	}
	return cr
}

// caseKanbanNoopRestoreIsNoop covers kanbanStorage's "noop" backend short-circuit in both
// directions: a backup taken while kanban was disabled, restored onto a now-enabled backend, must
// be a no-op (not an error, and must not wipe whatever the enabled backend already holds); the
// reverse (a backup taken while enabled, restored onto a now-disabled backend) must likewise be a
// no-op. Both legs also confirm afterRestore never fires - RestoreMigrate reports untouched=false
// for its noop short-circuit either way (see restoreMigrate's doc), which is what backup.Restore's
// afterRestore/touched contract (caseAfterRestoreOnlyFiresWhenTouched) relies on here.
func caseKanbanNoopRestoreIsNoop() test.CaseResult {
	name := "kanban-noop-restore-is-noop"

	cfg := configmanager.GetAppConfig()
	defer func() {
		if err := kanbanStorage.Init(cfg.KanbanEventsEnabled, cfg.KanbanEventsProvider, cfg.StoragePath); err != nil {
			logging.LogWarning(logging.KeyApp, "backuptest: failed to restore live kanban storage after %s: %v", name, err)
		}
	}()

	target, cleanup, err := scratchTarget()
	if err != nil {
		return errCase(name, err)
	}
	defer cleanup()

	// (a) backup taken while disabled -> restore onto an enabled backend is a no-op.
	disabledPath, err := os.MkdirTemp("", "knov-backuptest-kanban-noop-*")
	if err != nil {
		return errCase(name, err)
	}
	defer os.RemoveAll(disabledPath)
	if err := kanbanStorage.Init(false, "", disabledPath); err != nil {
		return errCase(name, err)
	}
	disabledSet, err := backup.Run(target, backup.SourceManual, "kanban")
	if err != nil {
		return errCase(name, err)
	}

	enabledPath, err := os.MkdirTemp("", "knov-backuptest-kanban-enabled-*")
	if err != nil {
		return errCase(name, err)
	}
	defer os.RemoveAll(enabledPath)
	if err := kanbanStorage.Init(true, "sqlite", enabledPath); err != nil {
		return errCase(name, err)
	}
	if err := kanbanStorage.LogEvent(probeKanbanFilePath, probeKanbanFolder, "", "inbox"); err != nil {
		return errCase(name, err)
	}

	var fired bool
	restoreOntoEnabledErr := backup.Restore(target, disabledSet, func() { fired = true })
	noopOntoEnabledOK := restoreOntoEnabledErr == nil && !fired

	eventsAfter, err := kanbanStorage.GetEvents(probeKanbanFolder, probeKanbanFilePath, nil, nil, 0)
	if err != nil {
		return errCase(name, err)
	}
	survivedOK := len(eventsAfter) == 1

	// (b) backup taken while enabled -> restore onto a now-disabled backend is also a no-op.
	enabledSet, err := backup.Run(target, backup.SourceManual, "kanban")
	if err != nil {
		return errCase(name, err)
	}
	if err := kanbanStorage.Init(false, "", disabledPath); err != nil {
		return errCase(name, err)
	}
	fired = false
	restoreOntoDisabledErr := backup.Restore(target, enabledSet, func() { fired = true })
	noopOntoDisabledOK := restoreOntoDisabledErr == nil && !fired

	success := noopOntoEnabledOK && survivedOK && noopOntoDisabledOK
	cr := test.CaseResult{
		Name:     name,
		Expected: "a noop<->enabled backend mismatch restores as a no-op in both directions, without an error, data loss, or afterRestore firing",
		Actual: fmt.Sprintf("noopOntoEnabledOK=%v survivedOK=%v noopOntoDisabledOK=%v restoreOntoEnabledErr=%v restoreOntoDisabledErr=%v",
			noopOntoEnabledOK, survivedOK, noopOntoDisabledOK, restoreOntoEnabledErr, restoreOntoDisabledErr),
		Success: success,
	}
	if !success {
		cr.Error = "kanbanStorage's noop short-circuit did not behave as a no-op in both directions as expected"
	}
	return cr
}

// caseYAMLTaggedBackupRestoresViaMigrate covers restoring a "yaml"-tagged backup onto sqlite/json
// via RestoreMigrate. yamlFrontmatterStorage.Backup snapshots front matter into a scratch sqlite
// file with the exact same on-disk shape a real sqlite metadata backup already has (see its doc),
// which is exactly why RestoreMigrate remaps a "yaml" fromBackendType to "sqlite" before opening
// it. This case exercises that remap by mislabeling a real sqlite-backed backup's manifest as
// "yaml" rather than by ever making the live yaml backend active: yamlFrontmatterStorage always
// operates on the real DataPath/docs front matter regardless of what storagePath it's given, and
// its Cleanup (which RestoreMigrate/Init's migration both call) strips front matter from every
// docs file in one pass - not something this suite may ever risk running against real data (see
// backuptest.go's safety doc). The remap logic under test only cares that fromBackendType=="yaml"
// opens srcDir as sqlite, which a mislabeled real sqlite backup exercises identically.
func caseYAMLTaggedBackupRestoresViaMigrate() test.CaseResult {
	name := "yaml-tagged-backup-restores-via-migrate"

	cfg := configmanager.GetAppConfig()
	defer func() {
		if err := metadataStorage.Init(cfg.MetadataStorageProvider, cfg.StoragePath); err != nil {
			logging.LogWarning(logging.KeyApp, "backuptest: failed to restore live metadata storage after %s: %v", name, err)
		}
	}()

	fromPath, err := os.MkdirTemp("", "knov-backuptest-yaml-from-*")
	if err != nil {
		return errCase(name, err)
	}
	defer os.RemoveAll(fromPath)
	if err := metadataStorage.Init("sqlite", fromPath); err != nil {
		return errCase(name, err)
	}
	if err := metadataStorage.Set(probeMetaKey, probeMetaValue); err != nil {
		return errCase(name, err)
	}

	target, cleanup, err := scratchTarget()
	if err != nil {
		return errCase(name, err)
	}
	defer cleanup()

	setName, err := backup.Run(target, backup.SourceManual, "metadata")
	if err != nil {
		return errCase(name, err)
	}

	yamlTaggedSet := setName + "-yaml-tagged"
	if err := rewriteBackupSet(target, setName, yamlTaggedSet, func(m *testManifest) {
		m.Backends["metadata"] = "yaml"
	}); err != nil {
		return errCase(name, err)
	}

	toPath, err := os.MkdirTemp("", "knov-backuptest-yaml-to-*")
	if err != nil {
		return errCase(name, err)
	}
	defer os.RemoveAll(toPath)
	if err := metadataStorage.Init("json", toPath); err != nil {
		return errCase(name, err)
	}

	if err := backup.Restore(target, yamlTaggedSet, func() {}); err != nil {
		return errCase(name, err)
	}

	val, err := metadataStorage.Get(probeMetaKey)
	if err != nil {
		return errCase(name, err)
	}
	convertedType := metadataStorage.GetBackendType()

	success := convertedType == "json" && metaTitle(val) == probeMetaTitle
	cr := test.CaseResult{
		Name:     name,
		Expected: `a "yaml"-tagged backup converts cleanly into a live json backend via RestoreMigrate's sqlite remap`,
		Actual:   fmt.Sprintf("convertedType=%s metaTitle=%q", convertedType, metaTitle(val)),
		Success:  success,
	}
	if !success {
		cr.Error = `a "yaml"-tagged backup did not restore via RestoreMigrate's sqlite remap as expected`
	}
	return cr
}

// caseRestoreRefreshesCache covers the cache-refresh half of backup.Restore's recovery contract:
// cache is deliberately not a registered storage (see cacheStorage.CacheStorage's doc), so nothing
// in backup.Restore itself touches it - restoreAndReinit (this suite's afterRestore, mirroring
// job.restoreJob's own files.CacheInvalidate) calls files.RebuildAllCaches instead. Seeds a
// poisoned value directly into the file-list cache entry so a restore that left it alone (rather
// than genuinely rebuilding it) would be caught.
func caseRestoreRefreshesCache() test.CaseResult {
	name := "restore-refreshes-cache"

	poison := []byte("backuptest-stale-poison")
	if err := cacheStorage.Set(string(files.CacheKeyFullFileList), poison); err != nil {
		return errCase(name, err)
	}

	target, cleanup, err := scratchTarget()
	if err != nil {
		return errCase(name, err)
	}
	defer cleanup()

	setName, err := backup.Run(target, backup.SourceManual, "config")
	if err != nil {
		return errCase(name, err)
	}

	if err := restoreAndReinit(target, setName); err != nil {
		return errCase(name, err)
	}

	afterVal, err := cacheStorage.Get(string(files.CacheKeyFullFileList))
	if err != nil {
		return errCase(name, err)
	}

	refreshed := !bytes.Equal(afterVal, poison)
	var parsed []files.File
	validJSON := json.Unmarshal(afterVal, &parsed) == nil

	success := refreshed && validJSON
	cr := test.CaseResult{
		Name:     name,
		Expected: "restore's afterRestore rebuilds the file-list cache (files.RebuildAllCaches) rather than leaving a stale/poisoned entry in place",
		Actual:   fmt.Sprintf("refreshed=%v validJSON=%v", refreshed, validJSON),
		Success:  success,
	}
	if !success {
		cr.Error = "restore did not refresh the file-list cache as expected"
	}
	return cr
}
