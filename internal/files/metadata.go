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
	"knov/internal/logging"
	"knov/internal/metadataStorage"
	"knov/internal/pathutils"
	"knov/internal/utils"
)

type EditorType string

const (
	EditorTypeMarkdown EditorType = "markdown-editor"
	EditorTypeTextarea EditorType = "textarea-editor"
	EditorTypeFilter   EditorType = "filter-editor"
	EditorTypeList     EditorType = "list-editor"
	EditorTypeTodo     EditorType = "todo-editor"
	EditorTypeIndex    EditorType = "index-editor"
)

// typed count maps for metadata aggregations
type TagCount map[string]int
type CollectionCount map[string]int
type FolderCount map[string]int
type EditorTypeCount map[string]int

// AllEditorTypes returns all available editor types
func AllEditorTypes() []EditorType {
	return []EditorType{
		EditorTypeMarkdown,
		EditorTypeTextarea,
		EditorTypeFilter,
		EditorTypeList,
		EditorTypeTodo,
		EditorTypeIndex,
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
		return EditorTypeTextarea
	default:
		return ""
	}
}

// Metadata represents file metadata
type Metadata struct {
	Path        string      `json:"path"`                 // auto
	Title       string      `json:"title"`                // auto
	CreatedAt   time.Time   `json:"createdAt"`            // auto
	LastEdited  time.Time   `json:"lastEdited"`           // auto
	Collection  string      `json:"collection"`           // auto / manual possible
	Folders     []string    `json:"folders"`              // auto
	Tags        []string    `json:"tags"`                 // manual
	Ancestor    []string    `json:"ancestor"`             // auto
	Parents     []string    `json:"parents"`              // manual
	Kids        []string    `json:"kids"`                 // auto
	UsedLinks   []string    `json:"usedLinks"`            // auto
	LinksToHere []string    `json:"linksToHere"`          // auto
	Editor      EditorType  `json:"editor"`               // manual - with add new
	Size        int64       `json:"size"`                 // auto
	References  []Reference `json:"references,omitempty"` // manual
}

// Reference represents an external resource linked to a file
type Reference struct {
	URL         string `json:"url"`
	Description string `json:"description"` // why this link was added
}

func metaDataUpdate(filePath string, newMetadata *Metadata) *Metadata {
	currentMetadata, _ := MetaDataGet(filePath)

	// determine if this is a media file or docs file based on the original path
	isMediaFile := pathutils.IsMedia(filePath)

	var fullPath string
	var metadataPath string

	if isMediaFile {
		// for media files, keep the media/ prefix in metadata but get actual file path
		normalizedPath := pathutils.ToRelative(filePath)
		fullPath = pathutils.ToMediaPath(normalizedPath)
		metadataPath = filePath // keep original path with media/ prefix
	} else {
		// for docs files, keep the docs/ prefix in metadata but get actual file path
		normalizedPath := pathutils.ToRelative(filePath)
		fullPath = pathutils.ToDocsPath(normalizedPath)
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
	normalizedPath := pathutils.ToRelative(filePath)

	folderPath := filepath.ToSlash(filepath.Dir(normalizedPath))
	if folderPath != "." && folderPath != "" {
		parts := strings.Split(folderPath, "/")
		currentMetadata.Folders = parts
		if newMetadata.Collection == "" {
			currentMetadata.Collection = parts[0]
		} else {
			currentMetadata.Collection = newMetadata.Collection
		}
	} else {
		currentMetadata.Folders = []string{}
		if newMetadata.Collection != "" {
			currentMetadata.Collection = newMetadata.Collection
		}
	}

	// handle optional fields from newMetadata - only update if provided
	if len(newMetadata.Tags) > 0 {
		currentMetadata.Tags = newMetadata.Tags
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
	if newMetadata.Editor != "" {
		currentMetadata.Editor = newMetadata.Editor
	}

	if currentMetadata.Editor == "" {
		if et := EditorFromExtension(metadataPath); et != "" {
			currentMetadata.Editor = et
		} else {
			currentMetadata.Editor = EditorTypeMarkdown
		}
	}

	if newMetadata.Collection != "" {
		currentMetadata.Collection = newMetadata.Collection
	}
	if !newMetadata.CreatedAt.IsZero() {
		currentMetadata.CreatedAt = newMetadata.CreatedAt
	}
	if newMetadata.References != nil {
		currentMetadata.References = newMetadata.References
	}

	// make sure required fields are initialized
	if currentMetadata.Tags == nil {
		currentMetadata.Tags = []string{}
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

	updateAncestors(currentMetadata, nil)
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

// MetaDataSaveRaw saves metadata without processing
func MetaDataSaveRaw(m *Metadata) error {
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
	if rebuildMetaGetCount != nil {
		*rebuildMetaGetCount++
	}
	// normalize path for metadata lookup - add docs/ prefix if not present and not media
	normalizedPath := pathutils.ToWithPrefix(filepath)

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

	allFiles, err := GetAllPhysicalFiles()
	if err != nil {
		return err
	}

	for _, file := range allFiles {
		// normalize path to ensure correct prefix for metadata storage
		normalizedPath := pathutils.ToWithPrefix(file.Path)

		// check if metadata already exists
		metadata, err := MetaDataGet(normalizedPath)
		if err != nil {
			logging.LogWarning("error checking metadata for %s: %v", normalizedPath, err)
			continue
		}

		// skip if metadata exists
		if metadata != nil {
			continue
		}

		// create new metadata
		newMetadata := &Metadata{Path: normalizedPath}
		if err := MetaDataSave(newMetadata); err != nil {
			logging.LogWarning("failed to initialize metadata for %s: %v", normalizedPath, err)
		} else {
			logging.LogInfo("initialized metadata for %s", normalizedPath)
		}
	}

	logging.LogInfo("metadata initialization completed")
	return nil
}

// MetaDataDelete removes metadata for a file path
func MetaDataDelete(filepath string) error {
	return metadataStorage.Delete(pathutils.ToWithPrefix(filepath))
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
