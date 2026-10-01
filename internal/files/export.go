package files

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"knov/internal/configmanager"
	"knov/internal/logging"
	"knov/internal/pathutils"
)

// ExportData passes every file of the data folder (except .git) to add, named by its
// forward-slash path relative to the data folder. The storage folder is skipped in case it is
// configured inside the data folder, so earlier exports are never exported again. Files that
// can't be opened are skipped, an unreadable folder or a read error mid-file fails the export.
// Each file is streamed to add, so large media never has to fit into memory. ctx cancels
// between files.
func ExportData(ctx context.Context, add func(name string, modified time.Time, r io.Reader) error) error {
	dataPath := configmanager.GetAppConfig().DataPath
	storage, _ := os.Stat(configmanager.GetAppConfig().StoragePath)

	return filepath.WalkDir(dataPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			if info, err := d.Info(); err == nil && storage != nil && os.SameFile(info, storage) {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, err := filepath.Rel(dataPath, path)
		if err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			logging.LogWarning(logging.KeyExport, "export: skip %s (open failed): %v", relPath, err)
			return nil
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			logging.LogWarning(logging.KeyExport, "export: skip %s (stat failed): %v", relPath, err)
			return nil
		}
		return add(pathutils.ToSlash(relPath), info.ModTime(), f)
	})
}
