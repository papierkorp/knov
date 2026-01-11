// Package contentStorage - Filesystem storage implementation
package contentStorage

import (
	"os"
	"path/filepath"
)

// FilesystemStorage implements ContentStorage for local filesystem
type FilesystemStorage struct {
	basePath  string
	docsPath  string
	mediaPath string
	gitPath   string
}

// NewFilesystemStorage creates a new filesystem storage
func NewFilesystemStorage(basePath string) *FilesystemStorage {
	return &FilesystemStorage{
		basePath:  basePath,
		docsPath:  filepath.Join(basePath, "docs"),
		mediaPath: filepath.Join(basePath, "media"),
		gitPath:   filepath.Join(basePath, ".git"),
	}
}

// ReadFile reads content from a file
func (fs *FilesystemStorage) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// WriteFile writes content to a file
func (fs *FilesystemStorage) WriteFile(path string, data []byte, perm os.FileMode) error {
	// ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, perm)
}

// DeleteFile removes a file
func (fs *FilesystemStorage) DeleteFile(path string) error {
	return os.Remove(path)
}

// FileExists checks if a file exists
func (fs *FilesystemStorage) FileExists(path string) (bool, error) {
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
func (fs *FilesystemStorage) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// ListFiles lists all files recursively
func (fs *FilesystemStorage) ListFiles() ([]string, error) {
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

// GetDocsPath returns the docs directory path
func (fs *FilesystemStorage) GetDocsPath() string {
	return fs.docsPath
}

// GetMediaPath returns the media directory path
func (fs *FilesystemStorage) GetMediaPath() string {
	return fs.mediaPath
}

// GetGitPath returns the git directory path
func (fs *FilesystemStorage) GetGitPath() string {
	return fs.gitPath
}
