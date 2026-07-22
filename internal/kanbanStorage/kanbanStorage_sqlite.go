package kanbanStorage

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

type sqliteKanbanStorage struct {
	db    *sql.DB
	mutex sync.RWMutex
}

func newSQLiteStorage(storagePath string) (*sqliteKanbanStorage, error) {
	fullPath := filepath.Join(storagePath, "kanban")
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(fullPath, "events.db")
	db, err := sql.Open("sqlite", dbPath+"?mode=rwc")
	if err != nil {
		return nil, fmt.Errorf("failed to open kanban events database: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		logging.LogWarning(logging.KeyApp, "kanban storage: failed to set wal mode: %v", err)
	}
	if _, err := db.Exec("PRAGMA synchronous=NORMAL"); err != nil {
		logging.LogWarning(logging.KeyApp, "kanban storage: failed to set synchronous mode: %v", err)
	}

	s := &sqliteKanbanStorage{db: db}
	if err := s.initialize(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *sqliteKanbanStorage) initialize() error {
	const version = 2
	steps := []dbmigration.Migration{
		{Up: migrationV1Up, Down: migrationV1Down},
		{Up: migrationV2Up, Down: migrationV2Down},
	}
	if err := dbmigration.Migrate(s.db, version, steps); err != nil {
		return fmt.Errorf("kanban storage migration failed: %w", err)
	}
	logging.LogDebug(logging.KeyApp, "kanban sqlite storage ready at version %d", version)
	return nil
}

func migrationV1Up(tx *sql.Tx) error {
	_, err := tx.Exec(`
	CREATE TABLE IF NOT EXISTS kanban_events (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		file_path   TEXT NOT NULL,
		collection  TEXT NOT NULL,
		from_status TEXT NOT NULL,
		to_status   TEXT NOT NULL,
		timestamp   DATETIME NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_kanban_events_collection ON kanban_events(collection);
	CREATE INDEX IF NOT EXISTS idx_kanban_events_file_path  ON kanban_events(file_path);
	`)
	return err
}

func migrationV1Down(tx *sql.Tx) error {
	_, err := tx.Exec(`DROP TABLE IF EXISTS kanban_events`)
	return err
}

// migrationV2Up renames the collection column to board_folder: kanban boards moved from
// auto-derived top-level collections to explicitly configured (and recursively matched)
// folders, so the column no longer holds a collection name.
func migrationV2Up(tx *sql.Tx) error {
	_, err := tx.Exec(`
	ALTER TABLE kanban_events RENAME COLUMN collection TO board_folder;
	DROP INDEX IF EXISTS idx_kanban_events_collection;
	CREATE INDEX IF NOT EXISTS idx_kanban_events_board_folder ON kanban_events(board_folder);
	`)
	return err
}

func migrationV2Down(tx *sql.Tx) error {
	_, err := tx.Exec(`
	ALTER TABLE kanban_events RENAME COLUMN board_folder TO collection;
	DROP INDEX IF EXISTS idx_kanban_events_board_folder;
	CREATE INDEX IF NOT EXISTS idx_kanban_events_collection ON kanban_events(collection);
	`)
	return err
}

func (s *sqliteKanbanStorage) LogEvent(filePath, boardFolder, fromStatus, toStatus string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	_, err := s.db.Exec(
		`INSERT INTO kanban_events (file_path, board_folder, from_status, to_status, timestamp) VALUES (?, ?, ?, ?, ?)`,
		filePath, boardFolder, fromStatus, toStatus, time.Now(),
	)
	return err
}

func (s *sqliteKanbanStorage) GetEvents(boardFolder, filePath string, from, to *time.Time, limit int) ([]Event, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	query := `SELECT file_path, board_folder, from_status, to_status, timestamp FROM kanban_events WHERE 1=1`
	args := []interface{}{}

	if boardFolder != "" {
		query += ` AND board_folder = ?`
		args = append(args, boardFolder)
	}
	if filePath != "" {
		query += ` AND file_path = ?`
		args = append(args, filePath)
	}
	if from != nil {
		query += ` AND timestamp >= ?`
		args = append(args, from)
	}
	if to != nil {
		query += ` AND timestamp <= ?`
		args = append(args, to)
	}

	query += ` ORDER BY timestamp DESC`

	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]Event, 0)
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.FilePath, &e.BoardFolder, &e.FromStatus, &e.ToStatus, &e.Timestamp); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}
