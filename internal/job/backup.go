// Package job - backup/restore, run through internal/job so they're logged into JobRun history
// like gitPushJob etc. Restore is destructive: backup.Restore always takes a fresh safety
// snapshot of the current state before overwriting anything, and neither job hot-swaps any live
// *sql.DB/in-memory state - a successful restore restarts the app afterwards, same as the
// existing "data path changed, needs restart" flow, so a fresh process picks up the restored
// files.
package job

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"knov/internal/backup"
	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/git"
	"knov/internal/logging"
	"knov/internal/system"
)

// backup can't import configmanager itself (configmanager already depends on configStorage, which
// depends back on backup for BackupFile/RestoreFile), so job - which already depends on both -
// wires the app's configured timezone into backup set names/parsing here instead.
func init() {
	backup.SetLocationFunc(configmanager.GetTimezone)
}

// DefaultBackupTarget returns the backup target every backup/restore call uses. It's an S3 bucket
// when KNOV_BACKUP_S3_BUCKET is set, otherwise the local filesystem rooted at KNOV_BACKUPS_PATH -
// a folder next to (not inside) the storages it snapshots, so it can be pointed at separate
// storage (e.g. a different disk) without touching StoragePath.
func DefaultBackupTarget() (backup.BackupTarget, error) {
	if cfg := configmanager.GetBackupS3Config(); cfg.Bucket != "" {
		return backup.NewS3Target(cfg)
	}
	return backup.NewLocalTarget(configmanager.GetBackupsPath())
}

// BackupStorageInfo describes where backup sets currently go, for the status line on
// /system/backup. Kind is "local" or "S3"; the remaining fields are populated per kind and left
// empty otherwise. Composing this into display text is render's job, not job's.
type BackupStorageInfo struct {
	Kind     string // "local" or "S3"
	Path     string // local: the KNOV_BACKUPS_PATH directory
	Bucket   string // S3: bucket name
	Prefix   string // S3: key prefix, "" for bucket root
	Endpoint string // S3: endpoint host
}

// BackupStorage reports the configured backup storage target - S3 when KNOV_BACKUP_S3_BUCKET is
// set, otherwise the local filesystem.
func BackupStorage() BackupStorageInfo {
	cfg := configmanager.GetBackupS3Config()
	if cfg.Bucket == "" {
		return BackupStorageInfo{Kind: "local", Path: configmanager.GetBackupsPath()}
	}
	return BackupStorageInfo{
		Kind:     "S3",
		Bucket:   cfg.Bucket,
		Prefix:   strings.Trim(cfg.Prefix, "/"),
		Endpoint: cfg.Endpoint,
	}
}

// ListBackupLog returns the full backup/restore history, newest first, enriched with each
// referenced set's current availability/default/locked state.
func ListBackupLog() ([]backup.LogEntry, error) {
	target, err := DefaultBackupTarget()
	if err != nil {
		return nil, fmt.Errorf("failed to open backup target: %w", err)
	}
	return backup.Log(target)
}

// RegisteredStorageNames returns every storage available to back up individually (e.g. "just
// metadata"), including optional ones (e.g. docs/media) not part of the default backup - for
// building a selection UI.
func RegisteredStorageNames() []string {
	return backup.RegisteredNames()
}

// DefaultStorageNames returns the storages a backup includes when none are explicitly selected -
// see RegisteredStorageNames for the full list a selection UI can offer beyond this.
func DefaultStorageNames() []string {
	return backup.DefaultNames()
}

// BackupManifest returns the storage names included in the named backup set.
func BackupManifest(setName string) ([]string, error) {
	target, err := DefaultBackupTarget()
	if err != nil {
		return nil, fmt.Errorf("failed to open backup target: %w", err)
	}
	return backup.Manifest(target, setName)
}

type backupJob struct {
	target  backup.BackupTarget
	source  backup.EventSource
	profile string // "" for a manual backup, otherwise the auto-backup profile that triggered it
	names   []string
	name    string
}

func (j *backupJob) Name() string { return "backup" }

func (j *backupJob) Run(_ context.Context) error {
	name, err := backup.RunProfile(j.target, j.source, j.profile, j.names...)
	if err != nil {
		return err
	}
	j.name = name
	if err := rotateBackups(j.target); err != nil {
		logging.LogWarning(logging.KeyApp, "backup: rotation failed: %v", err)
	}
	return nil
}

func (j *backupJob) Output() any { return j.name }

func (j *backupJob) Message() string { return fmt.Sprintf("created backup set %s", j.name) }

// rotateBackups trims target per KNOV_BACKUP_ROTATION_KEEP_DAYS / KNOV_BACKUP_ROTATION_KEEP_DEFAULT,
// always keeping locked sets regardless of either setting.
func rotateBackups(target backup.BackupTarget) error {
	return backup.Rotate(target, configmanager.GetBackupRotationKeepDays(), configmanager.GetBackupRotationKeepDefault())
}

// RunBackup creates a new backup set on the default target and trims expired sets per the
// configured rotation strategy, with dedup protection. names selects which storages to include -
// empty means DefaultStorageNames (the default backup), which excludes optional storages like
// docs/media unless named explicitly. Returns the created set's name.
func RunBackup(names ...string) (string, error) {
	return runBackup(backup.SourceManual, "", names...)
}

func runBackup(source backup.EventSource, profile string, names ...string) (string, error) {
	target, err := DefaultBackupTarget()
	if err != nil {
		return "", fmt.Errorf("failed to open backup target: %w", err)
	}
	j := &backupJob{target: target, source: source, profile: profile, names: names}
	if err := execute(&backupMu, j); err != nil {
		return "", err
	}
	return j.name, nil
}

// checkAutoBackup runs every configured auto-backup profile (KNOV_BACKUP_AUTO_PROFILES) that's
// due per its own cron schedule. Ticked from Start() on a fixed internal cadence
// (backupAutoCheckInterval) - the schedule users actually control is when each profile's backup
// is taken, not how often this check runs. Each profile is due-tracked independently (see
// backup.AutoBackupDue) off the newest set it itself created, so one profile being behind
// schedule (or never having run yet) never affects another's, and a manually-triggered backup
// never pushes any profile's next due time back.
func checkAutoBackup() {
	profiles := configmanager.GetBackupAutoProfiles()
	if len(profiles) == 0 {
		return
	}

	target, err := DefaultBackupTarget()
	if err != nil {
		logging.LogWarning(logging.KeyApp, "backup: failed to check auto-backup due time: %v", err)
		return
	}

	for _, p := range profiles {
		schedule, err := backup.ParseCronSchedule(p.Cron)
		if err != nil {
			logging.LogWarning(logging.KeyApp, "backup: profile %q has invalid cron %q: %v", p.Name, p.Cron, err)
			continue
		}
		due, err := backup.AutoBackupDue(target, schedule, p.Name)
		if err != nil {
			logging.LogWarning(logging.KeyApp, "backup: failed to check auto-backup due time for profile %q: %v", p.Name, err)
			continue
		}
		if !due {
			continue
		}
		if _, err := runBackup(backup.SourceScheduled, p.Name, p.Storages...); err != nil && !errors.Is(err, ErrAlreadyRunning) {
			logging.LogWarning(logging.KeyApp, "backup: automatic backup for profile %q failed: %v", p.Name, err)
		}
	}
}

// CheckAutoBackupNow runs checkAutoBackup synchronously - exported only for backuptest's
// enabled/disabled/due sub-cases, so they don't have to wait on the real ticker/Start()'s
// 5-minute startup delay.
func CheckAutoBackupNow() {
	checkAutoBackup()
}

type restoreArgs struct {
	SetName string `json:"setName"`
}

type restoreJob struct {
	target  backup.BackupTarget
	setName string
}

func (j *restoreJob) Name() string { return JobTypeRestore }

// Resumable is true because backup.Restore re-applying the same set to a storage is idempotent -
// a crash mid-restore is safely replayed from scratch on next boot via RecoverInterrupted.
func (j *restoreJob) Resumable() bool { return true }

// Run deliberately ignores ctx - restore is destructive and self-restarts the app once it's
// touched live storage (see below), so aborting mid-way would leave things in a worse state
// than either finishing or never having started; it isn't offered as cancellable in the UI.
func (j *restoreJob) Run(_ context.Context) error {
	// touched is set from inside the afterRestore callback backup.Restore requires - it only
	// runs when Restore got far enough to actually overwrite live storage (see backup.Restore),
	// which is also exactly when a restart is needed to recover any closed *sql.DB handles.
	var touched bool
	err := backup.Restore(j.target, j.setName, func() {
		touched = true
		// The safety snapshot Restore just took is otherwise never trimmed - rotate now so
		// repeated restores don't leave the backup dir growing unbounded.
		if rotateErr := rotateBackups(j.target); rotateErr != nil {
			logging.LogWarning(logging.KeyApp, "restore: rotation failed: %v", rotateErr)
		}
		// Restore rolled storages back, but the cache - deliberately not backed up, see
		// cacheStorage.CacheStorage's doc - still holds data derived from the pre-restore
		// state. A rebuild can't run here (restored storages' handles are already closed and
		// the restart below is what reopens them), so flush instead - the fresh process
		// rebuilds it on demand.
		if cacheErr := files.CacheInvalidate(); cacheErr != nil {
			logging.LogWarning(logging.KeyApp, "restore: cache invalidation failed: %v", cacheErr)
		}
		// docs/media storages restore straight onto the working tree without going through git,
		// so commit that now rather than leaving it for the next auto-commit cronjob tick to
		// silently sweep up as an unrelated "external changes" commit. Gated on the manifest
		// actually including docs/media - a restore of only the database-backed storages touches
		// nothing git tracks, so committing/pushing then would just be "git add -A" of whatever
		// else happens to be dirty in the working tree, unrelated to this restore. Uses
		// CommitRestoredFiles, not CommitAllPending, since the latter's remote-sync step would
		// fetch+hard-reset before committing and silently discard the files just restored - see
		// CommitRestoredFiles. If a remote is configured, force-push the result too: a restore is
		// a deliberate "this device's state is now authoritative" action, so the remote (and
		// whatever any other synced device picks up next) should reflect it rather than reverting
		// it on the next regular sync.
		manifest, manifestErr := BackupManifest(j.setName)
		if manifestErr != nil {
			logging.LogWarning(logging.KeyApp, "restore: failed to read manifest for set %s: %v", j.setName, manifestErr)
		} else if slices.Contains(manifest, files.DocsStorageName) || slices.Contains(manifest, files.MediaStorageName) {
			committed, commitErr := git.CommitRestoredFiles()
			if commitErr != nil {
				logging.LogWarning(logging.KeyApp, "restore: failed to commit restored files: %v", commitErr)
			} else if committed {
				if pushErr := git.ForcePushRestore(); pushErr != nil {
					logging.LogWarning(logging.KeyApp, "restore: failed to push restored state to remote, remote may now be out of sync with this device: %v", pushErr)
				}
			}
		}
		go func() {
			time.Sleep(500 * time.Millisecond)
			// must exit regardless of whether the restart below succeeds - the live storage
			// connections are already broken, so staying up is not an option
			if err := system.Restart(); err != nil {
				logging.LogError(logging.KeyApp, "restore: failed to restart, manual restart required: %v", err)
			}
			os.Exit(0)
		}()
	})
	if !touched {
		// plain failure (bad set name, safety snapshot failed, extraction failed) - no live
		// storage was touched, safe to just report the error and leave the process running
		return err
	}
	if err != nil {
		logging.LogError(logging.KeyApp, "restore: set %s applied with errors, restarting anyway to recover storage connections: %v", j.setName, err)
	} else {
		logging.LogInfo(logging.KeyApp, "restore: applied backup set %s, restarting to apply it", j.setName)
	}
	return err
}

func (j *restoreJob) Message() string {
	return fmt.Sprintf("restored backup set %s, restarting", j.setName)
}

// ErrUnknownBackupSet is returned when a named backup set does not exist, letting handlers map it
// to a 404 rather than a generic 500.
var ErrUnknownBackupSet = errors.New("unknown backup set")

// resolveExistingSet opens the default backup target and confirms name is an existing set on it -
// name also ends up as part of a filesystem path, so callers must reject it outright rather than
// trust it as-is.
func resolveExistingSet(name string) (backup.BackupTarget, error) {
	target, err := DefaultBackupTarget()
	if err != nil {
		return nil, fmt.Errorf("failed to open backup target: %w", err)
	}
	names, err := target.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list backups: %w", err)
	}
	if !slices.Contains(names, name) {
		return nil, fmt.Errorf("%w %q", ErrUnknownBackupSet, name)
	}
	return target, nil
}

// RunRestore takes a fresh safety snapshot, restores the named backup set onto disk, and
// restarts the app so the restored files take effect, with dedup protection against a concurrent
// backup/restore. Runs via StartAsync (not execute) so the triggering HTTP request returns
// immediately with a job id to poll instead of blocking until the restore - and the app restart
// right after it - completes; see restoreJob.Resumable for why that's safe. setName must match
// an existing set exactly - rejected otherwise, since it also ends up as part of a filesystem
// path.
func RunRestore(setName string) (string, error) {
	target, err := resolveExistingSet(setName)
	if err != nil {
		return "", err
	}
	args, err := json.Marshal(restoreArgs{SetName: setName})
	if err != nil {
		return "", fmt.Errorf("failed to encode job args: %w", err)
	}
	return StartAsync(&backupMu, &restoreJob{target: target, setName: setName}, string(args))
}

// LockBackup marks a backup set to never be deleted by automatic rotation, until UnlockBackup is
// called.
func LockBackup(name string) error {
	target, err := resolveExistingSet(name)
	if err != nil {
		return err
	}
	return target.Lock(name)
}

// UnlockBackup removes a backup set's protection from automatic rotation.
func UnlockBackup(name string) error {
	target, err := resolveExistingSet(name)
	if err != nil {
		return err
	}
	return target.Unlock(name)
}

// OpenBackup opens the named backup set's raw .tar.gz archive for download. name must match an
// existing set exactly - rejected otherwise, since it also ends up as part of a filesystem path.
func OpenBackup(name string) (io.ReadCloser, error) {
	target, err := resolveExistingSet(name)
	if err != nil {
		return nil, err
	}
	return target.Read(name)
}

// DeleteBackup permanently removes a backup set. Rejected while the set is locked - unlock it
// first via UnlockBackup - since a lock exists specifically to protect a set from deletion.
func DeleteBackup(name string) error {
	target, err := resolveExistingSet(name)
	if err != nil {
		return err
	}
	locked, err := target.Locked(name)
	if err != nil {
		return fmt.Errorf("failed to check lock on %s: %w", name, err)
	}
	if locked {
		return fmt.Errorf("backup set %s is locked, unlock it first", name)
	}
	return target.Delete(name)
}
