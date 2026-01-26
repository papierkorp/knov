// Package files - System-wide metadata operations (cache, aggregation, PARA)
package files

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"

	"knov/internal/cacheStorage"
	"knov/internal/logging"
	"knov/internal/utils"
)

// CacheKey represents system cache keys
type CacheKey string

const (
	CacheKeyTags          CacheKey = "all_tags"
	CacheKeyCollections   CacheKey = "all_collections"
	CacheKeyFolders       CacheKey = "all_folders"
	CacheKeyFolderPaths   CacheKey = "all_folder_paths"
	CacheKeyPARAProjects  CacheKey = "all_para_projects"
	CacheKeyPARAreas      CacheKey = "all_para_areas"
	CacheKeyPARAResources CacheKey = "all_para_resources"
	CacheKeyPARAArchive   CacheKey = "all_para_archive"
	CacheKeyFilePaths     CacheKey = "all_file_paths"
	CacheKeyOrphanedMedia CacheKey = "orphaned_media"
)

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
		if metadata.Folder != "" {
			folderCount[metadata.Folder]++
		}
	}

	return folderCount, nil
}

// GetAllFiletypes returns all unique filetypes with their counts
func GetAllFiletypes() (FiletypeCount, error) {
	allFiles, err := GetAllFiles()
	if err != nil {
		return nil, err
	}

	filetypeCount := make(FiletypeCount)
	for _, file := range allFiles {
		metadata, err := MetaDataGet(file.Path)
		if err != nil || metadata == nil {
			continue
		}
		if metadata.FileType != "" {
			filetypeCount[string(metadata.FileType)]++
		}
	}

	return filetypeCount, nil
}

// GetAllPriorities returns all unique priorities with their counts
func GetAllPriorities() (PriorityCount, error) {
	allFiles, err := GetAllFiles()
	if err != nil {
		return nil, err
	}

	priorityCount := make(PriorityCount)
	for _, file := range allFiles {
		metadata, err := MetaDataGet(file.Path)
		if err != nil || metadata == nil {
			continue
		}
		if metadata.Priority != "" {
			priorityCount[string(metadata.Priority)]++
		}
	}

	return priorityCount, nil
}

// GetAllStatuses returns all unique statuses with their counts
func GetAllStatuses() (StatusCount, error) {
	allFiles, err := GetAllFiles()
	if err != nil {
		return nil, err
	}

	statusCount := make(StatusCount)
	for _, file := range allFiles {
		metadata, err := MetaDataGet(file.Path)
		if err != nil || metadata == nil {
			continue
		}
		if metadata.Status != "" {
			statusCount[string(metadata.Status)]++
		}
	}

	return statusCount, nil
}

// GetAllPARAProjects returns all unique PARA projects with their counts
func GetAllPARAProjects() (PARAProjectCount, error) {
	allFiles, err := GetAllFiles()
	if err != nil {
		return nil, err
	}

	projectCount := make(PARAProjectCount)
	for _, file := range allFiles {
		metadata, err := MetaDataGet(file.Path)
		if err != nil || metadata == nil {
			continue
		}
		for _, project := range metadata.PARA.Projects {
			if project != "" {
				projectCount[project]++
			}
		}
	}

	return projectCount, nil
}

// GetAllPARAreas returns all unique PARA areas with their counts
func GetAllPARAreas() (PARAAreaCount, error) {
	allFiles, err := GetAllFiles()
	if err != nil {
		return nil, err
	}

	areaCount := make(PARAAreaCount)
	for _, file := range allFiles {
		metadata, err := MetaDataGet(file.Path)
		if err != nil || metadata == nil {
			continue
		}
		for _, area := range metadata.PARA.Areas {
			if area != "" {
				areaCount[area]++
			}
		}
	}

	return areaCount, nil
}

// GetAllPARAResources returns all unique PARA resources with their counts
func GetAllPARAResources() (PARAResourceCount, error) {
	allFiles, err := GetAllFiles()
	if err != nil {
		return nil, err
	}

	resourceCount := make(PARAResourceCount)
	for _, file := range allFiles {
		metadata, err := MetaDataGet(file.Path)
		if err != nil || metadata == nil {
			continue
		}
		for _, resource := range metadata.PARA.Resources {
			if resource != "" {
				resourceCount[resource]++
			}
		}
	}

	return resourceCount, nil
}

// GetAllPARAArchive returns all unique PARA archive with their counts
func GetAllPARAArchive() (PARAArchiveCount, error) {
	allFiles, err := GetAllFiles()
	if err != nil {
		return nil, err
	}

	archiveCount := make(PARAArchiveCount)
	for _, file := range allFiles {
		metadata, err := MetaDataGet(file.Path)
		if err != nil || metadata == nil {
			continue
		}
		for _, archive := range metadata.PARA.Archive {
			if archive != "" {
				archiveCount[archive]++
			}
		}
	}

	return archiveCount, nil
}

// SaveAllTagsToSystemData saves all unique tags to system storage
func SaveAllTagsToSystemData() error {
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

// GetAllTagsFromSystemData retrieves cached tags from system storage
func GetAllTagsFromSystemData() ([]string, error) {
	return getStringListFromCache(CacheKeyTags)
}

// SaveAllCollectionsToSystemData saves all unique collections to system storage
func SaveAllCollectionsToSystemData() error {
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

// GetAllCollectionsFromSystemData retrieves cached collections from system storage
func GetAllCollectionsFromSystemData() ([]string, error) {
	return getStringListFromCache(CacheKeyCollections)
}

// SaveAllFoldersToSystemData saves all unique folders to system storage
func SaveAllFoldersToSystemData() error {
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

// GetAllFoldersFromSystemData retrieves cached folders from system storage
func GetAllFoldersFromSystemData() ([]string, error) {
	return getStringListFromCache(CacheKeyFolders)
}

// SaveAllPARAProjectsToSystemData saves all PARA projects to system storage
func SaveAllPARAProjectsToSystemData() error {
	allProjects, err := GetAllPARAProjects()
	if err != nil {
		return err
	}

	var projectList []string
	for project := range allProjects {
		projectList = append(projectList, project)
	}

	return saveStringListToCache(CacheKeyPARAProjects, projectList)
}

// GetAllPARAProjectsFromSystemData retrieves cached PARA projects from system storage
func GetAllPARAProjectsFromSystemData() ([]string, error) {
	return getStringListFromCache(CacheKeyPARAProjects)
}

// SaveAllPARAAreasToSystemData saves all PARA areas to system storage
func SaveAllPARAAreasToSystemData() error {
	allAreas, err := GetAllPARAreas()
	if err != nil {
		return err
	}

	var areaList []string
	for area := range allAreas {
		areaList = append(areaList, area)
	}

	return saveStringListToCache(CacheKeyPARAreas, areaList)
}

// GetAllPARAAreasFromSystemData retrieves cached PARA areas from system storage
func GetAllPARAAreasFromSystemData() ([]string, error) {
	return getStringListFromCache(CacheKeyPARAreas)
}

// SaveAllPARAResourcesToSystemData saves all PARA resources to system storage
func SaveAllPARAResourcesToSystemData() error {
	allResources, err := GetAllPARAResources()
	if err != nil {
		return err
	}

	var resourceList []string
	for resource := range allResources {
		resourceList = append(resourceList, resource)
	}

	return saveStringListToCache(CacheKeyPARAResources, resourceList)
}

// GetAllPARAResourcesFromSystemData retrieves cached PARA resources from system storage
func GetAllPARAResourcesFromSystemData() ([]string, error) {
	return getStringListFromCache(CacheKeyPARAResources)
}

// SaveAllPARAArchiveToSystemData saves all PARA archive items to system storage
func SaveAllPARAArchiveToSystemData() error {
	allArchive, err := GetAllPARAArchive()
	if err != nil {
		return err
	}

	var archiveList []string
	for archive := range allArchive {
		archiveList = append(archiveList, archive)
	}

	return saveStringListToCache(CacheKeyPARAArchive, archiveList)
}

// GetAllPARAArchiveFromSystemData retrieves cached PARA archive from system storage
func GetAllPARAArchiveFromSystemData() ([]string, error) {
	return getStringListFromCache(CacheKeyPARAArchive)
}

// SaveAllFilePathsToSystemData saves all file paths to system storage
func SaveAllFilePathsToSystemData() error {
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

// GetAllFilePathsFromSystemData retrieves cached file paths from system storage
func GetAllFilePathsFromSystemData() ([]string, error) {
	return getStringListFromCache(CacheKeyFilePaths)
}

// MetadataCollector collects metadata across multiple files efficiently
type MetadataCollector struct {
	Tags          map[string]bool
	Collections   map[string]bool
	Folders       map[string]bool
	FolderPaths   map[string]bool
	PARAProjects  map[string]bool
	PARAreas      map[string]bool
	PARAResources map[string]bool
	PARAArchive   map[string]bool
	FilePaths     []string
	OrphanedMedia []string
}

// NewMetadataCollector creates a new metadata collector
func NewMetadataCollector() *MetadataCollector {
	return &MetadataCollector{
		Tags:          make(map[string]bool),
		Collections:   make(map[string]bool),
		Folders:       make(map[string]bool),
		FolderPaths:   make(map[string]bool),
		PARAProjects:  make(map[string]bool),
		PARAreas:      make(map[string]bool),
		PARAResources: make(map[string]bool),
		PARAArchive:   make(map[string]bool),
		FilePaths:     []string{},
		OrphanedMedia: []string{},
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

	// collect folders
	if metadata.Folder != "" {
		mc.Folders[metadata.Folder] = true
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

	// collect PARA data
	for _, project := range metadata.PARA.Projects {
		if project != "" {
			mc.PARAProjects[project] = true
		}
	}
	for _, area := range metadata.PARA.Areas {
		if area != "" {
			mc.PARAreas[area] = true
		}
	}
	for _, resource := range metadata.PARA.Resources {
		if resource != "" {
			mc.PARAResources[resource] = true
		}
	}
	for _, archive := range metadata.PARA.Archive {
		if archive != "" {
			mc.PARAArchive[archive] = true
		}
	}

	// collect orphaned media
	if strings.HasPrefix(filePath, "media/") && len(metadata.LinksToHere) == 0 {
		mc.OrphanedMedia = append(mc.OrphanedMedia, filePath)
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
	if err := saveStringListToCache(CacheKeyPARAProjects, utils.SetToSortedSlice(mc.PARAProjects)); err != nil {
		return err
	}
	if err := saveStringListToCache(CacheKeyPARAreas, utils.SetToSortedSlice(mc.PARAreas)); err != nil {
		return err
	}
	if err := saveStringListToCache(CacheKeyPARAResources, utils.SetToSortedSlice(mc.PARAResources)); err != nil {
		return err
	}
	if err := saveStringListToCache(CacheKeyPARAArchive, utils.SetToSortedSlice(mc.PARAArchive)); err != nil {
		return err
	}
	if err := saveStringListToCache(CacheKeyFilePaths, mc.FilePaths); err != nil {
		return err
	}
	if err := saveStringListToCache(CacheKeyOrphanedMedia, mc.OrphanedMedia); err != nil {
		return err
	}
	return nil
}

// SaveAllSystemDataToCache saves all metadata lists to system storage in a single pass
func SaveAllSystemDataToCache() error {
	logging.LogInfo("collecting all system metadata for cache update")

	collector := NewMetadataCollector()

	// collect from document files
	allFiles, err := GetAllFiles()
	if err != nil {
		return err
	}

	for _, file := range allFiles {
		metadata, err := MetaDataGet(file.Path)
		if err != nil || metadata == nil {
			continue
		}
		collector.CollectFromMetadata(file.Path, metadata)
	}

	// collect from media files (needed for orphaned media detection)
	mediaFiles, err := GetAllMediaFiles()
	if err != nil {
		logging.LogWarning("failed to get media files for cache update: %v", err)
	} else {
		for _, file := range mediaFiles {
			metadata, err := MetaDataGet(file.Path)
			if err != nil || metadata == nil {
				continue
			}
			collector.CollectFromMetadata(file.Path, metadata)
		}
	}

	if err := collector.SaveAllToCache(); err != nil {
		return err
	}

	logging.LogInfo("system metadata cache update completed")
	return nil
}

// GetAllFolderPathsFromSystemData retrieves cached folder path suggestions from system storage
func GetAllFolderPathsFromSystemData() ([]string, error) {
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
		dir := filepath.Dir(file.Path)
		if dir == "." || dir == "" {
			continue
		}

		// generate all parent paths
		parts := strings.Split(dir, string(filepath.Separator))
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

// SaveAllFolderPathsToSystemData saves all folder path suggestions to system storage
func SaveAllFolderPathsToSystemData() error {
	folderPaths, err := GetAllFolderPaths()
	if err != nil {
		return err
	}

	return saveStringListToCache(CacheKeyFolderPaths, folderPaths)
}

// GetOrphanedMediaFromCache retrieves cached orphaned media list from system storage
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
