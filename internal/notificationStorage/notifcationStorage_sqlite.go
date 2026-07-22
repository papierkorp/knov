// Package notificationStorage - SQLite backend implementation
package notificationStorage

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
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
	dir := filepath.Join(storagePath, "notifications")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create notification storage directory: %w", err)
	}

	dbPath := filepath.Join(dir, "notifications.db")

	db, err := sql.Open("sqlite", dbPath+"?mode=rwc")
	if err != nil {
		return nil, fmt.Errorf("failed to open notification database: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		logging.LogWarning(logging.KeyApp, "failed to set wal mode for notifications: %v", err)
	}
	if _, err := db.Exec("PRAGMA synchronous=NORMAL"); err != nil {
		logging.LogWarning(logging.KeyApp, "failed to set synchronous mode for notifications: %v", err)
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
				CREATE TABLE IF NOT EXISTS notifications (
					id         TEXT PRIMARY KEY,
					level      TEXT NOT NULL,
					message    TEXT NOT NULL,
					created_at DATETIME NOT NULL,
					pending    INTEGER NOT NULL DEFAULT 0
				);
				CREATE INDEX IF NOT EXISTS idx_notifications_created_at ON notifications(created_at);
				CREATE INDEX IF NOT EXISTS idx_notifications_pending ON notifications(pending);
				`)
				return err
			},
			Down: func(tx *sql.Tx) error {
				_, err := tx.Exec(`DROP TABLE IF EXISTS notifications`)
				return err
			},
		},
	}

	if err := dbmigration.Migrate(s.db, version, steps); err != nil {
		return fmt.Errorf("notification storage migration failed: %w", err)
	}

	logging.LogDebug(logging.KeyApp, "notification sqlite storage ready at version %d", version)
	return nil
}

func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), hex.EncodeToString(b))
}

func (s *sqliteStorage) Add(level, message string, pending bool) (*Notification, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	now := time.Now()
	id := generateID()

	pendingInt := 0
	if pending {
		pendingInt = 1
	}

	_, err := s.db.Exec(
		`INSERT INTO notifications (id, level, message, created_at, pending) VALUES (?, ?, ?, ?, ?)`,
		id, level, message, now, pendingInt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert notification: %w", err)
	}

	logging.LogDebug(logging.KeyApp, "stored notification: [%s] %s (pending=%v)", level, message, pending)
	return &Notification{ID: id, Level: level, Message: message, CreatedAt: now, Pending: pending}, nil
}

func (s *sqliteStorage) GetPending() (*Notification, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var n Notification
	err := s.db.QueryRow(
		`SELECT id, level, message, created_at FROM notifications WHERE pending = 1 ORDER BY created_at ASC LIMIT 1`,
	).Scan(&n.ID, &n.Level, &n.Message, &n.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get pending notification: %w", err)
	}
	n.Pending = true
	return &n, nil
}

func (s *sqliteStorage) ClearPending(id string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	_, err := s.db.Exec(`UPDATE notifications SET pending = 0 WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to clear pending notification: %w", err)
	}
	return nil
}

func (s *sqliteStorage) GetRecent(limit int) ([]Notification, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	rows, err := s.db.Query(
		`SELECT id, level, message, created_at, pending FROM notifications ORDER BY created_at DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query notifications: %w", err)
	}
	defer rows.Close()

	var notifications []Notification
	for rows.Next() {
		var n Notification
		var pendingInt int
		if err := rows.Scan(&n.ID, &n.Level, &n.Message, &n.CreatedAt, &pendingInt); err != nil {
			return nil, fmt.Errorf("failed to scan notification: %w", err)
		}
		n.Pending = pendingInt == 1
		notifications = append(notifications, n)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return notifications, nil
}

func (s *sqliteStorage) Purge(maxCount int, maxAgeDays int) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// remove entries older than maxAgeDays
	_, err := s.db.Exec(
		`DELETE FROM notifications WHERE created_at < datetime('now', ?)`,
		fmt.Sprintf("-%d days", maxAgeDays),
	)
	if err != nil {
		return fmt.Errorf("failed to purge old notifications: %w", err)
	}

	// enforce max count — keep the newest maxCount rows
	_, err = s.db.Exec(`
		DELETE FROM notifications
		WHERE id NOT IN (
			SELECT id FROM notifications ORDER BY created_at DESC LIMIT ?
		)`, maxCount,
	)
	if err != nil {
		return fmt.Errorf("failed to enforce notification count limit: %w", err)
	}

	logging.LogDebug(logging.KeyApp, "notification purge complete (max %d, max age %d days)", maxCount, maxAgeDays)
	return nil
}

func (s *sqliteStorage) DeleteByID(id string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, err := s.db.Exec(`DELETE FROM notifications WHERE id = ?`, id); err != nil {
		return fmt.Errorf("failed to delete notification: %w", err)
	}
	logging.LogDebug(logging.KeyApp, "deleted notification: %s", id)
	return nil
}

func (s *sqliteStorage) Clear() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, err := s.db.Exec(`DELETE FROM notifications`); err != nil {
		return fmt.Errorf("failed to clear notifications: %w", err)
	}

	logging.LogDebug(logging.KeyApp, "notifications cleared")
	return nil
}

func (s *sqliteStorage) GetBackendType() string {
	return "sqlite"
}
