// Package files handles file operations and metadata.
//
// Metadata writes go only through MetaDataMutate (user-owned fields), MetaDataSync /
// MetaDataSyncNoRefresh (derived fields), or MetaDataDelete/MetaDataDeleteNoRefresh - never
// get+modify+save with a caller-held *Metadata. A fn passed to MetaDataMutate may only touch
// the *Metadata it received; it must never call MetaDataMutate on another path itself (return
// a fan-out closure and run it after the outer call returns instead).
package files

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"knov/internal/chat"
	"knov/internal/configmanager"
	"knov/internal/keylock"
	"knov/internal/logging"
	"knov/internal/metadataStorage"
	"knov/internal/pathutils"
	"knov/internal/searchStorage"
	"knov/internal/utils"
)

// metaLocks closes a lost-update race: two independent writers of the SAME path (e.g. a
// kanban move racing file-sync's per-changed-file metadata refresh) can each read the current
// record, compute their own change, and save it back - whichever save lands second silently
// reverts the first, since neither knew about the other. Every read-modify-write sequence on a
// given path (read current metadata, compute changes, save it back) must hold this path's lock
// for its whole span - see internal/keylock for how that's enforced.
var metaLocks = keylock.New()

// lockMetaPath acquires the write lock for a single metadata path and returns the func that
// releases it - pair with `defer unlock()` around a read-modify-write sequence for that path.
// Only call this for a path whose lock isn't already held by the current goroutine - it's not
// reentrant, and fan-out onto other paths must wait until this path's lock is released (see
// MetaDataSyncNoRefresh's fanOut handling) to avoid an ABBA deadlock against another goroutine
// doing the mirror-image update.
func lockMetaPath(path string) (unlock func()) {
	return metaLocks.Lock(pathutils.ToWithPrefix(path))
}

// MetaDataMutate is the only general write primitive for metadata: the sole way to change
// a user-owned field (tags, parents, editor, references, conflict_*, kanban timestamps) is
// through this function or a thin helper wrapping it (SetEditor, SetTags, SetParents,
// SetConflictFile, kanban.MoveCard, …) - never a caller-assembled *Metadata passed to a
// blind save. It loads the current record under this path's write lock (creating a bare one
// if none exists), passes it to fn along with whether it existed, and - if fn returns
// save=true - writes it back via metaDataSaveRaw before releasing the lock. Because the read,
// fn, and write all happen under one lock acquisition, a concurrent MetaDataMutate or
// MetaDataSync on the same path can never interleave and revert fn's change - see
// metaLocks. fn should only touch the *Metadata passed to it; mutating a different
// path's metadata from within fn (including calling MetaDataMutate again) risks deadlock,
// since these locks aren't reentrant - return fan-out closures and run them after this
// function returns instead (see updateLinksToHereFanOut/parentChildFanOut).
func MetaDataMutate(path string, fn func(m *Metadata, existed bool) (save bool, err error)) error {
	normalized := pathutils.ToWithPrefix(path)
	unlock := lockMetaPath(normalized)
	defer unlock()

	metadata, err := MetaDataGet(normalized)
	if err != nil {
		return err
	}
	existed := metadata != nil
	if !existed {
		metadata = &Metadata{Path: normalized}
	}

	save, err := fn(metadata, existed)
	if err != nil || !save {
		return err
	}
	return metaDataSaveRaw(metadata)
}

type EditorType string

const (
	EditorTypeFilter     EditorType = "filter-editor"
	EditorTypeList       EditorType = "list-editor"
	EditorTypeTodo       EditorType = "todo-editor"
	EditorTypeIndex      EditorType = "index-editor"
	EditorTypeCodeMirror EditorType = "codemirror-editor"
)

// typed count maps for metadata aggregations
type TagCount map[string]int
type CollectionCount map[string]int
type FolderCount map[string]int
type EditorTypeCount map[string]int

// AllEditorTypes returns all available editor types
func AllEditorTypes() []EditorType {
	return []EditorType{
		EditorTypeFilter,
		EditorTypeList,
		EditorTypeTodo,
		EditorTypeIndex,
		EditorTypeCodeMirror,
	}
}

// EditorFromExtension infers an editor type from a file extension.
// Returns empty string for generic/ambiguous extensions (e.g. .md).
func EditorFromExtension(path string) EditorType {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".filter":
		return EditorTypeFilter
	case ".list":
		return EditorTypeList
	case ".todo":
		return EditorTypeTodo
	case ".index", ".moc":
		return EditorTypeIndex
	case ".txt":
		return EditorTypeCodeMirror
	default:
		return ""
	}
}

// Metadata represents file metadata
type Metadata struct {
	Path          string      `json:"path"`                    // auto
	Title         string      `json:"title"`                   // auto
	CreatedAt     time.Time   `json:"createdAt"`               // auto
	LastEdited    time.Time   `json:"lastEdited"`              // auto
	Collection    string      `json:"collection"`              // auto
	Folders       []string    `json:"folders"`                 // auto
	Tags          []string    `json:"tags"`                    // manual
	Ancestor      []string    `json:"ancestor"`                // auto
	Parents       []string    `json:"parents"`                 // manual
	Kids          []string    `json:"kids"`                    // auto
	UsedLinks     []string    `json:"usedLinks"`               // auto
	LinksToHere   []string    `json:"linksToHere"`             // auto
	Related       []string    `json:"related,omitempty"`       // auto
	Editor        EditorType  `json:"editor"`                  // manual
	Size          int64       `json:"size"`                    // auto
	References    []Reference `json:"references,omitempty"`    // manual
	ConflictFile  string      `json:"conflictFile,omitempty"`  // auto
	ConflictOf    string      `json:"conflictOf,omitempty"`    // auto
	KanbanAddedAt time.Time   `json:"kanbanAddedAt,omitempty"` // auto
	KanbanMovedAt time.Time   `json:"kanbanMovedAt,omitempty"` // auto
}

// Reference represents an external resource linked to a file
type Reference struct {
	URL         string    `json:"url"`
	Description string    `json:"description"` // why this link was added
	AddedAt     time.Time `json:"addedAt,omitempty"`
}

// kanbanStatusFromTags extracts the kanban status value from a tag list; returns "" if absent.
func kanbanStatusFromTags(tags []string) string {
	prefix := configmanager.GetKanbanPrefix() + "-status-"
	for _, t := range tags {
		if strings.HasPrefix(t, prefix) {
			return strings.TrimPrefix(t, prefix)
		}
	}
	return ""
}

// applyKanbanTimestamps updates KanbanAddedAt/KanbanMovedAt when the kanban
// status tag transitions to a new (non-empty) value.
func applyKanbanTimestamps(m *Metadata, oldStatus string) {
	newStatus := kanbanStatusFromTags(m.Tags)
	if newStatus == "" || newStatus == oldStatus {
		return
	}
	now := time.Now()
	if m.KanbanAddedAt.IsZero() {
		m.KanbanAddedAt = now
	}
	m.KanbanMovedAt = now
}

// recomputeDerivedFields refreshes every Sync-owned field on metadata from the filesystem/
// content at metadata.Path (size, folders/collection, last-edited, default editor when empty,
// used links, ancestors, title) and returns fan-out closures for the LinksToHere edges of
// files it links to. User-owned fields (Tags, Parents, References, Editor when already set,
// ConflictFile, kanban timestamps) are left untouched. The caller must invoke the returned
// closures only after releasing metadata.Path's own write lock - see updateUsedLinks.
func recomputeDerivedFields(metadata *Metadata) []func() {
	isMediaFile := pathutils.IsMedia(metadata.Path)

	var fullPath string
	if isMediaFile {
		fullPath = pathutils.ToMediaPath(pathutils.ToRelative(metadata.Path))
	} else {
		fullPath = pathutils.ToDocsPath(pathutils.ToRelative(metadata.Path))
	}

	if fileInfo, err := os.Stat(fullPath); err != nil {
		logging.LogWarning(logging.KeyApp, "failed to get file size for %s: %v", fullPath, err)
	} else {
		metadata.Size = fileInfo.Size()
	}

	metadata.LastEdited = time.Now()

	folderPath := FolderFromPath(metadata.Path)
	if folderPath != "" {
		metadata.Folders = strings.Split(folderPath, "/")
	} else {
		metadata.Folders = []string{}
	}
	metadata.Collection = CollectionFromPath(metadata.Path)

	// only infer editor type for docs files — media files are identified
	// by path prefix + mime type in filtering, not by editor type
	if !isMediaFile && metadata.Editor == "" {
		if et := EditorFromExtension(metadata.Path); et != "" {
			metadata.Editor = et
		} else {
			metadata.Editor = EditorType(configmanager.DefaultMarkdownEditor.Get())
		}
	}

	// make sure required fields are initialized
	if metadata.Tags == nil {
		metadata.Tags = []string{}
	}
	if metadata.Parents == nil {
		metadata.Parents = []string{}
	}
	if metadata.Kids == nil {
		metadata.Kids = []string{}
	}
	if metadata.UsedLinks == nil {
		metadata.UsedLinks = []string{}
	}
	if metadata.LinksToHere == nil {
		metadata.LinksToHere = []string{}
	}
	if metadata.Ancestor == nil {
		metadata.Ancestor = []string{}
	}
	if metadata.Folders == nil {
		metadata.Folders = []string{}
	}

	updateAncestors(metadata, nil)
	fanOut := updateUsedLinks(metadata)
	updateTitle(metadata)
	return fanOut
}

// MetaDataSync recomputes path's Sync-owned derived fields (size, folders, collection,
// used links, ancestors, title, default editor when empty) from the filesystem/content,
// then refreshes the aggregate caches. It never accepts caller-supplied field values, so
// unlike the old get→modify→save pattern, a concurrent MetaDataMutate on the same path
// (e.g. a kanban move) can never be reverted by a stale sync - see metaLocks. For
// syncing many files in one batch, use MetaDataSyncNoRefresh in the loop and RefreshCaches()
// once afterwards instead.
func MetaDataSync(path string) error {
	if err := MetaDataSyncNoRefresh(path); err != nil {
		return err
	}
	RefreshCaches()
	return nil
}

// MetaDataSyncNoRefresh is MetaDataSync without the aggregate cache refresh. See MetaDataSync.
func MetaDataSyncNoRefresh(path string) error {
	normalized := pathutils.ToWithPrefix(path)
	unlock := lockMetaPath(normalized)

	metadata, err := MetaDataGet(normalized)
	if err != nil {
		unlock()
		return err
	}
	if metadata == nil {
		metadata = &Metadata{Path: normalized, CreatedAt: time.Now()}
	}

	fanOut := recomputeDerivedFields(metadata)
	err = metaDataSaveRaw(metadata)
	unlock()
	if err != nil {
		return err
	}

	for _, apply := range fanOut {
		apply()
	}
	return nil
}

// SanitizeKanbanTags ensures at most one kanban status tag is present, that
// it is in the configured allowlist, and that no unknown kanban-prefixed tags
// (e.g. kb-anything-unknown) are accepted.
// Returns the cleaned tag list and an error describing any removed tags.
func SanitizeKanbanTags(tags []string) ([]string, error) {
	return sanitizeKanbanTags(tags)
}

// sanitizeKanbanTags is the internal implementation.
func sanitizeKanbanTags(tags []string) ([]string, error) {
	validStatuses := configmanager.GetKanbanStatuses()
	prefix := configmanager.GetKanbanPrefix()
	prefixDash := prefix + "-"

	// known sub-namespaces under the prefix (extend this list as new ones are added)
	knownSubNamespaces := []string{"status"}

	var kanbanTag string
	var invalidTags []string
	var nonKanban []string

	for _, t := range tags {
		if !strings.HasPrefix(t, prefixDash) {
			nonKanban = append(nonKanban, t)
			continue
		}

		// tag starts with kb- — determine sub-namespace
		rest := strings.TrimPrefix(t, prefixDash) // e.g. "status-inbox" or "foo-bar"
		parts := strings.SplitN(rest, "-", 2)
		subNamespace := parts[0] // "status", "foo", etc.

		// reject unknown sub-namespaces entirely
		known := false
		for _, ns := range knownSubNamespaces {
			if subNamespace == ns {
				known = true
				break
			}
		}
		if !known {
			invalidTags = append(invalidTags, t)
			continue
		}

		// for kb-status-* validate against the allowlist
		if subNamespace == "status" {
			statusValue := ""
			if len(parts) == 2 {
				statusValue = parts[1]
			}
			valid := false
			for _, s := range validStatuses {
				if s == statusValue {
					valid = true
					break
				}
			}
			if valid {
				kanbanTag = t // last valid one wins
			} else {
				invalidTags = append(invalidTags, t)
			}
		}
	}

	result := nonKanban
	if kanbanTag != "" {
		result = append(result, kanbanTag)
	}

	if len(invalidTags) > 0 {
		return result, fmt.Errorf("invalid kanban tag(s) removed: %s (allowed statuses: %s)",
			strings.Join(invalidTags, ", "), strings.Join(validStatuses, ", "))
	}
	return result, nil
}

// SetConflictFile sets the conflict file path on an original file's metadata.
// Overwrites any previous conflict file reference — only one is kept at a time. Errors if
// originalFilePath has no metadata yet — its caller always names an already-tracked file, so a
// miss means a bad path rather than something to paper over with a phantom record.
func SetConflictFile(originalFilePath, conflictFilePath string) error {
	return MetaDataMutate(originalFilePath, func(m *Metadata, existed bool) (bool, error) {
		if !existed {
			return false, fmt.Errorf("metadata not found for %s", originalFilePath)
		}
		m.ConflictFile = conflictFilePath
		return true, nil
	})
}

// SetConflictOf marks a file as being a conflict copy of another file. Unlike SetConflictFile,
// this auto-creates a bare record when conflictFilePath has none: its caller (git.HandleConflict)
// writes the conflict copy straight to disk without ever syncing metadata for it first, so this
// is the only place that record gets created.
func SetConflictOf(conflictFilePath, originalFilePath string) error {
	return MetaDataMutate(conflictFilePath, func(m *Metadata, existed bool) (bool, error) {
		m.ConflictOf = originalFilePath
		return true, nil
	})
}

// ClearConflictFile removes the conflict file reference from the original file's metadata.
func ClearConflictFile(originalFilePath string) error {
	return MetaDataMutate(originalFilePath, func(m *Metadata, existed bool) (bool, error) {
		if !existed {
			return false, nil
		}
		m.ConflictFile = ""
		return true, nil
	})
}

// SetEditor sets the editor type for path unconditionally (user override - unlike
// MetaDataSync's "default when empty" rule, this always takes effect regardless of the
// current value). Errors if path has no metadata yet - callers change an existing file's
// editor, they don't create metadata as a side effect.
func SetEditor(path string, editor EditorType) error {
	if err := SetEditorNoRefresh(path, editor); err != nil {
		return err
	}
	RefreshCaches()
	return nil
}

// SetEditorNoRefresh is SetEditor without the aggregate cache refresh - for callers that set
// several fields or loop over many files in one request, so use SetEditor and call
// RefreshCaches() once afterwards instead of paying for a full cache rebuild per field/file.
func SetEditorNoRefresh(path string, editor EditorType) error {
	return MetaDataMutate(path, func(m *Metadata, existed bool) (bool, error) {
		if !existed {
			return false, fmt.Errorf("metadata not found for %s", path)
		}
		m.Editor = editor
		return true, nil
	})
}

// SetTags sanitizes and sets path's tags, applying kanban add/moved timestamps on status
// transitions the same way the old field-merge path did.
func SetTags(path string, tags []string) error {
	if err := SetTagsNoRefresh(path, tags); err != nil {
		return err
	}
	RefreshCaches()
	return nil
}

// SetTagsNoRefresh is SetTags without the aggregate cache refresh. See SetEditorNoRefresh.
func SetTagsNoRefresh(path string, tags []string) error {
	return MetaDataMutate(path, func(m *Metadata, existed bool) (bool, error) {
		oldKanbanStatus := kanbanStatusFromTags(m.Tags)
		cleaned, err := sanitizeKanbanTags(tags)
		if err != nil {
			logging.LogWarning(logging.KeyApp, "tag sanitization for %s: %v", path, err)
		}
		m.Tags = cleaned
		applyKanbanTimestamps(m, oldKanbanStatus)
		return true, nil
	})
}

// PatchTags adds and/or removes tags under the path lock (so concurrent bulk ops / MoveCard
// cannot lose updates the way a stale unlocked read + SetTags would). Skips the save when the
// result would be empty - clearing every tag via bulk is not supported.
func PatchTags(path string, add, remove []string) error {
	changed, err := PatchTagsNoRefresh(path, add, remove)
	if err != nil || !changed {
		return err
	}
	RefreshCaches()
	return nil
}

// PatchTagsNoRefresh is PatchTags without the aggregate cache refresh, reporting whether a
// change was actually saved so a batch caller knows whether RefreshCaches() is needed at all.
// See SetEditorNoRefresh.
func PatchTagsNoRefresh(path string, add, remove []string) (changed bool, err error) {
	skipped := false
	err = MetaDataMutate(path, func(m *Metadata, existed bool) (bool, error) {
		if !existed {
			return false, nil
		}
		tags := append([]string(nil), m.Tags...)
		for _, a := range add {
			if !slices.Contains(tags, a) {
				tags = append(tags, a)
			}
		}
		if len(remove) > 0 {
			removeSet := make(map[string]bool, len(remove))
			for _, t := range remove {
				removeSet[t] = true
			}
			filtered := tags[:0]
			for _, t := range tags {
				if !removeSet[t] {
					filtered = append(filtered, t)
				}
			}
			tags = filtered
		}
		if len(tags) == 0 {
			skipped = true
			logging.LogWarning(logging.KeyApp, "bulk-update: skipping tag clear for %s (clearing all tags is not supported via bulk update)", path)
			return false, nil
		}
		oldKanbanStatus := kanbanStatusFromTags(m.Tags)
		cleaned, err := sanitizeKanbanTags(tags)
		if err != nil {
			logging.LogWarning(logging.KeyApp, "tag sanitization for %s: %v", path, err)
		}
		m.Tags = cleaned
		applyKanbanTimestamps(m, oldKanbanStatus)
		return true, nil
	})
	if err != nil || skipped {
		return false, err
	}
	return true, nil
}

// SetCreatedAt sets path's creation timestamp. Errors if path has no metadata yet.
func SetCreatedAt(path string, createdAt time.Time) error {
	return MetaDataMutate(path, func(m *Metadata, existed bool) (bool, error) {
		if !existed {
			return false, fmt.Errorf("metadata not found for %s", path)
		}
		m.CreatedAt = createdAt
		return true, nil
	})
}

// SetLastEdited sets path's last-edited timestamp (manual override of the Sync-stamped value).
// Errors if path has no metadata yet.
func SetLastEdited(path string, lastEdited time.Time) error {
	return MetaDataMutate(path, func(m *Metadata, existed bool) (bool, error) {
		if !existed {
			return false, fmt.Errorf("metadata not found for %s", path)
		}
		m.LastEdited = lastEdited
		return true, nil
	})
}

// SetReferences sets path's external reference list. Errors if path has no metadata yet.
func SetReferences(path string, references []Reference) error {
	return MetaDataMutate(path, func(m *Metadata, existed bool) (bool, error) {
		if !existed {
			return false, fmt.Errorf("metadata not found for %s", path)
		}
		m.References = references
		return true, nil
	})
}

// metaDataSaveRaw writes m as-is, with no field recomputation. Only reachable from
// MetaDataMutate/MetaDataSyncNoRefresh/delete internals while the path's write lock is held -
// never call this directly, and never export it again (see package godoc).
func metaDataSaveRaw(m *Metadata) error {
	data, err := json.Marshal(m)
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to marshal metadata: %v", err)
		return err
	}

	if err := metadataStorage.Set(m.Path, data); err != nil {
		logging.LogError(logging.KeyApp, "failed to save metadata for %s: %v", m.Path, err)
		return err
	}

	logging.LogDebug(logging.KeyApp, "raw metadata saved for: %s", m.Path)
	return nil
}

// MetaDataGet retrieves metadata for a file path
func MetaDataGet(filepath string) (*Metadata, error) {
	if rebuildMetaGetCount != nil {
		*rebuildMetaGetCount++
	}
	// normalize path for metadata lookup - add docs/ prefix if not present and not media
	normalizedPath := pathutils.ToWithPrefix(filepath)

	logging.LogDebug(logging.KeyApp, "MetaDataGet: filepath='%s' -> normalizedPath='%s'", filepath, normalizedPath)

	data, err := metadataStorage.Get(normalizedPath)
	if err != nil {
		return nil, err
	}

	if data == nil {
		return nil, nil
	}

	var metadata Metadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata for %s: %w", normalizedPath, err)
	}

	return &metadata, nil
}

// MetaDataInitializeAll initializes metadata for all files without metadata
func MetaDataInitializeAll() error {
	logging.LogInfo(logging.KeyApp, "initializing metadata for all files")

	allFiles, err := GetAllPhysicalFiles()
	if err != nil {
		return err
	}

	for _, file := range allFiles {
		normalizedPath := pathutils.ToWithPrefix(file.Path)

		metadata, err := MetaDataGet(normalizedPath)
		if err != nil {
			logging.LogWarning(logging.KeyApp, "error checking metadata for %s: %v", normalizedPath, err)
			continue
		}

		if metadata != nil {
			continue
		}

		if err := MetaDataSync(normalizedPath); err != nil {
			logging.LogWarning(logging.KeyApp, "failed to initialize metadata for %s: %v", normalizedPath, err)
		} else {
			logging.LogInfo(logging.KeyApp, "initialized metadata for %s", normalizedPath)
		}
	}

	// also initialize metadata for media files (e.g. imported from dokuwiki directly on disk)
	allMediaFiles, err := GetAllMediaFiles()
	if err != nil {
		logging.LogWarning(logging.KeyApp, "failed to get media files for initialization: %v", err)
	} else {
		for _, file := range allMediaFiles {
			normalizedPath := pathutils.ToWithPrefix(file.Path)

			created := false
			err := MetaDataMutate(normalizedPath, func(m *Metadata, existed bool) (bool, error) {
				if existed {
					return false, nil
				}
				created = true
				return true, nil
			})
			if err != nil {
				logging.LogWarning(logging.KeyApp, "failed to initialize metadata for %s: %v", normalizedPath, err)
			} else if created {
				logging.LogInfo(logging.KeyApp, "initialized metadata for media file %s", normalizedPath)
			}
		}
	}

	logging.LogInfo(logging.KeyApp, "metadata initialization completed")
	return nil
}

// MetaDataDelete removes metadata for a file path and refreshes the aggregate
// caches. For deleting many files in one request, call MetaDataDeleteNoRefresh
// in the loop and RefreshCaches() once afterwards instead - otherwise each
// deletion kicks off its own full background cache rebuild.
func MetaDataDelete(filepath string) error {
	if err := MetaDataDeleteNoRefresh(logging.KeyApp, filepath); err != nil {
		return err
	}
	RefreshCaches()
	return nil
}

// MetaDataDeleteNoRefresh removes metadata for a file path without refreshing
// the aggregate caches (tags/collections/folders/editors/file list). See
// MetaDataDelete.
func MetaDataDeleteNoRefresh(key logging.Key, filepath string) error {
	unlock := lockMetaPath(filepath)
	defer unlock()

	normalized := pathutils.ToWithPrefix(filepath)
	if err := chat.DeleteForFile(normalized); err != nil {
		logging.LogWarning(key, "failed to delete chat messages for %s: %v", normalized, err)
	}
	// remove from the live full-text search index too - otherwise a deleted
	// file's content stays searchable forever, since IndexAllFiles only ever
	// adds/updates entries for files that still exist, never prunes ones that
	// don't. Keyed by the un-prefixed relative path, matching how IndexFile
	// indexes it (via file.Path from GetAllPhysicalFiles). The trigram fallback
	// index is handled separately - it's fully rebuilt on each periodic
	// reindex (see search.IndexAllFiles), so deleted files drop out of it
	// within one reindex cycle without needing per-delete wiring here.
	if err := searchStorage.DeleteIndexedContent(pathutils.ToRelative(filepath)); err != nil {
		logging.LogWarning(key, "failed to remove %s from search index: %v", normalized, err)
	}
	return metadataStorage.Delete(normalized)
}

// MetaDataExportAll returns all metadata entries
func MetaDataExportAll() ([]*Metadata, error) {
	allFiles, err := GetAllPhysicalFiles()
	if err != nil {
		return nil, err
	}

	var allMetadata []*Metadata
	for _, file := range allFiles {
		metadata, err := MetaDataGet(file.Path)
		if err != nil {
			logging.LogWarning(logging.KeyApp, "failed to get metadata for %s: %v", file.Path, err)
			continue
		}
		if metadata != nil {
			allMetadata = append(allMetadata, metadata)
		}
	}

	return allMetadata, nil
}

// ValidateMediaMimeType checks if a MIME type is allowed for media uploads
func ValidateMediaMimeType(mimeType string) bool {
	if mimeType == "" {
		logging.LogWarning(logging.KeyApp, "empty mime type provided for validation")
		return false
	}

	// get current allowed mime types
	allowedTypes := configmanager.GetAllowedMimeTypes()

	// if no allowed types configured, deny by default for security
	if len(allowedTypes) == 0 {
		logging.LogWarning(logging.KeyApp, "no allowed mime types configured, denying upload")
		return false
	}

	// normalize MIME type using utils function
	mimeType = utils.Normalize(mimeType)
	logging.LogDebug(logging.KeyApp, "validating mime type: %s against allowed types: %v", mimeType, allowedTypes)

	// check exact matches first
	for _, allowedType := range allowedTypes {
		allowedType = utils.Normalize(allowedType)
		if allowedType == mimeType {
			logging.LogDebug(logging.KeyApp, "mime type %s allowed (exact match)", mimeType)
			return true
		}

		// handle wildcard patterns like "image/*"
		if strings.HasSuffix(allowedType, "/*") {
			category := strings.TrimSuffix(allowedType, "/*")
			if strings.HasPrefix(mimeType, category+"/") {
				logging.LogDebug(logging.KeyApp, "mime type %s allowed (wildcard match: %s)", mimeType, allowedType)
				return true
			}
		}
	}

	logging.LogWarning(logging.KeyApp, "mime type %s not allowed, blocked upload", mimeType)
	return false
}
