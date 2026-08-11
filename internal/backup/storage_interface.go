// Package backup provides type-aware snapshot/restore helpers for StoragePath-backed storages
// (metadata/cache/chat/kanban/notifications/config/search), plus the target abstraction and
// rotation used to keep a trimmed, restorable backup history. DataPath (docs/media) is already
// covered by git and is intentionally out of scope here.
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
