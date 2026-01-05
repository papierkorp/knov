// Package storage - SQLite base storage with flexible table definitions
package storage

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"knov/internal/logging"

	_ "github.com/mattn/go-sqlite3"
)

// ----------------------------------------------------------------------------------------
// -------------------------------- Table Definition --------------------------------------
// ----------------------------------------------------------------------------------------

// ColumnDef defines a table column
type ColumnDef struct {
	Name       string
	Type       string // SQL type: TEXT, INTEGER, BLOB, REAL
	Primary    bool   // is primary key
	NotNull    bool   // NOT NULL constraint
	Index      bool   // create index on this column
	IndexWhere string // WHERE clause for partial index
}

// IndexDef defines a table index
type IndexDef struct {
	Name    string
	Columns []string
	Where   string // WHERE clause for partial index
}

// TableDef defines a complete table schema
type TableDef struct {
	Columns []ColumnDef
	Indexes []IndexDef
}

// ----------------------------------------------------------------------------------------
// -------------------------------- Base SQLite Storage -----------------------------------
// ----------------------------------------------------------------------------------------

// baseSQLiteStorage provides common SQLite operations with flexible schema
type baseSQLiteStorage struct {
	db           *sql.DB
	tables       map[string]TableDef
	primaryTable string // main table for Get/Set/Delete operations
	mutex        sync.RWMutex
}

// newBaseSQLiteStorage creates a new base SQLite storage
func newBaseSQLiteStorage(dbPath string, tables map[string]TableDef, primaryTable string) (*baseSQLiteStorage, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	storage := &baseSQLiteStorage{
		db:           db,
		tables:       tables,
		primaryTable: primaryTable,
	}

	return storage, nil
}

// Init creates all tables and indexes
func (s *baseSQLiteStorage) Init() error {
	for tableName, tableDef := range s.tables {
		if err := s.createTable(tableName, tableDef); err != nil {
			return err
		}
	}
	return nil
}

func (s *baseSQLiteStorage) createTable(tableName string, def TableDef) error {
	// build column definitions
	var cols []string
	var primaryKeys []string

	for _, col := range def.Columns {
		colDef := fmt.Sprintf("%s %s", col.Name, col.Type)

		if col.Primary {
			primaryKeys = append(primaryKeys, col.Name)
		}

		if col.NotNull {
			colDef += " NOT NULL"
		}

		cols = append(cols, colDef)
	}

	// add primary key constraint
	if len(primaryKeys) > 0 {
		cols = append(cols, fmt.Sprintf("PRIMARY KEY (%s)", strings.Join(primaryKeys, ", ")))
	}

	// create table
	query := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n\t%s\n);",
		tableName, strings.Join(cols, ",\n\t"))

	if _, err := s.db.Exec(query); err != nil {
		return fmt.Errorf("failed to create table %s: %w", tableName, err)
	}

	// create column indexes
	for _, col := range def.Columns {
		if col.Index && !col.Primary {
			indexName := fmt.Sprintf("idx_%s_%s", tableName, col.Name)
			indexQuery := fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON %s(%s)",
				indexName, tableName, col.Name)

			if col.IndexWhere != "" {
				indexQuery += " WHERE " + col.IndexWhere
			}

			if _, err := s.db.Exec(indexQuery); err != nil {
				logging.LogWarning("failed to create index %s: %v", indexName, err)
			}
		}
	}

	// create composite indexes
	for _, idx := range def.Indexes {
		indexQuery := fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON %s(%s)",
			idx.Name, tableName, strings.Join(idx.Columns, ", "))

		if idx.Where != "" {
			indexQuery += " WHERE " + idx.Where
		}

		if _, err := s.db.Exec(indexQuery); err != nil {
			logging.LogWarning("failed to create index %s: %v", idx.Name, err)
		}
	}

	return nil
}

// ----------------------------------------------------------------------------------------
// -------------------------------- CRUD Operations ---------------------------------------
// ----------------------------------------------------------------------------------------

func (s *baseSQLiteStorage) Get(key string) ([]byte, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var value []byte
	query := fmt.Sprintf("SELECT value FROM %s WHERE key = ?", s.primaryTable)
	err := s.db.QueryRow(query, key).Scan(&value)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get key %s: %w", key, err)
	}

	logging.LogDebug("retrieved data for key: %s", key)
	return value, nil
}

func (s *baseSQLiteStorage) Set(key string, data []byte) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	query := fmt.Sprintf(`INSERT INTO %s (key, value, updated_at) VALUES (?, ?, ?)
	          ON CONFLICT(key) DO UPDATE SET value = ?, updated_at = ?`, s.primaryTable)
	now := time.Now().Unix()

	if _, err := s.db.Exec(query, key, data, now, data, now); err != nil {
		return fmt.Errorf("failed to set key %s: %w", key, err)
	}

	logging.LogDebug("stored data for key: %s", key)
	return nil
}

func (s *baseSQLiteStorage) Delete(key string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	query := fmt.Sprintf("DELETE FROM %s WHERE key = ?", s.primaryTable)
	if _, err := s.db.Exec(query, key); err != nil {
		return fmt.Errorf("failed to delete key %s: %w", key, err)
	}

	logging.LogDebug("deleted data for key: %s", key)
	return nil
}

func (s *baseSQLiteStorage) List(prefix string) ([]string, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	query := fmt.Sprintf("SELECT key FROM %s WHERE key LIKE ? ORDER BY key", s.primaryTable)
	rows, err := s.db.Query(query, prefix+"%")
	if err != nil {
		return nil, fmt.Errorf("failed to list keys with prefix %s: %w", prefix, err)
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			continue
		}
		keys = append(keys, key)
	}

	logging.LogDebug("listed %d keys with prefix: %s", len(keys), prefix)
	return keys, nil
}

func (s *baseSQLiteStorage) Exists(key string) bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var exists bool
	query := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s WHERE key = ?)", s.primaryTable)
	err := s.db.QueryRow(query, key).Scan(&exists)
	return err == nil && exists
}

func (s *baseSQLiteStorage) GetWithDefault(key string, defaultValue []byte) ([]byte, error) {
	data, err := s.Get(key)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return defaultValue, nil
	}
	return data, nil
}

func (s *baseSQLiteStorage) BulkSet(data map[string][]byte) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := fmt.Sprintf(`INSERT INTO %s (key, value, updated_at) VALUES (?, ?, ?)
	          ON CONFLICT(key) DO UPDATE SET value = ?, updated_at = ?`, s.primaryTable)
	stmt, err := tx.Prepare(query)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	now := time.Now().Unix()
	for key, value := range data {
		if _, err := stmt.Exec(key, value, now, value, now); err != nil {
			return fmt.Errorf("failed to set key %s: %w", key, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	logging.LogDebug("bulk set %d keys", len(data))
	return nil
}

func (s *baseSQLiteStorage) Close() error {
	return s.db.Close()
}
