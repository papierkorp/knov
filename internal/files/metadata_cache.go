// Package files - metadata cache operations (aggregation, persisted lookups)
package files

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"knov/internal/cacheStorage"
	"knov/internal/configmanager"
	"knov/internal/logging"
	"knov/internal/pathutils"
	"knov/internal/utils"
)

// CacheKey represents system cache keys
type CacheKey string

const (
	CacheKeyFolderPaths           CacheKey = "all_folder_paths"
	CacheKeyFilePaths             CacheKey = "all_file_paths_v2" // v2: docs paths with docs/ prefix
	CacheKeyTitles                CacheKey = "all_titles"
	CacheKeyOrphanedMedia         CacheKey = "orphaned_media"
	CacheKeyAncestorsInCollection CacheKey = "ancestors_in_collection/"
	CacheKeyFullFileList          CacheKey = "all_files_full_v2" // v2: docs paths with docs/ prefix
)

// in-memory memo of the decoded file list. cacheStorage.Get + json.Unmarshal of
// the whole list on every call is expensive with a few thousand files, so the
// decoded slice is kept here and reused until the list is invalidated.
// Treat entries as read-only - all callers share the same File/Metadata values.
//
// fileListMemoGen guards against a stale writer winning: RebuildAllCaches runs in
// the background, so a walk started before a mutation can finish after it and
// would otherwise memoize a file list that is already out of date. Writers capture
// the generation before reading and their result is dropped if it changed since.
var (
	fileListMemoMu  sync.RWMutex
	fileListMemo    []File
	fileListMemoGen uint64
)

// fileListGeneration returns the current file-list invalidation generation.
// Capture it before building a list that will later be handed to setFileListMemo.
func fileListGeneration() uint64 {
	fileListMemoMu.RLock()
	defer fileListMemoMu.RUnlock()
	return fileListMemoGen
}

// setFileListMemo replaces the in-memory decoded file list, unless the list was
// invalidated since gen was captured - in that case allFiles is already stale.
func setFileListMemo(allFiles []File, gen uint64) {
	fileListMemoMu.Lock()
	defer fileListMemoMu.Unlock()
	if fileListMemoGen != gen {
		return
	}
	fileListMemo = allFiles
}

// invalidateFileListMemo drops the memo and bumps the generation, so any read or
// rebuild already in flight will not store its now-stale result.
func invalidateFileListMemo() {
	fileListMemoMu.Lock()
	fileListMemo = nil
	fileListMemoGen++
	fileListMemoMu.Unlock()
}

// saveFileListToCache persists the full file list (including metadata) to cache storage
// and memoizes it. gen must be the generation captured before allFiles was built.
func saveFileListToCache(allFiles []File, gen uint64) error {
	logging.LogDebug(logging.KeyApp, "saving %s to cache", CacheKeyFullFileList)
	setFileListMemo(allFiles, gen)
	jsonData, err := json.Marshal(allFiles)
	if err != nil {
		return err
	}
	return cacheStorage.Set(string(CacheKeyFullFileList), jsonData)
}

// getFileListFromCache retrieves the full file list from cache storage.
// Returns (nil, nil) on a cache miss so callers can distinguish "not cached yet"
// from "cached but genuinely empty".
func getFileListFromCache() ([]File, error) {
	data, err := cacheStorage.Get(string(CacheKeyFullFileList))
	if err != nil {
		if strings.Contains(err.Error(), "key not found") ||
			strings.Contains(err.Error(), "no such file") {
			return nil, nil
		}
		return nil, err
	}
	if data == nil {
		return nil, nil
	}

	var result []File
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetAllFilesCached returns the same data as GetAllFiles (a full disk walk plus
// a metadata lookup per file), but serves it from cache storage when available,
// avoiding the O(n) walk + n metadata reads on every tree/list request.
// The cache is populated by the periodic RebuildAllCaches job and kept
// fresh in between by InvalidateFileListCache on mutations.
func GetAllFilesCached() ([]File, error) {
	fileListMemoMu.RLock()
	memo := fileListMemo
	gen := fileListMemoGen
	fileListMemoMu.RUnlock()
	if memo != nil {
		return memo, nil
	}

	cached, err := getFileListFromCache()
	if err != nil {
		logging.LogWarning(logging.KeyApp, "failed to read file list cache, falling back to live data: %v", err)
	} else if cached != nil {
		setFileListMemo(cached, gen)
		return cached, nil
	}

	allFiles, err := GetAllPhysicalFiles()
	if err != nil {
		return nil, err
	}

	if err := saveFileListToCache(allFiles, gen); err != nil {
		logging.LogWarning(logging.KeyApp, "failed to persist file list cache: %v", err)
	}

	return allFiles, nil
}

// InvalidateFileListCache forces the next GetAllFilesCached call to rebuild
// from disk. Called after any mutation that adds, removes, renames, or
// changes the visibility-relevant metadata of a file.
func InvalidateFileListCache() {
	invalidateFileListMemo()
	if err := cacheStorage.Delete(string(CacheKeyFullFileList)); err != nil {
		logging.LogWarning(logging.KeyApp, "failed to invalidate file list cache: %v", err)
	}
}

// RefreshCaches invalidates the file list cache immediately (so the very next
// request gets fresh data) and rebuilds all other caches - tags, collections,
// folders, file/folder paths, orphaned media - in the background. Call this
// after any mutation that adds, removes, renames, or changes the metadata of
// a file; otherwise those caches only catch up on the next periodic
// RebuildAllCaches cron run.
func RefreshCaches() {
	InvalidateFileListCache()
	refreshes.Go(func() {
		if err := RebuildAllCaches(); err != nil {
			logging.LogWarning(logging.KeyApp, "failed to refresh caches after mutation: %v", err)
		}
	})
}

// refreshes tracks the background rebuilds of RefreshCaches, see WaitForCacheRefreshes
var refreshes sync.WaitGroup

// WaitForCacheRefreshes blocks until every background rebuild started by RefreshCaches has
// finished - before the storage they write to is removed (knov --start-tests --remove).
func WaitForCacheRefreshes() {
	refreshes.Wait()
}

// withRefresh runs fn and, on success, refreshes the aggregate caches - the shared shape behind
// every SetX/SetXNoRefresh pair. Batch callers loop the NoRefresh variant instead and call
// RefreshCaches() once afterwards, so a single mutation and a batch of many both pay for exactly
// one cache rebuild.
func withRefresh(fn func() error) error {
	if err := fn(); err != nil {
		return err
	}
	RefreshCaches()
	return nil
}

// saveStringListToCache saves a sorted string list to cache storage
func saveStringListToCache(key CacheKey, data []string) error {
	logging.LogDebug(logging.KeyApp, "saving %s to cache", key)
	sortedData := make([]string, len(data))
	copy(sortedData, data)
	slices.Sort(sortedData)

	jsonData, err := json.Marshal(sortedData)
	if err != nil {
		return err
	}

	return cacheStorage.Set(string(key), jsonData)
}

// getStringListFromCache retrieves a string list from cache storage
func getStringListFromCache(key CacheKey) ([]string, error) {
	data, err := cacheStorage.Get(string(key))
	if err != nil {
		if strings.Contains(err.Error(), "key not found") ||
			strings.Contains(err.Error(), "no such file") {
			return []string{}, nil // return empty slice if not found
		}
		return nil, err
	}

	if data == nil {
		return []string{}, nil // return empty slice if data is nil
	}

	var result []string
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// countBy counts the names returned by keys across all files visible for scope (see FilterByVisibility).
// Computed live from the memoized file list - one pass, so no separate count cache is kept.
func countBy(scope string, keys func(*Metadata) []string) (map[string]int, error) {
	allFiles, err := GetAllFilesCached()
	if err != nil {
		return nil, err
	}

	counts := make(map[string]int)
	for _, file := range FilterByVisibility(allFiles, scope) {
		if file.Metadata == nil {
			continue
		}
		for _, k := range keys(file.Metadata) {
			if k != "" {
				counts[k]++
			}
		}
	}
	return counts, nil
}

// GetAllTags returns all unique tags with their counts, leaving out files hidden for scope.
func GetAllTags(scope string) (TagCount, error) {
	return countBy(scope, func(m *Metadata) []string {
		return slices.DeleteFunc(slices.Clone(m.Tags), configmanager.IsKanbanTag)
	})
}

// GetAllCollections returns all unique collections with their counts, leaving out files hidden for scope.
func GetAllCollections(scope string) (CollectionCount, error) {
	return countBy(scope, func(m *Metadata) []string { return []string{m.Collection} })
}

// GetAllFolders returns all unique folders with their counts, leaving out files hidden for scope.
func GetAllFolders(scope string) (FolderCount, error) {
	return countBy(scope, func(m *Metadata) []string { return m.Folders })
}

// GetAllEditors returns all unique editor types with their counts, leaving out files hidden for scope.
func GetAllEditors(scope string) (EditorTypeCount, error) {
	return countBy(scope, func(m *Metadata) []string { return []string{string(m.Editor)} })
}

// SaveAllFilePathsToCache saves all file paths to cache storage
func SaveAllFilePathsToCache() error {
	allFiles, err := GetAllFiles()
	if err != nil {
		return err
	}

	var fileList []string
	for _, file := range allFiles {
		fileList = append(fileList, file.Path)
	}

	return saveStringListToCache(CacheKeyFilePaths, fileList)
}

// GetAllFilePathsFromCache retrieves cached file paths from cache storage
func GetAllFilePathsFromCache() ([]string, error) {
	return getStringListFromCache(CacheKeyFilePaths)
}

// GetAllTitlesFromCache retrieves cached titles from cache storage
func GetAllTitlesFromCache() ([]string, error) {
	return getStringListFromCache(CacheKeyTitles)
}

// GetAllTitles returns all unique non-empty titles
func GetAllTitles() ([]string, error) {
	allFiles, err := GetAllFilesCached()
	if err != nil {
		return nil, err
	}
	allFiles = FilterByVisibility(allFiles, "")

	logging.LogInfo(logging.KeyApp, "getAllTitles: scanning %d files", len(allFiles))
	seen := make(map[string]bool)
	var titles []string
	for _, file := range allFiles {
		meta := file.Metadata
		if meta == nil {
			logging.LogDebug(logging.KeyApp, "getAllTitles: no metadata for %s", file.Path)
			continue
		}
		title := meta.Title
		logging.LogDebug(logging.KeyApp, "getAllTitles: %s -> %q", file.Path, title)
		if title != "" && !seen[title] {
			seen[title] = true
			titles = append(titles, title)
		}
	}
	logging.LogInfo(logging.KeyApp, "getAllTitles: found %d unique titles", len(titles))
	slices.Sort(titles)
	return titles, nil
}

// MetadataCollector collects metadata across multiple files efficiently
type MetadataCollector struct {
	FolderPaths           map[string]bool
	Titles                map[string]bool
	FilePaths             []string
	OrphanedMedia         []string
	AncestorsInCollection map[string]map[string]bool // collection → set of ancestor paths
}

// NewMetadataCollector creates a new metadata collector
func NewMetadataCollector() *MetadataCollector {
	return &MetadataCollector{
		FolderPaths:           make(map[string]bool),
		Titles:                make(map[string]bool),
		FilePaths:             []string{},
		OrphanedMedia:         []string{},
		AncestorsInCollection: make(map[string]map[string]bool),
	}
}

// CollectFromMetadata adds metadata to the collector
func (mc *MetadataCollector) CollectFromMetadata(filePath string, metadata *Metadata) {
	// collect file path
	mc.FilePaths = append(mc.FilePaths, filePath)

	// collect folder paths from file path
	for _, path := range ancestorFolderPaths(filePath) {
		mc.FolderPaths[path] = true
	}

	// collect orphaned media
	if strings.HasPrefix(filePath, "media/") && len(metadata.LinksToHere) == 0 {
		mc.OrphanedMedia = append(mc.OrphanedMedia, filePath)
	}

	// collect title - set by every metadata sync, a file without a header has none
	if metadata.Title != "" {
		mc.Titles[metadata.Title] = true
	}
	if metadata.Collection != "" && len(metadata.Ancestor) > 0 {
		root := metadata.Ancestor[0]
		if mc.AncestorsInCollection[metadata.Collection] == nil {
			mc.AncestorsInCollection[metadata.Collection] = make(map[string]bool)
		}
		mc.AncestorsInCollection[metadata.Collection][root] = true
	}
}

// SaveAllToCache saves all collected metadata to system cache
func (mc *MetadataCollector) SaveAllToCache() error {
	if err := saveStringListToCache(CacheKeyFolderPaths, utils.SetToSortedSlice(mc.FolderPaths)); err != nil {
		return err
	}
	if err := saveStringListToCache(CacheKeyFilePaths, mc.FilePaths); err != nil {
		return err
	}
	if err := saveStringListToCache(CacheKeyTitles, utils.SetToSortedSlice(mc.Titles)); err != nil {
		return err
	}
	if err := saveStringListToCache(CacheKeyOrphanedMedia, mc.OrphanedMedia); err != nil {
		return err
	}
	for collection, ancestors := range mc.AncestorsInCollection {
		key := CacheKey(string(CacheKeyAncestorsInCollection) + collection)
		if err := saveStringListToCache(key, utils.SetToSortedSlice(ancestors)); err != nil {
			return err
		}
	}
	return nil
}

// RebuildAllCaches saves all metadata lists to cache storage in a single pass
func RebuildAllCaches() error {
	logging.LogInfo(logging.KeyFileSync, "collecting all system metadata for cache update")

	collector := NewMetadataCollector()

	// captured before the walk so a mutation landing mid-rebuild discards this result
	gen := fileListGeneration()

	// collect from document files (pathsToFiles already attached metadata to each file)
	allFiles, err := GetAllFiles()
	if err != nil {
		return err
	}

	for _, file := range FilterByVisibility(allFiles, "") {
		if file.Metadata == nil {
			continue
		}
		collector.CollectFromMetadata(file.Path, file.Metadata)
	}

	// persist the full file list too, so tree/list requests can reuse this same
	// walk instead of triggering their own
	if err := saveFileListToCache(allFiles, gen); err != nil {
		logging.LogWarning(logging.KeyFileSync, "failed to persist file list cache: %v", err)
	}

	// collect from media files (needed for orphaned media detection)
	mediaFiles, err := GetAllMediaFiles()
	if err != nil {
		logging.LogWarning(logging.KeyFileSync, "failed to get media files for cache update: %v", err)
	} else {
		for _, file := range mediaFiles {
			normalizedPath := pathutils.ToWithPrefix(file.Path)
			if file.Metadata == nil {
				// no metadata → never referenced → orphaned
				collector.OrphanedMedia = append(collector.OrphanedMedia, normalizedPath)
				continue
			}
			collector.CollectFromMetadata(normalizedPath, file.Metadata)
		}
	}

	if err := collector.SaveAllToCache(); err != nil {
		return err
	}

	logging.LogInfo(logging.KeyFileSync, "system metadata cache update completed")
	return nil
}

// GetAllFolderPathsFromCache retrieves cached folder path suggestions from cache storage
func GetAllFolderPathsFromCache() ([]string, error) {
	return getStringListFromCache(CacheKeyFolderPaths)
}

// ancestorFolderPaths returns every ancestor folder of the docs file filePath (docs-relative, as
// the user picks them), each with a trailing slash.
// For xxx/yyy/zzz.md it returns: xxx/, xxx/yyy/, xxx/yyy/zzz/
func ancestorFolderPaths(filePath string) []string {
	dir := pathutils.ToSlash(filepath.Dir(pathutils.ToRelative(filePath)))
	if dir == "." || dir == "" {
		return nil
	}

	parts := strings.Split(dir, "/")
	paths := make([]string, len(parts))
	for i := range parts {
		paths[i] = strings.Join(parts[:i+1], "/") + "/"
	}
	return paths
}

// GetAllFolderPaths returns all unique folder paths for suggestions
// For xxx/yyy/zzz it returns: xxx/, xxx/yyy/, xxx/yyy/zzz/
func GetAllFolderPaths() ([]string, error) {
	allFiles, err := GetAllFiles()
	if err != nil {
		return nil, err
	}

	folderPaths := make(map[string]bool)

	for _, file := range allFiles {
		for _, path := range ancestorFolderPaths(file.Path) {
			folderPaths[path] = true
		}
	}

	var result []string
	for path := range folderPaths {
		result = append(result, path)
	}

	slices.Sort(result)
	return result, nil
}

// SaveAllFolderPathsToCache saves all folder path suggestions to cache storage
func SaveAllFolderPathsToCache() error {
	folderPaths, err := GetAllFolderPaths()
	if err != nil {
		return err
	}

	return saveStringListToCache(CacheKeyFolderPaths, folderPaths)
}

// GetOrphanedMediaFromCache retrieves cached orphaned media list from cache storage
func GetOrphanedMediaFromCache() ([]string, error) {
	return getStringListFromCache(CacheKeyOrphanedMedia)
}

// ScanOrphanedMedia rebuilds the orphaned media cache and returns its paths
func ScanOrphanedMedia() ([]string, error) {
	if err := UpdateOrphanedMediaCache(); err != nil {
		return nil, err
	}
	return GetOrphanedMediaFromCache()
}

// UpdateOrphanedMediaCache efficiently updates only the orphaned media cache
// by checking media files instead of all files
func UpdateOrphanedMediaCache() error {
	logging.LogDebug(logging.KeyApp, "updating orphaned media cache")

	mediaFiles, err := GetAllMediaFiles()
	if err != nil {
		return err
	}

	orphanedMedia := []string{}
	for _, mediaFile := range mediaFiles {
		metadata, err := MetaDataGet(mediaFile.Path)
		if err != nil || metadata == nil {
			continue
		}

		// media is orphaned if it has no links to it
		if len(metadata.LinksToHere) == 0 {
			orphanedMedia = append(orphanedMedia, mediaFile.Path)
		}
	}

	if err := saveStringListToCache(CacheKeyOrphanedMedia, orphanedMedia); err != nil {
		return err
	}

	logging.LogDebug(logging.KeyApp, "orphaned media cache updated: %d orphaned files", len(orphanedMedia))
	return nil
}

// UpdateOrphanedMediaCacheForFile incrementally updates orphaned media cache
// for media files affected by changes to a specific file
func UpdateOrphanedMediaCacheForFile(filePath string) error {
	logging.LogDebug(logging.KeyApp, "incrementally updating orphaned media cache for file: %s", filePath)

	// get file metadata to find affected media files
	metadata, err := MetaDataGet(filePath)
	if err != nil || metadata == nil {
		logging.LogDebug(logging.KeyApp, "no metadata found for %s, skipping cache update", filePath)
		return nil
	}

	// collect media files that might be affected (from UsedLinks)
	var affectedMediaFiles []string
	for _, link := range metadata.UsedLinks {
		if strings.HasPrefix(link, "media/") {
			affectedMediaFiles = append(affectedMediaFiles, link)
		}
	}

	// if no media files are affected, nothing to do
	if len(affectedMediaFiles) == 0 {
		logging.LogDebug(logging.KeyApp, "no media files affected by changes to %s", filePath)
		return nil
	}

	// get current orphaned media cache
	orphanedMedia, err := GetOrphanedMediaFromCache()
	if err != nil {
		logging.LogWarning(logging.KeyApp, "failed to get orphaned media cache, will rebuild: %v", err)
		return UpdateOrphanedMediaCache() // fallback to full rebuild
	}

	// if cache is empty, rebuild it instead of trying to update incrementally
	if len(orphanedMedia) == 0 {
		logging.LogDebug(logging.KeyApp, "orphaned media cache is empty, rebuilding instead of incremental update")
		return UpdateOrphanedMediaCache()
	}

	// create a set for efficient lookups and updates
	orphanedSet := make(map[string]bool)
	for _, media := range orphanedMedia {
		orphanedSet[media] = true
	}

	// check each affected media file and update orphaned status
	for _, mediaPath := range affectedMediaFiles {
		mediaMetadata, err := MetaDataGet(mediaPath)
		if err != nil || mediaMetadata == nil {
			continue
		}

		isOrphaned := len(mediaMetadata.LinksToHere) == 0

		if isOrphaned {
			orphanedSet[mediaPath] = true
		} else {
			delete(orphanedSet, mediaPath)
		}
	}

	// convert back to sorted slice
	updatedOrphanedMedia := make([]string, 0, len(orphanedSet))
	for media := range orphanedSet {
		updatedOrphanedMedia = append(updatedOrphanedMedia, media)
	}

	if err := saveStringListToCache(CacheKeyOrphanedMedia, updatedOrphanedMedia); err != nil {
		return err
	}

	logging.LogDebug(logging.KeyApp, "incrementally updated orphaned media cache: checked %d affected files", len(affectedMediaFiles))
	return nil
}

// CacheInvalidate removes all cache entries, forcing a rebuild on next access
func CacheInvalidate() error {
	// flushing the store alone would leave the decoded file list memo serving
	// stale data, since it is not backed by a cacheStorage read
	invalidateFileListMemo()
	if err := cacheStorage.Flush(); err != nil {
		return fmt.Errorf("failed to invalidate cache: %w", err)
	}
	logging.LogInfo(logging.KeyApp, "cache invalidated")
	return nil
}

// GetAncestorsInFolder returns unique ancestor paths from all files in a folder (and its subfolders).
func GetAncestorsInFolder(folderPath string) ([]string, error) {
	allFiles, err := GetAllFilesCached()
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{})
	var ancestors []string
	for _, f := range allFiles {
		if f.Metadata == nil {
			continue
		}
		dir := strings.Join(f.Metadata.Folders, "/")
		if dir != folderPath && !strings.HasPrefix(dir, folderPath+"/") {
			continue
		}
		if len(f.Metadata.Ancestor) == 0 {
			continue
		}
		root := f.Metadata.Ancestor[0]
		if _, ok := seen[root]; !ok {
			seen[root] = struct{}{}
			ancestors = append(ancestors, root)
		}
	}
	return ancestors, nil
}

// GetFilesInSameFolder returns other files whose folder path exactly matches filePath's
// (unlike GetAncestorsInFolder, this does not include subfolders).
func GetFilesInSameFolder(filePath string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 5
	}

	meta, err := MetaDataGet(filePath)
	if err != nil || meta == nil {
		return nil, err
	}
	folder := strings.Join(meta.Folders, "/")

	allFiles, err := GetAllFilesCached()
	if err != nil {
		return nil, err
	}
	allFiles = FilterByVisibility(allFiles, configmanager.HideScopeDetail)

	var result []string
	for _, f := range allFiles {
		if f.Metadata == nil || f.Metadata.Path == meta.Path {
			continue
		}
		if strings.Join(f.Metadata.Folders, "/") != folder {
			continue
		}
		result = append(result, f.Metadata.Path)
		if len(result) >= limit {
			break
		}
	}
	return result, nil
}

// GetFilesWithSameTags returns other files sharing at least one tag with filePath, ranked by
// number of shared tags.
func GetFilesWithSameTags(filePath string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 5
	}

	meta, err := MetaDataGet(filePath)
	if err != nil || meta == nil || len(meta.Tags) == 0 {
		return nil, err
	}

	allFiles, err := GetAllFilesCached()
	if err != nil {
		return nil, err
	}
	allFiles = FilterByVisibility(allFiles, configmanager.HideScopeDetail)

	type scored struct {
		path  string
		score int
	}
	var candidates []scored
	for _, f := range allFiles {
		if f.Metadata == nil || f.Metadata.Path == meta.Path {
			continue
		}
		score := 0
		for _, tag := range f.Metadata.Tags {
			if !configmanager.IsKanbanTag(tag) && slices.Contains(meta.Tags, tag) {
				score++
			}
		}
		if score > 0 {
			candidates = append(candidates, scored{f.Metadata.Path, score})
		}
	}
	slices.SortStableFunc(candidates, func(a, b scored) int { return b.score - a.score })

	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	result := make([]string, len(candidates))
	for i, c := range candidates {
		result[i] = c.path
	}
	return result, nil
}
