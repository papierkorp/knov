// Package jobStorage - SQLite backend implementation
package jobStorage

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

type sqliteStorage struct {
	db    *sql.DB
	mutex sync.RWMutex
}

func newSQLiteStorage(storagePath string) (*sqliteStorage, error) {
	dir := filepath.Join(storagePath, "jobs")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create job storage directory: %w", err)
	}

	dbPath := filepath.Join(dir, "jobs.db")

	db, err := sql.Open("sqlite", dbPath+"?mode=rwc")
	if err != nil {
		return nil, fmt.Errorf("failed to open job database: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		logging.LogWarning(logging.KeyApp, "failed to set wal mode for jobs: %v", err)
	}
	if _, err := db.Exec("PRAGMA synchronous=NORMAL"); err != nil {
		logging.LogWarning(logging.KeyApp, "failed to set synchronous mode for jobs: %v", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		logging.LogWarning(logging.KeyApp, "failed to set busy timeout for jobs: %v", err)
	}
	if _, err := db.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		logging.LogWarning(logging.KeyApp, "failed to checkpoint wal for jobs: %v", err)
	}

	s := &sqliteStorage{db: db}
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
				CREATE TABLE IF NOT EXISTS jobs (
					id          TEXT PRIMARY KEY,
					type        TEXT NOT NULL,
					args        TEXT NOT NULL DEFAULT '',
					status      TEXT NOT NULL,
					started_at  DATETIME NOT NULL,
					finished_at DATETIME,
					error       TEXT NOT NULL DEFAULT ''
				);
				CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
				`)
				return err
			},
			Down: func(tx *sql.Tx) error {
				_, err := tx.Exec(`DROP TABLE IF EXISTS jobs`)
				return err
			},
		},
	}

	if err := dbmigration.Migrate(s.db, version, steps); err != nil {
		return fmt.Errorf("job storage migration failed: %w", err)
	}

	logging.LogDebug(logging.KeyApp, "job sqlite storage ready at version %d", version)
	return nil
}

func (s *sqliteStorage) Create(id, jobType, args string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	_, err := s.db.Exec(
		`INSERT INTO jobs (id, type, args, status, started_at) VALUES (?, ?, ?, ?, ?)`,
		id, jobType, args, StatusRunning, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("failed to insert job: %w", err)
	}
	return nil
}

func (s *sqliteStorage) UpdateStatus(id, status, errMsg string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	var finishedAt any
	if status != StatusRunning {
		finishedAt = time.Now()
	}

	_, err := s.db.Exec(
		`UPDATE jobs SET status = ?, error = ?, finished_at = ? WHERE id = ?`,
		status, errMsg, finishedAt, id,
	)
	if err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}
	return nil
}

func (s *sqliteStorage) Get(id string) (*JobRecord, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var r JobRecord
	err := s.db.QueryRow(
		`SELECT id, type, args, status, started_at, finished_at, error FROM jobs WHERE id = ?`, id,
	).Scan(&r.ID, &r.Type, &r.Args, &r.Status, &r.StartedAt, &r.FinishedAt, &r.Error)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}
	return &r, nil
}

func (s *sqliteStorage) ListRunning() ([]JobRecord, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	rows, err := s.db.Query(
		`SELECT id, type, args, status, started_at, finished_at, error FROM jobs WHERE status = ? ORDER BY started_at ASC`,
		StatusRunning,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query running jobs: %w", err)
	}
	defer rows.Close()

	var out []JobRecord
	for rows.Next() {
		var r JobRecord
		if err := rows.Scan(&r.ID, &r.Type, &r.Args, &r.Status, &r.StartedAt, &r.FinishedAt, &r.Error); err != nil {
			return nil, fmt.Errorf("failed to scan job: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func (s *sqliteStorage) GetBackendType() string {
	return "sqlite"
}
