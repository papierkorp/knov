// Package storage ..
package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"knov/internal/logging"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStorage implements Storage interface using SQLite
type SQLiteStorage struct {
	db          *sql.DB
	storageType string
	mutex       sync.RWMutex
}

// NewSQLiteStorage creates a new SQLite storage instance
func NewSQLiteStorage(basePath, storageType string) (*SQLiteStorage, error) {
	// ensure storage directory exists with proper permissions
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	dbPath := filepath.Join(basePath, storageType+".db")

	// fix permissions on existing database file if it exists
	if _, err := os.Stat(dbPath); err == nil {
		if err := os.Chmod(dbPath, 0644); err != nil {
			logging.LogWarning("failed to fix database permissions: %v", err)
		}
	}

	// open database with explicit read-write mode
	db, err := sql.Open("sqlite3", dbPath+"?mode=rwc")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// set pragmas for better performance and safety
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		logging.LogWarning("failed to set WAL mode: %v", err)
	}
	if _, err := db.Exec("PRAGMA synchronous=NORMAL"); err != nil {
		logging.LogWarning("failed to set synchronous mode: %v", err)
	}

	storage := &SQLiteStorage{
		db:          db,
		storageType: storageType,
	}

	if err := storage.initialize(); err != nil {
		db.Close()
		return nil, err
	}

	return storage, nil
}

// initialize creates appropriate tables based on storage type
func (ss *SQLiteStorage) initialize() error {
	switch ss.storageType {
	case "metadata":
		return ss.initializeMetadataTables()
	case "config":
		return ss.initializeConfigTables()
	case "cache":
		return ss.initializeCacheTables()
	default:
		return fmt.Errorf("unknown storage type: %s", ss.storageType)
	}
}

// initializeMetadataTables creates metadata table with individual columns
func (ss *SQLiteStorage) initializeMetadataTables() error {
	query := `
	CREATE TABLE IF NOT EXISTS metadata (
		path TEXT PRIMARY KEY,
		name TEXT,
		title TEXT,
		created_at DATETIME,
		last_edited DATETIME,
		target_date DATETIME,
		collection TEXT,
		folders TEXT,
		tags TEXT,
		boards TEXT,
		ancestor TEXT,
		parents TEXT,
		kids TEXT,
		used_links TEXT,
		links_to_here TEXT,
		file_type TEXT,
		para_projects TEXT,
		para_areas TEXT,
		para_resources TEXT,
		para_archive TEXT,
		status TEXT,
		priority TEXT,
		size INTEGER
	);
	CREATE INDEX IF NOT EXISTS idx_collection ON metadata(collection);
	CREATE INDEX IF NOT EXISTS idx_file_type ON metadata(file_type);
	CREATE INDEX IF NOT EXISTS idx_status ON metadata(status);
	CREATE INDEX IF NOT EXISTS idx_priority ON metadata(priority);
	`

	_, err := ss.db.Exec(query)
	return err
}

// initializeConfigTables creates settings and theme_settings tables
func (ss *SQLiteStorage) initializeConfigTables() error {
	query := `
	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT
	);
	CREATE TABLE IF NOT EXISTS theme_settings (
		key TEXT,
		theme TEXT,
		value TEXT,
		PRIMARY KEY (key, theme)
	);
	`

	_, err := ss.db.Exec(query)
	return err
}

// initializeCacheTables creates FTS5 tables for search and cache tables
func (ss *SQLiteStorage) initializeCacheTables() error {
	// create basic cache table first (always needed)
	query1 := `CREATE TABLE IF NOT EXISTS cache_strings (
		key TEXT PRIMARY KEY,
		value TEXT
	)`

	if _, err := ss.db.Exec(query1); err != nil {
		logging.LogError("failed to create cache_strings table: %v", err)
		return fmt.Errorf("failed to create cache_strings table: %w", err)
	}

	// try to create FTS5 search index (optional, for advanced search)
	query2 := `CREATE VIRTUAL TABLE IF NOT EXISTS search_index USING fts5(
		path UNINDEXED,
		content,
		tokenize='porter'
	)`

	if _, err := ss.db.Exec(query2); err != nil {
		// FTS5 not available - log warning but continue
		logging.LogWarning("fts5 not available for search index (this is ok for basic usage): %v", err)
		// don't return error - cache storage can work without FTS5
	} else {
		logging.LogDebug("fts5 search index created successfully")
	}

	return nil
}

// Get retrieves data by key
func (ss *SQLiteStorage) Get(key string) ([]byte, error) {
	ss.mutex.RLock()
	defer ss.mutex.RUnlock()

	switch ss.storageType {
	case "metadata":
		return ss.getMetadata(key)
	case "config":
		return ss.getConfig(key)
	case "cache":
		return ss.getCache(key)
	default:
		return nil, fmt.Errorf("unknown storage type: %s", ss.storageType)
	}
}

// getMetadata retrieves metadata and converts to JSON
func (ss *SQLiteStorage) getMetadata(path string) ([]byte, error) {
	query := `
	SELECT name, title, created_at, last_edited, target_date, collection,
	       folders, tags, boards, ancestor, parents, kids, used_links, links_to_here,
	       file_type, para_projects, para_areas, para_resources, para_archive,
	       status, priority, size
	FROM metadata WHERE path = ?
	`

	var meta struct {
		Name          string
		Title         string
		CreatedAt     *time.Time
		LastEdited    *time.Time
		TargetDate    *time.Time
		Collection    string
		Folders       string
		Tags          string
		Boards        string
		Ancestor      string
		Parents       string
		Kids          string
		UsedLinks     string
		LinksToHere   string
		FileType      string
		PARAProjects  string
		PARAreas      string
		PARAResources string
		PARAArchive   string
		Status        string
		Priority      string
		Size          int64
	}

	err := ss.db.QueryRow(query, path).Scan(
		&meta.Name, &meta.Title, &meta.CreatedAt, &meta.LastEdited, &meta.TargetDate,
		&meta.Collection, &meta.Folders, &meta.Tags, &meta.Boards, &meta.Ancestor,
		&meta.Parents, &meta.Kids, &meta.UsedLinks, &meta.LinksToHere, &meta.FileType,
		&meta.PARAProjects, &meta.PARAreas, &meta.PARAResources, &meta.PARAArchive,
		&meta.Status, &meta.Priority, &meta.Size,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// convert to metadata JSON format
	result := map[string]interface{}{
		"path":       path,
		"name":       meta.Name,
		"title":      meta.Title,
		"collection": meta.Collection,
		"type":       meta.FileType,
		"status":     meta.Status,
		"priority":   meta.Priority,
		"size":       meta.Size,
	}

	if meta.CreatedAt != nil {
		result["createdAt"] = meta.CreatedAt.Format(time.RFC3339)
	}
	if meta.LastEdited != nil {
		result["lastEdited"] = meta.LastEdited.Format(time.RFC3339)
	}
	if meta.TargetDate != nil && !meta.TargetDate.IsZero() {
		result["targetDate"] = meta.TargetDate.Format(time.RFC3339)
	}

	// parse JSON arrays
	if meta.Folders != "" {
		var folders []string
		json.Unmarshal([]byte(meta.Folders), &folders)
		result["folders"] = folders
	}
	if meta.Tags != "" {
		var tags []string
		json.Unmarshal([]byte(meta.Tags), &tags)
		result["tags"] = tags
	}
	if meta.Boards != "" {
		var boards []string
		json.Unmarshal([]byte(meta.Boards), &boards)
		result["boards"] = boards
	}
	if meta.Ancestor != "" {
		var ancestor []string
		json.Unmarshal([]byte(meta.Ancestor), &ancestor)
		result["ancestor"] = ancestor
	}
	if meta.Parents != "" {
		var parents []string
		json.Unmarshal([]byte(meta.Parents), &parents)
		result["parents"] = parents
	}
	if meta.Kids != "" {
		var kids []string
		json.Unmarshal([]byte(meta.Kids), &kids)
		result["kids"] = kids
	}
	if meta.UsedLinks != "" {
		var usedLinks []string
		json.Unmarshal([]byte(meta.UsedLinks), &usedLinks)
		result["usedLinks"] = usedLinks
	}
	if meta.LinksToHere != "" {
		var linksToHere []string
		json.Unmarshal([]byte(meta.LinksToHere), &linksToHere)
		result["linksToHere"] = linksToHere
	}

	// parse PARA
	para := make(map[string]interface{})
	if meta.PARAProjects != "" {
		var projects []string
		json.Unmarshal([]byte(meta.PARAProjects), &projects)
		para["projects"] = projects
	}
	if meta.PARAreas != "" {
		var areas []string
		json.Unmarshal([]byte(meta.PARAreas), &areas)
		para["areas"] = areas
	}
	if meta.PARAResources != "" {
		var resources []string
		json.Unmarshal([]byte(meta.PARAResources), &resources)
		para["resources"] = resources
	}
	if meta.PARAArchive != "" {
		var archive []string
		json.Unmarshal([]byte(meta.PARAArchive), &archive)
		para["archive"] = archive
	}
	if len(para) > 0 {
		result["para"] = para
	}

	return json.Marshal(result)
}

// getConfig retrieves config value
func (ss *SQLiteStorage) getConfig(key string) ([]byte, error) {
	// check if it's a theme setting (format: theme/key)
	if strings.Contains(key, "/") {
		parts := strings.SplitN(key, "/", 2)
		if len(parts) == 2 {
			theme, settingKey := parts[0], parts[1]
			var value string
			err := ss.db.QueryRow("SELECT value FROM theme_settings WHERE theme = ? AND key = ?", theme, settingKey).Scan(&value)
			if err == sql.ErrNoRows {
				return nil, nil
			}
			if err != nil {
				return nil, err
			}
			return []byte(value), nil
		}
	}

	// regular setting
	var value string
	err := ss.db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return []byte(value), nil
}

// getCache retrieves cache value
func (ss *SQLiteStorage) getCache(key string) ([]byte, error) {
	var value string
	err := ss.db.QueryRow("SELECT value FROM cache_strings WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return []byte(value), nil
}

// Set stores data with key
func (ss *SQLiteStorage) Set(key string, data []byte) error {
	ss.mutex.Lock()
	defer ss.mutex.Unlock()

	switch ss.storageType {
	case "metadata":
		return ss.setMetadata(key, data)
	case "config":
		return ss.setConfig(key, data)
	case "cache":
		return ss.setCache(key, data)
	default:
		return fmt.Errorf("unknown storage type: %s", ss.storageType)
	}
}

// setMetadata stores metadata from JSON
func (ss *SQLiteStorage) setMetadata(path string, data []byte) error {
	var metadata map[string]interface{}
	if err := json.Unmarshal(data, &metadata); err != nil {
		return err
	}

	// helper to marshal arrays
	marshalArray := func(key string) string {
		if val, ok := metadata[key]; ok {
			if arr, ok := val.([]interface{}); ok && len(arr) > 0 {
				bytes, _ := json.Marshal(arr)
				return string(bytes)
			}
		}
		return ""
	}

	// helper to get string
	getString := func(key string) string {
		if val, ok := metadata[key]; ok {
			if str, ok := val.(string); ok {
				return str
			}
		}
		return ""
	}

	// helper to get time
	getTime := func(key string) *time.Time {
		if val, ok := metadata[key]; ok {
			if str, ok := val.(string); ok && str != "" {
				t, err := time.Parse(time.RFC3339, str)
				if err == nil {
					return &t
				}
			}
		}
		return nil
	}

	// extract PARA
	var paraProjects, paraAreas, paraResources, paraArchive string
	if para, ok := metadata["para"].(map[string]interface{}); ok {
		if projects, ok := para["projects"].([]interface{}); ok && len(projects) > 0 {
			bytes, _ := json.Marshal(projects)
			paraProjects = string(bytes)
		}
		if areas, ok := para["areas"].([]interface{}); ok && len(areas) > 0 {
			bytes, _ := json.Marshal(areas)
			paraAreas = string(bytes)
		}
		if resources, ok := para["resources"].([]interface{}); ok && len(resources) > 0 {
			bytes, _ := json.Marshal(resources)
			paraResources = string(bytes)
		}
		if archive, ok := para["archive"].([]interface{}); ok && len(archive) > 0 {
			bytes, _ := json.Marshal(archive)
			paraArchive = string(bytes)
		}
	}

	// get size
	var size int64
	if val, ok := metadata["size"]; ok {
		if num, ok := val.(float64); ok {
			size = int64(num)
		}
	}

	query := `
	INSERT OR REPLACE INTO metadata (
		path, name, title, created_at, last_edited, target_date, collection,
		folders, tags, boards, ancestor, parents, kids, used_links, links_to_here,
		file_type, para_projects, para_areas, para_resources, para_archive,
		status, priority, size
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := ss.db.Exec(query,
		path,
		getString("name"),
		getString("title"),
		getTime("createdAt"),
		getTime("lastEdited"),
		getTime("targetDate"),
		getString("collection"),
		marshalArray("folders"),
		marshalArray("tags"),
		marshalArray("boards"),
		marshalArray("ancestor"),
		marshalArray("parents"),
		marshalArray("kids"),
		marshalArray("usedLinks"),
		marshalArray("linksToHere"),
		getString("type"),
		paraProjects,
		paraAreas,
		paraResources,
		paraArchive,
		getString("status"),
		getString("priority"),
		size,
	)

	return err
}

// setConfig stores config value
func (ss *SQLiteStorage) setConfig(key string, data []byte) error {
	// check if it's a theme setting
	if strings.Contains(key, "/") {
		parts := strings.SplitN(key, "/", 2)
		if len(parts) == 2 {
			theme, settingKey := parts[0], parts[1]
			_, err := ss.db.Exec(
				"INSERT OR REPLACE INTO theme_settings (theme, key, value) VALUES (?, ?, ?)",
				theme, settingKey, string(data),
			)
			return err
		}
	}

	// regular setting
	_, err := ss.db.Exec(
		"INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)",
		key, string(data),
	)
	return err
}

// setCache stores cache value
func (ss *SQLiteStorage) setCache(key string, data []byte) error {
	_, err := ss.db.Exec(
		"INSERT OR REPLACE INTO cache_strings (key, value) VALUES (?, ?)",
		key, string(data),
	)
	return err
}

// Delete removes data by key
func (ss *SQLiteStorage) Delete(key string) error {
	ss.mutex.Lock()
	defer ss.mutex.Unlock()

	switch ss.storageType {
	case "metadata":
		_, err := ss.db.Exec("DELETE FROM metadata WHERE path = ?", key)
		return err
	case "config":
		if strings.Contains(key, "/") {
			parts := strings.SplitN(key, "/", 2)
			if len(parts) == 2 {
				theme, settingKey := parts[0], parts[1]
				_, err := ss.db.Exec("DELETE FROM theme_settings WHERE theme = ? AND key = ?", theme, settingKey)
				return err
			}
		}
		_, err := ss.db.Exec("DELETE FROM settings WHERE key = ?", key)
		return err
	case "cache":
		_, err := ss.db.Exec("DELETE FROM cache_strings WHERE key = ?", key)
		return err
	default:
		return fmt.Errorf("unknown storage type: %s", ss.storageType)
	}
}

// List returns all keys with given prefix
func (ss *SQLiteStorage) List(prefix string) ([]string, error) {
	ss.mutex.RLock()
	defer ss.mutex.RUnlock()

	var query string
	switch ss.storageType {
	case "metadata":
		query = "SELECT path FROM metadata WHERE path LIKE ?"
	case "config":
		query = "SELECT key FROM settings WHERE key LIKE ? UNION SELECT theme || '/' || key FROM theme_settings WHERE key LIKE ?"
	case "cache":
		query = "SELECT key FROM cache_strings WHERE key LIKE ?"
	default:
		return nil, fmt.Errorf("unknown storage type: %s", ss.storageType)
	}

	pattern := prefix + "%"
	var rows *sql.Rows
	var err error

	if ss.storageType == "config" {
		rows, err = ss.db.Query(query, pattern, pattern)
	} else {
		rows, err = ss.db.Query(query, pattern)
	}

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

// GetAll returns all key-value pairs
func (ss *SQLiteStorage) GetAll() (map[string][]byte, error) {
	ss.mutex.RLock()
	defer ss.mutex.RUnlock()

	result := make(map[string][]byte)

	switch ss.storageType {
	case "metadata":
		rows, err := ss.db.Query("SELECT path FROM metadata")
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var path string
			if err := rows.Scan(&path); err != nil {
				return nil, err
			}
			data, err := ss.getMetadata(path)
			if err != nil {
				logging.LogWarning("failed to get metadata for %s: %v", path, err)
				continue
			}
			result[path] = data
		}
		return result, rows.Err()

	case "config":
		// get settings
		rows, err := ss.db.Query("SELECT key, value FROM settings")
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var key, value string
			if err := rows.Scan(&key, &value); err != nil {
				return nil, err
			}
			result[key] = []byte(value)
		}
		rows.Close()

		// get theme settings
		rows, err = ss.db.Query("SELECT theme, key, value FROM theme_settings")
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var theme, key, value string
			if err := rows.Scan(&theme, &key, &value); err != nil {
				return nil, err
			}
			result[theme+"/"+key] = []byte(value)
		}
		return result, rows.Err()

	case "cache":
		rows, err := ss.db.Query("SELECT key, value FROM cache_strings")
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var key, value string
			if err := rows.Scan(&key, &value); err != nil {
				return nil, err
			}
			result[key] = []byte(value)
		}
		return result, rows.Err()

	default:
		return nil, fmt.Errorf("unknown storage type: %s", ss.storageType)
	}
}

// Exists checks if key exists
func (ss *SQLiteStorage) Exists(key string) bool {
	ss.mutex.RLock()
	defer ss.mutex.RUnlock()

	var exists bool
	switch ss.storageType {
	case "metadata":
		err := ss.db.QueryRow("SELECT EXISTS(SELECT 1 FROM metadata WHERE path = ?)", key).Scan(&exists)
		return err == nil && exists
	case "config":
		if strings.Contains(key, "/") {
			parts := strings.SplitN(key, "/", 2)
			if len(parts) == 2 {
				theme, settingKey := parts[0], parts[1]
				err := ss.db.QueryRow("SELECT EXISTS(SELECT 1 FROM theme_settings WHERE theme = ? AND key = ?)", theme, settingKey).Scan(&exists)
				return err == nil && exists
			}
		}
		err := ss.db.QueryRow("SELECT EXISTS(SELECT 1 FROM settings WHERE key = ?)", key).Scan(&exists)
		return err == nil && exists
	case "cache":
		err := ss.db.QueryRow("SELECT EXISTS(SELECT 1 FROM cache_strings WHERE key = ?)", key).Scan(&exists)
		return err == nil && exists
	default:
		return false
	}
}

// GetBackendType returns the backend type
func (ss *SQLiteStorage) GetBackendType() string {
	return "sqlite"
}

// SearchFiles performs FTS5 search (only for cache storage)
func (ss *SQLiteStorage) SearchFiles(query string, limit int) ([]string, error) {
	if ss.storageType != "cache" {
		return nil, fmt.Errorf("search is only available for cache storage")
	}

	ss.mutex.RLock()
	defer ss.mutex.RUnlock()

	sqlQuery := "SELECT path FROM search_index WHERE search_index MATCH ? LIMIT ?"
	rows, err := ss.db.Query(sqlQuery, query, limit)
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

// IndexFile indexes a file for FTS5 search (only for cache storage)
func (ss *SQLiteStorage) IndexFile(path, content string) error {
	if ss.storageType != "cache" {
		return fmt.Errorf("indexing is only available for cache storage")
	}

	ss.mutex.Lock()
	defer ss.mutex.Unlock()

	// delete existing entry
	_, err := ss.db.Exec("DELETE FROM search_index WHERE path = ?", path)
	if err != nil {
		return err
	}

	// insert new entry
	_, err = ss.db.Exec("INSERT INTO search_index (path, content) VALUES (?, ?)", path, content)
	return err
}

// Close closes the database connection
func (ss *SQLiteStorage) Close() error {
	return ss.db.Close()
}
