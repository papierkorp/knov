// Package searchStorage provides search storage functionality with FTS support
package searchStorage

import (
	"fmt"

	"knov/internal/logging"
)

// SearchStorage interface defines methods for search storage with FTS capabilities
type SearchStorage interface {
	IndexFile(path string, content []byte) error
	GetIndexedContent(path string) ([]byte, error)
	DeleteIndexedContent(path string) error
	ListAllIndexedFiles() ([]string, error)
	SearchContent(query string, limit int) ([]SearchResult, error)
	GetBackendType() string
}

// SearchResult represents a search result
type SearchResult struct {
	Path    string
	Content []byte
	Score   float64
}

var storage SearchStorage

// Init initializes search storage with the specified provider
func Init(provider, storagePath string) error {
	var err error

	switch provider {
	case "sqlite":
		storage, err = newSQLiteStorage(storagePath)
	default:
		logging.LogWarning("unknown search storage provider '%s', using sqlite", provider)
		storage, err = newSQLiteStorage(storagePath)
	}

	if err != nil {
		return fmt.Errorf("failed to initialize search storage: %w", err)
	}

	logging.LogInfo("search storage initialized: %s", provider)
	return nil
}

// IndexFile indexes a file's content for search
func IndexFile(path string, content []byte) error {
	return storage.IndexFile(path, content)
}

// GetIndexedContent retrieves indexed content for a file
func GetIndexedContent(path string) ([]byte, error) {
	return storage.GetIndexedContent(path)
}

// DeleteIndexedContent removes indexed content for a file
func DeleteIndexedContent(path string) error {
	return storage.DeleteIndexedContent(path)
}

// ListAllIndexedFiles returns all indexed file paths
func ListAllIndexedFiles() ([]string, error) {
	return storage.ListAllIndexedFiles()
}

// SearchContent performs full-text search
func SearchContent(query string, limit int) ([]SearchResult, error) {
	return storage.SearchContent(query, limit)
}

// GetBackendType returns the backend type
func GetBackendType() string {
	return storage.GetBackendType()
}
