package files

import "sync"

// docsOpMu and mediaOpMu each serialize physical mutations (rename/move/delete) of paths under
// their respective root, process-wide - split in two since a docs-root operation and a
// media-root operation can never touch the same path, so there's no reason to make them wait on
// each other. Within a root, unlike metaLocks (per-path, for metadata read-modify-write),
// physical mutations are rare/interactive rather than a hot path, so one lock per root is
// simpler than per-path locking and also covers a single-file operation racing a folder-level
// one (e.g. MoveFolder relocating a directory while a kanban status change tries to move a file
// inside it), which a per-path lock wouldn't catch without hierarchical/prefix-aware locking.
var (
	docsOpMu  sync.Mutex
	mediaOpMu sync.Mutex
)

// lockDocsOp acquires the process-wide docs-root physical-file-mutation lock and returns the
// func that releases it - pair with `defer unlock()` around any rename/move/delete of a docs
// file on disk (stat check through the rename/remove itself). Unexported on purpose: every
// physical mutation of the docs root must go through a named function in this file
// (movePhysical/DeleteFileNoRefresh/removeDirPhysical/moveFolderPhysical) so the lock can't be
// forgotten by a call site outside this package.
func lockDocsOp() (unlock func()) {
	docsOpMu.Lock()
	return docsOpMu.Unlock
}

// lockMediaOp is lockDocsOp for the media root.
func lockMediaOp() (unlock func()) {
	mediaOpMu.Lock()
	return mediaOpMu.Unlock
}
