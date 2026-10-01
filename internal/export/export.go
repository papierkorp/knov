// Package export builds the bulk export zip archives (all files, dokuwiki->markdown, pdf). files and
// markdown are streamed straight to the client (Write), pdf is slow so it is created by a background
// job in StoragePath/export (Create) and kept until it is removed or replaced by the next run. The
// content of each kind comes from the owning package (files, pdfexport, dokuwikiconverter).
package export

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"knov/internal/configmanager"
	"knov/internal/dokuwikiconverter"
	"knov/internal/files"
	"knov/internal/logging"
	"knov/internal/pdfexport"
)

// Kinds of bulk export.
const (
	KindFiles    = "files"
	KindMarkdown = "markdown"
	KindPDF      = "pdf"
)

// compressedExtensions are stored as-is, deflating already compressed files only costs time.
var compressedExtensions = []string{
	".jpg", ".jpeg", ".png", ".gif", ".webp", ".avif", ".heic",
	".mp3", ".ogg", ".m4a", ".mp4", ".mov", ".mkv", ".webm",
	".zip", ".gz", ".7z", ".rar", ".xz", ".bz2",
}

// dir is not a registered backup storage, so exports never end up in backups.
func dir() string {
	return filepath.Join(configmanager.GetAppConfig().StoragePath, "export")
}

// Path returns where the pdf archive is stored.
func Path() string {
	return filepath.Join(dir(), KindPDF+".zip")
}

// Available reports whether a finished pdf archive is waiting to be downloaded.
func Available() bool {
	_, err := os.Stat(Path())
	return err == nil
}

// Remove deletes the pdf archive.
func Remove() error {
	if err := os.Remove(Path()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Create creates the pdf archive and returns how many files were skipped because they failed to
// convert. It is written to a temp file first, so a failed or canceled run keeps the previous
// archive. report receives the progress, ctx cancels between files. Callers must not run Create
// concurrently.
func Create(ctx context.Context, report func(done, total int)) (skipped int, err error) {
	if err := os.MkdirAll(dir(), 0755); err != nil {
		return 0, err
	}
	// temp files of a run killed mid-export (crash, restart) are never cleaned up otherwise
	stale, _ := filepath.Glob(filepath.Join(dir(), "*.tmp"))
	for _, path := range stale {
		os.Remove(path)
	}
	tmp, err := os.CreateTemp(dir(), KindPDF+"-*.tmp")
	if err != nil {
		return 0, err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	skipped, err = Write(ctx, KindPDF, tmp, report)
	if err != nil {
		return skipped, err
	}
	if err := tmp.Close(); err != nil {
		return skipped, err
	}
	// on windows the previous archive can't be replaced while it is being downloaded
	if err := os.Rename(tmp.Name(), Path()); err != nil {
		return skipped, fmt.Errorf("failed to replace the previous pdf export, is it still being downloaded? %w", err)
	}
	return skipped, nil
}

// Write writes the zip archive of kind to w and returns how many files were skipped because they
// failed to convert. report receives the progress (pdf only), ctx cancels between files.
func Write(ctx context.Context, kind string, w io.Writer, report func(done, total int)) (skipped int, err error) {
	zw := zip.NewWriter(w)
	seen := map[string]bool{}
	add := func(name string, modified time.Time, r io.Reader) error {
		// e.g. a.txt converted to markdown collides with an existing a.md
		if seen[name] {
			ext := path.Ext(name)
			unique := name
			for i := 1; seen[unique]; i++ {
				unique = fmt.Sprintf("%s-%d%s", strings.TrimSuffix(name, ext), i, ext)
			}
			logging.LogWarning(logging.KeyExport, "export: duplicate entry %s stored as %s", name, unique)
			name = unique
		}
		seen[name] = true

		method := zip.Deflate
		if slices.Contains(compressedExtensions, strings.ToLower(path.Ext(name))) {
			method = zip.Store
		}
		entry, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: method, Modified: modified})
		if err == nil {
			_, err = io.Copy(entry, r)
		}
		if err != nil {
			return fmt.Errorf("failed to write %s to archive: %w", name, err)
		}
		return nil
	}

	switch kind {
	case KindPDF:
		skipped, err = pdfexport.ExportAll(ctx, add, report)
	case KindMarkdown:
		err = files.ExportData(ctx, func(name string, modified time.Time, r io.Reader) error {
			name, r, err := dokuwikiconverter.ConvertExportEntry(name, r)
			if err != nil {
				return err
			}
			return add(name, modified, r)
		})
	case KindFiles:
		err = files.ExportData(ctx, add)
	default:
		err = fmt.Errorf("unknown export kind: %s", kind)
	}
	if err != nil {
		return skipped, err
	}
	return skipped, zw.Close()
}
