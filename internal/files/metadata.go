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
	"errors"
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
func lockMetaPath(path pathutils.MetaPath) (unlock func()) {
	return metaLocks.Lock(path.String())
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
func MetaDataMutate(path pathutils.MetaPath, fn func(m *Metadata, existed bool) (save bool, err error)) error {
	unlock := lockMetaPath(path)
	defer unlock()

	metadata, err := MetaDataGet(path)
	if err != nil {
		return err
	}
	existed := metadata != nil
	if !existed {
		metadata = &Metadata{Path: path}
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
	EditorTypeTracker    EditorType = "tracker-editor"
	EditorTypeList       EditorType = "list-editor"
	EditorTypeTodo       EditorType = "todo-editor"
	EditorTypeIndex      EditorType = "index-editor"
	EditorTypeBook       EditorType = "book-editor"
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
		EditorTypeTracker,
		EditorTypeList,
		EditorTypeTodo,
		EditorTypeIndex,
		EditorTypeBook,
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
	case ".tracker":
		return EditorTypeTracker
	case ".book":
		return EditorTypeBook
	case ".txt":
		return EditorTypeCodeMirror
	default:
		return ""
	}
}

// ResolveEditor picks the editor for an existing file: metadata first, then the extension,
// then "" for the caller to default. The one place the metadata→extension precedence lives.
func ResolveEditor(path pathutils.MetaPath) EditorType {
	if meta, err := MetaDataGet(path); err == nil && meta != nil && meta.Editor != "" {
		return meta.Editor
	}
	return EditorFromExtension(path.String())
}

// IsBook reports whether the file at path is a book (shown as its composed document, not
// its raw entry list).
func IsBook(path pathutils.MetaPath) bool {
	return ResolveEditor(path) == EditorTypeBook
}

// Metadata represents file metadata
type Metadata struct {
	Path          pathutils.MetaPath   `json:"path"`                    // auto
	Title         string               `json:"title"`                   // auto
	CreatedAt     time.Time            `json:"createdAt"`               // auto
	LastEdited    time.Time            `json:"lastEdited"`              // auto
	Collection    string               `json:"collection"`              // auto
	Folders       []string             `json:"folders"`                 // auto
	Tags          []string             `json:"tags"`                    // manual
	Ancestor      []pathutils.MetaPath `json:"ancestor"`                // auto
	Parents       []pathutils.MetaPath `json:"parents"`                 // manual
	Kids          []pathutils.MetaPath `json:"kids"`                    // auto
	UsedLinks     []pathutils.MetaPath `json:"usedLinks"`               // auto
	LinksToHere   []pathutils.MetaPath `json:"linksToHere"`             // auto
	Related       []pathutils.MetaPath `json:"related,omitempty"`       // auto
	Editor        EditorType           `json:"editor"`                  // manual
	Size          int64                `json:"size"`                    // auto
	References    []Reference          `json:"references,omitempty"`    // manual
	ConflictFile  pathutils.MetaPath   `json:"conflictFile,omitempty"`  // auto
	ConflictOf    pathutils.MetaPath   `json:"conflictOf,omitempty"`    // auto
	KanbanAddedAt time.Time            `json:"kanbanAddedAt,omitempty"` // auto
	KanbanMovedAt time.Time            `json:"kanbanMovedAt,omitempty"` // auto
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
	isMediaFile := metadata.Path.IsMedia()
	fullPath := metadata.Path.FullPath()

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
		if et := EditorFromExtension(metadata.Path.String()); et != "" {
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
		metadata.Parents = []pathutils.MetaPath{}
	}
	if metadata.Kids == nil {
		metadata.Kids = []pathutils.MetaPath{}
	}
	if metadata.UsedLinks == nil {
		metadata.UsedLinks = []pathutils.MetaPath{}
	}
	if metadata.LinksToHere == nil {
		metadata.LinksToHere = []pathutils.MetaPath{}
	}
	if metadata.Ancestor == nil {
		metadata.Ancestor = []pathutils.MetaPath{}
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
func MetaDataSync(path pathutils.MetaPath) error {
	return withRefresh(func() error { return MetaDataSyncNoRefresh(path) })
}

// MetaDataSyncNoRefresh is MetaDataSync without the aggregate cache refresh. See MetaDataSync.
func MetaDataSyncNoRefresh(path pathutils.MetaPath) error {
	unlock := lockMetaPath(path)

	metadata, err := MetaDataGet(path)
	if err != nil {
		unlock()
		return err
	}
	if metadata == nil {
		metadata = &Metadata{Path: path, CreatedAt: time.Now()}
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

// ErrInvalidKanbanTags is returned (wrapped) by SanitizeKanbanTags and SetTagsStrict when tags
// adds an unknown kanban tag or a status outside the allowlist.
var ErrInvalidKanbanTags = errors.New("invalid kanban tag(s) removed")

// SanitizeKanbanTags checks the kanban tags that tags adds to oldTags: an unknown
// <prefix>-<x> tag or a status outside the allowlist is removed and reported. Tags already in
// oldTags are always kept, so a later kanban settings change never deletes existing tags - this
// includes a file that already carries several status tags, which keeps all of them. Only a newly
// added valid status tag replaces every other status tag (at most one status per file from then on).
// Returns the cleaned tag list and an error describing any removed tags.
func SanitizeKanbanTags(oldTags, tags []string) ([]string, error) {
	validStatuses := configmanager.GetKanbanStatuses()
	prefixDash := configmanager.GetKanbanPrefix() + "-"
	statusDash := prefixDash + "status-"

	var result, invalidTags []string
	var kanbanTag string
	for _, t := range tags {
		switch {
		case !strings.HasPrefix(t, prefixDash) || slices.Contains(oldTags, t):
			result = append(result, t)
		case strings.HasPrefix(t, statusDash) && slices.Contains(validStatuses, strings.TrimPrefix(t, statusDash)):
			kanbanTag = t // last valid one wins
		default:
			invalidTags = append(invalidTags, t)
		}
	}
	if kanbanTag != "" {
		result = append(slices.DeleteFunc(result, configmanager.IsKanbanTag), kanbanTag)
	}

	if len(invalidTags) > 0 {
		return result, fmt.Errorf("%w: %s (allowed statuses: %s)", ErrInvalidKanbanTags,
			strings.Join(invalidTags, ", "), strings.Join(validStatuses, ", "))
	}
	return result, nil
}

// SetConflictFile sets the conflict file path on an original file's metadata.
// Overwrites any previous conflict file reference — only one is kept at a time. Errors if
// originalFilePath has no metadata yet — its caller always names an already-tracked file, so a
// miss means a bad path rather than something to paper over with a phantom record.
func SetConflictFile(originalFilePath, conflictFilePath pathutils.MetaPath) error {
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
func SetConflictOf(conflictFilePath, originalFilePath pathutils.MetaPath) error {
	return MetaDataMutate(conflictFilePath, func(m *Metadata, existed bool) (bool, error) {
		m.ConflictOf = originalFilePath
		return true, nil
	})
}

// ClearConflictFile removes the conflict file reference from the original file's metadata.
func ClearConflictFile(originalFilePath pathutils.MetaPath) error {
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
func SetEditor(path pathutils.MetaPath, editor EditorType) error {
	return withRefresh(func() error { return SetEditorNoRefresh(path, editor) })
}

// SetEditorNoRefresh is SetEditor without the aggregate cache refresh - for callers that set
// several fields or loop over many files in one request, so use SetEditor and call
// RefreshCaches() once afterwards instead of paying for a full cache rebuild per field/file.
func SetEditorNoRefresh(path pathutils.MetaPath, editor EditorType) error {
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
func SetTags(path pathutils.MetaPath, tags []string) error {
	return withRefresh(func() error { return SetTagsNoRefresh(path, tags) })
}

// SetTagsStrict is SetTags for user input: the kanban check runs against the tags on disk under
// the path lock, and invalid kanban tags (ErrInvalidKanbanTags) reject the whole save instead
// of being dropped. Returns the tags before and after the save as seen under the lock.
func SetTagsStrict(path pathutils.MetaPath, tags []string) (oldTags, newTags []string, err error) {
	err = withRefresh(func() error {
		return MetaDataMutate(path, func(m *Metadata, existed bool) (bool, error) {
			cleaned, err := SanitizeKanbanTags(m.Tags, tags)
			if err != nil {
				return false, err
			}
			oldKanbanStatus := kanbanStatusFromTags(m.Tags)
			oldTags, newTags = m.Tags, cleaned
			m.Tags = cleaned
			applyKanbanTimestamps(m, oldKanbanStatus)
			return true, nil
		})
	})
	return oldTags, newTags, err
}

// SetTagsNoRefresh is SetTags without the aggregate cache refresh. See SetEditorNoRefresh.
func SetTagsNoRefresh(path pathutils.MetaPath, tags []string) error {
	return MetaDataMutate(path, func(m *Metadata, existed bool) (bool, error) {
		oldKanbanStatus := kanbanStatusFromTags(m.Tags)
		cleaned, err := SanitizeKanbanTags(m.Tags, tags)
		if err != nil {
			logging.LogWarning(logging.KeyApp, "tag sanitization for %s: %v", path, err)
		}
		m.Tags = cleaned
		applyKanbanTimestamps(m, oldKanbanStatus)
		return true, nil
	})
}

// PatchTagsNoRefresh adds and/or removes tags under the path lock (so concurrent bulk ops /
// MoveCard cannot lose updates the way a stale unlocked read + SetTags would), reporting whether
// a change was actually saved so a batch caller knows whether RefreshCaches() is needed at all.
// Skips the save when the result would be empty - clearing every tag via bulk is not supported.
func PatchTagsNoRefresh(path pathutils.MetaPath, add, remove []string) (changed bool, err error) {
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
		cleaned, err := SanitizeKanbanTags(m.Tags, tags)
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
func SetCreatedAt(path pathutils.MetaPath, createdAt time.Time) error {
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
func SetLastEdited(path pathutils.MetaPath, lastEdited time.Time) error {
	return MetaDataMutate(path, func(m *Metadata, existed bool) (bool, error) {
		if !existed {
			return false, fmt.Errorf("metadata not found for %s", path)
		}
		m.LastEdited = lastEdited
		return true, nil
	})
}

// SetReferences sets path's external reference list. Errors if path has no metadata yet.
func SetReferences(path pathutils.MetaPath, references []Reference) error {
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

	if err := metadataStorage.Set(m.Path.String(), data); err != nil {
		logging.LogError(logging.KeyApp, "failed to save metadata for %s: %v", m.Path, err)
		return err
	}

	logging.LogDebug(logging.KeyApp, "raw metadata saved for: %s", m.Path)
	return nil
}

// MetaDataGet retrieves metadata for a file path
func MetaDataGet(path pathutils.MetaPath) (*Metadata, error) {
	if rebuildMetaGetCount != nil {
		*rebuildMetaGetCount++
	}
	data, err := metadataStorage.Get(path.String())
	if err != nil {
		return nil, err
	}

	if data == nil {
		return nil, nil
	}

	var metadata Metadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata for %s: %w", path, err)
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
		normalizedPath := file.Path

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
			normalizedPath := file.Path

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
func MetaDataDelete(path pathutils.MetaPath) error {
	return withRefresh(func() error { return MetaDataDeleteNoRefresh(logging.KeyApp, path) })
}

// MetaDataDeleteNoRefresh removes metadata for a file path without refreshing
// the aggregate caches (tags/collections/folders/editors/file list). See
// MetaDataDelete.
func MetaDataDeleteNoRefresh(key logging.Key, path pathutils.MetaPath) error {
	unlock := lockMetaPath(path)
	defer unlock()

	normalized := path.String()
	if err := chat.DeleteForFile(normalized); err != nil {
		logging.LogWarning(key, "failed to delete chat messages for %s: %v", normalized, err)
	}
	// remove from the live full-text search index too - otherwise a deleted
	// file's content stays searchable forever, since IndexAllFiles only ever
	// adds/updates entries for files that still exist, never prunes ones that
	// don't. The index is keyed by the docs-relative path (search.IndexAllFiles), a media
	// file is never in it. The trigram fallback
	// index is handled separately - it's fully rebuilt on each periodic
	// reindex (see search.IndexAllFiles), so deleted files drop out of it
	// within one reindex cycle without needing per-delete wiring here.
	if rel, ok := path.DocsRel(); ok {
		if err := searchStorage.DeleteIndexedContent(rel); err != nil {
			logging.LogWarning(key, "failed to remove %s from search index: %v", normalized, err)
		}
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

// ValidateMediaType checks if a file is allowed for media uploads by its MIME type or extension
func ValidateMediaType(fileName, mimeType string) bool {
	// get current allowed media types
	allowedTypes := configmanager.GetAllowedMediaTypes()

	// if no allowed types configured, deny by default for security
	if len(allowedTypes) == 0 {
		logging.LogWarning(logging.KeyApp, "no allowed media types configured, denying upload")
		return false
	}

	logging.LogDebug(logging.KeyApp, "validating media type: %s (%s) against allowed types: %v", mimeType, fileName, allowedTypes)
	if configmanager.IsAllowedMediaType(fileName, mimeType) {
		return true
	}

	logging.LogWarning(logging.KeyApp, "media type %s (%s) not allowed, blocked upload", mimeType, fileName)
	return false
}
