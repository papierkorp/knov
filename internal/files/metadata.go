// Package files handles file operations and metadata
package files

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"knov/internal/configmanager"
	"knov/internal/logging"
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
	if dir != "." && dir != "/" && dir != "" {
		folders := strings.Split(strings.Trim(dir, "/"), "/")
		var validFolders []string
		for _, folder := range folders {
			if folder != "" {
				validFolders = append(validFolders, folder)
			}
		}
		currentMetadata.Folders = validFolders
	}

	if newMetadata != nil {
		if newMetadata.Collection != "" {
			currentMetadata.Collection = newMetadata.Collection
		}
		if len(newMetadata.Tags) > 0 {
			currentMetadata.Tags = newMetadata.Tags
		}
		if len(newMetadata.Boards) > 0 {
			currentMetadata.Boards = newMetadata.Boards
		}
		if len(newMetadata.Ancestor) > 0 {
			currentMetadata.Ancestor = newMetadata.Ancestor
		}
		if len(newMetadata.Parents) > 0 {
			currentMetadata.Parents = newMetadata.Parents
		}
		if len(newMetadata.Kids) > 0 {
			currentMetadata.Kids = newMetadata.Kids
		}
		if len(newMetadata.UsedLinks) > 0 {
			currentMetadata.UsedLinks = newMetadata.UsedLinks
		}
		if len(newMetadata.LinksToHere) > 0 {
			currentMetadata.LinksToHere = newMetadata.LinksToHere
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
	if currentMetadata.Collection == "" {
		currentMetadata.Collection = "default"
	}
	if currentMetadata.Tags == nil {
		currentMetadata.Tags = []string{}
	}
	if currentMetadata.Boards == nil {
		currentMetadata.Boards = []string{"default"}
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

	return currentMetadata
}

// MetaDataSave saves metadata using the configured storage method
func MetaDataSave(m *Metadata) error {
	finalMetadata := metaDataUpdate(m.Path, m)
	if finalMetadata == nil {
		return nil
	}

	storageMethod := configmanager.GetMetadataStorageMethod()

	switch storageMethod {
	case "json":
		return metaDataSaveAsJSON(finalMetadata)
	default:
		logging.LogWarning("unsupported metadata storage method: %s, using json", storageMethod)
		return metaDataSaveAsJSON(finalMetadata)
	}
}

func metaDataSaveAsJSON(m *Metadata) error {
	metadataFile := "config/.metadata/metadata.json"
	metadataDir := filepath.Dir(metadataFile)

	if err := os.MkdirAll(metadataDir, 0755); err != nil {
		logging.LogError("failed to create metadata directory: %v", err)
		return err
	}

	// TODO: to optimize switch to multiple json files e.g. alphabetically
	var allMetadata map[string]*Metadata
	if data, err := os.ReadFile(metadataFile); err == nil {
		json.Unmarshal(data, &allMetadata)
	}
	if allMetadata == nil {
		allMetadata = make(map[string]*Metadata)
	}

	allMetadata[m.Path] = m

	jsonData, err := json.MarshalIndent(allMetadata, "", "  ")
	if err != nil {
		logging.LogError("failed to marshal metadata: %v", err)
		return err
	}

	if err := os.WriteFile(metadataFile, jsonData, 0644); err != nil {
		logging.LogError("failed to write metadata file: %v", err)
		return err
	}

	logging.LogDebug("metadata saved to %s", metadataFile)
	return nil
}

func metaDataSaveAsMarkdown(m *Metadata) error {
	return nil
}

func metaDataSaveAsSQLITE(m *Metadata) error {
	return nil
}

func metaDataSaveAsPostgres(m *Metadata) error {
	return nil
}

// MetaDataGet retrieves metadata using the configured storage method
func MetaDataGet(filepath string) (*Metadata, error) {
	storageMethod := configmanager.GetMetadataStorageMethod()

	switch storageMethod {
	case "json":
		return metaDataGetJSON(filepath)
	default:
		logging.LogWarning("unsupported metadata storage method: %s, using json", storageMethod)
		return metaDataGetJSON(filepath)
	}
}

func metaDataGetJSON(filepath string) (*Metadata, error) {
	metadataFile := "config/.metadata/metadata.json"

	data, err := os.ReadFile(metadataFile)
	if err != nil {
		if os.IsNotExist(err) {
			logging.LogDebug("metadata file does not exist: %s", metadataFile)
			return nil, nil
		}
		logging.LogError("failed to read metadata file: %v", err)
		return nil, err
	}

	var allMetadata map[string]*Metadata
	if err := json.Unmarshal(data, &allMetadata); err != nil {
		logging.LogError("failed to unmarshal metadata: %v", err)
		return nil, err
	}

	metadata, exists := allMetadata[filepath]
	if !exists {
		logging.LogDebug("no metadata found for file: %s", filepath)
		return nil, nil
	}

	logging.LogDebug("metadata retrieved: %s", metadata.Name)
	return metadata, nil
}

func metaDataGetMarkdown(filepath string) (*Metadata, error) {
	return nil, nil
}

func metaDataGetSQLITE(filepath string) (*Metadata, error) {
	return nil, nil
}

func metaDataGetPostgres(filepath string) (*Metadata, error) {
	return nil, nil
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
