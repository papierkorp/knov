// Package metadataStorage - SQLite backend implementation
package metadataStorage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"knov/internal/logging"

	_ "github.com/mattn/go-sqlite3"
)

// sqliteStorage implements MetadataStorage interface using SQLite
type sqliteStorage struct {
	db       *sql.DB
	basePath string
	mutex    sync.RWMutex
}

// newSQLiteStorage creates a new SQLite metadata storage instance
func newSQLiteStorage(storagePath string) (*sqliteStorage, error) {
	fullPath := filepath.Join(storagePath)
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(fullPath, "metadata.db")

	// open database with explicit read-write mode
	db, err := sql.Open("sqlite3", dbPath+"?mode=rwc")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// set pragmas for better performance and safety
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		logging.LogWarning("failed to set wal mode: %v", err)
	}
	if _, err := db.Exec("PRAGMA synchronous=NORMAL"); err != nil {
		logging.LogWarning("failed to set synchronous mode: %v", err)
	}

	storage := &sqliteStorage{
		db:       db,
		basePath: fullPath,
	}

	if err := storage.initialize(); err != nil {
		db.Close()
		return nil, err
	}

	return storage, nil
}

// initialize creates the metadata table with individual columns
func (ss *sqliteStorage) initialize() error {
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
	if err != nil {
		logging.LogError("failed to initialize metadata tables: %v", err)
		return fmt.Errorf("failed to initialize metadata tables: %w", err)
	}

	logging.LogDebug("metadata sqlite tables initialized")
	return nil
}

// Get retrieves metadata by key and returns as JSON
func (ss *sqliteStorage) Get(key string) ([]byte, error) {
	ss.mutex.RLock()
	defer ss.mutex.RUnlock()

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

	err := ss.db.QueryRow(query, key).Scan(
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
		logging.LogError("failed to get metadata for key %s: %v", key, err)
		return nil, err
	}

	// convert to metadata JSON format
	result := map[string]interface{}{
		"path":       key,
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
		if err := json.Unmarshal([]byte(meta.Folders), &folders); err == nil {
			result["folders"] = folders
		}
	}
	if meta.Tags != "" {
		var tags []string
		if err := json.Unmarshal([]byte(meta.Tags), &tags); err == nil {
			result["tags"] = tags
		}
	}
	if meta.Boards != "" {
		var boards []string
		if err := json.Unmarshal([]byte(meta.Boards), &boards); err == nil {
			result["boards"] = boards
		}
	}
	if meta.Ancestor != "" {
		var ancestor []string
		if err := json.Unmarshal([]byte(meta.Ancestor), &ancestor); err == nil {
			result["ancestor"] = ancestor
		}
	}
	if meta.Parents != "" {
		var parents []string
		if err := json.Unmarshal([]byte(meta.Parents), &parents); err == nil {
			result["parents"] = parents
		}
	}
	if meta.Kids != "" {
		var kids []string
		if err := json.Unmarshal([]byte(meta.Kids), &kids); err == nil {
			result["kids"] = kids
		}
	}
	if meta.UsedLinks != "" {
		var usedLinks []string
		if err := json.Unmarshal([]byte(meta.UsedLinks), &usedLinks); err == nil {
			result["usedLinks"] = usedLinks
		}
	}
	if meta.LinksToHere != "" {
		var linksToHere []string
		if err := json.Unmarshal([]byte(meta.LinksToHere), &linksToHere); err == nil {
			result["linksToHere"] = linksToHere
		}
	}

	// parse PARA
	para := make(map[string]interface{})
	if meta.PARAProjects != "" {
		var projects []string
		if err := json.Unmarshal([]byte(meta.PARAProjects), &projects); err == nil {
			para["projects"] = projects
		}
	}
	if meta.PARAreas != "" {
		var areas []string
		if err := json.Unmarshal([]byte(meta.PARAreas), &areas); err == nil {
			para["areas"] = areas
		}
	}
	if meta.PARAResources != "" {
		var resources []string
		if err := json.Unmarshal([]byte(meta.PARAResources), &resources); err == nil {
			para["resources"] = resources
		}
	}
	if meta.PARAArchive != "" {
		var archive []string
		if err := json.Unmarshal([]byte(meta.PARAArchive), &archive); err == nil {
			para["archive"] = archive
		}
	}
	if len(para) > 0 {
		result["para"] = para
	}

	data, err := json.Marshal(result)
	if err != nil {
		logging.LogError("failed to marshal metadata for key %s: %v", key, err)
		return nil, err
	}

	logging.LogDebug("retrieved metadata for key: %s", key)
	return data, nil
}

// Set stores metadata from JSON data
func (ss *sqliteStorage) Set(key string, data []byte) error {
	ss.mutex.Lock()
	defer ss.mutex.Unlock()

	if len(data) == 0 {
		return fmt.Errorf("empty data provided")
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(data, &metadata); err != nil {
		logging.LogError("failed to unmarshal metadata for key %s: %v", key, err)
		return fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	// helper functions
	getString := func(field string) string {
		if val, ok := metadata[field]; ok {
			if str, ok := val.(string); ok {
				return str
			}
		}
		return ""
	}

	getTime := func(field string) *time.Time {
		if val, ok := metadata[field]; ok {
			if str, ok := val.(string); ok && str != "" {
				if t, err := time.Parse(time.RFC3339, str); err == nil {
					return &t
				}
			}
		}
		return nil
	}

	marshalArray := func(field string) string {
		if val, ok := metadata[field]; ok {
			if arr, ok := val.([]interface{}); ok && len(arr) > 0 {
				if data, err := json.Marshal(arr); err == nil {
					return string(data)
				}
			}
		}
		return ""
	}

	// handle PARA
	var paraProjects, paraAreas, paraResources, paraArchive string
	if para, ok := metadata["para"].(map[string]interface{}); ok {
		if projects, ok := para["projects"].([]interface{}); ok && len(projects) > 0 {
			if data, err := json.Marshal(projects); err == nil {
				paraProjects = string(data)
			}
		}
		if areas, ok := para["areas"].([]interface{}); ok && len(areas) > 0 {
			if data, err := json.Marshal(areas); err == nil {
				paraAreas = string(data)
			}
		}
		if resources, ok := para["resources"].([]interface{}); ok && len(resources) > 0 {
			if data, err := json.Marshal(resources); err == nil {
				paraResources = string(data)
			}
		}
		if archive, ok := para["archive"].([]interface{}); ok && len(archive) > 0 {
			if data, err := json.Marshal(archive); err == nil {
				paraArchive = string(data)
			}
		}
	}

	// handle size
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
		key,
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

	if err != nil {
		logging.LogError("failed to store metadata for key %s: %v", key, err)
		return err
	}

	logging.LogDebug("stored metadata for key: %s", key)
	return nil
}

// Delete removes metadata by key
func (ss *sqliteStorage) Delete(key string) error {
	ss.mutex.Lock()
	defer ss.mutex.Unlock()

	_, err := ss.db.Exec("DELETE FROM metadata WHERE path = ?", key)
	if err != nil {
		logging.LogError("failed to delete metadata for key %s: %v", key, err)
		return err
	}

	logging.LogDebug("deleted metadata for key: %s", key)
	return nil
}

// GetAll returns all metadata key-value pairs as JSON
func (ss *sqliteStorage) GetAll() (map[string][]byte, error) {
	ss.mutex.RLock()
	defer ss.mutex.RUnlock()

	result := make(map[string][]byte)

	rows, err := ss.db.Query("SELECT path FROM metadata")
	if err != nil {
		logging.LogError("failed to get all metadata paths: %v", err)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			logging.LogWarning("failed to scan path: %v", err)
			continue
		}

		data, err := ss.Get(path)
		if err != nil {
			logging.LogWarning("failed to get metadata for %s: %v", path, err)
			continue
		}
		if data != nil {
			result[path] = data
		}
	}

	if err := rows.Err(); err != nil {
		logging.LogError("error iterating metadata rows: %v", err)
		return nil, err
	}

	logging.LogDebug("retrieved %d metadata entries", len(result))
	return result, nil
}

// Exists checks if metadata key exists
func (ss *sqliteStorage) Exists(key string) bool {
	ss.mutex.RLock()
	defer ss.mutex.RUnlock()

	var exists bool
	err := ss.db.QueryRow("SELECT EXISTS(SELECT 1 FROM metadata WHERE path = ?)", key).Scan(&exists)
	return err == nil && exists
}

// GetBackendType returns the backend type
func (ss *sqliteStorage) GetBackendType() string {
	return "sqlite"
}
