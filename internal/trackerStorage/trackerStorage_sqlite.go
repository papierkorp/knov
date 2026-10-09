package trackerStorage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"knov/internal/backup"
	"knov/internal/dbmigration"
	"knov/internal/logging"

	_ "modernc.org/sqlite"
)

const trackerDBFile = "tracker.db"

// sqliteStorage implements TrackerStorage using SQLite.
type sqliteStorage struct {
	db     *sql.DB
	dbPath string
	mutex  sync.RWMutex
}

// newSQLiteStorage creates a new SQLite tracker storage instance under storagePath/tracker.
func newSQLiteStorage(storagePath string) (*sqliteStorage, error) {
	dir := filepath.Join(storagePath, "tracker")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create tracker storage directory: %w", err)
	}
	dbPath := filepath.Join(dir, trackerDBFile)

	db, err := sql.Open("sqlite", dbPath+"?mode=rwc&_time_format=sqlite")
	if err != nil {
		return nil, fmt.Errorf("failed to open tracker database: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		logging.LogWarning(logging.KeyApp, "tracker storage: failed to set wal mode: %v", err)
	}
	if _, err := db.Exec("PRAGMA synchronous=NORMAL"); err != nil {
		logging.LogWarning(logging.KeyApp, "tracker storage: failed to set synchronous mode: %v", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		logging.LogWarning(logging.KeyApp, "tracker storage: failed to set busy timeout: %v", err)
	}

	s := &sqliteStorage{db: db, dbPath: dbPath}
	if err := s.initialize(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *sqliteStorage) initialize() error {
	const version = 1
	steps := []dbmigration.Migration{
		{
			Up: func(tx *sql.Tx) error {
				_, err := tx.Exec(`
				CREATE TABLE IF NOT EXISTS tracker_days (
					tracker_id TEXT NOT NULL,
					counter_id TEXT NOT NULL,
					day        TEXT NOT NULL,
					delta      INTEGER NOT NULL,
					PRIMARY KEY (tracker_id, counter_id, day)
				);
				`)
				return err
			},
			Down: func(tx *sql.Tx) error {
				_, err := tx.Exec(`DROP TABLE IF EXISTS tracker_days`)
				return err
			},
		},
	}
	if err := dbmigration.Migrate(s.db, version, steps); err != nil {
		return fmt.Errorf("tracker storage migration failed: %w", err)
	}
	logging.LogDebug(logging.KeyApp, "tracker sqlite storage ready at version %d", version)
	return nil
}

// GetDays returns counterID's recorded day->net-delta map, or an empty map if it has none.
func (s *sqliteStorage) GetDays(trackerID, counterID string) (map[string]int, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	rows, err := s.db.Query(`SELECT day, delta FROM tracker_days WHERE tracker_id = ? AND counter_id = ?`, trackerID, counterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	days := map[string]int{}
	for rows.Next() {
		var day string
		var delta int
		if err := rows.Scan(&day, &delta); err != nil {
			return nil, err
		}
		days[day] = delta
	}
	return days, rows.Err()
}

// AddDelta atomically adds delta to counterID's bucket for day.
func (s *sqliteStorage) AddDelta(trackerID, counterID, day string, delta int) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	_, err := s.db.Exec(`
		INSERT INTO tracker_days (tracker_id, counter_id, day, delta) VALUES (?, ?, ?, ?)
		ON CONFLICT(tracker_id, counter_id, day) DO UPDATE SET delta = delta + excluded.delta
	`, trackerID, counterID, day, delta)
	return err
}

// ResetCounter clears every recorded day for counterID.
func (s *sqliteStorage) ResetCounter(trackerID, counterID string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	_, err := s.db.Exec(`DELETE FROM tracker_days WHERE tracker_id = ? AND counter_id = ?`, trackerID, counterID)
	return err
}

// DeleteTracker clears every counter's days for trackerID.
func (s *sqliteStorage) DeleteTracker(trackerID string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	_, err := s.db.Exec(`DELETE FROM tracker_days WHERE tracker_id = ?`, trackerID)
	return err
}

// GetBackendType returns the backend type.
func (s *sqliteStorage) GetBackendType() string { return "sqlite" }

// Backup snapshots the tracker database into destDir via VACUUM INTO.
func (s *sqliteStorage) Backup(destDir string) error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return backup.BackupSQLite(s.db, destDir, trackerDBFile)
}

// Restore overwrites the live tracker database file with a previously backed-up snapshot from
// srcDir. The backup is read and validated before the current db handle is closed, so a bad
// backup fails without leaving this storage without a usable connection. Closing first (once
// validated) matters on Windows, where overwriting a file still opened by this process fails
// with a sharing violation. Safe only because the caller restarts the app right after a
// successful restore; this connection is never used again in this process.
func (s *sqliteStorage) Restore(srcDir string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	data, err := backup.ReadSQLiteBackup(srcDir, trackerDBFile)
	if err != nil {
		return err
	}
	if err := s.db.Close(); err != nil {
		logging.LogWarning(logging.KeyApp, "tracker restore: failed to close db: %v", err)
	}
	return backup.RestoreSQLite(data, s.dbPath)
}

// Close closes the database, see backup.CloseStorage.
func (s *sqliteStorage) Close() error { return s.db.Close() }
