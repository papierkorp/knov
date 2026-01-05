// Package storage ..
package storage

import (
	"encoding/json"
	"fmt"
	"time"

	"knov/internal/logging"
	"knov/internal/types"
)

// ----------------------------------------------------------------------------------------
// ---------------------------------- Base Interface --------------------------------------
// ----------------------------------------------------------------------------------------

// Storage is the base interface that all storage types implement
type Storage interface {
	Get(key string) ([]byte, error)
	GetWithDefault(key string, defaultValue []byte) ([]byte, error)
	Set(key string, data []byte) error
	Delete(key string) error
	List(prefix string) ([]string, error)
	Exists(key string) bool
	Close() error
}

// ----------------------------------------------------------------------------------------
// -------------------------------- Specialized Interfaces --------------------------------
// ----------------------------------------------------------------------------------------

// MetadataStorage handles file metadata storage with specialized operations
type MetadataStorage interface {
	Storage
	Query(criteria []types.Criteria, logic string) ([][]byte, error)
	BulkSet(items map[string][]byte) error
	GetAll() (map[string][]byte, error)
}

// SearchOptions configures search behavior
type SearchOptions struct {
	Limit   int
	Offset  int
	Fields  []string
	Filters map[string]string
}

// SearchResult represents a single search result
type SearchResult struct {
	Key      string
	Score    float64
	Snippet  string
	Metadata map[string]string
}

// SearchResults contains search results and metadata
type SearchResults struct {
	Results []*SearchResult
	Total   int
	Took    time.Duration
}

// CacheStorage handles cached data with search capabilities
type CacheStorage interface {
	Storage
	Search(query string, opts SearchOptions) (*SearchResults, error)
	IndexContent(key string, content []byte, metadata map[string]string) error
	Rebuild() error
	Invalidate(pattern string) error
}

// ----------------------------------------------------------------------------------------
// ---------------------------------- Global Managers -------------------------------------
// ----------------------------------------------------------------------------------------

var (
	globalConfigStorage   Storage
	globalMetadataStorage MetadataStorage
	globalCacheStorage    CacheStorage
)

// Init initializes all storage backends
func Init(configProvider, metadataProvider, cacheProvider, configPath, metadataPath, cachePath string) error {
	var err error

	// initialize config storage
	switch configProvider {
	case "json":
		globalConfigStorage, err = NewJSONConfigStorage(configPath)
	case "sqlite":
		globalConfigStorage, err = NewSQLiteConfigStorage(configPath)
	case "postgres":
		logging.LogWarning("postgres not implemented yet, using json")
		globalConfigStorage, err = NewJSONConfigStorage(configPath)
	default:
		logging.LogWarning("unknown provider '%s', using json", configProvider)
		globalConfigStorage, err = NewJSONConfigStorage(configPath)
	}
	if err != nil {
		return fmt.Errorf("failed to initialize config storage: %w", err)
	}
	logging.LogInfo("initialized config storage: %s at %s", configProvider, configPath)

	// initialize metadata storage
	switch metadataProvider {
	case "json":
		globalMetadataStorage, err = NewJSONMetadataStorage(metadataPath)
	case "sqlite":
		globalMetadataStorage, err = NewSQLiteMetadataStorage(metadataPath)
	case "postgres":
		logging.LogWarning("postgres not implemented yet, using json")
		globalMetadataStorage, err = NewJSONMetadataStorage(metadataPath)
	default:
		logging.LogWarning("unknown provider '%s', using json", metadataProvider)
		globalMetadataStorage, err = NewJSONMetadataStorage(metadataPath)
	}
	if err != nil {
		return fmt.Errorf("failed to initialize metadata storage: %w", err)
	}
	logging.LogInfo("initialized metadata storage: %s at %s", metadataProvider, metadataPath)

	// initialize cache storage
	switch cacheProvider {
	case "json":
		globalCacheStorage, err = NewJSONCacheStorage(cachePath)
	case "sqlite":
		globalCacheStorage, err = NewSQLiteCacheStorage(cachePath)
	case "postgres":
		logging.LogWarning("postgres not implemented yet, using json")
		globalCacheStorage, err = NewJSONCacheStorage(cachePath)
	default:
		logging.LogWarning("unknown provider '%s', using json", cacheProvider)
		globalCacheStorage, err = NewJSONCacheStorage(cachePath)
	}
	if err != nil {
		return fmt.Errorf("failed to initialize cache storage: %w", err)
	}
	logging.LogInfo("initialized cache storage: %s at %s", cacheProvider, cachePath)

	return nil
}

// AutoMigrate performs automatic migration if migration env vars are set
func AutoMigrate(configOldProvider, configOldPath, metadataOldProvider, metadataOldPath, cacheOldProvider, cacheOldPath string) error {
	// check if migration already completed
	migrationData, err := GetSystemData("migration_completed")
	if err == nil && migrationData != nil && migrationData.Value == "true" {
		logging.LogInfo("migration already completed, skipping")
		return nil
	}

	logging.LogInfo("starting automatic storage migration...")

	hasMigrations := false

	// migrate config if old provider specified
	if configOldProvider != "" && configOldPath != "" {
		logging.LogInfo("migrating config storage from %s to current provider...", configOldProvider)

		oldConfig, err := createConfigStorage(configOldProvider, configOldPath)
		if err != nil {
			return fmt.Errorf("failed to open old config storage: %w", err)
		}
		defer oldConfig.Close()

		if err := MigrateConfig(oldConfig, globalConfigStorage); err != nil {
			return fmt.Errorf("config migration failed: %w", err)
		}

		hasMigrations = true
	}

	// migrate metadata if old provider specified
	if metadataOldProvider != "" && metadataOldPath != "" {
		logging.LogInfo("migrating metadata storage from %s to current provider...", metadataOldProvider)

		oldMetadata, err := createMetadataStorage(metadataOldProvider, metadataOldPath)
		if err != nil {
			return fmt.Errorf("failed to open old metadata storage: %w", err)
		}
		defer oldMetadata.Close()

		if err := MigrateMetadata(oldMetadata, globalMetadataStorage); err != nil {
			return fmt.Errorf("metadata migration failed: %w", err)
		}

		hasMigrations = true
	}

	// migrate cache if old provider specified
	if cacheOldProvider != "" && cacheOldPath != "" {
		logging.LogInfo("migrating cache storage from %s to current provider...", cacheOldProvider)

		oldCache, err := createCacheStorage(cacheOldProvider, cacheOldPath)
		if err != nil {
			return fmt.Errorf("failed to open old cache storage: %w", err)
		}
		defer oldCache.Close()

		if err := MigrateCache(oldCache, globalCacheStorage); err != nil {
			return fmt.Errorf("cache migration failed: %w", err)
		}

		hasMigrations = true
	}

	if hasMigrations {
		// mark migration as completed
		if err := SaveSystemData(SystemData{
			Key:   "migration_completed",
			Value: "true",
		}); err != nil {
			logging.LogWarning("failed to save migration status: %v", err)
		}

		logging.LogInfo("automatic migration completed successfully")
		logging.LogInfo("you can now remove KNOV_STORAGE_MIGRATE and KNOV_STORAGE_*_OLD_* env vars")
	} else {
		logging.LogWarning("migration requested but no old storage providers specified")
	}

	return nil
}

// helper functions to create storage instances
func createConfigStorage(provider, path string) (Storage, error) {
	switch provider {
	case "json":
		return NewJSONConfigStorage(path)
	case "sqlite":
		return NewSQLiteConfigStorage(path)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

func createMetadataStorage(provider, path string) (MetadataStorage, error) {
	switch provider {
	case "json":
		return NewJSONMetadataStorage(path)
	case "sqlite":
		return NewSQLiteMetadataStorage(path)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

func createCacheStorage(provider, path string) (CacheStorage, error) {
	switch provider {
	case "json":
		return NewJSONCacheStorage(path)
	case "sqlite":
		return NewSQLiteCacheStorage(path)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

// GetConfigStorage returns the global config storage
func GetConfigStorage() Storage {
	return globalConfigStorage
}

// GetMetadataStorage returns the global metadata storage
func GetMetadataStorage() MetadataStorage {
	return globalMetadataStorage
}

// GetCacheStorage returns the global cache storage
func GetCacheStorage() CacheStorage {
	return globalCacheStorage
}

// Close closes all storage backends
func Close() error {
	var errs []error

	if globalConfigStorage != nil {
		if err := globalConfigStorage.Close(); err != nil {
			errs = append(errs, fmt.Errorf("config storage: %w", err))
		}
	}

	if globalMetadataStorage != nil {
		if err := globalMetadataStorage.Close(); err != nil {
			errs = append(errs, fmt.Errorf("metadata storage: %w", err))
		}
	}

	if globalCacheStorage != nil {
		if err := globalCacheStorage.Close(); err != nil {
			errs = append(errs, fmt.Errorf("cache storage: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing storage: %v", errs)
	}

	return nil
}

// ----------------------------------------------------------------------------------------
// -------------------------------- Helper Functions --------------------------------------
// ----------------------------------------------------------------------------------------

// SystemData represents system metadata stored in cache
type SystemData struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	UpdatedAt int64  `json:"updated_at"`
}

// SaveSystemData stores system data with a given key (for cache storage)
func SaveSystemData(data SystemData) error {
	data.UpdatedAt = time.Now().Unix()

	jsonData, err := json.Marshal(data)
	if err != nil {
		logging.LogError("failed to marshal system data for key %s: %v", data.Key, err)
		return err
	}

	systemKey := fmt.Sprintf(".system/%s", data.Key)
	return globalCacheStorage.Set(systemKey, jsonData)
}

// GetSystemData retrieves system data by key (from cache storage)
func GetSystemData(key string) (*SystemData, error) {
	systemKey := fmt.Sprintf(".system/%s", key)
	data, err := globalCacheStorage.Get(systemKey)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}

	var systemData SystemData
	if err := json.Unmarshal(data, &systemData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal system data: %w", err)
	}

	return &systemData, nil
}
