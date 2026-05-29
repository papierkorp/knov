// Package dbmigration provides minimal version-based schema migrations for sqlite.
//
// Each storage owns its own version counter and migration list; this package
// only tracks the current version and runs the steps needed to reach a target.
package dbmigration

import (
	"database/sql"
	"fmt"

	"knov/internal/logging"
)

// Migration is a single forward/backward schema step. Both run inside a
// transaction together with the version bump, so a failure leaves the db
// untouched. Down may be nil if a step is irreversible.
type Migration struct {
	Up   func(*sql.Tx) error
	Down func(*sql.Tx) error
}

// Migrate moves the database to target by running the missing Up steps (when
// the db is behind) or Down steps in reverse (when the db is ahead).
// len(migrations) must equal the highest version number used as target.
func Migrate(db *sql.DB, target int, migrations []Migration) error {
	if target < 0 || target > len(migrations) {
		return fmt.Errorf("invalid target version %d (have %d migrations)", target, len(migrations))
	}

	if err := ensureVersionTable(db); err != nil {
		return err
	}

	current, err := getVersion(db)
	if err != nil {
		return err
	}

	switch {
	case current < target:
		for i := current; i < target; i++ {
			if err := runStep(db, i, i+1, migrations[i].Up); err != nil {
				return err
			}
		}
	case current > target:
		for i := current; i > target; i-- {
			step := migrations[i-1].Down
			if step == nil {
				return fmt.Errorf("migration %d→%d has no down step", i, i-1)
			}
			if err := runStep(db, i, i-1, step); err != nil {
				return err
			}
		}
	}

	return nil
}

// runStep executes one migration and its version bump atomically.
func runStep(db *sql.DB, from, to int, step func(*sql.Tx) error) error {
	logging.LogInfo("db migration %d→%d", from, to)

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	if err := step(tx); err != nil {
		tx.Rollback()
		return fmt.Errorf("migration %d→%d failed: %w", from, to, err)
	}

	if _, err := tx.Exec(`UPDATE schema_version SET version = ?`, to); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to set version %d: %w", to, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit migration %d→%d: %w", from, to, err)
	}

	logging.LogInfo("db migration %d→%d done", from, to)
	return nil
}

// ensureVersionTable creates the version table and seeds it with 0 if empty.
func ensureVersionTable(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)`); err != nil {
		return fmt.Errorf("failed to create schema_version table: %w", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_version`).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		if _, err := db.Exec(`INSERT INTO schema_version (version) VALUES (0)`); err != nil {
			return err
		}
	}
	return nil
}

// getVersion returns the current schema version stored in the database.
func getVersion(db *sql.DB) (int, error) {
	var v int
	if err := db.QueryRow(`SELECT version FROM schema_version`).Scan(&v); err != nil {
		return 0, fmt.Errorf("failed to read schema version: %w", err)
	}
	return v, nil
}
