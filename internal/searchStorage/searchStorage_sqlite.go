// Package searchStorage - SQLite FTS5 backend implementation
package searchStorage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"

	"knov/internal/backup"
	"knov/internal/dbmigration"
	"knov/internal/logging"
	"knov/internal/pathutils"

	_ "modernc.org/sqlite"
)

const searchDBFile = "search.db"

// sqliteStorage implements SearchStorage interface using SQLite FTS5
type sqliteStorage struct {
	db     *sql.DB
	dbPath string
	mutex  sync.RWMutex
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
	dbPath := filepath.Join(searchDir, searchDBFile)

	// fix permissions on existing database file if it exists
	if _, err := os.Stat(dbPath); err == nil {
		if err := os.Chmod(dbPath, 0644); err != nil {
			logging.LogWarning(logging.KeyApp, "failed to fix search database permissions: %v", err)
		}
	}

	// open database with explicit read-write mode
	db, err := sql.Open("sqlite", dbPath+"?mode=rwc&_time_format=sqlite")
	if err != nil {
		return nil, fmt.Errorf("failed to open search database: %w", err)
	}

	// set pragmas for better performance and safety
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		logging.LogWarning(logging.KeyApp, "failed to set WAL mode for search: %v", err)
	}
	if _, err := db.Exec("PRAGMA synchronous=NORMAL"); err != nil {
		logging.LogWarning(logging.KeyApp, "failed to set synchronous mode for search: %v", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		logging.LogWarning(logging.KeyApp, "failed to set busy timeout for search: %v", err)
	}
	if _, err := db.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		logging.LogWarning(logging.KeyApp, "failed to checkpoint wal for search: %v", err)
	}

	storage := &sqliteStorage{
		db:     db,
		dbPath: dbPath,
	}

	if err := storage.initialize(); err != nil {
		db.Close()
		return nil, err
	}

	return storage, nil
}

// initialize runs all pending migrations for this storage.
func (ss *sqliteStorage) initialize() error {
	const version = 4
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
		{
			// move off `porter ascii` (folds only ASCII, so "Müller" never
			// matches "muller") to a unicode tokenizer that also strips
			// diacritics. The tokenizer is fixed at CREATE, so rebuild both FTS
			// tables from the *_content tables.
			Up:   retokenize("porter unicode61 remove_diacritics 2"),
			Down: retokenize("porter ascii"),
		},
	}
	if err := dbmigration.Migrate(ss.db, version, steps); err != nil {
		return fmt.Errorf("search storage migration failed: %w", err)
	}
	logging.LogDebug(logging.KeyApp, "search sqlite storage ready at version %d", version)
	return nil
}

// retokenize returns a migration step that drops both FTS5 tables and recreates
// them with the given tokenizer, repopulating from the content tables.
func retokenize(tokenizer string) func(*sql.Tx) error {
	return func(tx *sql.Tx) error {
		// full-corpus rebuild in one step - log the row count so a slow first
		// boot after upgrade isn't mistaken for a hang.
		var rows int
		_ = tx.QueryRow(`SELECT
			(SELECT count(*) FROM search_content) +
			(SELECT count(*) FROM deleted_search_content)`).Scan(&rows)
		logging.LogInfo(logging.KeyDBMigration, "search: rebuilding FTS index (%d rows) with tokenizer %q", rows, tokenizer)
		_, err := tx.Exec(fmt.Sprintf(`
			DROP TABLE IF EXISTS search_index;
			CREATE VIRTUAL TABLE search_index USING fts5(
				path UNINDEXED, content, tokenize='%[1]s'
			);
			INSERT INTO search_index (path, content)
				SELECT path, CAST(content AS TEXT) FROM search_content;

			DROP TABLE IF EXISTS deleted_search_index;
			CREATE VIRTUAL TABLE deleted_search_index USING fts5(
				path UNINDEXED, content, tokenize='%[1]s'
			);
			INSERT INTO deleted_search_index (path, content)
				SELECT path, CAST(content AS TEXT) FROM deleted_search_content;
		`, tokenizer))
		return err
	}
}

// indexKey normalizes the git path of a deleted file to the key of the deleted-files table - the
// live index is keyed by the docs-relative path itself (IndexFile).
func indexKey(path string) string {
	return pathutils.ToRelative(path)
}

// IndexFile indexes a file's content for search
func (ss *sqliteStorage) IndexFile(rel pathutils.DocsRel, content []byte) error {
	ss.mutex.Lock()
	defer ss.mutex.Unlock()

	path := rel.String()
	now := time.Now().UTC()

	// one transaction: a failed FTS insert must not leave a fresh indexed_at
	// behind, or the reindex would skip the file and it'd stay missing from FTS.
	tx, err := ss.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("INSERT OR REPLACE INTO search_content (path, content, indexed_at) VALUES (?, ?, ?)", path, content, now); err != nil {
		logging.LogError(logging.KeyApp, "failed to store search content for %s: %v", path, err)
		return err
	}

	// path is UNINDEXED, so INSERT OR REPLACE would append a dup, not replace it.
	if _, err := tx.Exec("DELETE FROM search_index WHERE path = ?", path); err != nil {
		logging.LogError(logging.KeyApp, "failed to clear old search index for %s: %v", path, err)
		return err
	}

	if _, err := tx.Exec("INSERT INTO search_index (path, content) VALUES (?, ?)", path, string(content)); err != nil {
		logging.LogError(logging.KeyApp, "failed to index file %s: %v", path, err)
		return err
	}

	if err := tx.Commit(); err != nil {
		logging.LogError(logging.KeyApp, "failed to commit search index for %s: %v", path, err)
		return err
	}

	logging.LogDebug(logging.KeyApp, "indexed file: %s", path)
	return nil
}

// GetIndexedAt returns the time a file was last indexed, or zero time if not indexed.
func (ss *sqliteStorage) GetIndexedAt(rel pathutils.DocsRel) (time.Time, error) {
	ss.mutex.RLock()
	defer ss.mutex.RUnlock()

	path := rel.String()
	var t time.Time
	err := ss.db.QueryRow("SELECT indexed_at FROM search_content WHERE path = ?", path).Scan(&t)
	if err == sql.ErrNoRows {
		return time.Time{}, nil
	}
	return t, err
}

// GetIndexedContent retrieves indexed content for a file
func (ss *sqliteStorage) GetIndexedContent(rel pathutils.DocsRel) ([]byte, error) {
	ss.mutex.RLock()
	defer ss.mutex.RUnlock()

	path := rel.String()
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
func (ss *sqliteStorage) DeleteIndexedContent(rel pathutils.DocsRel) error {
	ss.mutex.Lock()
	defer ss.mutex.Unlock()

	path := rel.String()
	// remove from FTS index
	_, err := ss.db.Exec("DELETE FROM search_index WHERE path = ?", path)
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to delete from search index %s: %v", path, err)
		return err
	}

	// remove from content table
	_, err = ss.db.Exec("DELETE FROM search_content WHERE path = ?", path)
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to delete search content %s: %v", path, err)
		return err
	}

	logging.LogDebug(logging.KeyApp, "deleted indexed content: %s", path)
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

const (
	// clause cap: a multi-KB paste otherwise builds a MATCH expression FTS5
	// rejects for complexity.
	maxSearchTokens = 32
	// rune cap: a long unbroken blob (base64, minified line) is one token the
	// clause cap never trims; dropped rather than truncated.
	maxSearchTokenRunes = 64
)

// buildMatchQueries turns a raw user string into two safe FTS5 MATCH
// expressions - every operator char is dropped while tokenizing, so the input
// can't cause a syntax error. strict ANDs every token (precise). widened is the
// fallback when strict finds nothing: exact phrase OR any token, last token
// prefix-matched so a half-typed word still hits.
//
// Deliberate: a symbol-only / lone-char query ("C++", "---") yields "" here, so
// ftsMatchSearch returns nothing and file search falls through to the trigram
// fallback. That trade is accepted.
func buildMatchQueries(raw string) (strict, widened string) {
	tokens := strings.FieldsFunc(raw, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	// drop tokens FTS5 can't use well (measured in runes, not bytes):
	//   - lone ascii chars ("c" from "C++"): floods strict, which counts as a
	//     hit and starves the trigram fallback. keep lone multi-byte runes (CJK).
	//   - over-long blobs: would exact-match nothing anyway.
	tokens = slices.DeleteFunc(tokens, func(t string) bool {
		r := []rune(t)
		return (len(r) == 1 && r[0] < 128) || len(r) > maxSearchTokenRunes
	})
	if len(tokens) == 0 {
		return "", ""
	}
	if len(tokens) > maxSearchTokens {
		logging.LogDebug(logging.KeyApp, "search query capped from %d to %d tokens", len(tokens), maxSearchTokens)
		tokens = tokens[:maxSearchTokens]
	}

	quoted := make([]string, len(tokens))
	for i, t := range tokens {
		quoted[i] = `"` + t + `"`
	}
	strict = strings.Join(quoted, " AND ")

	prefixed := append([]string(nil), quoted...)
	prefixed[len(prefixed)-1] += `*` // prefix-match the final token for as-you-type

	parts := make([]string, 0, len(prefixed)+1)
	if len(tokens) > 1 {
		parts = append(parts, `"`+strings.Join(tokens, " ")+`"`)
	}
	parts = append(parts, prefixed...)
	widened = strings.Join(parts, " OR ")
	return strict, widened
}

// ftsMatchSearch runs an FTS5 MATCH against indexTable (a literal table name,
// never user input), trying the conjunctive query first and widening to the
// disjunctive one only when it returns nothing.
func (ss *sqliteStorage) ftsMatchSearch(indexTable, query string, limit int) ([]SearchResult, error) {
	strict, widened := buildMatchQueries(query)
	if strict == "" {
		return nil, nil
	}
	results, err := ss.runFTSMatch(indexTable, strict, limit)
	if err != nil || len(results) > 0 {
		return results, err
	}
	return ss.runFTSMatch(indexTable, widened, limit)
}

// runFTSMatch runs one FTS5 MATCH, yielding paths best-first by bm25. indexTable
// is interpolated into SQL, so it is checked against the known tables, not
// trusted.
func (ss *sqliteStorage) runFTSMatch(indexTable, match string, limit int) ([]SearchResult, error) {
	if indexTable != "search_index" && indexTable != "deleted_search_index" {
		return nil, fmt.Errorf("unknown search index table %q", indexTable)
	}
	sqlQuery := fmt.Sprintf(
		"SELECT path FROM %[1]s WHERE %[1]s MATCH ? ORDER BY bm25(%[1]s) LIMIT ?",
		indexTable,
	)

	rows, err := ss.db.Query(sqlQuery, match, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var result SearchResult
		if err := rows.Scan(&result.Path); err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, rows.Err()
}

// SearchContent performs full-text search using FTS5
func (ss *sqliteStorage) SearchContent(query string, limit int) ([]SearchResult, error) {
	ss.mutex.RLock()
	defer ss.mutex.RUnlock()

	results, err := ss.ftsMatchSearch("search_index", query, limit)
	if err != nil {
		logging.LogWarning(logging.KeyApp, "search query '%s' failed: %v", query, err)
		return nil, err
	}
	logging.LogDebug(logging.KeyApp, "search query '%s' returned %d results", query, len(results))
	return results, nil
}

// IndexDeletedFile indexes a deleted file's pre-deletion content in the
// separate deleted-files FTS table, so content search over deleted files
// doesn't need to walk the commit log.
func (ss *sqliteStorage) IndexDeletedFile(path string, content []byte) error {
	ss.mutex.Lock()
	defer ss.mutex.Unlock()

	path = indexKey(path)
	now := time.Now().UTC()

	tx, err := ss.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("INSERT OR REPLACE INTO deleted_search_content (path, content, indexed_at) VALUES (?, ?, ?)", path, content, now); err != nil {
		logging.LogError(logging.KeyApp, "failed to store deleted search content for %s: %v", path, err)
		return err
	}

	// path is UNINDEXED, so INSERT OR REPLACE would append a dup, not replace it.
	if _, err := tx.Exec("DELETE FROM deleted_search_index WHERE path = ?", path); err != nil {
		logging.LogError(logging.KeyApp, "failed to clear old deleted search index for %s: %v", path, err)
		return err
	}

	if _, err := tx.Exec("INSERT INTO deleted_search_index (path, content) VALUES (?, ?)", path, string(content)); err != nil {
		logging.LogError(logging.KeyApp, "failed to index deleted file %s: %v", path, err)
		return err
	}

	if err := tx.Commit(); err != nil {
		logging.LogError(logging.KeyApp, "failed to commit deleted search index for %s: %v", path, err)
		return err
	}

	logging.LogDebug(logging.KeyApp, "indexed deleted file: %s", path)
	return nil
}

// SearchDeletedContent performs full-text search over deleted files' pre-deletion content
func (ss *sqliteStorage) SearchDeletedContent(query string, limit int) ([]SearchResult, error) {
	ss.mutex.RLock()
	defer ss.mutex.RUnlock()

	results, err := ss.ftsMatchSearch("deleted_search_index", query, limit)
	if err != nil {
		logging.LogWarning(logging.KeyApp, "deleted-file search query '%s' failed: %v", query, err)
		return nil, err
	}
	logging.LogDebug(logging.KeyApp, "deleted-file search query '%s' returned %d results", query, len(results))
	return results, nil
}

// GetBackendType returns the backend type
func (ss *sqliteStorage) GetBackendType() string {
	return "sqlite-fts5"
}

// Backup snapshots the search index database into destDir via VACUUM INTO.
func (ss *sqliteStorage) Backup(destDir string) error {
	ss.mutex.RLock()
	defer ss.mutex.RUnlock()
	return backup.BackupSQLite(ss.db, destDir, searchDBFile)
}

// Restore overwrites the live search index database file with a previously backed-up snapshot
// from srcDir. The backup is read and validated before the current db handle is closed, so a bad
// backup fails without leaving this storage without a usable connection. Closing first (once
// validated) matters on Windows, where overwriting a file still opened by this process fails
// with a sharing violation. Safe only because the caller restarts the app right after a
// successful restore; this connection is never used again in this process.
func (ss *sqliteStorage) Restore(srcDir string) error {
	ss.mutex.Lock()
	defer ss.mutex.Unlock()
	data, err := backup.ReadSQLiteBackup(srcDir, searchDBFile)
	if err != nil {
		return err
	}
	if err := ss.db.Close(); err != nil {
		logging.LogWarning(logging.KeyApp, "search restore: failed to close db: %v", err)
	}
	return backup.RestoreSQLite(data, ss.dbPath)
}

// Close closes the database, see backup.CloseStorage.
func (ss *sqliteStorage) Close() error { return ss.db.Close() }
