// Package files - Link management for metadata
package files

import (
	"os"
	"slices"

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
