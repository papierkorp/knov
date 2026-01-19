// Package contentStorage - Filesystem storage implementation
package contentStorage

import (
	"fmt"
	"os"
	"path/filepath"

	"knov/internal/logging"
)

// filesystemStorage implements ContentStorage for local filesystem
type filesystemStorage struct {
	basePath  string
	docsPath  string
	mediaPath string
	gitPath   string
}

// newFilesystemStorage creates a new filesystem storage
func newFilesystemStorage(basePath string) (*filesystemStorage, error) {
	fs := &filesystemStorage{
		basePath:  basePath,
		docsPath:  filepath.Join(basePath, "docs"),
		mediaPath: filepath.Join(basePath, "media"),
		gitPath:   filepath.Join(basePath, ".git"),
	}

	// initialize directories
	if err := fs.initialize(); err != nil {
		return nil, err
	}

	return fs, nil
}

// initialize creates content storage directories
func (fs *filesystemStorage) initialize() error {
	// create main data directory
	if err := os.MkdirAll(fs.basePath, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// create docs subdirectory
	if err := os.MkdirAll(fs.docsPath, 0755); err != nil {
		return fmt.Errorf("failed to create docs directory: %w", err)
	}

	// create media subdirectory
	if err := os.MkdirAll(fs.mediaPath, 0755); err != nil {
		return fmt.Errorf("failed to create media directory: %w", err)
	}

	logging.LogDebug("content storage directories initialized")
	return nil
}

// ReadFile reads content from a file
func (fs *filesystemStorage) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// WriteFile writes content to a file
func (fs *filesystemStorage) WriteFile(path string, data []byte, perm os.FileMode) error {
	// ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, perm)
}

// DeleteFile removes a file
func (fs *filesystemStorage) DeleteFile(path string) error {
	return os.Remove(path)
}

// FileExists checks if a file exists
func (fs *filesystemStorage) FileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// MkdirAll creates a directory path
func (fs *filesystemStorage) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// ListFiles lists all files recursively
func (fs *filesystemStorage) ListFiles() ([]string, error) {
	var files []string

	err := filepath.Walk(fs.docsPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// skip .git directory
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		if !info.IsDir() {
			// get relative path from docs directory
			relPath, err := filepath.Rel(fs.docsPath, path)
			if err != nil {
				return err
			}
			files = append(files, relPath)
		}
		return nil
	})

	return files, err
}

// ListMediaFiles lists all media files recursively
func (fs *filesystemStorage) ListMediaFiles() ([]string, error) {
	var files []string

	// check if media directory exists
	if _, err := os.Stat(fs.mediaPath); os.IsNotExist(err) {
		return files, nil // return empty slice, not error
	}

	err := filepath.Walk(fs.mediaPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil // skip directories
		}

		// get relative path from media directory
		relPath, err := filepath.Rel(fs.mediaPath, path)
		if err != nil {
			return err
		}
		files = append(files, relPath)
		return nil
	})

	return files, err
}

// GetFileInfo returns file information for the given path
func (fs *filesystemStorage) GetFileInfo(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

// GetDocsPath returns the docs directory path
func (fs *filesystemStorage) GetDocsPath() string {
	return fs.docsPath
}

// GetMediaPath returns the media directory path
func (fs *filesystemStorage) GetMediaPath() string {
	return fs.mediaPath
}

// GetGitPath returns the git directory path
func (fs *filesystemStorage) GetGitPath() string {
	return fs.gitPath
}

// GetBackendType returns the backend type
func (fs *filesystemStorage) GetBackendType() string {
	return "filesystem"
}
