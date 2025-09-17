// Package files - Link management for metadata
package files

import (
	"os"
	"regexp"
	"slices"
	"strings"

	"knov/internal/configmanager"
	"knov/internal/logging"
	"knov/internal/utils"
)

func metaDataLinksAdd(filePath string) error {
	metadata, err := MetaDataGet(filePath)
	if err != nil {
		return err
	}
	if metadata == nil {
		return nil
	}

	updateAncestors(metadata)
	updateUsedLinks(metadata)

	return MetaDataSave(metadata)
}

// MetaDataLinksRebuild ..
func MetaDataLinksRebuild() error {
	logging.LogInfo("rebuilding all metadata links")
	files, err := GetAllFiles()
	if err != nil {
		return err
	}

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
		updateUsedLinks(metadata)

		updateKidsAndLinksToHere(metadata)

		if err := MetaDataSave(metadata); err != nil {
			logging.LogWarning("failed to save metadata for %s: %v", metadata.Path, err)
		}
	}

	logging.LogInfo("metadata links rebuild completed")
	return nil
}

func updateAncestors(metadata *Metadata) {
	if len(metadata.Parents) == 0 {
		return
	}

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
	content, err := os.ReadFile(fullPath)
	if err != nil {
		logging.LogWarning("failed to read file %s: %v", fullPath, err)
		return
	}

	linkRegexes := configmanager.GetMetadataLinkRegex()
	var usedLinks []string
	linkSet := make(map[string]bool)

	for _, regexPattern := range linkRegexes {
		re, err := regexp.Compile(regexPattern)
		if err != nil {
			logging.LogWarning("invalid regex pattern %s: %v", regexPattern, err)
			continue
		}

		matches := re.FindAllStringSubmatch(string(content), -1)
		for _, match := range matches {
			if len(match) > 1 {
				link := match[1]
				if idx := strings.Index(link, "|"); idx != -1 {
					link = link[:idx]
				}
				if idx := strings.Index(link, "]]"); idx != -1 {
					link = link[:idx]
				}
				if idx := strings.Index(link, "}}"); idx != -1 {
					link = link[:idx]
				}
				if strings.HasPrefix(link, "../") {
					link = strings.TrimPrefix(link, "../")
				}
				link = strings.TrimSpace(link)
				if strings.HasSuffix(link, ".md") && !strings.Contains(link, "\n") && len(link) < 100 {
					if link != metadata.Path && !linkSet[link] {
						linkSet[link] = true
						usedLinks = append(usedLinks, link)
					}
				}
			}
		}
	}

	metadata.UsedLinks = usedLinks
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
