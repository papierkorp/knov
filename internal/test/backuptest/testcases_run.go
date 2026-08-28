package backuptest

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"knov/internal/backup"
	"knov/internal/configStorage"
	"knov/internal/metadataStorage"
	"knov/internal/test"
)

// extractedStorageDirs reports which of names have a non-empty subdirectory in extractDir - a
// backup archive already extracted via extractTarGz.
func extractedStorageDirs(extractDir string, names []string) (nonEmpty []string, err error) {
	for _, n := range names {
		hasContent := false
		walkErr := filepath.Walk(filepath.Join(extractDir, n), func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // missing subdirectory entirely - not an error, just not present
			}
			if !info.IsDir() && info.Size() > 0 {
				hasContent = true
			}
			return nil
		})
		if walkErr != nil {
			return nil, walkErr
		}
		if hasContent {
			nonEmpty = append(nonEmpty, n)
		}
	}
	return nonEmpty, nil
}

// runAndExtract runs backup.Run(target, backup.SourceManual, names...) and extracts the
// resulting archive into a fresh temp dir the caller must os.RemoveAll.
func runAndExtract(target backup.BackupTarget, names ...string) (extractDir string, err error) {
	setName, err := backup.Run(target, backup.SourceManual, names...)
	if err != nil {
		return "", err
	}
	rc, err := target.Read(setName)
	if err != nil {
		return "", err
	}
	defer rc.Close()

	extractDir, err = os.MkdirTemp("", "knov-backuptest-extract-*")
	if err != nil {
		return "", err
	}
	if err := extractTarGz(rc, extractDir); err != nil {
		os.RemoveAll(extractDir)
		return "", err
	}
	return extractDir, nil
}

// caseRunProducesDefaultSet covers backup.Run with no explicit selection producing a set with
// every default storage's subdirectory populated with real content, while the optional storages
// (docs, media - see backup.RegisterOptional) are left out entirely, not just empty. Seeds one
// probe per storage first (see seedProbes), then extracts the resulting archive directly (not via
// Restore) to check.
func caseRunProducesDefaultSet() test.CaseResult {
	name := "run-produces-default-set"

	if _, err := seedProbes(); err != nil {
		return errCase(name, err)
	}

	target, cleanup, err := scratchTarget()
	if err != nil {
		return errCase(name, err)
	}
	defer cleanup()

	extractDir, err := runAndExtract(target)
	if err != nil {
		return errCase(name, err)
	}
	defer os.RemoveAll(extractDir)

	defaultNames := backup.DefaultNames()
	withContent, err := extractedStorageDirs(extractDir, defaultNames)
	if err != nil {
		return errCase(name, err)
	}
	var missing []string
	for _, n := range defaultNames {
		if !slices.Contains(withContent, n) {
			missing = append(missing, n)
		}
	}

	optionalWithContent, err := extractedStorageDirs(extractDir, optionalNames())
	if err != nil {
		return errCase(name, err)
	}

	success := len(missing) == 0 && len(optionalWithContent) == 0
	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("every default storage %v has a non-empty subdirectory, and no optional storage does", defaultNames),
		Actual:   fmt.Sprintf("missing default: %v, optional present: %v", missing, optionalWithContent),
		Success:  success,
	}
	if !success {
		cr.Error = "backup.Run with no explicit selection did not produce exactly the default storages as expected"
	}
	return cr
}

// optionalNames returns every registered storage not in DefaultNames - the storages the default
// backup set must exclude.
func optionalNames() []string {
	def := backup.DefaultNames()
	var opt []string
	for _, n := range backup.RegisteredNames() {
		if !slices.Contains(def, n) {
			opt = append(opt, n)
		}
	}
	return opt
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
// afterward. Uses an explicit storage selection (not the default backup) so the temporarily
// registered failing storage never has to be part of every future default backup.
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
