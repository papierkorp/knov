package files

import (
	"os"
	"path/filepath"

	"knov/internal/logging"
	"knov/internal/pathutils"
)

// reservedDocsFolders are the top-level docs folders whose files were keyed as another file before
// docs paths carried their docs/ prefix: docs/media/x.md as the media file media/x.md, docs/docs/x.md
// and docs/files/x.md as docs/x.md.
var reservedDocsFolders = []string{"docs", "media", "files"}

// MigrateReservedFolderMetadata moves the metadata of the docs files in docs/docs/, docs/media/ and
// docs/files/ from the key they were stored under before (the path without the docs/ prefix) to
// their own docs/ key, when no file lives at that old key anymore - otherwise the record belongs
// to the media or docs file of that name, and the docs file starts with a fresh one. Safe to run
// again. Returns the number of moved records.
func MigrateReservedFolderMetadata() int {
	moved := 0
	walkReservedFolders(func(oldKey, newKey string) {
		if fileExists(pathutils.ToFullPath(oldKey)) {
			return
		}
		if old, _ := MetaDataGet(pathutils.GuessMeta(oldKey)); old == nil {
			return
		}
		if existing, _ := MetaDataGet(pathutils.GuessMeta(newKey)); existing != nil {
			return
		}
		if err := moveFileMetadata(logging.KeyApp, oldKey, newKey); err != nil {
			logging.LogWarning(logging.KeyApp, "failed to migrate metadata %s -> %s: %v", oldKey, newKey, err)
			return
		}
		moved++
	})
	if moved > 0 {
		RefreshCaches()
		logging.LogInfo(logging.KeyApp, "migrated the metadata of %d docs files in docs/docs, docs/media and docs/files", moved)
	}
	return moved
}

// walkReservedFolders calls fn with the legacy key (the path without the docs/ prefix) and the own
// docs/ key of every docs file in docs/docs/, docs/media/ and docs/files/.
func walkReservedFolders(fn func(oldKey, newKey string)) {
	for _, top := range reservedDocsFolders {
		_ = filepath.Walk(filepath.Join(pathutils.DocsRoot(), top), func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			newKey := pathutils.ToWithPrefix(p)
			rel, err := filepath.Rel(pathutils.DocsRoot(), p)
			if err != nil {
				return nil
			}
			if oldKey := pathutils.ToWithPrefix(pathutils.ToSlash(rel)); oldKey != newKey {
				fn(oldKey, newKey)
			}
			return nil
		})
	}
}
