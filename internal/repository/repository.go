// Package repository provides data access layer using storage backends
package repository

import (
	"knov/internal/logging"
	"knov/internal/storage"
)

// Global repository instances
var (
	fileRepo   *FileRepository
	searchRepo *SearchRepository
)

// InitRepositories initializes all repositories
// Must be called after storage.InitStorages()
func InitRepositories() {
	fileRepo = &FileRepository{
		metadataStorage: storage.GetMetadataStorage(),
		cacheStorage:    storage.GetCacheStorage(),
	}

	searchRepo = &SearchRepository{
		cacheStorage: storage.GetCacheStorage(),
	}

	logging.LogInfo("repositories initialized")
}

// GetFileRepository returns the global file repository instance
func GetFileRepository() *FileRepository {
	return fileRepo
}

// GetSearchRepository returns the global search repository instance
func GetSearchRepository() *SearchRepository {
	return searchRepo
}
