package kanban

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/kanbanStorage"
	"knov/internal/logging"
	"knov/internal/pathutils"
)

// RenameResult is the outcome of a RenameStatus run.
type RenameResult struct {
	Folders     int `json:"folders"`     // foldersync status folders renamed
	Retagged    int `json:"retagged"`    // files whose status tag was renamed
	LinksFailed int `json:"linksFailed"` // moved files whose links couldn't be updated
}

// ErrInvalidRename is returned by RenameStatus for a rename that can't be applied as requested
// (invalid or duplicate name, existing target folder) - nothing was changed.
var ErrInvalidRename = errors.New("invalid status rename")

// errRenameIncomplete wraps a failed step - every step is idempotent and the settings change
// last, so running the same rename again finishes it.
func errRenameIncomplete(err error) error {
	return fmt.Errorf("%w - run the rename again to finish it", err)
}

// RenameStatus renames a kanban status everywhere it's stored: the status folders of foldersync
// boards (a folder move, so links and git history follow), the status tag of every file, the
// stored card order of every board, the logged move events and finally the kanban settings.
// Renaming isn't a move, so KanbanMovedAt stays and no event is logged. Until the settings change
// newStatus isn't a status, so the file-sync cronjob ignores the renamed folders and retagged
// files in between. A failed step stops the rename before the settings change. Only concurrent
// renames are blocked - a card moved into oldStatus while the rename runs can keep the old tag.
// A moved file whose metadata couldn't be moved along (counted in LinksFailed) isn't retagged
// here, foldersync retags it from its new folder once the file-sync cronjob picks it up.
func RenameStatus(oldStatus, newStatus string) (RenameResult, error) {
	var result RenameResult
	// validate before touching the filesystem - newStatus becomes part of a path below
	if err := configmanager.ValidateKanbanStatusRename(oldStatus, newStatus); err != nil {
		return result, fmt.Errorf("%w: %v", ErrInvalidRename, err)
	}

	var folders []string
	for _, b := range configmanager.GetKanbanBoards() {
		if !b.FolderSync {
			continue
		}
		boardDir := pathutils.ToDocsPath(b.FolderPath)
		oldInfo, err := os.Stat(filepath.Join(boardDir, oldStatus))
		if err != nil {
			continue
		}
		if newInfo, err := os.Stat(filepath.Join(boardDir, newStatus)); err == nil {
			// case-insensitive filesystem (windows, macos): newStatus only differs in case
			if os.SameFile(oldInfo, newInfo) {
				return result, fmt.Errorf("%w: %s and %s are the same folder on this filesystem - rename to a different name first, then to %s", ErrInvalidRename, oldStatus, newStatus, newStatus)
			}
			return result, fmt.Errorf("%w: folder %s/%s already exists - move its files into %s/%s or remove it first", ErrInvalidRename, b.FolderPath, newStatus, b.FolderPath, oldStatus)
		}
		folders = append(folders, b.FolderPath)
	}

	// move the folders before retagging - until the settings change newStatus isn't a status, so
	// foldersync ignores the renamed folders still holding files tagged oldStatus
	for _, board := range folders {
		boardDir := pathutils.ToDocsPath(board)
		oldDir, newDir := filepath.Join(boardDir, oldStatus), filepath.Join(boardDir, newStatus)
		_, linksFailed, err := files.MoveFolder(logging.KeyApp, oldDir, newDir)
		if err != nil {
			return result, errRenameIncomplete(fmt.Errorf("failed to rename folder %s/%s: %w", board, oldStatus, err))
		}
		result.Folders++
		result.LinksFailed += linksFailed
	}

	allFiles, err := files.GetAllFiles()
	if err != nil {
		return result, errRenameIncomplete(err)
	}
	oldTag, newTag := configmanager.KanbanStatusTag(oldStatus), configmanager.KanbanStatusTag(newStatus)
	failed := 0
	for _, f := range allFiles {
		if f.Metadata == nil || !slices.Contains(f.Metadata.Tags, oldTag) {
			continue
		}
		// a plain tag swap under the path lock, not PatchTags - that would bump KanbanMovedAt
		changed := false
		err := files.MetaDataMutate(f.Metadata.Path, func(m *files.Metadata, existed bool) (bool, error) {
			i := slices.Index(m.Tags, oldTag)
			if !existed || i < 0 {
				return false, nil
			}
			changed = true
			if slices.Contains(m.Tags, newTag) {
				m.Tags = slices.Delete(m.Tags, i, i+1)
			} else {
				m.Tags[i] = newTag
			}
			return true, nil
		})
		if err != nil {
			logging.LogError(logging.KeyApp, "kanban: failed to rename status tag of %s: %v", f.Metadata.Path, err)
			failed++
			continue
		}
		if changed {
			result.Retagged++
		}
	}
	files.RefreshCaches()
	if failed > 0 {
		return result, errRenameIncomplete(fmt.Errorf("failed to rename the status tag of %d files", failed))
	}

	for _, b := range configmanager.GetKanbanBoards() {
		if o, err := GetOrder(b.FolderPath); err != nil || o[oldStatus] == nil {
			continue
		}
		err := MutateOrder(b.FolderPath, func(o Order) {
			o[newStatus] = append(o[newStatus], o[oldStatus]...)
			delete(o, oldStatus)
		})
		if err != nil {
			return result, errRenameIncomplete(fmt.Errorf("failed to rename the status in the card order of board %s: %w", b.FolderPath, err))
		}
	}

	if err := kanbanStorage.RenameStatus(oldStatus, newStatus); err != nil {
		return result, errRenameIncomplete(fmt.Errorf("failed to rename the status in the events: %w", err))
	}

	if err := configmanager.RenameKanbanStatus(oldStatus, newStatus); err != nil {
		return result, errRenameIncomplete(err)
	}
	files.RefreshCaches() // hide files by tag may have changed

	logging.LogInfo(logging.KeyApp, "kanban: renamed status %s to %s", oldStatus, newStatus)
	return result, nil
}
