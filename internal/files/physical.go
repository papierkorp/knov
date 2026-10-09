package files

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"knov/internal/configmanager"
	"knov/internal/logging"
	"knov/internal/pathutils"
)

// ErrMoveSourceMissing is returned by MoveFileNoRefresh when oldRelPath doesn't exist.
var ErrMoveSourceMissing = errors.New("source file does not exist")

// ErrMoveTargetExists is returned by MoveFileNoRefresh when newRelPath is already taken.
var ErrMoveTargetExists = errors.New("target file already exists")

// ErrLinkUpdateFailed wraps a failure from updating other files' links after the move itself
// already succeeded on disk - callers may treat it as non-fatal instead of reporting the whole
// move as failed.
var ErrLinkUpdateFailed = errors.New("failed to update links after move")

// OnFileMoved is called after a doc file's on-disk location changes, with its docs-relative old and
// new paths (the docs/ prefix of the metadata path MoveFileNoRefresh/MoveFolder take dropped). Packages that
// keep their own stored reference to a file's path (kanban board order, dashboard widgets)
// register here at startup (see main.go) to patch that reference instead of silently going
// stale - files can't import them directly without an import cycle (kanban already imports
// files). Fired once the physical move has actually succeeded, regardless of whether the
// subsequent link-content update below also succeeds.
var OnFileMoved func(oldRel, newRel pathutils.DocsRel)

// ErrNotDocsFile is returned by MoveFileNoRefresh for a path that is no docs file (a media path).
var ErrNotDocsFile = errors.New("not a docs file")

func notifyFileMoved(oldPath, newPath pathutils.MetaPath) {
	oldRel, oldOK := oldPath.DocsRel()
	newRel, newOK := newPath.DocsRel()
	if OnFileMoved != nil && oldOK && newOK {
		OnFileMoved(oldRel, newRel)
	}
}

// movePhysical performs just the on-disk rename of a single file under the root's physical-op
// lock (docs or media, per isMedia) - the lock already serializes every physical mutation of
// that root, so the stat-then-rename below can't race a concurrent mover in this process. Kept
// to just the syscalls themselves, mirroring moveFolderPhysical, so a caller's link/metadata
// update pass afterward doesn't hold the lock.
func movePhysical(oldFullPath, newFullPath string, isMedia bool) error {
	var unlock func()
	if isMedia {
		unlock = lockMediaOp()
	} else {
		unlock = lockDocsOp()
	}
	defer unlock()

	if _, err := os.Stat(oldFullPath); os.IsNotExist(err) {
		return ErrMoveSourceMissing
	}
	if _, err := os.Stat(newFullPath); err == nil {
		return ErrMoveTargetExists
	}
	if err := os.MkdirAll(filepath.Dir(newFullPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	if err := os.Rename(oldFullPath, newFullPath); err != nil {
		return fmt.Errorf("failed to move file: %w", err)
	}
	return nil
}

// moveDocsToMedia is movePhysical for a docs file moving into the media root - it holds the docs
// lock too, since it removes a docs file (docs before media, same order as git.CommitAllPending).
func moveDocsToMedia(oldFullPath, newFullPath string) error {
	unlock := lockDocsOp()
	defer unlock()
	return movePhysical(oldFullPath, newFullPath, true)
}

// MoveFileNoRefresh moves a single doc file from oldPath to newPath on disk and updates
// the links of every file that referenced it. For refreshing the aggregate caches afterwards,
// call RefreshCaches once - not on every call, same reasoning as MoveFolder.
func MoveFileNoRefresh(key logging.Key, oldPath, newPath pathutils.MetaPath) error {
	if oldPath.IsMedia() || newPath.IsMedia() {
		return ErrNotDocsFile
	}
	if err := pathutils.CheckTarget(oldPath.FullPath(), newPath.FullPath()); err != nil {
		return err
	}
	if err := movePhysical(oldPath.FullPath(), newPath.FullPath(), false); err != nil {
		return err
	}
	notifyFileMoved(oldPath, newPath)
	if err := UpdateLinksForMovedFileNoRefresh(key, oldPath.String(), newPath.String()); err != nil {
		return fmt.Errorf("%w: %v", ErrLinkUpdateFailed, err)
	}
	return nil
}

// MoveMediaFileNoRefresh moves a single media file from oldRelPath to newRelPath on disk,
// updates the links of every doc file that referenced it, and moves its metadata. For
// refreshing the aggregate caches afterwards, call RefreshCaches once - not on every call, same
// reasoning as MoveFileNoRefresh. The single on-disk rename runs under the media physical-op
// lock (via movePhysical); the link scan and metadata move don't touch the physical file, so
// they run outside it - same reasoning as MoveFolder not holding the lock for its link-update
// pass.
func MoveMediaFileNoRefresh(oldRelPath, newRelPath string) error {
	oldMediaPath := "media/" + oldRelPath
	newMediaPath := "media/" + newRelPath

	if err := pathutils.CheckTarget(pathutils.ToMediaPath(oldRelPath), pathutils.ToMediaPath(newRelPath)); err != nil {
		return err
	}
	if err := movePhysical(pathutils.ToMediaPath(oldRelPath), pathutils.ToMediaPath(newRelPath), true); err != nil {
		return err
	}

	// update links in docs before moving metadata so LinksToHere is still readable - doesn't
	// touch the physical file, so a failure here is soft: reported below, but doesn't block the
	// move itself. Runs only after the physical move has actually succeeded, so a failed move
	// (source missing, target taken) never leaves docs pointing at a media file that isn't there.
	linkErr := UpdateLinksForMovedMedia(oldMediaPath, newMediaPath)

	metaErr := MoveMediaMetadata(oldMediaPath, newMediaPath)

	if linkErr != nil || metaErr != nil {
		return errors.Join(ErrLinkUpdateFailed, linkErr, metaErr)
	}
	return nil
}

// DeleteFileNoRefresh removes a single docs file from disk under the docs physical-op lock -
// kept to just the syscall itself so a large BulkDeleteFiles batch doesn't hold the lock for the
// whole batch, only for each individual delete. For refreshing the aggregate caches afterwards,
// call RefreshCaches once, same reasoning as MoveFileNoRefresh. Returns os.Remove's error
// unwrapped - callers rely on os.IsNotExist(err) working directly on it.
func DeleteFileNoRefresh(fullPath string) error {
	unlock := lockDocsOp()
	defer unlock()
	return os.Remove(fullPath)
}

// ListFilesInFolder returns the full path of every regular file recursively inside fullPath.
// Used to snapshot a folder's contents once before a delete, so an operation resumed after a
// crash deletes exactly that snapshot rather than re-walking (and possibly picking up files
// added to the folder in the meantime).
func ListFilesInFolder(fullPath string) ([]string, error) {
	var out []string
	err := filepath.Walk(fullPath, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		out = append(out, p)
		return nil
	})
	return out, err
}

// RemoveEmptyDirTree removes fullPath and its subdirectories bottom-up, but only the ones left
// empty by a prior delete - unlike os.RemoveAll, it never deletes a file, so anything written
// into the tree after the delete's file snapshot was taken survives. Returns an error naming a
// directory that still has content instead of removing it. Only the final removal of each
// directory runs under the docs physical-op lock - the os.ReadDir passes are read-only, so
// pruning a large tree doesn't hold the lock for the whole walk.
func RemoveEmptyDirTree(fullPath string) error {
	entries, err := os.ReadDir(fullPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			return fmt.Errorf("%s is not empty: %s", fullPath, entry.Name())
		}
		if err := RemoveEmptyDirTree(filepath.Join(fullPath, entry.Name())); err != nil {
			return err
		}
	}
	return removeDirPhysical(fullPath)
}

// emptyFolderGracePeriod keeps recently modified folders out of RemoveEmptyFolders, so a folder
// that was just created (by a git pull, a file write or the user) isn't removed before its
// content lands.
const emptyFolderGracePeriod = 10 * time.Minute

// RemoveEmptyFolders removes every empty folder below the docs root bottom-up, keeping the
// root itself, dot folders (e.g. syncthing's .stfolder marker), configured kanban board folders
// (otherwise flagged as missing) and their status folders, and folders modified within
// emptyFolderGracePeriod. A folder that gets content between the read and the removal just
// survives, and an unreadable folder is skipped (its error returned) without stopping the rest of
// the walk. Unlike RemoveEmptyDirTree, which refuses a non-empty tree, this leaves non-empty
// folders alone.
func RemoveEmptyFolders() error {
	var keep []string
	for _, b := range configmanager.GetKanbanBoards() {
		keep = append(keep, pathutils.ToDocsPath(b.FolderPath))
	}
	_, err := removeEmptySubdirs(pathutils.DocsRoot(), time.Now().Add(-emptyFolderGracePeriod), keep)
	return err
}

// removeEmptySubdirs prunes the empty subdirectories of dir last modified before cutoff (except
// the ones in keep and their direct children) and reports whether dir is empty afterwards.
func removeEmptySubdirs(dir string, cutoff time.Time, keep []string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	empty := true
	var errs []error
	for _, entry := range entries {
		sub := filepath.Join(dir, entry.Name())
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			empty = false
			continue
		}
		// stat before pruning, since removing its children bumps the folder's mtime
		info, err := entry.Info()
		if err != nil {
			errs = append(errs, err)
			empty = false
			continue
		}
		subEmpty, err := removeEmptySubdirs(sub, cutoff, keep)
		if err != nil {
			errs = append(errs, err)
		}
		if !subEmpty || info.ModTime().After(cutoff) || slices.Contains(keep, sub) || slices.Contains(keep, dir) {
			empty = false
		} else if err := removeDirPhysical(sub); err != nil {
			logging.LogDebug(logging.KeyFileSync, "failed to remove empty folder %s: %v", sub, err)
			empty = false
		}
	}
	return empty, errors.Join(errs...)
}

// removeDirPhysical removes an already-confirmed-empty docs directory under the docs
// physical-op lock.
func removeDirPhysical(fullPath string) error {
	unlock := lockDocsOp()
	defer unlock()
	return os.Remove(fullPath)
}

// MoveFolder moves currentFullPath to newFullPath and updates the links of every file that
// was inside it, then refreshes the aggregate caches once. Returns the number of files whose
// links were updated successfully and the number that failed.
func MoveFolder(key logging.Key, currentFullPath, newFullPath string) (updated, failed int, err error) {
	if err := pathutils.CheckTarget(currentFullPath, newFullPath); err != nil {
		return 0, 0, err
	}
	// collect all files before the move so we can update their links
	var filesToUpdate []struct{ oldMeta, newMeta pathutils.MetaPath }
	_ = filepath.Walk(currentFullPath, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		suffix := strings.TrimPrefix(p, currentFullPath)
		filesToUpdate = append(filesToUpdate, struct{ oldMeta, newMeta pathutils.MetaPath }{pathutils.FromFullPath(p), pathutils.FromFullPath(newFullPath + suffix)})
		return nil
	})

	if err := moveFolderPhysical(currentFullPath, newFullPath); err != nil {
		return 0, 0, err
	}

	movedAlong := make(map[string]string, len(filesToUpdate))
	for _, f := range filesToUpdate {
		movedAlong[f.oldMeta.String()] = f.newMeta.String()
	}
	for _, f := range filesToUpdate {
		notifyFileMoved(f.oldMeta, f.newMeta)
		if err := updateLinksForMovedFile(key, f.oldMeta.String(), f.newMeta.String(), movedAlong); err != nil {
			logging.LogWarning(key, "move-folder: failed to update links for %s -> %s: %v", f.oldMeta, f.newMeta, err)
			failed++
			continue
		}
		updated++
	}
	// a link between two moved files ("./q.md") only reaches the target's linked from once both moved
	for _, f := range filesToUpdate {
		if err := UpdateLinksForSingleFile(f.newMeta.String()); err != nil {
			logging.LogWarning(key, "move-folder: failed to resync links of %s: %v", f.newMeta, err)
		}
	}
	if len(filesToUpdate) > 0 {
		RefreshCaches()
	}
	return updated, failed, nil
}

// moveFolderPhysical performs just the on-disk directory rename under the docs physical-op lock
// - kept as short as possible so MoveFolder's link-update pass over a large folder afterwards
// doesn't hold the lock and stall unrelated docs moves/deletes elsewhere in the app.
func moveFolderPhysical(currentFullPath, newFullPath string) error {
	unlock := lockDocsOp()
	defer unlock()
	if err := os.MkdirAll(filepath.Dir(newFullPath), 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}
	if err := os.Rename(currentFullPath, newFullPath); err != nil {
		return fmt.Errorf("failed to move folder: %w", err)
	}
	return nil
}
