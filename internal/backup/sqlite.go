package backup

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"knov/internal/logging"
	"knov/internal/utils"
)

// sqliteMagic is the fixed 16-byte header every valid SQLite database file starts with.
var sqliteMagic = []byte("SQLite format 3\x00")

// BackupSQLite snapshots db into destDir/filename via VACUUM INTO. Callers must hold their
// storage's own read lock around this call for a consistent snapshot - VACUUM INTO takes none
// of its own.
func BackupSQLite(db *sql.DB, destDir, filename string) error {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}
	dest := filepath.Join(destDir, filename)
	os.Remove(dest) // VACUUM INTO refuses to overwrite an existing file
	if _, err := db.Exec("VACUUM INTO ?", dest); err != nil {
		return fmt.Errorf("VACUUM INTO %s: %w", dest, err)
	}
	return nil
}

// ReadSQLiteBackup reads and validates filename from srcDir as a restorable sqlite database,
// returning its contents. Callers must do this before closing their live *sql.DB to swap it out -
// otherwise a missing or corrupt backup file closes a working connection for nothing, bricking
// that storage until the app is manually restarted.
func ReadSQLiteBackup(srcDir, filename string) ([]byte, error) {
	src := filepath.Join(srcDir, filename)
	data, err := os.ReadFile(src)
	if err != nil {
		return nil, fmt.Errorf("failed to read backed-up database %s: %w", src, err)
	}
	if len(data) < len(sqliteMagic) || string(data[:len(sqliteMagic)]) != string(sqliteMagic) {
		return nil, fmt.Errorf("%s is not a valid sqlite database", src)
	}
	return data, nil
}

// RestoreSQLite writes data (from a prior ReadSQLiteBackup) onto destPath. No live *sql.DB is
// touched - the caller is expected to restart the app afterwards so a fresh connection opens the
// restored file, same as the existing "data path changed, needs restart" flow. Also best-effort
// removes any leftover -wal/-shm sidecar files next to destPath from the database being replaced
// - VACUUM INTO never produces these (its output is always a clean, checkpointed file), so the
// pre-restore backup never had them, but the live db being overwritten may.
func RestoreSQLite(data []byte, destPath string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}
	if err := utils.WriteFileAtomic(destPath, data, 0644); err != nil {
		return err
	}
	// Best-effort: destPath itself was already restored successfully above, so a leftover
	// sidecar failing to delete (e.g. still held open) must not fail the whole restore over it -
	// the next fresh *sql.DB to open destPath after restart checkpoints/replaces it regardless.
	for _, sidecar := range []string{destPath + "-wal", destPath + "-shm"} {
		if err := os.Remove(sidecar); err != nil && !os.IsNotExist(err) {
			logging.LogWarning(logging.KeyApp, "restore: failed to remove stale %s: %v", sidecar, err)
		}
	}
	return nil
}
