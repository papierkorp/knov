package files

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"knov/internal/configmanager"
	"knov/internal/logging"
)

var searchDB *sql.DB

func IndexAllFiles() error {
	logging.LogDebug("start indexing all files")

	// Clear existing index
	searchDB.Exec("DELETE FROM search_index")

	files, err := GetAllFiles()
	if err != nil {
		return err
	}

	dataDir := configmanager.DataPath

	for _, file := range files {
		// Construct full path since file.Path is now relative
		fullPath := filepath.Join(dataDir, file.Path)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			logging.LogWarning("failed to read file for indexing: %s", file.Path)
			continue
		}

		// Metadata expects data/ prefix
		metadataPath := filepath.Join(dataDir, file.Path)
		metadata, _ := MetaDataGet(metadataPath)
		tags := ""
		if metadata != nil && len(metadata.Tags) > 0 {
			tags = strings.Join(metadata.Tags, " ")
		}

		_, err = searchDB.Exec(
			"INSERT INTO search_index(path, title, content, tags) VALUES(?, ?, ?, ?)",
			file.Path, file.Name, string(content), tags,
		)
		if err != nil {
			logging.LogWarning("failed to index file: %s - %v", file.Path, err)
		}
	}

	logging.LogInfo("indexed %d files", len(files))
	return nil
}

func InitSearch() error {
	dbPath := filepath.Join("config", "search.db")

	var err error
	searchDB, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		logging.LogError("failed to open search database: %v", err)
		return err
	}

	// Create regular table instead of FTS5
	_, err = searchDB.Exec(`
        CREATE TABLE IF NOT EXISTS search_index (
            path TEXT PRIMARY KEY,
            title TEXT,
            content TEXT,
            tags TEXT
        )
    `)
	if err != nil {
		logging.LogError("failed to create search table: %v", err)
		return err
	}

	logging.LogInfo("search database initialized")
	return nil
}

func SearchFiles(query string, limit int) ([]File, error) {
	if searchDB == nil {
		return nil, fmt.Errorf("search database not initialized")
	}

	if limit <= 0 {
		limit = 20
	}

	// Use LIKE for simple text search
	searchPattern := "%" + strings.ToLower(query) + "%"
	rows, err := searchDB.Query(`
        SELECT path FROM search_index 
        WHERE LOWER(title) LIKE ? OR LOWER(content) LIKE ? OR LOWER(tags) LIKE ?
        LIMIT ?`,
		searchPattern, searchPattern, searchPattern, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []File
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			continue
		}

		results = append(results, File{
			Name: filepath.Base(path),
			Path: path,
		})
	}

	return results, nil
}
