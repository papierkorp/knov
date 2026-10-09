// Package utils provides file copy utilities
package utils

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
)

// WriteFileAtomic writes data to a temp file in the same directory as path,
// then renames it into place, so readers never observe a partially-written file.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := renameReplace(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}

// renameReplace is os.Rename, retried for a moment on windows: replacing a file fails there with a
// sharing violation while anyone has it open - a reader of the old content, a virus scanner on
// the new one - and that ends within milliseconds. Other hosts replace at once.
func renameReplace(from, to string) error {
	err := os.Rename(from, to)
	for i := 1; i <= 20 && err != nil && runtime.GOOS == "windows" && IsSharingViolation(err); i++ {
		time.Sleep(time.Duration(i) * 5 * time.Millisecond)
		err = os.Rename(from, to)
	}
	return err
}

// IsSharingViolation reports whether err is windows refusing to open or replace a file that is
// open elsewhere ("access denied" or ERROR_SHARING_VIOLATION); false on every other host.
func IsSharingViolation(err error) bool {
	const errorSharingViolation = syscall.Errno(32)
	return runtime.GOOS == "windows" && (errors.Is(err, fs.ErrPermission) || errors.Is(err, errorSharingViolation))
}
