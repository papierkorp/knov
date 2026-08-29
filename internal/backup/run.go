package backup

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"knov/internal/logging"
)

// nameLayout is both the backup set name's timestamp portion and, parsed back, the timestamp
// Rotate buckets by. namePrefix identifies the app that created a set, in case target's root is
// ever shared with other backups (e.g. a bucket also used by something else).
const nameLayout = "2006-01-02T15-04-05"
const namePrefix = "knov-"

// profileTagSuffix marks which named auto-backup profile (see RunProfile) created a set, appended
// after any "_<storages>" partial-selection suffix - e.g. "..._docs-media@weekly-docs". Kept
// distinct from that suffix (and from IsDefaultSet, which only looks at "_") so a profile's own
// due-tracking (AutoBackupDue) never depends on, and never affects, whether a set counts as
// default-content.
const profileTagSuffix = "@"

// manifestFile holds the list of storages included in a backup set, so a partial set (e.g.
// "just metadata") can be told apart from a default one without extracting the whole archive.
const manifestFile = "manifest.json"

type manifest struct {
	Storages []string `json:"storages"`
	// Backends records each storage's GetBackendType() at backup time, so Restore can detect a
	// provider that has since changed and convert the data instead of applying it as-is. Absent
	// or missing entries (backups made before this field existed) are treated as "unknown" by
	// Restore, which falls back to applying the backup as-is - the pre-existing behavior.
	Backends map[string]string `json:"backends,omitempty"`
}

// locationFunc returns the timezone backup set names are stamped with and parsed back in -
// defaults to time.Local so this package works standalone. job overrides it (via
// SetLocationFunc) to configmanager.GetTimezone at startup, so backup set timestamps follow the
// app's configured timezone like every other displayed date instead of the server process's own -
// this package can't import configmanager directly, since configmanager already depends on
// configStorage, which depends back on this package for BackupFile/RestoreFile.
var locationFunc = func() *time.Location { return time.Local }

// SetLocationFunc overrides the timezone used for backup set names - see locationFunc.
func SetLocationFunc(f func() *time.Location) {
	locationFunc = f
}

// ParseSetTime parses a backup set's name back into the timestamp it was created at. namePrefix
// is stripped first if present (tolerant, not required, so sets from before namePrefix existed
// still parse), then only the fixed-width leading timestamp is read - Run appends a
// "_<storages>" suffix to partial sets' names, which time.Parse would otherwise reject outright
// as trailing garbage. Uses locationFunc so the parsed instant matches what Run stamped the name
// with, not an arbitrary zone.
func ParseSetTime(name string) (time.Time, error) {
	name = strings.TrimPrefix(name, namePrefix)
	if len(name) < len(nameLayout) {
		return time.Time{}, fmt.Errorf("invalid backup set name %q", name)
	}
	return time.ParseInLocation(nameLayout, name[:len(nameLayout)], locationFunc())
}

// IsDefaultSet reports whether name is a default backup set (created with exactly DefaultNames,
// so Run appended no "_<storages>" partial-selection suffix) rather than an explicit/partial one
// like "just metadata" or "everything, including the optional docs/media storages". nameLayout's
// timestamp never itself contains an underscore, so any underscore in name is guaranteed to be
// that suffix, regardless of any "-N" dedup suffix uniqueSetName may have added on top.
func IsDefaultSet(name string) bool {
	return !strings.Contains(name, "_")
}

// HasProfileTag reports whether name is a set RunProfile created for the named profile - see
// profileTagSuffix. Used by AutoBackupDue to scope its "newest set" search to one profile.
func HasProfileTag(name, profile string) bool {
	return strings.HasSuffix(name, profileTagSuffix+profile)
}

// Run snapshots the given storages (by the names passed to Register/RegisterOptional;
// DefaultNames when names is empty, which skips optional storages like docs/media) into a new,
// timestamped backup set on target. source is recorded on the logged event only (see
// BackupTarget.LogEvent) - it has no effect on what gets backed up. Storages are snapshotted
// sequentially, each under its own lock - not one cross-storage transaction. A failure partway
// through discards the whole set rather than writing an incomplete one, since a backup that looks
// valid but silently missed a storage is worse than no backup at all. Returns the created set's
// name.
func Run(target BackupTarget, source EventSource, names ...string) (string, error) {
	return run(target, source, "", names...)
}

// RunProfile is Run for a named automatic-backup profile (see AutoBackupDue): the created set's
// name additionally records profile, via profileTagSuffix, so each profile's own due-check only
// ever looks at backups it itself created - independent of any other profile's schedule, and of
// IsDefaultSet, which still reflects storage content only.
func RunProfile(target BackupTarget, source EventSource, profile string, names ...string) (string, error) {
	return run(target, source, profile, names...)
}

func run(target BackupTarget, source EventSource, profile string, names ...string) (string, error) {
	defaultNames := DefaultNames()
	if len(names) == 0 {
		names = defaultNames
	} else {
		names = append([]string(nil), names...)
		sort.Strings(names)
	}

	base := namePrefix + time.Now().In(locationFunc()).Format(nameLayout)
	if !slices.Equal(names, defaultNames) {
		base += "_" + strings.Join(names, "-")
	}
	tag := ""
	if profile != "" {
		tag = profileTagSuffix + profile
	}
	// Second-resolution timestamps collide when two sets are created within the same second (e.g.
	// a manual backup immediately followed by a restore's own pre-restore safety snapshot, or the
	// same profile firing twice in a row) - disambiguate up front rather than letting
	// target.Write's collision guard fail the whole run.
	setName, err := uniqueSetName(target, base, tag)
	if err != nil {
		return "", fmt.Errorf("failed to check existing backup sets: %w", err)
	}

	staging, err := os.MkdirTemp("", "knov-backup-*")
	if err != nil {
		return "", fmt.Errorf("failed to create staging dir: %w", err)
	}
	defer os.RemoveAll(staging)

	backends := make(map[string]string, len(names))
	for _, n := range names {
		s, ok := lookupStorage(n)
		if !ok {
			return "", fmt.Errorf("unknown storage %q", n)
		}
		destDir := filepath.Join(staging, n)
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return "", fmt.Errorf("backup %s: %w", n, err)
		}
		if err := s.Backup(destDir); err != nil {
			logging.LogError(logging.KeyApp, "backup: %s failed, discarding set %s: %v", n, setName, err)
			return "", fmt.Errorf("backup %s failed: %w", n, err)
		}
		backends[n] = s.GetBackendType()
		logging.LogDebug(logging.KeyApp, "backup: snapshotted %s for set %s", n, setName)
	}

	manifestBytes, err := json.Marshal(manifest{Storages: names, Backends: backends})
	if err != nil {
		return "", fmt.Errorf("failed to encode manifest: %w", err)
	}

	pr, pw := io.Pipe()
	go func() {
		gz := gzip.NewWriter(pw)
		tw := tar.NewWriter(gz)
		// manifest.json is written first so Manifest can read it back by decompressing just the
		// start of the archive, without scanning through every storage's data first.
		err := writeTarBytes(tw, manifestFile, manifestBytes)
		if err == nil {
			err = tarDir(tw, staging)
		}
		if cerr := tw.Close(); err == nil {
			err = cerr
		}
		if cerr := gz.Close(); err == nil {
			err = cerr
		}
		pw.CloseWithError(err)
	}()

	if err := target.Write(setName, pr); err != nil {
		return "", fmt.Errorf("failed to store backup set %s: %w", setName, err)
	}
	if err := target.LogEvent(EventBackup, setName, source); err != nil {
		logging.LogWarning(logging.KeyApp, "backup: failed to log event for set %s: %v", setName, err)
	}

	logging.LogInfo(logging.KeyApp, "backup: created set %s (%d storages: %s)", setName, len(names), strings.Join(names, ", "))
	return setName, nil
}

// Manifest returns the storage names included in the named backup set, read directly from the
// archive without extracting anything else - lets a caller (e.g. the backup list page) show
// whether a set is a default backup or a partial one (e.g. "metadata only").
func Manifest(target BackupTarget, name string) ([]string, error) {
	rc, err := target.Read(name)
	if err != nil {
		return nil, fmt.Errorf("backup set %s not found: %w", name, err)
	}
	defer rc.Close()

	gz, err := gzip.NewReader(rc)
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("backup set %s has no manifest", name)
		}
		if err != nil {
			return nil, err
		}
		if hdr.Name != manifestFile {
			continue
		}
		var m manifest
		if err := json.NewDecoder(tr).Decode(&m); err != nil {
			return nil, fmt.Errorf("failed to decode manifest for %s: %w", name, err)
		}
		return m.Storages, nil
	}
}

// ErrRestoreIncomplete wraps a failure that happened after Restore started overwriting live
// storages. At that point one or more sqlite-backed storages may already have closed their live
// *sql.DB handle (see each such storage's Restore doc) even though Restore itself failed - the
// caller must still restart the app to recover a working connection, not just surface the error.
var ErrRestoreIncomplete = errors.New("restore incomplete")

// Restore applies a previously created backup set back onto every registered storage's live
// files on disk, then calls afterRestore - exactly once, synchronously, before returning -
// whenever live storage may have been touched. That signal is deliberately conservative:
// Storage.Restore reports no touched/untouched outcome of its own, so any attempt counts - even
// one whose backend turns out to be a no-op (e.g. yaml metadata) - a spurious recovery beat is
// cheaper than a missed one. It never hot-swaps any live *sql.DB or in-memory state itself:
// afterRestore is the caller's one required chance to recover that,
// e.g. by restarting the process for real (job.restoreJob) or re-running every storage's Init
// in-process (backuptest.restoreAndReinit) - both need the live storages picking the restored
// (or converted) data back up, just via different means. Making afterRestore a required
// parameter here, rather than a "you must call X afterwards" convention documented next to
// Restore, means a future caller can't add a new call site that quietly forgets the recovery
// step - two independent ones (job and backuptest) had already each hand-rolled their own
// version of exactly this "only if touched" check before this existed.
func Restore(target BackupTarget, name string, afterRestore func()) error {
	touched, err := restore(target, name)
	if touched {
		afterRestore()
	}
	return err
}

// restore is Restore's implementation. Before overwriting anything, it takes a fresh safety
// snapshot of the current state, so a bad restore can itself be undone. touched, not the
// presence or type of err, is Restore's sole signal for whether afterRestore runs - every return
// below reports it explicitly rather than making the caller re-derive it from err.
func restore(target BackupTarget, name string) (touched bool, err error) {
	// Confirm the requested set actually exists before paying for a safety snapshot - a typo'd
	// or already-rotated-away name would otherwise still burn a default backup cycle before
	// failing. Closed again immediately rather than held open across the safety snapshot below
	// (which can take a while): fine for a local file, but holding a target.Read stream open
	// across an unrelated, potentially slow operation would be fragile for a future remote target
	// (S3/NFS).
	rc, err := target.Read(name)
	if err != nil {
		return false, fmt.Errorf("backup set %s not found: %w", name, err)
	}
	rc.Close()

	// The safety snapshot must cover whatever this restore is about to overwrite, not just
	// DefaultNames - a set backed up with an explicit selection (e.g. including the optional
	// docs/media storages) can touch storages the default set alone wouldn't capture. Storages
	// the manifest names that are no longer registered are dropped, same as the restore loop
	// below tolerates - Run would otherwise reject an unknown name outright.
	manifestNames, err := Manifest(target, name)
	if err != nil {
		return false, fmt.Errorf("failed to read manifest for set %s: %w", name, err)
	}
	var toSnapshot []string
	for _, n := range manifestNames {
		if _, ok := lookupStorage(n); ok {
			toSnapshot = append(toSnapshot, n)
		}
	}
	// An empty toSnapshot means either the manifest was itself empty, or every storage it recorded
	// has since been deregistered - either way there is nothing safe to snapshot, so refuse to
	// proceed rather than silently restoring with no way back.
	if len(toSnapshot) == 0 {
		return false, fmt.Errorf("pre-restore safety snapshot failed: none of set %s's storages (%v) are still registered", name, manifestNames)
	}
	if _, err := Run(target, SourceRestore, toSnapshot...); err != nil {
		return false, fmt.Errorf("pre-restore safety snapshot failed, aborting restore: %w", err)
	}

	rc, err = target.Read(name)
	if err != nil {
		return false, fmt.Errorf("backup set %s disappeared during restore: %w", name, err)
	}
	defer rc.Close()

	staging, err := os.MkdirTemp("", "knov-restore-*")
	if err != nil {
		return false, fmt.Errorf("failed to create staging dir: %w", err)
	}
	defer os.RemoveAll(staging)

	if err := untar(rc, staging); err != nil {
		return false, fmt.Errorf("failed to extract backup set %s: %w", name, err)
	}

	m, err := readManifest(staging)
	if err != nil {
		return false, fmt.Errorf("failed to read manifest for set %s: %w", name, err)
	}
	included := append([]string(nil), m.Storages...)
	sort.Strings(included)

	// Driven by the manifest, not by which staging subdirectories exist - a storage that was
	// genuinely empty at backup time has no extracted subdirectory either, but is still meant to
	// be restored (to empty), not silently left as-is. Every storage is attempted even if an
	// earlier one failed (best effort, errors collected) rather than aborting on the first
	// failure - once we get this far, some storages' handles may already be closed regardless, so
	// stopping early would only restore less without avoiding that risk.
	//
	// touched (the named return) tracks whether *any* storage's live backend may actually have
	// been altered, as opposed to a failure that never got past reading/validating the backup.
	// It, not just len(errs), decides whether ErrRestoreIncomplete applies below - a batch where
	// every failure left live storage untouched (e.g. every mismatched backend lacked Migratable
	// support) must not force the recovery/restart ErrRestoreIncomplete triggers in callers,
	// since nothing there needs recovering. Storages no longer registered are silently skipped
	// rather than counted as failures at all.
	var errs []error
	for _, n := range included {
		s, ok := lookupStorage(n)
		if !ok {
			continue // storage no longer registered (e.g. removed since the backup was made)
		}

		// fromType is "" for backups made before the Backends field existed, or for a storage
		// backup.Run didn't record for some other reason - treated as "unknown", applied as-is
		// via the pre-existing same-backend Restore path rather than assumed mismatched.
		fromType := m.Backends[n]
		currentType := s.GetBackendType()
		if fromType != "" && fromType != currentType {
			mig, ok := s.(Migratable)
			if !ok {
				errs = append(errs, fmt.Errorf("restore %s failed: backup was made with %s backend, current is %s backend, which cannot auto-convert between backends", n, fromType, currentType))
				continue
			}
			migTouched, err := mig.RestoreMigrate(filepath.Join(staging, n), fromType)
			touched = touched || migTouched
			if err != nil {
				errs = append(errs, fmt.Errorf("restore %s failed: %w", n, err))
				continue
			}
			logging.LogInfo(logging.KeyApp, "restore: converted %s from %s backup into %s backend for set %s", n, fromType, currentType, name)
			continue
		}

		// Storage.Restore has no touched/untouched signal of its own - a sqlite-backed
		// implementation may close its live *sql.DB handle before returning an error, so any
		// attempt here is conservatively treated as touched regardless of outcome.
		touched = true
		if err := s.Restore(filepath.Join(staging, n)); err != nil {
			errs = append(errs, fmt.Errorf("restore %s failed: %w", n, err))
			continue
		}
		logging.LogDebug(logging.KeyApp, "restore: applied %s from set %s", n, name)
	}

	// restores are always user-triggered - there's no scheduled restore feature
	if err := target.LogEvent(EventRestore, name, SourceManual); err != nil {
		logging.LogWarning(logging.KeyApp, "restore: failed to log event for set %s: %v", name, err)
	}

	if len(errs) > 0 {
		if touched {
			return touched, fmt.Errorf("%w: %w", ErrRestoreIncomplete, errors.Join(errs...))
		}
		return touched, errors.Join(errs...)
	}

	logging.LogInfo(logging.KeyApp, "restore: applied backup set %s", name)
	return touched, nil
}

// uniqueSetName returns base+tag, or base with a "-N" suffix inserted before tag if base+tag
// already exists on target. Disambiguating before tag (rather than at the very end) keeps tag -
// RunProfile's profileTagSuffix+profile, when present - an exact, literal trailing suffix of the
// returned name no matter how many same-second collisions it took to get there, so HasProfileTag
// can keep doing a plain, unambiguous suffix match instead of having to guess whether a trailing
// "-N" is a dedup suffix or part of another profile's own name.
func uniqueSetName(target BackupTarget, base, tag string) (string, error) {
	existing, err := target.List()
	if err != nil {
		return "", err
	}
	taken := make(map[string]bool, len(existing))
	for _, n := range existing {
		taken[n] = true
	}
	name := base + tag
	for i := 1; taken[name]; i++ {
		name = fmt.Sprintf("%s-%d%s", base, i, tag)
	}
	return name, nil
}

// readManifest reads back an extracted backup set's (in staging) manifest.
func readManifest(staging string) (manifest, error) {
	data, err := os.ReadFile(filepath.Join(staging, manifestFile))
	if err != nil {
		return manifest{}, err
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return manifest{}, err
	}
	return m, nil
}
