// Package repository ..
package repository

import (
	"knov/internal/logging"
	"knov/internal/storage"
)

// SearchRepository handles search operations
type SearchRepository struct {
	cacheStorage *storage.StorageManager
}

// IndexFile indexes a file's content for search (stores raw bytes)
func (r *SearchRepository) IndexFile(path string, content []byte) error {
	cacheKey := "search_content/" + path
	if err := r.cacheStorage.Set(cacheKey, content); err != nil {
		logging.LogError("failed to index file %s: %v", path, err)
		return err
	}
	logging.LogDebug("indexed file: %s", path)
	return nil
}

// GetIndexedContent retrieves indexed content for a file
func (r *SearchRepository) GetIndexedContent(path string) ([]byte, error) {
	cacheKey := "search_content/" + path
	data, err := r.cacheStorage.Get(cacheKey)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// DeleteIndexedContent removes indexed content for a file
func (r *SearchRepository) DeleteIndexedContent(path string) error {
	cacheKey := "search_content/" + path
	return r.cacheStorage.Delete(cacheKey)
}

// ListAllIndexedFiles returns all indexed file paths
func (r *SearchRepository) ListAllIndexedFiles() ([]string, error) {
	keys, err := r.cacheStorage.List("search_content/")
	if err != nil {
		return nil, err
	}

	// remove prefix from keys
	var paths []string
	for _, key := range keys {
		if len(key) > 15 { // len("search_content/")
			paths = append(paths, key[15:])
		}
	}

	return paths, nil
}
