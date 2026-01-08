// Package cacheStorage - SQLite backend implementation
package cacheStorage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"knov/internal/logging"

	_ "github.com/mattn/go-sqlite3"
)

// sqliteStorage implements CacheStorage interface using SQLite
type sqliteStorage struct {
	db    *sql.DB
	mutex sync.RWMutex
}

// newSQLiteStorage creates a new SQLite cache storage instance
func newSQLiteStorage(storagePath string) (*sqliteStorage, error) {
	// ensure storage directory exists with proper permissions
	if err := os.MkdirAll(storagePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	dbPath := filepath.Join(storagePath, "cache.db")

	// fix permissions on existing database file if it exists
	if _, err := os.Stat(dbPath); err == nil {
		if err := os.Chmod(dbPath, 0644); err != nil {
			logging.LogWarning("failed to fix cache database permissions: %v", err)
		}
	}

	// open database with explicit read-write mode
	db, err := sql.Open("sqlite3", dbPath+"?mode=rwc")
	if err != nil {
		return nil, fmt.Errorf("failed to open cache database: %w", err)
	}

	// set pragmas for better performance and safety
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		logging.LogWarning("failed to set WAL mode for cache: %v", err)
	}
	if _, err := db.Exec("PRAGMA synchronous=NORMAL"); err != nil {
		logging.LogWarning("failed to set synchronous mode for cache: %v", err)
	}

	storage := &sqliteStorage{
		db: db,
	}

	if err := storage.initialize(); err != nil {
		db.Close()
		return nil, err
	}

	return storage, nil
}

// initialize creates cache table
func (ss *sqliteStorage) initialize() error {
	query := `
		CREATE TABLE IF NOT EXISTS cache (
			key TEXT PRIMARY KEY,
			value BLOB
		);
		CREATE INDEX IF NOT EXISTS idx_cache_key ON cache(key);
	`

	if _, err := ss.db.Exec(query); err != nil {
		return fmt.Errorf("failed to create cache table: %w", err)
	}

	return nil
}

// Get retrieves data by key
func (ss *sqliteStorage) Get(key string) ([]byte, error) {
	ss.mutex.RLock()
	defer ss.mutex.RUnlock()

	var value []byte
	err := ss.db.QueryRow("SELECT value FROM cache WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return value, nil
}

// Set stores data with key
func (ss *sqliteStorage) Set(key string, data []byte) error {
	ss.mutex.Lock()
	defer ss.mutex.Unlock()

	_, err := ss.db.Exec("INSERT OR REPLACE INTO cache (key, value) VALUES (?, ?)", key, data)
	if err != nil {
		logging.LogError("failed to set cache key %s: %v", key, err)
		return err
	}

	logging.LogDebug("stored cache data for key: %s", key)
	return nil
}

// Delete removes data by key
func (ss *sqliteStorage) Delete(key string) error {
	ss.mutex.Lock()
	defer ss.mutex.Unlock()

	_, err := ss.db.Exec("DELETE FROM cache WHERE key = ?", key)
	if err != nil {
		logging.LogError("failed to delete cache key %s: %v", key, err)
		return err
	}

	logging.LogDebug("deleted cache key: %s", key)
	return nil
}

// List returns all keys with given prefix
func (ss *sqliteStorage) List(prefix string) ([]string, error) {
	ss.mutex.RLock()
	defer ss.mutex.RUnlock()

	rows, err := ss.db.Query("SELECT key FROM cache WHERE key LIKE ?", prefix+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}

	return keys, rows.Err()
}

// Exists checks if key exists
func (ss *sqliteStorage) Exists(key string) bool {
	ss.mutex.RLock()
	defer ss.mutex.RUnlock()

	var exists bool
	err := ss.db.QueryRow("SELECT EXISTS(SELECT 1 FROM cache WHERE key = ?)", key).Scan(&exists)
	return err == nil && exists
}

// GetBackendType returns the backend type
func (ss *sqliteStorage) GetBackendType() string {
	return "sqlite"
}
