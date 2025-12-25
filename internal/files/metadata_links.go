// Package files - Link management for metadata
package files

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"knov/internal/logging"
	"knov/internal/utils"
)

// MetaDataLinksRebuild ..
func MetaDataLinksRebuild() error {
	logging.LogInfo("rebuilding all metadata links")
	files, err := GetAllFiles()
	if err != nil {
		return err
	}

	// first pass: clear old data and update ancestors and usedlinks
	for _, file := range files {
		metadata, err := MetaDataGet(file.Path)
		if err != nil {
			logging.LogWarning("failed to load metadata for %s: %v", file.Path, err)
			continue
		}
		if metadata == nil {
			continue
		}

		metadata.Ancestor = []string{}
		metadata.Kids = []string{}
		metadata.UsedLinks = []string{}
		metadata.LinksToHere = []string{}

		updateAncestors(metadata)

		// extract used links without updating linkstohere yet
		fullPath := utils.ToFullPath(metadata.Path)
		contentData, err := os.ReadFile(fullPath)
		if err == nil {
			handler := parserRegistry.GetHandler(fullPath)
			if handler != nil {
				links := handler.ExtractLinks(contentData)
				for _, link := range links {
					cleanLink := utils.CleanLink(link)
					if cleanLink != "" && cleanLink != metadata.Path && !slices.Contains(metadata.UsedLinks, cleanLink) {
						metadata.UsedLinks = append(metadata.UsedLinks, cleanLink)
					}
				}
			}
		}

		if err := MetaDataSave(metadata); err != nil {
			logging.LogWarning("failed to save metadata for %s: %v", metadata.Path, err)
		}
	}

	// second pass: update kids and linkstohere for all files
	for _, file := range files {
		metadata, err := MetaDataGet(file.Path)
		if err != nil || metadata == nil {
			continue
		}

		updateKidsAndLinksToHere(metadata)

		if err := MetaDataSave(metadata); err != nil {
			logging.LogWarning("failed to save metadata for %s: %v", metadata.Path, err)
		}
	}

	logging.LogInfo("metadata links rebuild completed")
	return nil
}

func updateAncestors(metadata *Metadata) {
	visited := make(map[string]bool)
	var ancestors []string

	for _, parent := range metadata.Parents {
		if visited[parent] {
			continue
		}
		visited[parent] = true

		ancestor := findTopAncestor(parent, make(map[string]bool))
		if ancestor != "" && ancestor != metadata.Path {
			ancestors = append(ancestors, ancestor)
		}
	}

	metadata.Ancestor = ancestors
}

func findTopAncestor(filePath string, visited map[string]bool) string {
	if visited[filePath] {
		logging.LogWarning("cycle detected in parent chain for %s", filePath)
		return ""
	}
	visited[filePath] = true

	metadata, err := MetaDataGet(filePath)
	if err != nil || metadata == nil {
		logging.LogWarning("cannot find metadata for parent %s", filePath)
		return filePath
	}

	if len(metadata.Parents) == 0 {
		return filePath
	}

	for _, parent := range metadata.Parents {
		return findTopAncestor(parent, visited)
	}

	return filePath
}

func updateUsedLinks(metadata *Metadata) {
	fullPath := utils.ToFullPath(metadata.Path)

	logging.LogInfo("processing file for links: %s", fullPath)

	contentData, err := os.ReadFile(fullPath)
	if err != nil {
		logging.LogWarning("failed to read file %s: %v", fullPath, err)
		return
	}

	handler := parserRegistry.GetHandler(fullPath)
	if handler == nil {
		logging.LogWarning("no handler found for file %s", fullPath)
		return
	}

	links := handler.ExtractLinks(contentData)
	logging.LogInfo("extracted %d links from %s", len(links), metadata.Path)

	// store old links to detect removals
	oldUsedLinks := make([]string, len(metadata.UsedLinks))
	copy(oldUsedLinks, metadata.UsedLinks)

	metadata.UsedLinks = []string{}

	for _, link := range links {
		cleanLink := utils.CleanLink(link)

		if cleanLink == "" || cleanLink == metadata.Path {
			continue
		}

		if !slices.Contains(metadata.UsedLinks, cleanLink) {
			metadata.UsedLinks = append(metadata.UsedLinks, cleanLink)
		}
	}

	logging.LogDebug("cleaned used links for %s: %v", metadata.Path, metadata.UsedLinks)

	// update linkstohere in the found files
	updateLinksToHere(metadata, oldUsedLinks)
}

func updateLinksToHere(metadata *Metadata, oldUsedLinks []string) {
	logging.LogInfo("updating linkstohere for linked files from %s", metadata.Path)

	// add current file to linkstohere for new links
	for _, usedLink := range metadata.UsedLinks {
		linkedMetadata, err := MetaDataGet(usedLink)
		if err != nil || linkedMetadata == nil {
			logging.LogDebug("skipping linkstohere update for %s: metadata not found", usedLink)
			continue
		}

		// add current file to linkstohere if not already present
		if !slices.Contains(linkedMetadata.LinksToHere, metadata.Path) {
			linkedMetadata.LinksToHere = append(linkedMetadata.LinksToHere, metadata.Path)

			if err := metaDataSaveRaw(linkedMetadata); err != nil {
				logging.LogWarning("failed to save linkstohere for %s: %v", usedLink, err)
			} else {
				logging.LogInfo("added %s to linkstohere of %s", metadata.Path, usedLink)
			}
		}
	}

	// remove current file from linkstohere for removed links
	for _, oldLink := range oldUsedLinks {
		if !slices.Contains(metadata.UsedLinks, oldLink) {
			linkedMetadata, err := MetaDataGet(oldLink)
			if err != nil || linkedMetadata == nil {
				continue
			}

			// remove current file from linkstohere
			if idx := slices.Index(linkedMetadata.LinksToHere, metadata.Path); idx != -1 {
				linkedMetadata.LinksToHere = slices.Delete(linkedMetadata.LinksToHere, idx, idx+1)

				if err := metaDataSaveRaw(linkedMetadata); err != nil {
					logging.LogWarning("failed to save linkstohere for %s: %v", oldLink, err)
				} else {
					logging.LogInfo("removed %s from linkstohere of %s", metadata.Path, oldLink)
				}
			}
		}
	}
}

func updateKidsAndLinksToHere(metadata *Metadata) {
	files, err := GetAllFiles()
	if err != nil {
		logging.LogWarning("failed to get all files for updating kids and links: %v", err)
		return
	}

	var kids []string
	var linksToHere []string

	for _, file := range files {
		if file.Path == metadata.Path {
			continue
		}

		otherMetadata, err := MetaDataGet(file.Path)
		if err != nil || otherMetadata == nil {
			continue
		}

		// check if this file is a parent (making the other file a kid)
		if slices.Contains(otherMetadata.Parents, metadata.Path) {
			kids = append(kids, file.Path)
		}

		// check if this file is in usedLinks (making it a link to here)
		if slices.Contains(otherMetadata.UsedLinks, metadata.Path) {
			linksToHere = append(linksToHere, file.Path)
		}
	}

	metadata.Kids = kids
	metadata.LinksToHere = linksToHere
}

// UpdateLinksForMovedFile updates all files that link to a moved file with the new path
func UpdateLinksForMovedFile(oldPath, newPath string) error {
	logging.LogInfo("updating links for moved file: %s -> %s", oldPath, newPath)

	// get metadata for the old path to find files that link to it
	oldMetadata, err := MetaDataGet(oldPath)
	if err != nil {
		logging.LogWarning("could not get metadata for moved file %s: %v", oldPath, err)
		return err
	}

	if oldMetadata == nil || len(oldMetadata.LinksToHere) == 0 {
		logging.LogInfo("no files link to %s, skipping link updates", oldPath)
		// still move metadata even if no files link to it
		if err := moveFileMetadata(oldPath, newPath); err != nil {
			logging.LogError("failed to move metadata for %s: %v", oldPath, err)
			return err
		}
		return nil
	}

	logging.LogInfo("found %d files linking to %s from metadata", len(oldMetadata.LinksToHere), oldPath)

	// update content in linking files
	updatedFiles := 0
	for _, linkingFilePath := range oldMetadata.LinksToHere {
		if err := updateLinksInFile(linkingFilePath, oldPath, newPath); err != nil {
			logging.LogError("failed to update links in file %s: %v", linkingFilePath, err)
			continue
		}
		updatedFiles++
	}

	// move metadata from old path to new path
	if err := moveFileMetadata(oldPath, newPath); err != nil {
		logging.LogError("failed to move metadata for %s: %v", oldPath, err)
		// don't return error here, file content updates are more important
	}

	logging.LogInfo("successfully updated links in %d files for moved file %s -> %s", updatedFiles, oldPath, newPath)
	return nil
}

// updateLinksInFile updates links within a single file
func updateLinksInFile(filePath, oldPath, newPath string) error {
	fullPath := utils.ToFullPath(filePath)

	// read file content
	contentData, err := os.ReadFile(fullPath)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	content := string(contentData)
	originalContent := content

	// get file handler for link format detection
	handler := parserRegistry.GetHandler(fullPath)
	if handler == nil {
		logging.LogWarning("no handler found for file %s, skipping link update", filePath)
		return nil
	}

	// update different link formats
	updated := false

	// markdown-style links: [text](oldPath) -> [text](newPath)
	oldMarkdownLink := fmt.Sprintf("](%s)", oldPath)
	newMarkdownLink := fmt.Sprintf("](%s)", newPath)
	if strings.Contains(content, oldMarkdownLink) {
		content = strings.ReplaceAll(content, oldMarkdownLink, newMarkdownLink)
		updated = true
		logging.LogDebug("updated markdown links in %s", filePath)
	}

	// wiki-style links: [[oldPath]] -> [[newPath]]
	oldWikiLink := fmt.Sprintf("[[%s]]", oldPath)
	newWikiLink := fmt.Sprintf("[[%s]]", newPath)
	if strings.Contains(content, oldWikiLink) {
		content = strings.ReplaceAll(content, oldWikiLink, newWikiLink)
		updated = true
		logging.LogDebug("updated wiki links in %s", filePath)
	}

	// dokuwiki-style links: [[oldPath|text]] -> [[newPath|text]]
	// this is more complex as we need to preserve the text part
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.Contains(line, oldPath) {
			// look for dokuwiki link pattern
			start := strings.Index(line, "[["+oldPath)
			if start != -1 {
				end := strings.Index(line[start:], "]]")
				if end != -1 {
					end += start
					linkPart := line[start+2 : end] // remove [[ and ]]

					if strings.HasPrefix(linkPart, oldPath) {
						if len(linkPart) == len(oldPath) {
							// simple link [[oldPath]]
							lines[i] = strings.Replace(line, "[["+oldPath+"]]", "[["+newPath+"]]", 1)
							updated = true
						} else if linkPart[len(oldPath)] == '|' {
							// link with text [[oldPath|text]]
							textPart := linkPart[len(oldPath):]
							lines[i] = strings.Replace(line, "[["+oldPath+textPart+"]]", "[["+newPath+textPart+"]]", 1)
							updated = true
						}
					}
				}
			}
		}
	}

	if updated {
		content = strings.Join(lines, "\n")
	}

	// write back if changed
	if updated && content != originalContent {
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write updated content to %s: %w", filePath, err)
		}

		logging.LogInfo("updated links in file %s: %s -> %s", filePath, oldPath, newPath)

		// trigger metadata update for the modified file to update its links
		metadata := &Metadata{Path: filePath}
		if err := MetaDataSave(metadata); err != nil {
			logging.LogWarning("failed to update metadata for modified file %s: %v", filePath, err)
		}
	}

	return nil
}

// moveFileMetadata moves metadata from old path to new path
func moveFileMetadata(oldPath, newPath string) error {
	// get existing metadata
	metadata, err := MetaDataGet(oldPath)
	if err != nil {
		logging.LogDebug("no metadata found for %s, creating new metadata for %s", oldPath, newPath)
		// create new metadata for new path
		newMetadata := &Metadata{Path: newPath}
		return MetaDataSave(newMetadata)
	}

	if metadata == nil {
		// create new metadata for new path
		newMetadata := &Metadata{Path: newPath}
		return MetaDataSave(newMetadata)
	}

	// update path in metadata
	metadata.Path = newPath

	// save metadata with new path
	if err := MetaDataSave(metadata); err != nil {
		return fmt.Errorf("failed to save metadata for new path %s: %w", newPath, err)
	}

	// delete old metadata
	if err := MetaDataDelete(oldPath); err != nil {
		logging.LogWarning("failed to delete old metadata for %s: %v", oldPath, err)
		// don't fail the operation for this
	}

	logging.LogInfo("moved metadata: %s -> %s", oldPath, newPath)
	return nil
}
