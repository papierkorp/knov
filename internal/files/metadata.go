// Package files handles file operations and metadata
package files

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	FileTypeTodo    Filetype = "todo"
	FileTypeNote    Filetype = "note"
	FileTypeJournal Filetype = "journal"

	StatusDraft     Status = "draft"
	StatusPublished Status = "published"
	StatusArchived  Status = "archived"

	PriorityLow    Priority = "low"
	PriorityMedium Priority = "medium"
	PriorityHigh   Priority = "high"
)

// Metadata represents file metadata
type Metadata struct {
	Name        string    `json:"name"`        // manual filename
	Path        string    `json:"path"`        // auto
	CreatedAt   time.Time `json:"createdAt"`   // auto
	LastEdited  time.Time `json:"lastEdited"`  // auto
	TargetDate  time.Time `json:"targetDate"`  // auto
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
	Status      Status    `json:"status"`      // manual
	Priority    Priority  `json:"priority"`    // manual
	Size        int64     `json:"size"`        // auto
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
		if len(newMetadata.Tags) > 0 {
			currentMetadata.Tags = newMetadata.Tags
		}
		if len(newMetadata.Parents) > 0 {
			normalized := make([]string, 0, len(newMetadata.Parents))
			for _, parent := range newMetadata.Parents {
				normalized = append(normalized, utils.CleanLink(parent))
			}
			currentMetadata.Parents = normalized
		}
		if newMetadata.FileType != "" {
			currentMetadata.FileType = newMetadata.FileType
		}
		if newMetadata.Status != "" {
			currentMetadata.Status = newMetadata.Status
		}
		if newMetadata.Priority != "" {
			currentMetadata.Priority = newMetadata.Priority
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
		currentMetadata.FileType = FileTypeJournal
	}
	if currentMetadata.Status == "" {
		currentMetadata.Status = StatusPublished
	}
	if currentMetadata.Priority == "" {
		currentMetadata.Priority = PriorityMedium
	}

	updateAncestors(currentMetadata)
	updateUsedLinks(currentMetadata)
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
func GetAllTags() (map[string]int, error) {
	allFiles, err := GetAllFiles()
	if err != nil {
		return nil, err
	}

	tagCount := make(map[string]int)
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
func GetAllCollections() (map[string]int, error) {
	allFiles, err := GetAllFiles()
	if err != nil {
		return nil, err
	}

	collectionCount := make(map[string]int)
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
func GetAllFolders() (map[string]int, error) {
	allFiles, err := GetAllFiles()
	if err != nil {
		return nil, err
	}

	folderCount := make(map[string]int)
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
