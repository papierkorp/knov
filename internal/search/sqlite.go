// Package search provides different search engine implementations
package search

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3" // sqlite full text search
	"knov/internal/configmanager"
	"knov/internal/files"
	"knov/internal/utils"
)

// SQLiteEngine ..
type SQLiteEngine struct {
	db *sql.DB
}

// Initialize ..
func (s *SQLiteEngine) Initialize() error {
	dbPath := filepath.Join("config", "search.db")
	var err error
	s.db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
        CREATE TABLE IF NOT EXISTS search_index (
            path TEXT PRIMARY KEY,
            title TEXT,
            content TEXT,
            tags TEXT
        )
    `)
	return err
}

// IndexAllFiles ..
func (s *SQLiteEngine) IndexAllFiles() error {
	s.db.Exec("DELETE FROM search_index")
	allFiles, err := files.GetAllFiles()
	if err != nil {
		return err
	}

	dataDir := configmanager.GetAppConfig().DataPath
	for _, file := range allFiles {
		fullPath := utils.ToFullPath(file.Path)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}

		metadata, _ := files.MetaDataGet(filepath.Join(dataDir, file.Path))
		tags := ""
		if metadata != nil && len(metadata.Tags) > 0 {
			tags = strings.Join(metadata.Tags, " ")
		}

		s.db.Exec("INSERT INTO search_index(path, title, content, tags) VALUES(?, ?, ?, ?)",
			file.Path, file.Name, string(content), tags)
	}
	return nil
}

// SearchFiles ..
func (s *SQLiteEngine) SearchFiles(query string, limit int) ([]files.File, error) {
	if limit <= 0 {
		limit = 20
	}

	searchPattern := "%" + strings.ToLower(query) + "%"
	rows, err := s.db.Query(`
        SELECT path FROM search_index 
        WHERE LOWER(title) LIKE ? OR LOWER(content) LIKE ? OR LOWER(tags) LIKE ?
        LIMIT ?`,
		searchPattern, searchPattern, searchPattern, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []files.File
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			continue
		}
		results = append(results, files.File{
			Name: filepath.Base(path),
			Path: path,
		})
	}
	return results, nil
}
