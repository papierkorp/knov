// Package storage - SQLite cache storage implementation
package storage

import (
	"fmt"
	"strings"
	"time"

	"knov/internal/logging"
)

// SQLiteCacheStorage implements CacheStorage using SQLite with FTS5 search
type SQLiteCacheStorage struct {
	*baseSQLiteStorage
	hasFTS5 bool
}

// NewSQLiteCacheStorage creates a new SQLite cache storage
func NewSQLiteCacheStorage(dbPath string) (*SQLiteCacheStorage, error) {
	tables := map[string]TableDef{
		"cache": {
			Columns: []ColumnDef{
				{Name: "key", Type: "TEXT", Primary: true, NotNull: true},
				{Name: "value", Type: "BLOB", NotNull: true},
				{Name: "updated_at", Type: "INTEGER", NotNull: true, Index: true},
			},
		},
		"metadata_tags": {
			Columns: []ColumnDef{
				{Name: "path", Type: "TEXT", Primary: true, NotNull: true},
				{Name: "tag", Type: "TEXT", Primary: true, NotNull: true},
			},
			Indexes: []IndexDef{
				{Name: "idx_tags_tag", Columns: []string{"tag"}},
			},
		},
		"metadata_folders": {
			Columns: []ColumnDef{
				{Name: "path", Type: "TEXT", Primary: true, NotNull: true},
				{Name: "folder", Type: "TEXT", Primary: true, NotNull: true},
			},
			Indexes: []IndexDef{
				{Name: "idx_folders_folder", Columns: []string{"folder"}},
			},
		},
		"metadata_para_projects": {
			Columns: []ColumnDef{
				{Name: "path", Type: "TEXT", Primary: true, NotNull: true},
				{Name: "project", Type: "TEXT", Primary: true, NotNull: true},
			},
			Indexes: []IndexDef{
				{Name: "idx_para_projects", Columns: []string{"project"}},
			},
		},
		"metadata_para_areas": {
			Columns: []ColumnDef{
				{Name: "path", Type: "TEXT", Primary: true, NotNull: true},
				{Name: "area", Type: "TEXT", Primary: true, NotNull: true},
			},
			Indexes: []IndexDef{
				{Name: "idx_para_areas", Columns: []string{"area"}},
			},
		},
		"metadata_para_resources": {
			Columns: []ColumnDef{
				{Name: "path", Type: "TEXT", Primary: true, NotNull: true},
				{Name: "resource", Type: "TEXT", Primary: true, NotNull: true},
			},
			Indexes: []IndexDef{
				{Name: "idx_para_resources", Columns: []string{"resource"}},
			},
		},
		"metadata_para_archive": {
			Columns: []ColumnDef{
				{Name: "path", Type: "TEXT", Primary: true, NotNull: true},
				{Name: "archive", Type: "TEXT", Primary: true, NotNull: true},
			},
			Indexes: []IndexDef{
				{Name: "idx_para_archive", Columns: []string{"archive"}},
			},
		},
	}

	base, err := newBaseSQLiteStorage(dbPath, tables, "cache")
	if err != nil {
		return nil, err
	}

	storage := &SQLiteCacheStorage{
		baseSQLiteStorage: base,
	}

	// initialize base tables
	if err := base.Init(); err != nil {
		return nil, err
	}

	// create search index (FTS5 or fallback)
	if err := storage.initSearchIndex(); err != nil {
		return nil, err
	}

	return storage, nil
}

func (s *SQLiteCacheStorage) initSearchIndex() error {
	// try FTS5 first
	ftsQuery := `CREATE VIRTUAL TABLE IF NOT EXISTS search_index USING fts5(
		key UNINDEXED,
		content,
		tags,
		title,
		tokenize = 'porter unicode61'
	);`

	if _, err := s.db.Exec(ftsQuery); err != nil {
		logging.LogWarning("fts5 not available, using basic search: %v", err)
		s.hasFTS5 = false

		// create fallback table
		fallbackQuery := `
		CREATE TABLE IF NOT EXISTS search_index (
			key TEXT PRIMARY KEY,
			content TEXT,
			tags TEXT,
			title TEXT
		);
		CREATE INDEX IF NOT EXISTS idx_search_content ON search_index(content);
		CREATE INDEX IF NOT EXISTS idx_search_tags ON search_index(tags);
		CREATE INDEX IF NOT EXISTS idx_search_title ON search_index(title);
		`

		if _, err := s.db.Exec(fallbackQuery); err != nil {
			return fmt.Errorf("failed to create fallback search index: %w", err)
		}
	} else {
		s.hasFTS5 = true
		logging.LogInfo("fts5 search index enabled")
	}

	return nil
}

// Delete removes from cache and search index
func (s *SQLiteCacheStorage) Delete(key string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// delete from cache
	if _, err := s.db.Exec("DELETE FROM cache WHERE key = ?", key); err != nil {
		return fmt.Errorf("failed to delete key %s: %w", key, err)
	}

	// delete from search index
	if _, err := s.db.Exec("DELETE FROM search_index WHERE key = ?", key); err != nil {
		logging.LogWarning("failed to delete search index for key %s: %v", key, err)
	}

	logging.LogDebug("deleted data for key: %s", key)
	return nil
}

// Search performs full-text search
func (s *SQLiteCacheStorage) Search(query string, opts SearchOptions) (*SearchResults, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	start := time.Now()

	if opts.Limit == 0 {
		opts.Limit = 50
	}

	var results []*SearchResult

	if s.hasFTS5 {
		// use FTS5
		sqlQuery := `SELECT key, snippet(search_index, 1, '<mark>', '</mark>', '...', 32) as snippet,
		             rank FROM search_index WHERE search_index MATCH ?
		             ORDER BY rank LIMIT ? OFFSET ?`

		rows, err := s.db.Query(sqlQuery, query, opts.Limit, opts.Offset)
		if err != nil {
			return nil, fmt.Errorf("failed to search: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var key, snippet string
			var rank float64
			if err := rows.Scan(&key, &snippet, &rank); err != nil {
				continue
			}

			results = append(results, &SearchResult{
				Key:     key,
				Score:   -rank,
				Snippet: snippet,
			})
		}
	} else {
		// fallback to LIKE
		pattern := "%" + query + "%"
		sqlQuery := `SELECT key, content FROM search_index
		             WHERE content LIKE ? OR tags LIKE ? OR title LIKE ?
		             LIMIT ? OFFSET ?`

		rows, err := s.db.Query(sqlQuery, pattern, pattern, pattern, opts.Limit, opts.Offset)
		if err != nil {
			return nil, fmt.Errorf("failed to search: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var key, content string
			if err := rows.Scan(&key, &content); err != nil {
				continue
			}

			snippet := s.extractSnippet(content, query, 100)
			results = append(results, &SearchResult{
				Key:     key,
				Score:   1.0,
				Snippet: snippet,
			})
		}
	}

	took := time.Since(start)
	logging.LogDebug("search for '%s' returned %d results in %v", query, len(results), took)

	return &SearchResults{
		Results: results,
		Total:   len(results),
		Took:    took,
	}, nil
}

func (s *SQLiteCacheStorage) extractSnippet(content, query string, maxLen int) string {
	lowerContent := strings.ToLower(content)
	lowerQuery := strings.ToLower(query)

	pos := strings.Index(lowerContent, lowerQuery)
	if pos == -1 {
		if len(content) > maxLen {
			return content[:maxLen] + "..."
		}
		return content
	}

	start := pos - 40
	if start < 0 {
		start = 0
	}

	end := pos + len(query) + 40
	if end > len(content) {
		end = len(content)
	}

	snippet := content[start:end]
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(content) {
		snippet = snippet + "..."
	}

	return snippet
}

// IndexContent indexes content for search
func (s *SQLiteCacheStorage) IndexContent(key string, content []byte, metadata map[string]string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	contentStr := string(content)
	tags := metadata["tags"]
	title := metadata["title"]

	if s.hasFTS5 {
		query := `INSERT INTO search_index (key, content, tags, title) VALUES (?, ?, ?, ?)
		          ON CONFLICT(key) DO UPDATE SET content = ?, tags = ?, title = ?`
		_, err := s.db.Exec(query, key, contentStr, tags, title, contentStr, tags, title)
		return err
	}

	// fallback table
	query := `INSERT OR REPLACE INTO search_index (key, content, tags, title) VALUES (?, ?, ?, ?)`
	_, err := s.db.Exec(query, key, contentStr, tags, title)
	return err
}

// Rebuild recreates the search index
func (s *SQLiteCacheStorage) Rebuild() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, err := s.db.Exec("DELETE FROM search_index"); err != nil {
		return fmt.Errorf("failed to clear search index: %w", err)
	}

	logging.LogInfo("search index rebuilt")
	return nil
}

// Invalidate removes cache entries matching pattern
func (s *SQLiteCacheStorage) Invalidate(pattern string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	query := "DELETE FROM cache WHERE key LIKE ?"
	result, err := s.db.Exec(query, pattern)
	if err != nil {
		return fmt.Errorf("failed to invalidate pattern %s: %w", pattern, err)
	}

	rows, _ := result.RowsAffected()
	logging.LogDebug("invalidated %d cache entries matching pattern: %s", rows, pattern)
	return nil
}
