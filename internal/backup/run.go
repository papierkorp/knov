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
	"sort"
	"strings"
	"time"

	"knov/internal/logging"
)

// nameLayout is both the backup set name and, parsed back, the timestamp Rotate buckets by.
const nameLayout = "2006-01-02T15-04-05"

// manifestFile holds the list of storages included in a backup set, so a partial set (e.g.
// "just metadata") can be told apart from a full one without extracting the whole archive.
const manifestFile = "manifest.json"

type manifest struct {
	Storages []string `json:"storages"`
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

// ParseSetTime parses a backup set's name back into the timestamp it was created at. Only the
// fixed-width leading timestamp is read - Run appends a "_<storages>" suffix to partial sets'
// names, which time.Parse would otherwise reject outright as trailing garbage. Uses locationFunc
// so the parsed instant matches what Run stamped the name with, not an arbitrary zone.
func ParseSetTime(name string) (time.Time, error) {
	if len(name) < len(nameLayout) {
		return time.Time{}, fmt.Errorf("invalid backup set name %q", name)
	}
	return time.ParseInLocation(nameLayout, name[:len(nameLayout)], locationFunc())
}

// IsFullSet reports whether name is a full backup set (created with every storage registered at
// the time, so Run appended no "_<storages>" partial-selection suffix) rather than a partial one
// like "just metadata". nameLayout's timestamp never itself contains an underscore, so any
// underscore in name is guaranteed to be that suffix, regardless of any "-N" dedup suffix
// uniqueSetName may have added on top.
func IsFullSet(name string) bool {
	return !strings.Contains(name, "_")
}

// Run snapshots the given storages (by the names passed to Register; all registered storages
// when names is empty) into a new, timestamped backup set on target. source is recorded on the
// logged event only (see BackupTarget.LogEvent) - it has no effect on what gets backed up.
// Storages are snapshotted sequentially, each under its own lock - not one cross-storage
// transaction. A failure partway through discards the whole set rather than writing an
// incomplete one, since a backup that looks valid but silently missed a storage is worse than no
// backup at all. Returns the created set's name.
func Run(target BackupTarget, source EventSource, names ...string) (string, error) {
	if len(names) == 0 {
		names = RegisteredNames()
	} else {
		names = append([]string(nil), names...)
		sort.Strings(names)
	}

	setName := time.Now().In(locationFunc()).Format(nameLayout)
	if len(names) < len(RegisteredNames()) {
		setName += "_" + strings.Join(names, "-")
	}
	// Second-resolution timestamps collide when two sets are created within the same second (e.g.
	// a manual backup immediately followed by a restore's own pre-restore safety snapshot) -
	// disambiguate up front rather than letting target.Write's collision guard fail the whole run.
	setName, err := uniqueSetName(target, setName)
	if err != nil {
		return "", fmt.Errorf("failed to check existing backup sets: %w", err)
	}

	staging, err := os.MkdirTemp("", "knov-backup-*")
	if err != nil {
		return "", fmt.Errorf("failed to create staging dir: %w", err)
	}
	defer os.RemoveAll(staging)

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
		logging.LogDebug(logging.KeyApp, "backup: snapshotted %s for set %s", n, setName)
	}

	manifestBytes, err := json.Marshal(manifest{Storages: names})
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
// whether a set is a full backup or a partial one (e.g. "metadata only").
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
// files on disk. It does not hot-swap any live *sql.DB or in-memory state - the caller must
// restart the app afterwards for the restored files to take effect. Before overwriting
// anything, it takes a fresh safety snapshot of the current state, so a bad restore can itself
// be undone.
func Restore(target BackupTarget, name string) error {
	// Confirm the requested set actually exists before paying for a safety snapshot - a typo'd
	// or already-rotated-away name would otherwise still burn a full backup cycle before failing.
	// Closed again immediately rather than held open across the safety snapshot below (which can
	// take a while): fine for a local file, but holding a target.Read stream open across an
	// unrelated, potentially slow operation would be fragile for a future remote target (S3/NFS).
	rc, err := target.Read(name)
	if err != nil {
		return fmt.Errorf("backup set %s not found: %w", name, err)
	}
	rc.Close()

	if _, err := Run(target, SourceRestore); err != nil {
		return fmt.Errorf("pre-restore safety snapshot failed, aborting restore: %w", err)
	}

	rc, err = target.Read(name)
	if err != nil {
		return fmt.Errorf("backup set %s disappeared during restore: %w", name, err)
	}
	defer rc.Close()

	staging, err := os.MkdirTemp("", "knov-restore-*")
	if err != nil {
		return fmt.Errorf("failed to create staging dir: %w", err)
	}
	defer os.RemoveAll(staging)

	if err := untar(rc, staging); err != nil {
		return fmt.Errorf("failed to extract backup set %s: %w", name, err)
	}

	included, err := readManifest(staging)
	if err != nil {
		return fmt.Errorf("failed to read manifest for set %s: %w", name, err)
	}
	sort.Strings(included)

	// Driven by the manifest, not by which staging subdirectories exist - a storage that was
	// genuinely empty at backup time has no extracted subdirectory either, but is still meant to
	// be restored (to empty), not silently left as-is. Every storage is attempted even if an
	// earlier one failed (best effort, errors collected) rather than aborting on the first
	// failure - once we get this far, some storages' handles may already be closed regardless, so
	// stopping early would only restore less without avoiding that risk.
	var errs []error
	for _, n := range included {
		s, ok := lookupStorage(n)
		if !ok {
			continue // storage no longer registered (e.g. removed since the backup was made)
		}
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
		return fmt.Errorf("%w: %w", ErrRestoreIncomplete, errors.Join(errs...))
	}

	logging.LogInfo(logging.KeyApp, "restore: applied backup set %s", name)
	return nil
}

// uniqueSetName returns base, or base with a "-N" suffix appended if a set named base already
// exists on target.
func uniqueSetName(target BackupTarget, base string) (string, error) {
	existing, err := target.List()
	if err != nil {
		return "", err
	}
	taken := make(map[string]bool, len(existing))
	for _, n := range existing {
		taken[n] = true
	}
	name := base
	for i := 1; taken[name]; i++ {
		name = fmt.Sprintf("%s-%d", base, i)
	}
	return name, nil
}

// readManifest reads back the storage names an extracted backup set (in staging) contains.
func readManifest(staging string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(staging, manifestFile))
	if err != nil {
		return nil, err
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m.Storages, nil
}
