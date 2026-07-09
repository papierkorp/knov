// Package searchStorage - SQLite FTS5 backend implementation
package searchStorage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"knov/internal/dbmigration"
	"knov/internal/logging"

	_ "modernc.org/sqlite"
)

// sqliteStorage implements SearchStorage interface using SQLite FTS5
type sqliteStorage struct {
	db    *sql.DB
	mutex sync.RWMutex
}

// newSQLiteStorage creates a new SQLite search storage instance with FTS5
func newSQLiteStorage(storagePath string) (*sqliteStorage, error) {
	// ensure storage directory exists with proper permissions
	if err := os.MkdirAll(storagePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	searchDir := filepath.Join(storagePath, "search")
	if err := os.MkdirAll(searchDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create search directory: %w", err)
	}
	dbPath := filepath.Join(searchDir, "search.db")

	// fix permissions on existing database file if it exists
	if _, err := os.Stat(dbPath); err == nil {
		if err := os.Chmod(dbPath, 0644); err != nil {
			logging.LogWarning("failed to fix search database permissions: %v", err)
		}
	}

	// open database with explicit read-write mode
	db, err := sql.Open("sqlite", dbPath+"?mode=rwc")
	if err != nil {
		return nil, fmt.Errorf("failed to open search database: %w", err)
	}

	// set pragmas for better performance and safety
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		logging.LogWarning("failed to set WAL mode for search: %v", err)
	}
	if _, err := db.Exec("PRAGMA synchronous=NORMAL"); err != nil {
		logging.LogWarning("failed to set synchronous mode for search: %v", err)
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

// initialize runs all pending migrations for this storage.
func (ss *sqliteStorage) initialize() error {
	const version = 3
	steps := []dbmigration.Migration{
		{
			Up: func(tx *sql.Tx) error {
				_, err := tx.Exec(`
				CREATE VIRTUAL TABLE IF NOT EXISTS search_index USING fts5(
					path UNINDEXED,
					content,
					tokenize='porter ascii'
				);
				CREATE TABLE IF NOT EXISTS search_content (
					path TEXT PRIMARY KEY,
					content BLOB
				);
				CREATE INDEX IF NOT EXISTS idx_search_content_path ON search_content(path);
				`)
				return err
			},
			Down: func(tx *sql.Tx) error {
				_, err := tx.Exec(`
				DROP TABLE IF EXISTS search_index;
				DROP TABLE IF EXISTS search_content;
				`)
				return err
			},
		},
		{
			Up: func(tx *sql.Tx) error {
				_, err := tx.Exec(`ALTER TABLE search_content ADD COLUMN indexed_at DATETIME`)
				return err
			},
			Down: func(tx *sql.Tx) error {
				// sqlite < 3.35 workaround not needed for this project; DROP COLUMN supported
				_, err := tx.Exec(`ALTER TABLE search_content DROP COLUMN indexed_at`)
				return err
			},
		},
		{
			// separate tables (not a "deleted" column on search_index) because
			// FTS5 virtual tables don't support ALTER TABLE ADD COLUMN. Indexed
			// once by the cronjob when it detects a deletion, so deleted-file
			// content search reads this instead of walking the full commit log.
			Up: func(tx *sql.Tx) error {
				_, err := tx.Exec(`
				CREATE VIRTUAL TABLE IF NOT EXISTS deleted_search_index USING fts5(
					path UNINDEXED,
					content,
					tokenize='porter ascii'
				);
				CREATE TABLE IF NOT EXISTS deleted_search_content (
					path TEXT PRIMARY KEY,
					content BLOB,
					indexed_at DATETIME
				);
				CREATE INDEX IF NOT EXISTS idx_deleted_search_content_path ON deleted_search_content(path);
				`)
				return err
			},
			Down: func(tx *sql.Tx) error {
				_, err := tx.Exec(`
				DROP TABLE IF EXISTS deleted_search_index;
				DROP TABLE IF EXISTS deleted_search_content;
				`)
				return err
			},
		},
	}
	if err := dbmigration.Migrate(ss.db, version, steps); err != nil {
		return fmt.Errorf("search storage migration failed: %w", err)
	}
	logging.LogDebug("search sqlite storage ready at version %d", version)
	return nil
}

// IndexFile indexes a file's content for search
func (ss *sqliteStorage) IndexFile(path string, content []byte) error {
	ss.mutex.Lock()
	defer ss.mutex.Unlock()

	now := time.Now().UTC()

	_, err := ss.db.Exec("INSERT OR REPLACE INTO search_content (path, content, indexed_at) VALUES (?, ?, ?)", path, content, now)
	if err != nil {
		logging.LogError("failed to store search content for %s: %v", path, err)
		return err
	}

	_, err = ss.db.Exec("INSERT OR REPLACE INTO search_index (path, content) VALUES (?, ?)", path, string(content))
	if err != nil {
		logging.LogError("failed to index file %s: %v", path, err)
		return err
	}

	logging.LogDebug("indexed file: %s", path)
	return nil
}

// GetIndexedAt returns the time a file was last indexed, or zero time if not indexed.
func (ss *sqliteStorage) GetIndexedAt(path string) (time.Time, error) {
	ss.mutex.RLock()
	defer ss.mutex.RUnlock()

	var t time.Time
	err := ss.db.QueryRow("SELECT indexed_at FROM search_content WHERE path = ?", path).Scan(&t)
	if err == sql.ErrNoRows {
		return time.Time{}, nil
	}
	return t, err
}

// GetIndexedContent retrieves indexed content for a file
func (ss *sqliteStorage) GetIndexedContent(path string) ([]byte, error) {
	ss.mutex.RLock()
	defer ss.mutex.RUnlock()

	var content []byte
	err := ss.db.QueryRow("SELECT content FROM search_content WHERE path = ?", path).Scan(&content)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return content, nil
}

// DeleteIndexedContent removes indexed content for a file
func (ss *sqliteStorage) DeleteIndexedContent(path string) error {
	ss.mutex.Lock()
	defer ss.mutex.Unlock()

	// remove from FTS index
	_, err := ss.db.Exec("DELETE FROM search_index WHERE path = ?", path)
	if err != nil {
		logging.LogError("failed to delete from search index %s: %v", path, err)
		return err
	}

	// remove from content table
	_, err = ss.db.Exec("DELETE FROM search_content WHERE path = ?", path)
	if err != nil {
		logging.LogError("failed to delete search content %s: %v", path, err)
		return err
	}

	logging.LogDebug("deleted indexed content: %s", path)
	return nil
}

// ListAllIndexedFiles returns all indexed file paths
func (ss *sqliteStorage) ListAllIndexedFiles() ([]string, error) {
	ss.mutex.RLock()
	defer ss.mutex.RUnlock()

	rows, err := ss.db.Query("SELECT path FROM search_content")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}

	return paths, rows.Err()
}

// SearchContent performs full-text search using FTS5
func (ss *sqliteStorage) SearchContent(query string, limit int) ([]SearchResult, error) {
	ss.mutex.RLock()
	defer ss.mutex.RUnlock()

	// use FTS5 match query with BM25 ranking
	sqlQuery := `
		SELECT
			si.path,
			sc.content,
			bm25(search_index) as score
		FROM search_index si
		JOIN search_content sc ON si.path = sc.path
		WHERE search_index MATCH ?
		ORDER BY score
		LIMIT ?
	`

	rows, err := ss.db.Query(sqlQuery, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var result SearchResult
		if err := rows.Scan(&result.Path, &result.Content, &result.Score); err != nil {
			return nil, err
		}
		results = append(results, result)
	}

	logging.LogDebug("search query '%s' returned %d results", query, len(results))
	return results, rows.Err()
}

// IndexDeletedFile indexes a deleted file's pre-deletion content in the
// separate deleted-files FTS table, so content search over deleted files
// doesn't need to walk the commit log.
func (ss *sqliteStorage) IndexDeletedFile(path string, content []byte) error {
	ss.mutex.Lock()
	defer ss.mutex.Unlock()

	now := time.Now().UTC()

	_, err := ss.db.Exec("INSERT OR REPLACE INTO deleted_search_content (path, content, indexed_at) VALUES (?, ?, ?)", path, content, now)
	if err != nil {
		logging.LogError("failed to store deleted search content for %s: %v", path, err)
		return err
	}

	_, err = ss.db.Exec("INSERT OR REPLACE INTO deleted_search_index (path, content) VALUES (?, ?)", path, string(content))
	if err != nil {
		logging.LogError("failed to index deleted file %s: %v", path, err)
		return err
	}

	logging.LogDebug("indexed deleted file: %s", path)
	return nil
}

// SearchDeletedContent performs full-text search over deleted files' pre-deletion content
func (ss *sqliteStorage) SearchDeletedContent(query string, limit int) ([]SearchResult, error) {
	ss.mutex.RLock()
	defer ss.mutex.RUnlock()

	sqlQuery := `
		SELECT
			si.path,
			sc.content,
			bm25(deleted_search_index) as score
		FROM deleted_search_index si
		JOIN deleted_search_content sc ON si.path = sc.path
		WHERE deleted_search_index MATCH ?
		ORDER BY score
		LIMIT ?
	`

	rows, err := ss.db.Query(sqlQuery, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var result SearchResult
		if err := rows.Scan(&result.Path, &result.Content, &result.Score); err != nil {
			return nil, err
		}
		results = append(results, result)
	}

	logging.LogDebug("deleted-file search query '%s' returned %d results", query, len(results))
	return results, rows.Err()
}

// GetBackendType returns the backend type
func (ss *sqliteStorage) GetBackendType() string {
	return "sqlite-fts5"
}
