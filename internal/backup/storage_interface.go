// Package backup provides type-aware snapshot/restore helpers for StoragePath-backed storages
// (metadata/chat/kanban/notifications/config/search), plus the target abstraction and rotation
// used to keep a trimmed, restorable backup history. DataPath (docs/media) is already covered by
// git and is intentionally out of scope here, as is cache - see cacheStorage.CacheStorage's doc.
//
// Storages self-register via Register in their own init(), the same pattern externalsuite.go
// uses for test suites - this package must never import a storage package directly, since every
// storage imports this one for the Backup/Restore/File helpers below.
package backup

import (
	"sort"
	"sync"
)

// Storage is implemented by every backup-participating storage package.
// destDir/srcDir are that storage's own subdirectory within a backup set.
type Storage interface {
	Backup(destDir string) error
	Restore(srcDir string) error
	// GetBackendType identifies which backend (e.g. "sqlite", "json") is currently active, so a
	// backup set can record what produced it and Restore can tell whether the provider has since
	// changed (see Migratable).
	GetBackendType() string
}

// Migratable is implemented by storages whose data can be converted between backend types on
// the fly. Restore uses it when a backup set was made with a different backend than the one
// currently configured (e.g. a sqlite-era backup restored after switching to json) - applying
// such a backup as-is would silently corrupt or no-op the live storage, so Restore converts it
// instead of calling Restore directly.
type Migratable interface {
	// RestoreMigrate restores from srcDir - this storage's own subdirectory within the backup
	// set, the same directory Restore is given - produced by a backend of type fromBackendType,
	// converting the data into the currently active backend.
	//
	// touched reports whether the live backend was altered before err occurred, i.e. whether the
	// call passed its point of no return (typically a Cleanup/Flush of the live backend) - not
	// merely whether an error occurred. restore() uses it to decide whether ErrRestoreIncomplete
	// applies to this storage: a failure before touched would ever become true (e.g. the backup
	// couldn't even be opened) left the live backend untouched and is safe to just report, while
	// touched=true means the live backend may already be in a different state than before the
	// call, regardless of err, and recovery (restart/reinit) is required.
	RestoreMigrate(srcDir, fromBackendType string) (touched bool, err error)
}

// registryMu guards registry - storages normally only register once, at package init time, but
// backuptest also registers/unregisters a temporary failing Storage at runtime (see Unregister),
// concurrently with real Run/Restore calls (a scheduled auto-backup, or another request) reading
// it via RegisteredNames/lookupStorage. Without a lock that's a concurrent map read/write, which
// panics the whole process.
var (
	registryMu sync.RWMutex
	registry   = map[string]Storage{}
)

// Register wires a storage package into Run/Restore. name becomes that storage's subdirectory
// name inside every backup set.
func Register(name string, s Storage) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[name] = s
}

// Unregister removes name from the registry - used by backuptest to install and then remove a
// temporary failing Storage for the "partial failure discards the whole set" case, without
// permanently breaking every future full backup with a storage that no longer exists.
func Unregister(name string) {
	registryMu.Lock()
	defer registryMu.Unlock()
	delete(registry, name)
}

// RegisteredNames returns every storage name available for backup/restore, sorted - the set
// Run backs up when called with no explicit selection, and the choices a caller can pick a
// subset from (e.g. "just metadata").
func RegisteredNames() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// lookupStorage returns the registered Storage for name, if any.
func lookupStorage(name string) (Storage, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	s, ok := registry[name]
	return s, ok
}
