package backuptest

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"knov/internal/backup"
	"knov/internal/configStorage"
	"knov/internal/metadataStorage"
	"knov/internal/test"
)

// caseRunProducesFullSet covers backup.Run producing a set with every registered storage's
// subdirectory populated with real content - seeds one probe per storage first (see
// seedProbes), then extracts the resulting archive directly (not via Restore) to check each
// subdirectory landed with a non-empty file.
func caseRunProducesFullSet() test.CaseResult {
	name := "run-produces-full-set"

	if _, err := seedProbes(); err != nil {
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

	rc, err := target.Read(setName)
	if err != nil {
		return errCase(name, err)
	}
	defer rc.Close()

	extractDir, err := os.MkdirTemp("", "knov-backuptest-extract-*")
	if err != nil {
		return errCase(name, err)
	}
	defer os.RemoveAll(extractDir)
	if err := extractTarGz(rc, extractDir); err != nil {
		return errCase(name, err)
	}

	registered := backup.RegisteredNames()
	var empty []string
	for _, n := range registered {
		hasContent := false
		walkErr := filepath.Walk(filepath.Join(extractDir, n), func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && info.Size() > 0 {
				hasContent = true
			}
			return nil
		})
		if walkErr != nil || !hasContent {
			empty = append(empty, n)
		}
	}

	success := len(empty) == 0
	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("every registered storage %v has a non-empty subdirectory in the extracted archive", registered),
		Actual:   fmt.Sprintf("empty or missing: %v", empty),
		Success:  success,
	}
	if !success {
		cr.Error = "backup.Run did not produce real content for every registered storage as expected"
	}
	return cr
}

// failingStorage is a fake backup.Storage that always fails, temporarily registered under a
// unique test-only name so caseRunAbortsOnPartialFailure can force a mid-run failure without
// touching any real storage's Backup implementation.
type failingStorage struct{}

func (failingStorage) Backup(destDir string) error { return fmt.Errorf("backuptest: forced failure") }
func (failingStorage) Restore(srcDir string) error { return nil }
func (failingStorage) GetBackendType() string      { return "test" }

// caseRunAbortsOnPartialFailure covers backup.Run aborting and discarding the whole set when
// one registered storage's Backup fails mid-run - no partial .tar.gz left on the target
// afterward. Uses an explicit storage selection (not a full backup) so the temporarily
// registered failing storage never has to be part of every future full backup.
func caseRunAbortsOnPartialFailure() test.CaseResult {
	name := "run-aborts-on-partial-failure"

	target, cleanup, err := scratchTarget()
	if err != nil {
		return errCase(name, err)
	}
	defer cleanup()

	const failName = "backuptest-fail"
	backup.Register(failName, failingStorage{})
	defer backup.Unregister(failName)

	_, runErr := backup.Run(target, backup.SourceManual, "config", failName)
	failed := runErr != nil

	sets, listErr := target.List()
	if listErr != nil {
		return errCase(name, listErr)
	}
	noPartialSet := len(sets) == 0

	success := failed && noPartialSet
	cr := test.CaseResult{
		Name:     name,
		Expected: "Run returns an error and leaves no partial set on the target when one storage's Backup fails",
		Actual:   fmt.Sprintf("runErr=%v setsOnTarget=%v", runErr, sets),
		Success:  success,
	}
	if !success {
		cr.Error = "Run did not abort/discard the whole set on a mid-run storage failure as expected"
	}
	return cr
}

// caseSelectiveBackup covers backup.Run(target, source, "metadata") - the resulting manifest is
// exactly ["metadata"], only metadata/ exists in the extracted archive, and restoring it leaves
// every other storage's live data untouched.
func caseSelectiveBackup() test.CaseResult {
	name := "selective-backup"

	p, err := seedProbes()
	if err != nil {
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

	manifest, err := backup.Manifest(target, setName)
	if err != nil {
		return errCase(name, err)
	}
	manifestOK := len(manifest) == 1 && manifest[0] == "metadata"

	rc, err := target.Read(setName)
	if err != nil {
		return errCase(name, err)
	}
	extractDir, err := os.MkdirTemp("", "knov-backuptest-extract-*")
	if err != nil {
		rc.Close()
		return errCase(name, err)
	}
	extractErr := extractTarGz(rc, extractDir)
	rc.Close()
	defer os.RemoveAll(extractDir)
	if extractErr != nil {
		return errCase(name, extractErr)
	}

	onlyMetadata := true
	for _, n := range backup.RegisteredNames() {
		if n == "metadata" {
			continue
		}
		if _, statErr := os.Stat(filepath.Join(extractDir, n)); statErr == nil {
			onlyMetadata = false
		}
	}

	// mutate every probe (metadata, the included storage, plus config, an excluded one), restore
	// the selective set, and confirm only metadata came back - config must stay mutated since it
	// was never part of this set.
	if err := mutateProbes(p); err != nil {
		return errCase(name, err)
	}
	if err := restoreAndReinit(target, setName); err != nil {
		return errCase(name, err)
	}

	metaVal, err := metadataStorage.Get(probeMetaKey)
	if err != nil {
		return errCase(name, err)
	}
	metaRestored := metaTitle(metaVal) == probeMetaTitle
	configVal, err := configStorage.Get(probeConfigKey)
	if err != nil {
		return errCase(name, err)
	}
	configStillMutated := bytes.Equal(configVal, []byte("mutated"))

	success := manifestOK && onlyMetadata && metaRestored && configStillMutated
	cr := test.CaseResult{
		Name:     name,
		Expected: "manifest is exactly [metadata], only metadata/ exists in the archive, restore brings metadata back but leaves config (excluded) mutated",
		Actual:   fmt.Sprintf("manifest=%v onlyMetadata=%v metaRestored=%v configStillMutated=%v", manifest, onlyMetadata, metaRestored, configStillMutated),
		Success:  success,
	}
	if !success {
		cr.Error = "selective backup/restore did not scope itself to just the requested storage as expected"
	}
	return cr
}
