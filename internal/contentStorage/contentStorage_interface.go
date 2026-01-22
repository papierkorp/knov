// Package contentStorage provides content storage functionality
package contentStorage

import (
	"fmt"
	"os"

	"knov/internal/configmanager"
	"knov/internal/logging"
)

// ContentStorage interface defines methods for content storage
type ContentStorage interface {
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte, perm os.FileMode) error
	DeleteFile(path string) error
	FileExists(path string) (bool, error)
	MkdirAll(path string, perm os.FileMode) error
	ListFiles() ([]string, error)
	ListMediaFiles() ([]string, error)
	GetFileInfo(path string) (os.FileInfo, error)
	GetDocsPath() string
	GetMediaPath() string
	GetGitPath() string
	GetBackendType() string
}

var storage ContentStorage

// Init initializes content storage with the specified provider
func Init() error {
	dataPath := configmanager.GetAppConfig().DataPath

	// for now, only filesystem provider is supported
	var err error
	storage, err = newFilesystemStorage(dataPath)
	if err != nil {
		return fmt.Errorf("failed to initialize content storage: %w", err)
	}

	logging.LogInfo("content storage initialized: filesystem")
	return nil
}

// ReadFile reads content from a file
func ReadFile(path string) ([]byte, error) {
	return storage.ReadFile(path)
}

// WriteFile writes content to a file
func WriteFile(path string, data []byte, perm os.FileMode) error {
	return storage.WriteFile(path, data, perm)
}

// DeleteFile removes a file
func DeleteFile(path string) error {
	return storage.DeleteFile(path)
}

// FileExists checks if a file exists
func FileExists(path string) (bool, error) {
	return storage.FileExists(path)
}

// MkdirAll creates a directory path
func MkdirAll(path string, perm os.FileMode) error {
	return storage.MkdirAll(path, perm)
}

// ListFiles lists all files recursively
func ListFiles() ([]string, error) {
	return storage.ListFiles()
}

// ListMediaFiles lists all media files recursively
func ListMediaFiles() ([]string, error) {
	return storage.ListMediaFiles()
}

// GetFileInfo returns file information for the given path
func GetFileInfo(path string) (os.FileInfo, error) {
	return storage.GetFileInfo(path)
}

// GetBackendType returns the backend type
func GetBackendType() string {
	return storage.GetBackendType()
}

// GetGitPath returns the full path to git directory
func GetGitPath() string {
	return storage.GetGitPath()
}

// GetDocsPath returns the full path to docs directory
func GetDocsPath() string {
	return storage.GetDocsPath()
}

// GetMediaPath returns the full path to media directory
func GetMediaPath() string {
	return storage.GetMediaPath()
}
