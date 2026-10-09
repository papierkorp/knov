package files

import (
	"context"
	"os"

	"knov/internal/logging"
	"knov/internal/pathutils"
)

// BulkDeleteFiles removes each file in fullPaths from disk and deletes its metadata, then
// refreshes the aggregate caches once for the whole batch. Returns the full paths that are
// now gone - including ones already absent before this call (e.g. a resumed job re-deleting
// its snapshot after a crash) - since it's safe to treat those as deleted too. Skips - with a
// warning - only paths that failed to remove for another reason (permission, locked file),
// which are NOT safe to report as deleted since they may still exist on disk. Stops early if
// ctx is canceled, leaving the remaining paths untouched.
func BulkDeleteFiles(ctx context.Context, key logging.Key, fullPaths []string, report func(done, total int)) []string {
	var deleted []string
	for i, fullPath := range fullPaths {
		if ctx.Err() != nil {
			break
		}
		if err := DeleteFileNoRefresh(fullPath); err != nil && !os.IsNotExist(err) {
			logging.LogWarning(key, "bulk-delete-files: failed to delete %s: %v", fullPath, err)
		} else {
			if err := MetaDataDeleteNoRefresh(key, pathutils.GuessMeta(fullPath)); err != nil {
				logging.LogWarning(key, "bulk-delete-files: failed to delete metadata for %s: %v", fullPath, err)
			}
			deleted = append(deleted, fullPath)
		}
		// report after the item is handled, so the bar reaches N/N only once the work is done
		if report != nil {
			report(i+1, len(fullPaths))
		}
	}
	if len(deleted) > 0 {
		RefreshCaches()
	}
	return deleted
}

// BulkUpdatePatch describes a metadata patch applied to many files at once.
type BulkUpdatePatch struct {
	Editor     *EditorType
	TagsAdd    []string
	TagsRemove []string
}

// BulkUpdateMetadata applies patch to every file in matched, then refreshes the aggregate
// caches once. Returns the number of files updated and the number that failed.
func BulkUpdateMetadata(key logging.Key, matched []File, patch BulkUpdatePatch, report func(done, total int)) (updated, failed int) {
	for i, f := range matched {
		if err := applyBulkUpdatePatch(f.Metadata, patch); err != nil {
			logging.LogError(key, "bulk-update-metadata: failed to save %s: %v", f.Metadata.Path, err)
			failed++
		}
		// report after the item is handled, so the bar reaches N/N only once the work is done
		if report != nil {
			report(i+1, len(matched))
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
		return SetEditorNoRefresh(current.Path.String(), *p.Editor)
	}
	if len(p.TagsAdd) > 0 || len(p.TagsRemove) > 0 {
		// PatchTagsNoRefresh re-reads tags under the path lock - never apply add/remove against
		// the unlocked snapshot in current (that lost concurrent MoveCard/bulk edits).
		_, err := PatchTagsNoRefresh(current.Path.String(), p.TagsAdd, p.TagsRemove)
		return err
	}
	return nil
}
