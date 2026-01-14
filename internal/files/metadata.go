// Package files handles file operations and metadata
package files

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"knov/internal/configmanager"
	"knov/internal/contentStorage"
	"knov/internal/logging"
	"knov/internal/metadataStorage"
	"knov/internal/utils"
)

// getFilePathForMetadata returns the correct file system path for a metadata path
func getFilePathForMetadata(metadataPath string) string {
	if strings.HasPrefix(metadataPath, "media/") {
		normalizedPath := contentStorage.ToRelativePath(metadataPath)
		return contentStorage.ToMediaPath(normalizedPath)
	}

	// default to docs path (handles both docs/ prefixed and non-prefixed paths)
	normalizedPath := contentStorage.ToRelativePath(metadataPath)
	return contentStorage.ToDocsPath(normalizedPath)
}

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
	FileTypeImage      Filetype = "image"
	FileTypeVideo      Filetype = "video"
	FileTypePDF        Filetype = "pdf"
	FileTypeText       Filetype = "text"

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
		FileTypeImage,
		FileTypeVideo,
		FileTypePDF,
		FileTypeText,
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
	Folder      string    `json:"folder"`      // auto
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

	// determine if this is a media file or docs file based on the original path
	isMediaFile := strings.HasPrefix(filePath, "media/")

	var fullPath string
	var metadataPath string

	if isMediaFile {
		// for media files, keep the media/ prefix in metadata but get actual file path
		normalizedPath := contentStorage.ToRelativePath(filePath)
		fullPath = contentStorage.ToMediaPath(normalizedPath)
		metadataPath = filePath // keep original path with media/ prefix
	} else {
		// for docs files, keep the docs/ prefix in metadata but get actual file path
		normalizedPath := contentStorage.ToRelativePath(filePath)
		fullPath = contentStorage.ToDocsPath(normalizedPath)
		metadataPath = filePath // keep original path with docs/ prefix
	}

	// get file size
	fileInfo, err := os.Stat(fullPath)
	if err != nil {
		logging.LogWarning("failed to get file size for %s: %v", fullPath, err)
	} else {
		newMetadata.Size = fileInfo.Size()
	}

	if currentMetadata == nil {
		// initialize new metadata
		currentMetadata = &Metadata{
			Path:      metadataPath,
			CreatedAt: time.Now(),
		}
	}

	// update path and time fields
	currentMetadata.Path = metadataPath
	currentMetadata.LastEdited = time.Now()
	currentMetadata.Size = newMetadata.Size

	// update collection and folder based on folder structure (use path without docs/media prefix)
	normalizedPath := contentStorage.ToRelativePath(filePath)

	if newMetadata.Collection == "" {
		folderPath := filepath.Dir(normalizedPath)
		if folderPath != "." && folderPath != "" {
			currentMetadata.Collection = folderPath
		}
	} else {
		currentMetadata.Collection = newMetadata.Collection
	}

	// update folder field
	folderPath := filepath.Dir(normalizedPath)
	if folderPath == "." {
		currentMetadata.Folder = ""
	} else {
		currentMetadata.Folder = folderPath
	}

	// handle optional fields from newMetadata - only update if provided
	if len(newMetadata.Tags) > 0 {
		currentMetadata.Tags = newMetadata.Tags
	}
	if len(newMetadata.Boards) > 0 {
		currentMetadata.Boards = newMetadata.Boards
	}
	if len(newMetadata.Parents) > 0 {
		// store old parents for cleanup
		var oldParents []string
		if currentMetadata.Parents != nil {
			oldParents = make([]string, len(currentMetadata.Parents))
			copy(oldParents, currentMetadata.Parents)
		}

		// normalize parent links and update metadata
		var normalized []string
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
	// handle target date - allow both setting and clearing (zero time)
	currentMetadata.TargetDate = newMetadata.TargetDate
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

	// update name with filename if not set
	if currentMetadata.Name == "" {
		filename := filepath.Base(currentMetadata.Path)
		currentMetadata.Name = strings.TrimSuffix(filename, filepath.Ext(filename))
	}

	// make sure required fields are initialized
	if currentMetadata.Tags == nil {
		currentMetadata.Tags = []string{}
	}
	if currentMetadata.Boards == nil {
		currentMetadata.Boards = []string{}
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
	if currentMetadata.Ancestor == nil {
		currentMetadata.Ancestor = []string{}
	}
	if currentMetadata.Folders == nil {
		currentMetadata.Folders = []string{}
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

	if err := metadataStorage.Set(finalMetadata.Path, data); err != nil {
		logging.LogError("failed to save metadata for %s: %v", finalMetadata.Path, err)
		return err
	}

	logging.LogDebug("metadata saved for: %s", finalMetadata.Path)
	return nil
}

// metaDataSaveRaw saves metadata without processing
func metaDataSaveRaw(m *Metadata) error {
	data, err := json.Marshal(m)
	if err != nil {
		logging.LogError("failed to marshal metadata: %v", err)
		return err
	}

	if err := metadataStorage.Set(m.Path, data); err != nil {
		logging.LogError("failed to save metadata for %s: %v", m.Path, err)
		return err
	}

	logging.LogDebug("raw metadata saved for: %s", m.Path)
	return nil
}

// MetaDataGet retrieves metadata for a file path
func MetaDataGet(filepath string) (*Metadata, error) {
	// if the path already has docs/ or media/ prefix, use it as-is for metadata lookup
	var normalizedPath string
	if strings.HasPrefix(filepath, "docs/") || strings.HasPrefix(filepath, "media/") {
		normalizedPath = filepath
	} else {
		normalizedPath = contentStorage.ToRelativePath(filepath)
	}

	logging.LogDebug("MetaDataGet: filepath='%s' -> normalizedPath='%s'", filepath, normalizedPath)

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
	logging.LogInfo("initializing metadata for all files")

	allFiles, err := GetAllFiles()
	if err != nil {
		return err
	}

	for _, file := range allFiles {
		// check if metadata already exists
		metadata, err := MetaDataGet(file.Path)
		if err != nil {
			logging.LogWarning("error checking metadata for %s: %v", file.Path, err)
			continue
		}

		// skip if metadata exists
		if metadata != nil {
			continue
		}

		// create new metadata
		newMetadata := &Metadata{Path: file.Path}
		if err := MetaDataSave(newMetadata); err != nil {
			logging.LogWarning("failed to initialize metadata for %s: %v", file.Path, err)
		} else {
			logging.LogInfo("initialized metadata for %s", file.Path)
		}
	}

	logging.LogInfo("metadata initialization completed")
	return nil
}

// MetaDataDelete removes metadata for a file path
func MetaDataDelete(filepath string) error {
	normalizedPath := contentStorage.ToRelativePath(filepath)
	return metadataStorage.Delete(normalizedPath)
}

// MetaDataExportAll returns all metadata entries
func MetaDataExportAll() ([]*Metadata, error) {
	allFiles, err := GetAllFiles()
	if err != nil {
		return nil, err
	}

	var allMetadata []*Metadata
	for _, file := range allFiles {
		metadata, err := MetaDataGet(file.Path)
		if err != nil {
			logging.LogWarning("failed to get metadata for %s: %v", file.Path, err)
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
		logging.LogWarning("empty mime type provided for validation")
		return false
	}

	// get current media settings
	mediaSettings := configmanager.GetUserSettings().MediaSettings
	allowedTypes := mediaSettings.AllowedMimeTypes

	// if no allowed types configured, deny by default for security
	if len(allowedTypes) == 0 {
		logging.LogWarning("no allowed mime types configured, denying upload")
		return false
	}

	// normalize MIME type using utils function
	mimeType = utils.Normalize(mimeType)
	logging.LogDebug("validating mime type: %s against allowed types: %v", mimeType, allowedTypes)

	// check exact matches first
	for _, allowedType := range allowedTypes {
		allowedType = utils.Normalize(allowedType)
		if allowedType == mimeType {
			logging.LogDebug("mime type %s allowed (exact match)", mimeType)
			return true
		}

		// handle wildcard patterns like "image/*"
		if strings.HasSuffix(allowedType, "/*") {
			category := strings.TrimSuffix(allowedType, "/*")
			if strings.HasPrefix(mimeType, category+"/") {
				logging.LogDebug("mime type %s allowed (wildcard match: %s)", mimeType, allowedType)
				return true
			}
		}
	}

	logging.LogWarning("mime type %s not allowed, blocked upload", mimeType)
	return false
}
