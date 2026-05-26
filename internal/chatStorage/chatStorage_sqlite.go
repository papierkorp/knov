// Package chatStorage - SQLite backend implementation
package chatStorage

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"knov/internal/logging"

	_ "modernc.org/sqlite"
)

type sqliteStorage struct {
	db    *sql.DB
	mutex sync.RWMutex
}

func newSQLiteStorage(storagePath string) (*sqliteStorage, error) {
	chatDir := filepath.Join(storagePath, "chat")
	if err := os.MkdirAll(chatDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create chat storage directory: %w", err)
	}

	dbPath := filepath.Join(chatDir, "chat.db")

	db, err := sql.Open("sqlite", dbPath+"?mode=rwc")
	if err != nil {
		return nil, fmt.Errorf("failed to open chat database: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		logging.LogWarning("failed to set wal mode for chat: %v", err)
	}
	if _, err := db.Exec("PRAGMA synchronous=NORMAL"); err != nil {
		logging.LogWarning("failed to set synchronous mode for chat: %v", err)
	}

	s := &sqliteStorage{db: db}
	if err := s.initialize(); err != nil {
		db.Close()
		return nil, err
	}

	return s, nil
}

func (s *sqliteStorage) initialize() error {
	query := `
	CREATE TABLE IF NOT EXISTS messages (
		id          TEXT PRIMARY KEY,
		content     TEXT NOT NULL,
		created_at  DATETIME NOT NULL,
		updated_at  DATETIME NOT NULL,
		file_path   TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_messages_file_path ON messages(file_path);
	CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at);
	`
	if _, err := s.db.Exec(query); err != nil {
		return fmt.Errorf("failed to initialize chat tables: %w", err)
	}
	logging.LogDebug("chat sqlite tables initialized")
	return nil
}

func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), hex.EncodeToString(b))
}

func (s *sqliteStorage) Add(content, filePath string) (*Message, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	now := time.Now()
	id := generateID()

	var fp *string
	if filePath != "" {
		fp = &filePath
	}

	_, err := s.db.Exec(
		`INSERT INTO messages (id, content, created_at, updated_at, file_path) VALUES (?, ?, ?, ?, ?)`,
		id, content, now, now, fp,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert message: %w", err)
	}

	logging.LogDebug("added chat message: %s", id)
	return &Message{ID: id, Content: content, CreatedAt: now, UpdatedAt: now, FilePath: filePath}, nil
}

func (s *sqliteStorage) Delete(id string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	_, err := s.db.Exec(`DELETE FROM messages WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}
	logging.LogDebug("deleted chat message: %s", id)
	return nil
}

func (s *sqliteStorage) GetByID(id string) (*Message, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var m Message
	var fp sql.NullString
	err := s.db.QueryRow(
		`SELECT id, content, created_at, updated_at, file_path FROM messages WHERE id = ?`, id,
	).Scan(&m.ID, &m.Content, &m.CreatedAt, &m.UpdatedAt, &fp)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get message: %w", err)
	}
	if fp.Valid {
		m.FilePath = fp.String
	}
	return &m, nil
}

func (s *sqliteStorage) GetPage(filePath string, limit, offset int) ([]Message, int, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var total int
	var countRow *sql.Row

	if filePath == "" {
		countRow = s.db.QueryRow(`SELECT COUNT(*) FROM messages WHERE file_path IS NULL`)
	} else {
		countRow = s.db.QueryRow(`SELECT COUNT(*) FROM messages WHERE file_path = ?`, filePath)
	}
	if err := countRow.Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count messages: %w", err)
	}

	var (
		rows *sql.Rows
		err  error
	)
	if filePath == "" {
		rows, err = s.db.Query(
			`SELECT id, content, created_at, updated_at, COALESCE(file_path, '') FROM messages
			 WHERE file_path IS NULL ORDER BY created_at DESC LIMIT ? OFFSET ?`,
			limit, offset,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT id, content, created_at, updated_at, COALESCE(file_path, '') FROM messages
			 WHERE file_path = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`,
			filePath, limit, offset,
		)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query messages: %w", err)
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.Content, &m.CreatedAt, &m.UpdatedAt, &m.FilePath); err != nil {
			return nil, 0, fmt.Errorf("failed to scan message: %w", err)
		}
		messages = append(messages, m)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return messages, total, nil
}

func (s *sqliteStorage) GetBackendType() string {
	return "sqlite"
}
