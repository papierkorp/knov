// Package job - backup/restore, run through internal/job so they're logged into JobRun history
// like gitPushJob etc. Restore is destructive: backup.Restore always takes a fresh safety
// snapshot of the current state before overwriting anything, and neither job hot-swaps any live
// *sql.DB/in-memory state - a successful restore restarts the app afterwards, same as the
// existing "data path changed, needs restart" flow, so a fresh process picks up the restored
// files.
package job

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"time"

	"knov/internal/backup"
	"knov/internal/configmanager"
	"knov/internal/logging"
	"knov/internal/system"
)

// backup can't import configmanager itself (configmanager already depends on configStorage, which
// depends back on backup for BackupFile/RestoreFile), so job - which already depends on both -
// wires the app's configured timezone into backup set names/parsing here instead.
func init() {
	backup.SetLocationFunc(configmanager.GetTimezone)
}

// DefaultBackupTarget returns the local-filesystem backup target every backup/restore call uses,
// rooted at KNOV_BACKUPS_PATH - a folder next to (not inside) the storages it snapshots, so it
// can be pointed at separate storage (e.g. a different disk) without touching StoragePath.
func DefaultBackupTarget() (backup.BackupTarget, error) {
	return backup.NewLocalTarget(configmanager.GetBackupsPath())
}

// ListBackupLog returns the full backup/restore history, newest first, enriched with each
// referenced set's current availability/full/locked state.
func ListBackupLog() ([]backup.LogEntry, error) {
	target, err := DefaultBackupTarget()
	if err != nil {
		return nil, fmt.Errorf("failed to open backup target: %w", err)
	}
	return backup.Log(target)
}

// RegisteredStorageNames returns every storage available to back up individually (e.g. "just
// metadata"), for building a selection UI.
func RegisteredStorageNames() []string {
	return backup.RegisteredNames()
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
	target backup.BackupTarget
	source backup.EventSource
	names  []string
	name   string
}

func (j *backupJob) Name() string { return "backup" }

func (j *backupJob) Run() error {
	name, err := backup.Run(j.target, j.source, j.names...)
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

// rotateBackups trims target per KNOV_BACKUP_ROTATION_KEEP_DAYS / KNOV_BACKUP_ROTATION_KEEP_FULL,
// always keeping locked sets regardless of either setting.
func rotateBackups(target backup.BackupTarget) error {
	return backup.Rotate(target, configmanager.GetBackupRotationKeepDays(), configmanager.GetBackupRotationKeepFull())
}

// RunBackup creates a new backup set on the default target and trims expired sets per the
// configured rotation strategy, with dedup protection. names selects which storages to include -
// empty means every registered storage (a full backup). Returns the created set's name.
func RunBackup(names ...string) (string, error) {
	return runBackup(backup.SourceManual, names...)
}

func runBackup(source backup.EventSource, names ...string) (string, error) {
	target, err := DefaultBackupTarget()
	if err != nil {
		return "", fmt.Errorf("failed to open backup target: %w", err)
	}
	j := &backupJob{target: target, source: source, names: names}
	if err := execute(&backupMu, j); err != nil {
		return "", err
	}
	return j.name, nil
}

// checkAutoBackup runs a full backup if automatic backups are enabled and the configured
// interval has elapsed since the newest existing full set. Ticked from Start() on a fixed
// internal cadence (backupAutoCheckInterval) - the interval users actually control is how often a
// backup is taken, not how often this check runs. Only full sets count towards the interval - a
// manually-triggered partial backup (e.g. "just metadata") must not push back when the next
// scheduled full backup is due.
func checkAutoBackup() {
	if !configmanager.GetBackupAutoEnabled() {
		return
	}

	interval, err := time.ParseDuration(configmanager.GetBackupAutoInterval())
	if err != nil {
		logging.LogWarning(logging.KeyApp, "backup: invalid KNOV_BACKUP_AUTO_INTERVAL %q: %v", configmanager.GetBackupAutoInterval(), err)
		return
	}

	target, err := DefaultBackupTarget()
	if err != nil {
		logging.LogWarning(logging.KeyApp, "backup: failed to check auto-backup due time: %v", err)
		return
	}
	due, err := backup.AutoBackupDue(target, interval)
	if err != nil {
		logging.LogWarning(logging.KeyApp, "backup: failed to check auto-backup due time: %v", err)
		return
	}
	if !due {
		return
	}

	if _, err := runBackup(backup.SourceScheduled); err != nil && !errors.Is(err, ErrAlreadyRunning) {
		logging.LogWarning(logging.KeyApp, "backup: automatic backup failed: %v", err)
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

func (j *restoreJob) Run() error {
	err := backup.Restore(j.target, j.setName)
	// A plain failure (bad set name, safety snapshot failed, extraction failed) touched no live
	// storage - safe to just report the error and leave the process running. But
	// ErrRestoreIncomplete means Restore got far enough to overwrite at least one storage before
	// failing, which may have already closed that storage's live *sql.DB handle (see each sqlite
	// storage's Restore doc) - the process must restart regardless to recover a working
	// connection, so fall through to the restart below instead of returning early.
	if err != nil && !errors.Is(err, backup.ErrRestoreIncomplete) {
		return err
	}
	// The safety snapshot Restore just took is otherwise never trimmed - rotate now so repeated
	// restores don't leave the backup dir growing unbounded.
	if rotateErr := rotateBackups(j.target); rotateErr != nil {
		logging.LogWarning(logging.KeyApp, "restore: rotation failed: %v", rotateErr)
	}
	if err != nil {
		logging.LogError(logging.KeyApp, "restore: set %s applied with errors, restarting anyway to recover storage connections: %v", j.setName, err)
	} else {
		logging.LogInfo(logging.KeyApp, "restore: applied backup set %s, restarting to apply it", j.setName)
	}
	go func() {
		time.Sleep(500 * time.Millisecond)
		// must exit regardless of whether the restart below succeeds - the live storage
		// connections are already broken (see comment above), so staying up is not an option
		if err := system.Restart(); err != nil {
			logging.LogError(logging.KeyApp, "restore: failed to restart, manual restart required: %v", err)
		}
		os.Exit(0)
	}()
	return err
}

func (j *restoreJob) Message() string {
	return fmt.Sprintf("restored backup set %s, restarting", j.setName)
}

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
		return nil, fmt.Errorf("unknown backup set %q", name)
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
