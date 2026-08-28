package files

import (
	"knov/internal/backup"
	"knov/internal/pathutils"
)

// DocsStorageName/MediaStorageName are the storage names docsBackup/mediaBackup register under -
// exported so a caller outside this package (job.restoreJob) can tell whether a given restore
// touched the working tree at all, without hardcoding these names itself.
const (
	DocsStorageName  = "docs"
	MediaStorageName = "media"
)

func init() {
	backup.RegisterOptional(DocsStorageName, docsBackup{})
	backup.RegisterOptional(MediaStorageName, mediaBackup{})
}

// docsBackup/mediaBackup wire DataPath's docs/media folders into Run/Restore alongside the
// database-backed storages - plain file trees, so there's no backend to distinguish. Registered
// as optional (see backup.RegisterOptional): selectable explicitly, but left out of the default
// backup set, since a full docs/media copy is a much larger, slower operation than the
// database-backed storages a default/scheduled backup otherwise runs.
type docsBackup struct{}

// Backup/Restore hold docsOpMu for the duration of the copy, the same lock every physical
// mutation of the docs root (movePhysical/DeleteFileNoRefresh/moveFolderPhysical) takes, so a
// backup can't be taken mid-rename and a restore's RemoveAll can't race a physical mutation.
// Restore intentionally bypasses git entirely (no staging, no commit) - it overwrites the
// working tree directly and leaves it to the caller (job.restoreJob) to fold the result into a
// single commit before the app restarts, rather than performing a git operation here in a
// package that has no business owning commit semantics.
func (docsBackup) Backup(destDir string) error {
	unlock := lockDocsOp()
	defer unlock()
	return backup.BackupFile(pathutils.DocsRoot(), destDir)
}

func (docsBackup) Restore(srcDir string) error {
	unlock := lockDocsOp()
	defer unlock()
	return backup.RestoreFile(srcDir, pathutils.DocsRoot())
}

func (docsBackup) GetBackendType() string { return "file" }

type mediaBackup struct{}

// Backup/Restore hold mediaOpMu - see docsBackup's Backup/Restore.
func (mediaBackup) Backup(destDir string) error {
	unlock := lockMediaOp()
	defer unlock()
	return backup.BackupFile(pathutils.MediaRoot(), destDir)
}

func (mediaBackup) Restore(srcDir string) error {
	unlock := lockMediaOp()
	defer unlock()
	return backup.RestoreFile(srcDir, pathutils.MediaRoot())
}

func (mediaBackup) GetBackendType() string { return "file" }
