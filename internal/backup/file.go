package backup

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"knov/internal/utils"
)

// BackupFile recursively copies srcDir's file tree into destDir, preserving relative paths.
// Used by config/metadata/cache storages whose data is a directory of files (json today) rather
// than a single database - kept generic (not JSON-specific) since it operates at the file-tree
// level, not the encoding level.
func BackupFile(srcDir, destDir string) error {
	return copyTree(srcDir, destDir)
}

// RestoreFile replaces destDir's contents with a previously backed-up snapshot from srcDir.
// destDir is cleared first - copyTree only ever adds/overwrites files, so without this, files
// created or renamed after the backup was taken would silently survive a "restore".
func RestoreFile(srcDir, destDir string) error {
	if err := os.RemoveAll(destDir); err != nil {
		return fmt.Errorf("failed to clear %s before restore: %w", destDir, err)
	}
	return copyTree(srcDir, destDir)
}

// copyTree copies srcDir's file tree onto destDir file-by-file via WriteFileAtomic, so a restore
// interrupted mid-copy (crash, disk full) can't leave a live file torn.
func copyTree(srcDir, destDir string) error {
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return nil // nothing stored yet
	}
	return filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destDir, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return utils.WriteFileAtomic(target, data, info.Mode().Perm())
	})
}
