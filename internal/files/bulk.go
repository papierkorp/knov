package files

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"knov/internal/logging"
	"knov/internal/pathutils"
)

// BulkDeleteFiles removes each file in fullPaths from disk and deletes its metadata, then
// refreshes the aggregate caches once for the whole batch. Returns the full paths that were
// actually deleted (skips - with a warning - any that failed to remove).
func BulkDeleteFiles(key logging.Key, fullPaths []string) []string {
	var deleted []string
	for _, fullPath := range fullPaths {
		if err := os.Remove(fullPath); err != nil {
			logging.LogWarning(key, "bulk-delete-files: failed to delete %s: %v", fullPath, err)
			continue
		}
		if err := MetaDataDeleteNoRefresh(key, pathutils.ToRelative(fullPath)); err != nil {
			logging.LogWarning(key, "bulk-delete-files: failed to delete metadata for %s: %v", fullPath, err)
		}
		deleted = append(deleted, fullPath)
	}
	if len(deleted) > 0 {
		RefreshCaches()
	}
	return deleted
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

// MoveFolder moves currentFullPath to newFullPath and updates the links of every file that
// was inside it, then refreshes the aggregate caches once. Returns the number of files whose
// links were updated successfully and the number that failed.
func MoveFolder(key logging.Key, currentFullPath, newFullPath string) (updated, failed int, err error) {
	// collect all files before the move so we can update their links
	var filesToUpdate []struct{ oldRel, newRel string }
	_ = filepath.Walk(currentFullPath, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		oldRel := pathutils.ToRelative(p)
		suffix := strings.TrimPrefix(p, currentFullPath)
		newRel := pathutils.ToRelative(newFullPath + suffix)
		filesToUpdate = append(filesToUpdate, struct{ oldRel, newRel string }{oldRel, newRel})
		return nil
	})

	if err := os.MkdirAll(filepath.Dir(newFullPath), 0755); err != nil {
		return 0, 0, fmt.Errorf("failed to create parent directory: %w", err)
	}
	if err := os.Rename(currentFullPath, newFullPath); err != nil {
		return 0, 0, fmt.Errorf("failed to move folder: %w", err)
	}

	for _, f := range filesToUpdate {
		if err := UpdateLinksForMovedFileNoRefresh(key, f.oldRel, f.newRel); err != nil {
			logging.LogWarning(key, "move-folder: failed to update links for %s -> %s: %v", f.oldRel, f.newRel, err)
			failed++
			continue
		}
		updated++
	}
	if len(filesToUpdate) > 0 {
		RefreshCaches()
	}
	return updated, failed, nil
}

// BulkUpdatePatch describes a metadata patch applied to many files at once.
type BulkUpdatePatch struct {
	Editor     *EditorType
	TagsAdd    []string
	TagsRemove []string
}

// BulkUpdateMetadata applies patch to every file in matched, then refreshes the aggregate
// caches once. Returns the number of files updated and the number that failed.
func BulkUpdateMetadata(key logging.Key, matched []File, patch BulkUpdatePatch) (updated, failed int) {
	for _, f := range matched {
		if err := applyBulkUpdatePatch(f.Metadata, patch); err != nil {
			logging.LogError(key, "bulk-update-metadata: failed to save %s: %v", f.Metadata.Path, err)
			failed++
		}
	}
	RefreshCaches()
	return len(matched) - failed, failed
}

// applyBulkUpdatePatch applies a single bulk metadata patch to one file. Uses the NoRefresh
// setters so a bulk update over many files rebuilds the aggregate caches once after the loop
// instead of once per file.
func applyBulkUpdatePatch(current *Metadata, p BulkUpdatePatch) error {
	if p.Editor != nil {
		return SetEditorNoRefresh(current.Path, *p.Editor)
	}
	if len(p.TagsAdd) > 0 || len(p.TagsRemove) > 0 {
		// PatchTagsNoRefresh re-reads tags under the path lock - never apply add/remove against
		// the unlocked snapshot in current (that lost concurrent MoveCard/bulk edits).
		_, err := PatchTagsNoRefresh(current.Path, p.TagsAdd, p.TagsRemove)
		return err
	}
	return nil
}
