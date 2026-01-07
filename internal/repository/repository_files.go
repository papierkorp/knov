// Package repository ..
package repository

import (
	"encoding/json"
	"fmt"
	"knov/internal/logging"
	"knov/internal/storage"
)

// FileRepository handles file metadata operations
type FileRepository struct {
	metadataStorage *storage.StorageManager
	cacheStorage    *storage.StorageManager
}

// SaveToCache saves data to cache with JSON marshaling
func (r *FileRepository) SaveToCache(key string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		logging.LogError("failed to marshal cache data for key %s: %v", key, err)
		return err
	}

	return r.cacheStorage.Set(key, data)
}

// GetFromCache retrieves data from cache with JSON unmarshaling
func (r *FileRepository) GetFromCache(key string, target interface{}) error {
	data, err := r.cacheStorage.Get(key)
	if err != nil {
		return err
	}

	if data == nil {
		return fmt.Errorf("key not found: %s", key)
	}

	return json.Unmarshal(data, target)
}

// CacheExists checks if a cache key exists
func (r *FileRepository) CacheExists(key string) bool {
	return r.cacheStorage.Exists(key)
}

// GetMetadata retrieves raw metadata bytes for a file
func (r *FileRepository) GetMetadata(path string) ([]byte, error) {
	data, err := r.metadataStorage.Get(path)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// SaveMetadata saves raw metadata bytes for a file
func (r *FileRepository) SaveMetadata(path string, data []byte) error {
	return r.metadataStorage.Set(path, data)
}

// DeleteMetadata deletes metadata for a file
func (r *FileRepository) DeleteMetadata(path string) error {
	return r.metadataStorage.Delete(path)
}

// GetAllMetadata retrieves all metadata as raw bytes
func (r *FileRepository) GetAllMetadata() (map[string][]byte, error) {
	allData, err := r.metadataStorage.GetAll()
	if err != nil {
		logging.LogError("failed to get all metadata: %v", err)
		return nil, err
	}

	logging.LogDebug("retrieved %d metadata entries", len(allData))
	return allData, nil
}

// MetadataExists checks if metadata exists for a path
func (r *FileRepository) MetadataExists(path string) bool {
	return r.metadataStorage.Exists(path)
}
