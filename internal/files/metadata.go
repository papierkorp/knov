// Package files handles file operations and metadata
package files

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"knov/internal/logging"
	"knov/internal/storage"
	"knov/internal/utils"
)

type Filetype string
type Status string
type Priority string

const (
	FileTypeTodo       Filetype = "todo"
	FileTypeFleeting   Filetype = "fleeting"
	FileTypeLiterature Filetype = "literature"
	FileTypeMOC        Filetype = "moc" // maps of content - indexes to link related notes
	FileTypePermanent  Filetype = "permanent"
	FileTypeFilter     Filetype = "filter"
	FileTypeJournaling Filetype = "journaling"

	StatusDraft     Status = "draft"
	StatusPublished Status = "published"
	StatusArchived  Status = "archived"

	PriorityLow    Priority = "low"
	PriorityMedium Priority = "medium"
	PriorityHigh   Priority = "high"
)

// typed count maps for metadata aggregations
type TagCount map[string]int
type CollectionCount map[string]int
type FolderCount map[string]int
type FiletypeCount map[string]int
type PriorityCount map[string]int
type StatusCount map[string]int
type PARAProjectCount map[string]int
type PARAAreaCount map[string]int
type PARAResourceCount map[string]int
type PARAArchiveCount map[string]int

// AllFiletypes returns all available file types
func AllFiletypes() []Filetype {
	return []Filetype{
		FileTypeTodo,
		FileTypeFleeting,
		FileTypeLiterature,
		FileTypeMOC,
		FileTypePermanent,
		FileTypeFilter,
		FileTypeJournaling,
	}
}

// AllPriorities returns all available priorities
func AllPriorities() []Priority {
	return []Priority{
		PriorityLow,
		PriorityMedium,
		PriorityHigh,
	}
}

// AllStatuses returns all available statuses
func AllStatuses() []Status {
	return []Status{
		StatusDraft,
		StatusPublished,
		StatusArchived,
	}
}

// IsValidFiletype checks if a filetype is valid
func IsValidFiletype(ft Filetype) bool {
	for _, valid := range AllFiletypes() {
		if ft == valid {
			return true
		}
	}
	return false
}

// IsValidPriority checks if a priority is valid
func IsValidPriority(p Priority) bool {
	for _, valid := range AllPriorities() {
		if p == valid {
			return true
		}
	}
	return false
}

// IsValidStatus checks if a status is valid
func IsValidStatus(s Status) bool {
	for _, valid := range AllStatuses() {
		if s == valid {
			return true
		}
	}
	return false
}

// Metadata represents file metadata
type Metadata struct {
	Name        string    `json:"name"`        // manual filename
	Path        string    `json:"path"`        // auto
	Title       string    `json:"title"`       // auto
	CreatedAt   time.Time `json:"createdAt"`   // auto
	LastEdited  time.Time `json:"lastEdited"`  // auto
	TargetDate  time.Time `json:"targetDate"`  // manual
	Collection  string    `json:"collection"`  // auto / manual possible
	Folders     []string  `json:"folders"`     // auto
	Tags        []string  `json:"tags"`        // manual
	Boards      []string  `json:"boards"`      // auto
	Ancestor    []string  `json:"ancestor"`    // auto
	Parents     []string  `json:"parents"`     // manual
	Kids        []string  `json:"kids"`        // auto
	UsedLinks   []string  `json:"usedLinks"`   // auto
	LinksToHere []string  `json:"linksToHere"` // auto
	FileType    Filetype  `json:"type"`        // manual - with add new
	PARA        PARA      `json:"para"`        // manual
	Status      Status    `json:"status"`      // manual
	Priority    Priority  `json:"priority"`    // manual
	Size        int64     `json:"size"`        // auto
}

// PARA represents PARA organization
type PARA struct {
	Projects  []string `json:"projects,omitempty"`  // Active projects with deadlines
	Areas     []string `json:"areas,omitempty"`     // Ongoing responsibilities
	Resources []string `json:"resources,omitempty"` // Future reference materials
	Archive   []string `json:"archive,omitempty"`   // Inactive items
}

func metaDataUpdate(filePath string, newMetadata *Metadata) *Metadata {
	currentMetadata, _ := MetaDataGet(filePath)

	normalizedPath := utils.ToRelativePath(filePath)
	fullPath := utils.ToFullPath(normalizedPath)

	fileInfo, err := os.Stat(fullPath)

	if err != nil {
		logging.LogError("failed to get file info for %s: %v", filePath, err)
		return nil
	}

	if currentMetadata == nil {
		currentMetadata = &Metadata{}
	}

	currentMetadata.Name = fileInfo.Name()
	currentMetadata.Path = normalizedPath
	if currentMetadata.CreatedAt.IsZero() {
		currentMetadata.CreatedAt = fileInfo.ModTime()
	}
	currentMetadata.LastEdited = fileInfo.ModTime()
	currentMetadata.Size = fileInfo.Size()

	pathParts := strings.Split(strings.Trim(normalizedPath, "/"), "/")
	if len(pathParts) > 1 && pathParts[0] != "" {
		currentMetadata.Collection = pathParts[0]
	} else {
		currentMetadata.Collection = "default"
	}

	dir := filepath.Dir(utils.ToRelativePath(fullPath))
	if dir != "." && dir != "" {
		currentMetadata.Folders = strings.Split(dir, string(filepath.Separator))
	}

	// save previous data
	if newMetadata != nil {
		// store old parents for cleanup
		var oldParents []string
		if currentMetadata.Parents != nil {
			oldParents = make([]string, len(currentMetadata.Parents))
			copy(oldParents, currentMetadata.Parents)
		}

		// allow explicit setting of empty arrays by checking if field was provided
		if newMetadata.Tags != nil {
			logging.LogInfo("updating tags for %s: old=%v, new=%v", currentMetadata.Path, currentMetadata.Tags, newMetadata.Tags)
			currentMetadata.Tags = newMetadata.Tags
		}
		if newMetadata.Parents != nil {
			normalized := make([]string, 0, len(newMetadata.Parents))
			for _, parent := range newMetadata.Parents {
				normalized = append(normalized, utils.CleanLink(parent))
			}
			currentMetadata.Parents = normalized
			// update parent-child relationships when parents change
			updateParentChildRelationships(currentMetadata, oldParents)
		}
		if newMetadata.FileType != "" {
			currentMetadata.FileType = newMetadata.FileType
		}
		if newMetadata.Collection != "" {
			currentMetadata.Collection = newMetadata.Collection
		}
		if newMetadata.Status != "" {
			currentMetadata.Status = newMetadata.Status
		}
		if !newMetadata.CreatedAt.IsZero() {
			currentMetadata.CreatedAt = newMetadata.CreatedAt
		}
		if newMetadata.Priority != "" {
			currentMetadata.Priority = newMetadata.Priority
		}
		if newMetadata.PARA.Projects != nil {
			currentMetadata.PARA.Projects = newMetadata.PARA.Projects
		}
		if newMetadata.PARA.Areas != nil {
			currentMetadata.PARA.Areas = newMetadata.PARA.Areas
		}
		if newMetadata.PARA.Resources != nil {
			currentMetadata.PARA.Resources = newMetadata.PARA.Resources
		}
		if newMetadata.PARA.Archive != nil {
			currentMetadata.PARA.Archive = newMetadata.PARA.Archive
		}
	}

	// init
	if currentMetadata.Tags == nil {
		currentMetadata.Tags = []string{}
	}
	if currentMetadata.Folders == nil {
		currentMetadata.Folders = []string{}
	}
	if currentMetadata.Boards == nil {
		currentMetadata.Boards = []string{}
	}
	if currentMetadata.Ancestor == nil {
		currentMetadata.Ancestor = []string{}
	}
	if currentMetadata.Parents == nil {
		currentMetadata.Parents = []string{}
	}
	if currentMetadata.Kids == nil {
		currentMetadata.Kids = []string{}
	}
	if currentMetadata.UsedLinks == nil {
		currentMetadata.UsedLinks = []string{}
	}
	if currentMetadata.LinksToHere == nil {
		currentMetadata.LinksToHere = []string{}
	}
	if currentMetadata.FileType == "" {
		currentMetadata.FileType = FileTypeFleeting
	}
	if currentMetadata.Status == "" {
		currentMetadata.Status = StatusPublished
	}
	if currentMetadata.Priority == "" {
		currentMetadata.Priority = PriorityMedium
	}
	if currentMetadata.PARA.Projects == nil {
		currentMetadata.PARA.Projects = []string{}
	}
	if currentMetadata.PARA.Areas == nil {
		currentMetadata.PARA.Areas = []string{}
	}
	if currentMetadata.PARA.Resources == nil {
		currentMetadata.PARA.Resources = []string{}
	}
	if currentMetadata.PARA.Archive == nil {
		currentMetadata.PARA.Archive = []string{}
	}

	updateAncestors(currentMetadata)
	updateUsedLinks(currentMetadata)
	updateTitle(currentMetadata)
	// updateKidsAndLinksToHere(currentMetadata) // shouldnt run with every filesave since it loops through all files

	return currentMetadata
}

// MetaDataSave saves metadata using the configured storage method
func MetaDataSave(m *Metadata) error {
	finalMetadata := metaDataUpdate(m.Path, m)
	if finalMetadata == nil {
		return nil
	}

	data, err := json.Marshal(finalMetadata)
	if err != nil {
		logging.LogError("failed to marshal metadata: %v", err)
		return err
	}

	key := finalMetadata.Path
	if err := storage.GetStorage().Set(key, data); err != nil {
		logging.LogError("failed to save metadata for %s: %v", finalMetadata.Path, err)
		return err
	}

	logging.LogDebug("metadata saved for: %s", finalMetadata.Path)
	return nil
}

// metaDataSaveRaw saves metadata directly without triggering metaDataUpdate
// used internally to avoid cascading updates when updating link relationships
func metaDataSaveRaw(m *Metadata) error {
	data, err := json.Marshal(m)
	if err != nil {
		logging.LogError("failed to marshal metadata: %v", err)
		return err
	}

	key := m.Path
	if err := storage.GetStorage().Set(key, data); err != nil {
		logging.LogError("failed to save metadata for %s: %v", m.Path, err)
		return err
	}

	logging.LogDebug("metadata saved (raw) for: %s", m.Path)
	return nil
}

// MetaDataGet retrieves metadata using the configured storage method
func MetaDataGet(filepath string) (*Metadata, error) {
	key := utils.ToRelativePath(filepath)
	data, err := storage.GetStorage().Get(key)
	if err != nil {
		logging.LogError("failed to get metadata for %s: %v", filepath, err)
		return nil, err
	}

	if data == nil {
		logging.LogDebug("no metadata found for file: %s", filepath)
		return nil, nil
	}

	var metadata Metadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		logging.LogError("failed to unmarshal metadata for %s: %v", filepath, err)
		return nil, err
	}

	return &metadata, nil
}

// MetaDataInitializeAll creates metadata for all files that don't have it yet
func MetaDataInitializeAll() error {
	files, err := GetAllFiles()
	if err != nil {
		logging.LogError("failed to get all files: %v", err)
		return err
	}

	for _, file := range files {
		existing, err := MetaDataGet(file.Path)
		if err != nil {
			logging.LogError("failed to check existing metadata for %s: %v", file.Path, err)
			continue
		}

		if existing == nil {
			metadata := &Metadata{Path: file.Path}
			if err := MetaDataSave(metadata); err != nil {
				logging.LogError("failed to save metadata for %s: %v", file.Path, err)
				continue
			}
			logging.LogDebug("created metadata for %s", file.Path)
		} else {
			logging.LogDebug("metadata already exists for %s", file.Path)
		}
	}

	logging.LogInfo("metadata initialization completed")
	return nil
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
		for _, folder := range metadata.Folders {
			if folder != "" {
				folderCount[folder]++
			}
		}
	}
	return folderCount, nil
}

// GetAllFiletypes returns all unique file types with their counts
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

// MetaDataDelete removes metadata for a file
func MetaDataDelete(filepath string) error {
	key := utils.ToRelativePath(filepath)
	if err := storage.GetStorage().Delete(key); err != nil {
		logging.LogError("failed to delete metadata for %s: %v", filepath, err)
		return err
	}

	logging.LogDebug("metadata deleted for: %s", filepath)
	return nil
}

// GetAllPARAProjects returns all unique PARA projects with their counts
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

// MetaDataExportAll exports all metadata in the specified format
func MetaDataExportAll() ([]*Metadata, error) {
	keys, err := storage.GetStorage().List("")
	if err != nil {
		logging.LogError("failed to list metadata keys: %v", err)
		return nil, err
	}

	var allMetadata []*Metadata
	for _, key := range keys {
		data, err := storage.GetStorage().Get(key)
		if err != nil {
			logging.LogWarning("failed to get metadata for key %s: %v", key, err)
			continue
		}

		if data == nil {
			continue
		}

		var metadata Metadata
		if err := json.Unmarshal(data, &metadata); err != nil {
			logging.LogWarning("failed to unmarshal metadata for key %s: %v", key, err)
			continue
		}

		allMetadata = append(allMetadata, &metadata)
	}

	logging.LogDebug("exported %d metadata entries", len(allMetadata))
	return allMetadata, nil
}

// updateTitle extracts title from the first header line in the file content
func updateTitle(metadata *Metadata) {
	fullPath := utils.ToFullPath(metadata.Path)

	logging.LogDebug("extracting title for %s", metadata.Path)

	file, err := os.Open(fullPath)
	if err != nil {
		logging.LogWarning("failed to open file %s: %v", fullPath, err)
		return
	}
	defer file.Close()

	// read only the first few lines to find the header
	buffer := make([]byte, 1024)
	n, err := file.Read(buffer)
	if err != nil && n == 0 {
		logging.LogWarning("failed to read file %s: %v", fullPath, err)
		return
	}

	content := string(buffer[:n])
	lines := strings.Split(content, "\n")

	// look for first header in the first few lines
	for i, line := range lines {
		if i > 10 { // only check first 10 lines
			break
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var title string

		// markdown headers: # Header
		if strings.HasPrefix(line, "#") {
			title = strings.TrimSpace(strings.TrimLeft(line, "#"))
		}

		// dokuwiki headers: ====== Header ======
		if strings.HasPrefix(line, "======") && strings.HasSuffix(line, "======") {
			title = strings.TrimSpace(strings.Trim(line, "="))
		} else if strings.HasPrefix(line, "=====") && strings.HasSuffix(line, "=====") {
			title = strings.TrimSpace(strings.Trim(line, "="))
		} else if strings.HasPrefix(line, "====") && strings.HasSuffix(line, "====") {
			title = strings.TrimSpace(strings.Trim(line, "="))
		}

		if title != "" {
			metadata.Title = title
			logging.LogDebug("extracted title for %s: %s", metadata.Path, title)
			return
		}
	}

	// no title found, clear any existing title
	metadata.Title = ""
	logging.LogDebug("no title found for %s", metadata.Path)
}

// updateParentChildRelationships updates parent-child relationships when parents change
func updateParentChildRelationships(metadata *Metadata, oldParents []string) {
	logging.LogInfo("updating parent-child relationships for %s: old=%v, new=%v", metadata.Path, oldParents, metadata.Parents)

	// remove this file from old parents' kids lists
	for _, oldParent := range oldParents {
		if !slices.Contains(metadata.Parents, oldParent) {
			// this parent was removed, update its kids list
			parentMetadata, err := MetaDataGet(oldParent)
			if err != nil || parentMetadata == nil {
				logging.LogWarning("failed to get metadata for former parent %s: %v", oldParent, err)
				continue
			}

			// remove current file from parent's kids list
			if idx := slices.Index(parentMetadata.Kids, metadata.Path); idx != -1 {
				parentMetadata.Kids = slices.Delete(parentMetadata.Kids, idx, idx+1)

				if err := metaDataSaveRaw(parentMetadata); err != nil {
					logging.LogWarning("failed to update kids list for %s: %v", oldParent, err)
				} else {
					logging.LogInfo("removed %s from kids list of %s", metadata.Path, oldParent)
				}
			}
		}
	}

	// add this file to new parents' kids lists
	for _, newParent := range metadata.Parents {
		if !slices.Contains(oldParents, newParent) {
			// this parent was added, update its kids list
			parentMetadata, err := MetaDataGet(newParent)
			if err != nil || parentMetadata == nil {
				logging.LogWarning("failed to get metadata for new parent %s: %v", newParent, err)
				continue
			}

			// add current file to parent's kids list if not already there
			if !slices.Contains(parentMetadata.Kids, metadata.Path) {
				parentMetadata.Kids = append(parentMetadata.Kids, metadata.Path)

				if err := metaDataSaveRaw(parentMetadata); err != nil {
					logging.LogWarning("failed to update kids list for %s: %v", newParent, err)
				} else {
					logging.LogInfo("added %s to kids list of %s", metadata.Path, newParent)
				}
			}
		}
	}
}
