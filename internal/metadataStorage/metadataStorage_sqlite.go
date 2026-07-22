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

	"knov/internal/dbmigration"
	"knov/internal/logging"

	_ "modernc.org/sqlite"
)

// sqliteStorage implements MetadataStorage interface using SQLite
type sqliteStorage struct {
	db       *sql.DB
	basePath string
	mutex    sync.RWMutex
}

// newSQLiteStorage creates a new SQLite metadata storage instance
func newSQLiteStorage(storagePath string) (*sqliteStorage, error) {
	fullPath := filepath.Join(storagePath, "metadata")
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(fullPath, "metadata.db")

	// open database with explicit read-write mode
	db, err := sql.Open("sqlite", dbPath+"?mode=rwc")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// set pragmas for better performance and safety
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		logging.LogWarning(logging.KeyApp, "failed to set wal mode: %v", err)
	}
	if _, err := db.Exec("PRAGMA synchronous=NORMAL"); err != nil {
		logging.LogWarning(logging.KeyApp, "failed to set synchronous mode: %v", err)
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

// initialize runs all pending migrations for this storage.
// Bump version and append a step whenever the schema changes.
func (ss *sqliteStorage) initialize() error {
	const version = 4
	steps := []dbmigration.Migration{
		{Up: migrationV1Up, Down: migrationV1Down},
		{Up: migrationV2Up, Down: migrationV2Down},
		{Up: migrationV3Up, Down: migrationV3Down},
		{Up: migrationV4Up, Down: migrationV4Down},
	}
	if err := dbmigration.Migrate(ss.db, version, steps); err != nil {
		return fmt.Errorf("metadata storage migration failed: %w", err)
	}
	logging.LogDebug(logging.KeyApp, "metadata sqlite storage ready at version %d", version)
	return nil
}

func migrationV1Up(tx *sql.Tx) error {
	_, err := tx.Exec(`
	CREATE TABLE IF NOT EXISTS metadata (
		path TEXT PRIMARY KEY,
		title TEXT,
		created_at DATETIME,
		last_edited DATETIME,
		collection TEXT,
		folders TEXT,
		tags TEXT,
		ancestor TEXT,
		parents TEXT,
		kids TEXT,
		used_links TEXT,
		links_to_here TEXT,
		related TEXT,
		editor TEXT,
		size INTEGER,
		"references" TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_collection ON metadata(collection);
	CREATE INDEX IF NOT EXISTS idx_editor ON metadata(editor);
	`)
	return err
}

func migrationV1Down(tx *sql.Tx) error {
	_, err := tx.Exec(`DROP TABLE IF EXISTS metadata`)
	return err
}

func migrationV2Up(tx *sql.Tx) error {
	_, err := tx.Exec(`
        ALTER TABLE metadata ADD COLUMN conflict_file TEXT;
        ALTER TABLE metadata ADD COLUMN conflict_of TEXT;
    `)
	return err
}

func migrationV2Down(tx *sql.Tx) error {
	_, err := tx.Exec(`
        ALTER TABLE metadata DROP COLUMN conflict_file;
        ALTER TABLE metadata DROP COLUMN conflict_of;
    `)
	return err
}

func migrationV3Up(tx *sql.Tx) error {
	if _, err := tx.Exec(`ALTER TABLE metadata ADD COLUMN kanban_added_at DATETIME`); err != nil {
		return err
	}
	_, err := tx.Exec(`ALTER TABLE metadata ADD COLUMN kanban_moved_at DATETIME`)
	return err
}

func migrationV3Down(tx *sql.Tx) error {
	if _, err := tx.Exec(`ALTER TABLE metadata DROP COLUMN kanban_added_at`); err != nil {
		return err
	}
	_, err := tx.Exec(`ALTER TABLE metadata DROP COLUMN kanban_moved_at`)
	return err
}

func migrationV4Up(tx *sql.Tx) error {
	_, err := tx.Exec(`UPDATE metadata SET editor = 'toastui-editor' WHERE editor = 'markdown-editor'`)
	return err
}

func migrationV4Down(tx *sql.Tx) error {
	_, err := tx.Exec(`UPDATE metadata SET editor = 'markdown-editor' WHERE editor = 'toastui-editor'`)
	return err
}

// Get retrieves metadata by key and returns as JSON
func (ss *sqliteStorage) Get(key string) ([]byte, error) {
	ss.mutex.RLock()
	defer ss.mutex.RUnlock()

	query := `
	SELECT title, created_at, last_edited, collection,
	       folders, tags, ancestor, parents, kids, used_links, links_to_here, related,
	       editor, size, COALESCE("references", '') as "references",
	       COALESCE(conflict_file, '') as conflict_file, COALESCE(conflict_of, '') as conflict_of,
	       kanban_added_at, kanban_moved_at
	FROM metadata WHERE path = ?
	`

	var meta struct {
		Title         string
		CreatedAt     *time.Time
		LastEdited    *time.Time
		Collection    string
		Folders       string
		Tags          string
		Ancestor      string
		Parents       string
		Kids          string
		UsedLinks     string
		LinksToHere   string
		Related       string
		Editor        string
		Size          int64
		References    string
		ConflictFile  string
		ConflictOf    string
		KanbanAddedAt *time.Time
		KanbanMovedAt *time.Time
	}

	err := ss.db.QueryRow(query, key).Scan(
		&meta.Title, &meta.CreatedAt, &meta.LastEdited,
		&meta.Collection, &meta.Folders, &meta.Tags, &meta.Ancestor,
		&meta.Parents, &meta.Kids, &meta.UsedLinks, &meta.LinksToHere, &meta.Related,
		&meta.Editor, &meta.Size, &meta.References,
		&meta.ConflictFile, &meta.ConflictOf,
		&meta.KanbanAddedAt, &meta.KanbanMovedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to get metadata for key %s: %v", key, err)
		return nil, err
	}

	// convert to metadata JSON format
	result := map[string]interface{}{
		"path":       key,
		"title":      meta.Title,
		"collection": meta.Collection,
		"editor":     meta.Editor,
		"size":       meta.Size,
	}

	if meta.CreatedAt != nil {
		result["createdAt"] = meta.CreatedAt.Format(time.RFC3339)
	}
	if meta.LastEdited != nil {
		result["lastEdited"] = meta.LastEdited.Format(time.RFC3339)
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
	if meta.Related != "" {
		var related []string
		if err := json.Unmarshal([]byte(meta.Related), &related); err == nil {
			result["related"] = related
		}
	}

	if meta.References != "" {
		var refs []interface{}
		if err := json.Unmarshal([]byte(meta.References), &refs); err == nil {
			result["references"] = refs
		}
	}
	if meta.ConflictFile != "" {
		result["conflictFile"] = meta.ConflictFile
	}
	if meta.ConflictOf != "" {
		result["conflictOf"] = meta.ConflictOf
	}
	if meta.KanbanAddedAt != nil {
		result["kanbanAddedAt"] = meta.KanbanAddedAt.Format(time.RFC3339)
	}
	if meta.KanbanMovedAt != nil {
		result["kanbanMovedAt"] = meta.KanbanMovedAt.Format(time.RFC3339)
	}

	data, err := json.Marshal(result)
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to marshal metadata for key %s: %v", key, err)
		return nil, err
	}

	logging.LogDebug(logging.KeyApp, "retrieved metadata for key: %s", key)
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
		logging.LogError(logging.KeyApp, "failed to unmarshal metadata for key %s: %v", key, err)
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

	// handle size
	var size int64
	if val, ok := metadata["size"]; ok {
		if num, ok := val.(float64); ok {
			size = int64(num)
		}
	}

	// handle references
	var referencesJSON string
	if refs, ok := metadata["references"].([]interface{}); ok && len(refs) > 0 {
		if data, err := json.Marshal(refs); err == nil {
			referencesJSON = string(data)
		}
	}

	query := `
	INSERT OR REPLACE INTO metadata (
		path, title, created_at, last_edited, collection,
		folders, tags, ancestor, parents, kids, used_links, links_to_here, related,
		editor, size, "references", conflict_file, conflict_of,
		kanban_added_at, kanban_moved_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := ss.db.Exec(query,
		key,
		getString("title"),
		getTime("createdAt"),
		getTime("lastEdited"),
		getString("collection"),
		marshalArray("folders"),
		marshalArray("tags"),
		marshalArray("ancestor"),
		marshalArray("parents"),
		marshalArray("kids"),
		marshalArray("usedLinks"),
		marshalArray("linksToHere"),
		marshalArray("related"),
		getString("editor"),
		size,
		referencesJSON,
		getString("conflictFile"),
		getString("conflictOf"),
		getTime("kanbanAddedAt"),
		getTime("kanbanMovedAt"),
	)

	if err != nil {
		logging.LogError(logging.KeyApp, "failed to store metadata for key %s: %v", key, err)
		return err
	}

	logging.LogDebug(logging.KeyApp, "stored metadata for key: %s", key)
	return nil
}

// Delete removes metadata by key
func (ss *sqliteStorage) Delete(key string) error {
	ss.mutex.Lock()
	defer ss.mutex.Unlock()

	_, err := ss.db.Exec("DELETE FROM metadata WHERE path = ?", key)
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to delete metadata for key %s: %v", key, err)
		return err
	}

	logging.LogDebug(logging.KeyApp, "deleted metadata for key: %s", key)
	return nil
}

// GetAll returns all metadata key-value pairs as JSON
func (ss *sqliteStorage) GetAll() (map[string][]byte, error) {
	ss.mutex.RLock()
	defer ss.mutex.RUnlock()

	result := make(map[string][]byte)

	rows, err := ss.db.Query("SELECT path FROM metadata")
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to get all metadata paths: %v", err)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			logging.LogWarning(logging.KeyApp, "failed to scan path: %v", err)
			continue
		}

		data, err := ss.Get(path)
		if err != nil {
			logging.LogWarning(logging.KeyApp, "failed to get metadata for %s: %v", path, err)
			continue
		}
		if data != nil {
			result[path] = data
		}
	}

	if err := rows.Err(); err != nil {
		logging.LogError(logging.KeyApp, "error iterating metadata rows: %v", err)
		return nil, err
	}

	logging.LogDebug(logging.KeyApp, "retrieved %d metadata entries", len(result))
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

// Cleanup closes the db and removes the db file
func (ss *sqliteStorage) Cleanup() error {
	ss.mutex.Lock()
	defer ss.mutex.Unlock()

	if err := ss.db.Close(); err != nil {
		logging.LogWarning(logging.KeyApp, "sqlite metadata cleanup: failed to close db: %v", err)
	}

	dbPath := filepath.Join(ss.basePath, "metadata.db")
	for _, f := range []string{dbPath, dbPath + "-wal", dbPath + "-shm"} {
		if err := os.Remove(f); err != nil && !os.IsNotExist(err) {
			logging.LogError(logging.KeyApp, "sqlite metadata cleanup: failed to remove %s: %v", f, err)
			return err
		}
	}

	logging.LogInfo(logging.KeyApp, "sqlite metadata cleanup: removed db at %s", ss.basePath)
	return nil
}
