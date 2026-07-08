// Package files - metadata cache operations (aggregation, persisted lookups)
package files

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"knov/internal/cacheStorage"
	"knov/internal/logging"
	"knov/internal/pathutils"
	"knov/internal/utils"
)

// CacheKey represents system cache keys
type CacheKey string

const (
	CacheKeyTags                  CacheKey = "all_tags"
	CacheKeyCollections           CacheKey = "all_collections"
	CacheKeyFolders               CacheKey = "all_folders"
	CacheKeyFolderPaths           CacheKey = "all_folder_paths"
	CacheKeyFilePaths             CacheKey = "all_file_paths"
	CacheKeyTitles                CacheKey = "all_titles"
	CacheKeyOrphanedMedia         CacheKey = "orphaned_media"
	CacheKeyAncestorsInCollection CacheKey = "ancestors_in_collection/"
	CacheKeyFullFileList          CacheKey = "all_files_full"
)

// saveFileListToCache persists the full file list (including metadata) to cache storage
func saveFileListToCache(allFiles []File) error {
	logging.LogDebug("saving %s to cache", CacheKeyFullFileList)
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
	cached, err := getFileListFromCache()
	if err != nil {
		logging.LogWarning("failed to read file list cache, falling back to live data: %v", err)
	} else if cached != nil {
		return cached, nil
	}

	allFiles, err := GetAllPhysicalFiles()
	if err != nil {
		return nil, err
	}

	if err := saveFileListToCache(allFiles); err != nil {
		logging.LogWarning("failed to persist file list cache: %v", err)
	}

	return allFiles, nil
}

// InvalidateFileListCache forces the next GetAllFilesCached call to rebuild
// from disk. Called after any mutation that adds, removes, renames, or
// changes the visibility-relevant metadata of a file.
func InvalidateFileListCache() {
	if err := cacheStorage.Delete(string(CacheKeyFullFileList)); err != nil {
		logging.LogWarning("failed to invalidate file list cache: %v", err)
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
	go func() {
		if err := RebuildAllCaches(); err != nil {
			logging.LogWarning("failed to refresh caches after mutation: %v", err)
		}
	}()
}

// saveStringListToCache saves a sorted string list to cache storage
func saveStringListToCache(key CacheKey, data []string) error {
	logging.LogDebug("saving %s to cache", key)
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

// GetAllTags returns all unique tags with their counts
func GetAllTags() (TagCount, error) {
	allFiles, err := GetAllFiles()
	if err != nil {
		return nil, err
	}

	tagCount := make(TagCount)
	for _, file := range allFiles {
		metadata, err := MetaDataGet(file.Path)
		if err != nil || metadata == nil {
			continue
		}
		for _, tag := range metadata.Tags {
			if tag != "" {
				tagCount[tag]++
			}
		}
	}

	return tagCount, nil
}

// GetAllCollections returns all unique collections with their counts
func GetAllCollections() (CollectionCount, error) {
	allFiles, err := GetAllFiles()
	if err != nil {
		return nil, err
	}

	collectionCount := make(CollectionCount)
	for _, file := range allFiles {
		metadata, err := MetaDataGet(file.Path)
		if err != nil || metadata == nil {
			continue
		}
		if metadata.Collection != "" {
			collectionCount[metadata.Collection]++
		}
	}

	return collectionCount, nil
}

// GetAllFolders returns all unique folders with their counts
func GetAllFolders() (FolderCount, error) {
	allFiles, err := GetAllFiles()
	if err != nil {
		return nil, err
	}

	folderCount := make(FolderCount)
	for _, file := range allFiles {
		metadata, err := MetaDataGet(file.Path)
		if err != nil || metadata == nil {
			continue
		}
		for _, f := range metadata.Folders {
			if f != "" {
				folderCount[f]++
			}
		}
	}

	return folderCount, nil
}

// GetAllEditors returns all unique filetypes with their counts
func GetAllEditors() (EditorTypeCount, error) {
	allFiles, err := GetAllFiles()
	if err != nil {
		return nil, err
	}

	editorTypeCount := make(EditorTypeCount)
	for _, file := range allFiles {
		metadata, err := MetaDataGet(file.Path)
		if err != nil || metadata == nil {
			continue
		}
		if metadata.Editor != "" {
			editorTypeCount[string(metadata.Editor)]++
		}
	}

	return editorTypeCount, nil
}

// SaveAllTagsToCache saves all unique tags to cache storage
func SaveAllTagsToCache() error {
	allTags, err := GetAllTags()
	if err != nil {
		return err
	}

	var tagList []string
	for tag := range allTags {
		tagList = append(tagList, tag)
	}

	return saveStringListToCache(CacheKeyTags, tagList)
}

// GetAllTagsFromCache retrieves cached tags from cache storage
func GetAllTagsFromCache() ([]string, error) {
	return getStringListFromCache(CacheKeyTags)
}

// SaveAllCollectionsToCache saves all unique collections to cache storage
func SaveAllCollectionsToCache() error {
	allCollections, err := GetAllCollections()
	if err != nil {
		return err
	}

	var collectionList []string
	for collection := range allCollections {
		collectionList = append(collectionList, collection)
	}

	return saveStringListToCache(CacheKeyCollections, collectionList)
}

// GetAllCollectionsFromCache retrieves cached collections from cache storage
func GetAllCollectionsFromCache() ([]string, error) {
	return getStringListFromCache(CacheKeyCollections)
}

// SaveAllFoldersToCache saves all unique folders to cache storage
func SaveAllFoldersToCache() error {
	allFolders, err := GetAllFolders()
	if err != nil {
		return err
	}

	var folderList []string
	for folder := range allFolders {
		folderList = append(folderList, folder)
	}

	return saveStringListToCache(CacheKeyFolders, folderList)
}

// GetAllFoldersFromCache retrieves cached folders from cache storage
func GetAllFoldersFromCache() ([]string, error) {
	return getStringListFromCache(CacheKeyFolders)
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

// GetAllTitles returns all unique non-empty titles, reading from file content if the DB title is empty
func GetAllTitles() ([]string, error) {
	allFiles, err := GetAllFiles()
	if err != nil {
		return nil, err
	}

	logging.LogInfo("getAllTitles: scanning %d files", len(allFiles))
	seen := make(map[string]bool)
	var titles []string
	for _, file := range allFiles {
		meta := file.Metadata
		if meta == nil {
			logging.LogDebug("getAllTitles: no metadata for %s", file.Path)
			continue
		}
		title := meta.Title
		if title == "" {
			updateTitle(meta)
			title = meta.Title
		}
		logging.LogDebug("getAllTitles: %s -> %q", file.Path, title)
		if title != "" && !seen[title] {
			seen[title] = true
			titles = append(titles, title)
		}
	}
	logging.LogInfo("getAllTitles: found %d unique titles", len(titles))
	slices.Sort(titles)
	return titles, nil
}

// MetadataCollector collects metadata across multiple files efficiently
type MetadataCollector struct {
	Tags                  map[string]bool
	Collections           map[string]bool
	Folders               map[string]bool
	FolderPaths           map[string]bool
	Titles                map[string]bool
	FilePaths             []string
	OrphanedMedia         []string
	AncestorsInCollection map[string]map[string]bool // collection → set of ancestor paths
}

// NewMetadataCollector creates a new metadata collector
func NewMetadataCollector() *MetadataCollector {
	return &MetadataCollector{
		Tags:                  make(map[string]bool),
		Collections:           make(map[string]bool),
		Folders:               make(map[string]bool),
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

	// collect tags
	for _, tag := range metadata.Tags {
		if tag != "" {
			mc.Tags[tag] = true
		}
	}

	// collect collections
	if metadata.Collection != "" {
		mc.Collections[metadata.Collection] = true
	}

	for _, f := range metadata.Folders {
		if f != "" {
			mc.Folders[f] = true
		}
	}

	// collect folder paths from file path
	dir := filepath.Dir(filePath)
	if dir != "." && dir != "" {
		// generate all parent paths: xxx/, xxx/yyy/, xxx/yyy/zzz/
		parts := strings.Split(dir, string(filepath.Separator))
		for i := 1; i <= len(parts); i++ {
			path := strings.Join(parts[:i], "/") + "/"
			mc.FolderPaths[path] = true
		}
	}

	// collect orphaned media
	if strings.HasPrefix(filePath, "media/") && len(metadata.LinksToHere) == 0 {
		mc.OrphanedMedia = append(mc.OrphanedMedia, filePath)
	}

	// collect title (fall back to reading from file content if not in DB)
	title := metadata.Title
	if title == "" {
		updateTitle(metadata)
		title = metadata.Title
	}
	if title != "" {
		mc.Titles[title] = true
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
	if err := saveStringListToCache(CacheKeyTags, utils.SetToSortedSlice(mc.Tags)); err != nil {
		return err
	}
	if err := saveStringListToCache(CacheKeyCollections, utils.SetToSortedSlice(mc.Collections)); err != nil {
		return err
	}
	if err := saveStringListToCache(CacheKeyFolders, utils.SetToSortedSlice(mc.Folders)); err != nil {
		return err
	}
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
	logging.LogInfo("collecting all system metadata for cache update")

	collector := NewMetadataCollector()

	// collect from document files (pathsToFiles already attached metadata to each file)
	allFiles, err := GetAllFiles()
	if err != nil {
		return err
	}

	for _, file := range allFiles {
		if file.Metadata == nil {
			continue
		}
		collector.CollectFromMetadata(file.Path, file.Metadata)
	}

	// persist the full file list too, so tree/list requests can reuse this same
	// walk instead of triggering their own
	if err := saveFileListToCache(allFiles); err != nil {
		logging.LogWarning("failed to persist file list cache: %v", err)
	}

	// collect from media files (needed for orphaned media detection)
	mediaFiles, err := GetAllMediaFiles()
	if err != nil {
		logging.LogWarning("failed to get media files for cache update: %v", err)
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

	logging.LogInfo("system metadata cache update completed")
	return nil
}

// GetAllFolderPathsFromCache retrieves cached folder path suggestions from cache storage
func GetAllFolderPathsFromCache() ([]string, error) {
	return getStringListFromCache(CacheKeyFolderPaths)
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
		// get directory path
		dir := filepath.ToSlash(filepath.Dir(file.Path))
		if dir == "." || dir == "" {
			continue
		}

		// generate all parent paths
		parts := strings.Split(dir, "/")
		for i := 1; i <= len(parts); i++ {
			path := strings.Join(parts[:i], "/") + "/"
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

// UpdateOrphanedMediaCache efficiently updates only the orphaned media cache
// by checking media files instead of all files
func UpdateOrphanedMediaCache() error {
	logging.LogDebug("updating orphaned media cache")

	mediaFiles, err := GetAllMediaFiles()
	if err != nil {
		return err
	}

	var orphanedMedia []string
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

	logging.LogDebug("orphaned media cache updated: %d orphaned files", len(orphanedMedia))
	return nil
}

// UpdateOrphanedMediaCacheForFile incrementally updates orphaned media cache
// for media files affected by changes to a specific file
func UpdateOrphanedMediaCacheForFile(filePath string) error {
	logging.LogDebug("incrementally updating orphaned media cache for file: %s", filePath)

	// get file metadata to find affected media files
	metadata, err := MetaDataGet(filePath)
	if err != nil || metadata == nil {
		logging.LogDebug("no metadata found for %s, skipping cache update", filePath)
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
		logging.LogDebug("no media files affected by changes to %s", filePath)
		return nil
	}

	// get current orphaned media cache
	orphanedMedia, err := GetOrphanedMediaFromCache()
	if err != nil {
		logging.LogWarning("failed to get orphaned media cache, will rebuild: %v", err)
		return UpdateOrphanedMediaCache() // fallback to full rebuild
	}

	// if cache is empty, rebuild it instead of trying to update incrementally
	if len(orphanedMedia) == 0 {
		logging.LogDebug("orphaned media cache is empty, rebuilding instead of incremental update")
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

	logging.LogDebug("incrementally updated orphaned media cache: checked %d affected files", len(affectedMediaFiles))
	return nil
}

// CacheInvalidate removes all cache entries, forcing a rebuild on next access
func CacheInvalidate() error {
	if err := cacheStorage.Flush(); err != nil {
		return fmt.Errorf("failed to invalidate cache: %w", err)
	}
	logging.LogInfo("cache invalidated")
	return nil
}

// GetAncestorsInCollection returns unique ancestor paths from all files in a collection
func GetAncestorsInCollection(collection string) ([]string, error) {
	allFiles, err := GetAllPhysicalFiles()
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{})
	var ancestors []string
	for _, f := range allFiles {
		if f.Metadata == nil || f.Metadata.Collection != collection {
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
