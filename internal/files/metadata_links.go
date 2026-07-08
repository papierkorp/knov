// Package files - Link management for metadata
package files

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"knov/internal/chat"
	"knov/internal/contentStorage"
	"knov/internal/logging"
	"knov/internal/parser"
	"knov/internal/pathutils"
	"knov/internal/utils"
)

var rebuildMetaGetCount *int

// OnMetadataRebuild is called after every full or single-file metadata rebuild.
// Register filter.RegenerateAllIndexes here at startup to keep filter indexes in sync.
var OnMetadataRebuild func()

// MetaDataLinksRebuild rebuilds all link metadata from scratch.
func MetaDataLinksRebuild() error {
	log := logging.LogBuilder("rebuild-metadata")
	log.Printf("=== metadata links rebuild started ===")

	paths, err := contentStorage.ListFiles()
	if err != nil {
		return err
	}
	log.Printf("docs files to process: %d", len(paths))

	// load media files once — used in zeroth pass and final media pass
	allMediaFiles, err := GetAllMediaFiles()
	if err != nil {
		logging.LogWarning("failed to get media files for link rebuild: %v", err)
		log.Printf("warning: failed to get media files: %v", err)
		allMediaFiles = nil
	}
	log.Printf("media files found: %d", len(allMediaFiles))

	// zeroth pass: clear LinksToHere on all media files so stale references don't persist
	for _, file := range allMediaFiles {
		normalizedPath := pathutils.ToWithPrefix(file.Path)
		metadata, err := MetaDataGet(normalizedPath)
		if err != nil || metadata == nil {
			continue
		}
		metadata.LinksToHere = []string{}
		if err := MetaDataSaveRaw(metadata); err != nil {
			logging.LogWarning("failed to clear media linkstohere for %s: %v", normalizedPath, err)
		}
	}

	// pre-populate cache so findTopAncestor never hits storage during pass 1
	metaCache := make(map[string]*Metadata, len(paths))
	for _, rawPath := range paths {
		normalizedPath := pathutils.ToWithPrefix(rawPath)
		metadata, err := MetaDataGet(normalizedPath)
		if err != nil || metadata == nil {
			continue
		}
		metaCache[normalizedPath] = metadata
	}

	// first pass: rebuild UsedLinks + Ancestors using cache,
	// build reverse maps in memory for pass 2
	linksToHereMap := make(map[string][]string) // target → []sources
	kidsMap := make(map[string][]string)        // parent → []children

	for _, rawPath := range paths {
		normalizedPath := pathutils.ToWithPrefix(rawPath)

		metadata := metaCache[normalizedPath]
		if metadata == nil {
			continue
		}

		metadata.Ancestor = []string{}
		metadata.Kids = []string{}
		metadata.UsedLinks = []string{}
		metadata.LinksToHere = []string{}

		updateAncestors(metadata, metaCache)

		fullPath := pathutils.ToDocsPath(metadata.Path)
		contentData, err := os.ReadFile(fullPath)
		if err == nil {
			handler := parser.GetParserRegistry().GetHandler(fullPath)
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

		for _, link := range metadata.UsedLinks {
			normalized := pathutils.ToWithPrefix(link)
			linksToHereMap[normalized] = append(linksToHereMap[normalized], normalizedPath)
		}
		for _, parent := range metadata.Parents {
			kidsMap[parent] = append(kidsMap[parent], normalizedPath)
		}

		updateTitle(metadata)

		if err := MetaDataSaveRaw(metadata); err != nil {
			logging.LogWarning("failed to save metadata for %s: %v", metadata.Path, err)
		}
	}

	// second pass: apply reverse maps from cache — no MetaDataGet
	for _, rawPath := range paths {
		normalizedPath := pathutils.ToWithPrefix(rawPath)

		metadata := metaCache[normalizedPath]
		if metadata == nil {
			continue
		}

		metadata.Kids = kidsMap[normalizedPath]
		if metadata.Kids == nil {
			metadata.Kids = []string{}
		}
		metadata.LinksToHere = linksToHereMap[normalizedPath]
		if metadata.LinksToHere == nil {
			metadata.LinksToHere = []string{}
		}

		if err := MetaDataSaveRaw(metadata); err != nil {
			logging.LogWarning("failed to save metadata for %s: %v", normalizedPath, err)
		}
	}

	// third pass: compute related files from cache — no I/O
	for _, rawPath := range paths {
		normalizedPath := pathutils.ToWithPrefix(rawPath)
		metadata := metaCache[normalizedPath]
		if metadata == nil {
			continue
		}
		metadata.Related = computeRelated(metadata, metaCache, 5)
		if err := MetaDataSaveRaw(metadata); err != nil {
			logging.LogWarning("failed to save related for %s: %v", normalizedPath, err)
		}
	}

	// apply media LinksToHere from the same reverse map
	mediaCount := 0
	for _, file := range allMediaFiles {
		normalizedPath := pathutils.ToWithPrefix(file.Path)
		mediaMeta, err := MetaDataGet(normalizedPath)
		if err != nil || mediaMeta == nil {
			continue
		}
		mediaMeta.LinksToHere = linksToHereMap[normalizedPath]
		if mediaMeta.LinksToHere == nil {
			mediaMeta.LinksToHere = []string{}
		}
		if err := MetaDataSaveRaw(mediaMeta); err != nil {
			logging.LogWarning("failed to save media linkstohere for %s: %v", normalizedPath, err)
			continue
		}
		mediaCount++
	}
	log.Printf("media files with linkstohere updated: %d", mediaCount)
	logging.LogInfo("media refs found in docs usedlinks: %d media files referenced", mediaCount)

	log.Printf("=== metadata links rebuild completed ===")
	logging.LogInfo("metadata links rebuild completed")
	if OnMetadataRebuild != nil {
		OnMetadataRebuild()
	}
	return nil
}

// MetaDataLinksRebuildForFile rebuilds link metadata for a single file.
func MetaDataLinksRebuildForFile(filePath string) error {
	normalizedPath := pathutils.ToWithPrefix(filePath)
	logging.LogInfo("rebuilding metadata links for file: %s", normalizedPath)

	metadata, err := MetaDataGet(normalizedPath)
	if err != nil {
		return err
	}
	if metadata == nil {
		return fmt.Errorf("metadata not found for %s", normalizedPath)
	}

	metadata.Ancestor = []string{}
	metadata.Kids = []string{}
	metadata.UsedLinks = []string{}
	metadata.LinksToHere = []string{}

	updateAncestors(metadata, nil)
	updateUsedLinks(metadata)
	updateTitle(metadata)

	if err := MetaDataSaveRaw(metadata); err != nil {
		return err
	}

	updateKidsAndLinksToHere(metadata)
	metadata.Related = computeRelated(metadata, nil, 5)

	if err := MetaDataSaveRaw(metadata); err != nil {
		return err
	}

	logging.LogInfo("metadata links rebuild completed for file: %s", normalizedPath)
	if OnMetadataRebuild != nil {
		OnMetadataRebuild()
	}
	return nil
}

func updateAncestors(metadata *Metadata, cache map[string]*Metadata) {
	visited := make(map[string]bool)
	var ancestors []string

	for _, parent := range metadata.Parents {
		if visited[parent] {
			continue
		}
		visited[parent] = true

		ancestor := findTopAncestor(parent, make(map[string]bool), cache)
		if ancestor != "" && ancestor != metadata.Path {
			ancestors = append(ancestors, ancestor)
		}
	}

	metadata.Ancestor = ancestors
}

func findTopAncestor(filePath string, visited map[string]bool, cache map[string]*Metadata) string {
	if visited[filePath] {
		logging.LogWarning("cycle detected in parent chain for %s", filePath)
		return ""
	}
	visited[filePath] = true

	var metadata *Metadata
	if cache != nil {
		cacheKey := pathutils.ToWithPrefix(filePath)
		metadata = cache[cacheKey]

	}

	if metadata == nil {
		var err error
		metadata, err = MetaDataGet(filePath)
		if err != nil || metadata == nil {
			logging.LogWarning("cannot find metadata for parent %s", filePath)
			return filePath
		}
	}

	if len(metadata.Parents) == 0 {
		return filePath
	}

	for _, parent := range metadata.Parents {
		return findTopAncestor(parent, visited, cache)
	}

	return filePath
}

func updateUsedLinks(metadata *Metadata) {
	// skip link extraction for media files
	if strings.HasPrefix(metadata.Path, "media/") {
		return
	}

	fullPath := pathutils.ToFullPath(metadata.Path)

	logging.LogInfo("processing file for links: %s", fullPath)

	contentData, err := os.ReadFile(fullPath)
	if err != nil {
		logging.LogWarning("failed to read file %s: %v", fullPath, err)
		return
	}

	handler := parser.GetParserRegistry().GetHandler(fullPath)
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

		if !slices.Contains(linkedMetadata.LinksToHere, metadata.Path) {
			linkedMetadata.LinksToHere = append(linkedMetadata.LinksToHere, metadata.Path)

			if err := MetaDataSaveRaw(linkedMetadata); err != nil {
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

			if idx := slices.Index(linkedMetadata.LinksToHere, metadata.Path); idx != -1 {
				linkedMetadata.LinksToHere = slices.Delete(linkedMetadata.LinksToHere, idx, idx+1)

				if err := MetaDataSaveRaw(linkedMetadata); err != nil {
					logging.LogWarning("failed to save linkstohere for %s: %v", oldLink, err)
				} else {
					logging.LogInfo("removed %s from linkstohere of %s", metadata.Path, oldLink)
				}
			}
		}
	}
}

func updateKidsAndLinksToHere(metadata *Metadata) {
	files, err := GetAllPhysicalFiles()
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

		if slices.Contains(otherMetadata.Parents, metadata.Path) {
			kids = append(kids, file.Path)
		}

		if slices.Contains(otherMetadata.UsedLinks, metadata.Path) {
			linksToHere = append(linksToHere, file.Path)
		}
	}

	metadata.Kids = kids
	metadata.LinksToHere = linksToHere
}

// UpdateLinksForMovedFile updates all files that link to a moved file with the new path.
func UpdateLinksForMovedFile(oldPath, newPath string) error {
	logging.LogInfo("updating links for moved file: %s -> %s", oldPath, newPath)

	normalizedOldPath := pathutils.ToWithPrefix(oldPath)
	normalizedNewPath := pathutils.ToWithPrefix(newPath)

	oldMetadata, err := MetaDataGet(normalizedOldPath)
	if err != nil {
		logging.LogWarning("could not get metadata for moved file %s: %v", normalizedOldPath, err)
		return err
	}

	if err := moveFileMetadata(oldPath, newPath); err != nil {
		logging.LogError("failed to move metadata for %s: %v", oldPath, err)
		return err
	}

	if err := chat.MoveFilePath(normalizedOldPath, normalizedNewPath); err != nil {
		logging.LogWarning("failed to move chat messages for %s -> %s: %v", normalizedOldPath, normalizedNewPath, err)
	}

	movedMetadata, err := MetaDataGet(normalizedNewPath)
	if err != nil || movedMetadata == nil {
		logging.LogWarning("could not get moved metadata for %s: %v", normalizedNewPath, err)
	}

	// step 1: rebuild outbound links for the moved file
	if movedMetadata != nil {
		logging.LogInfo("rebuilding outbound links for moved file %s", normalizedNewPath)
		updateUsedLinks(movedMetadata)
		if err := MetaDataSaveRaw(movedMetadata); err != nil {
			logging.LogWarning("failed to save rebuilt links for moved file %s: %v", normalizedNewPath, err)
		}
	}

	// step 2: update file content in files that linked to the old path
	if oldMetadata != nil && len(oldMetadata.LinksToHere) > 0 {
		logging.LogInfo("found %d files linking to %s, updating their content", len(oldMetadata.LinksToHere), normalizedOldPath)

		updatedFiles := 0
		for _, linkingFilePath := range oldMetadata.LinksToHere {
			if err := updateLinksInFile(linkingFilePath, oldPath, newPath); err != nil {
				logging.LogError("failed to update links in file %s: %v", linkingFilePath, err)
				continue
			}
			updatedFiles++
		}
		logging.LogInfo("updated links in %d files", updatedFiles)
	}

	// step 3: update LinksToHere in files the moved file links to
	if movedMetadata != nil && len(movedMetadata.UsedLinks) > 0 {
		logging.LogInfo("updating LinksToHere in %d files that moved file links to", len(movedMetadata.UsedLinks))

		for _, linkedPath := range movedMetadata.UsedLinks {
			linkedMetadata, err := MetaDataGet(linkedPath)
			if err != nil || linkedMetadata == nil {
				logging.LogWarning("could not get metadata for linked file %s: %v", linkedPath, err)
				continue
			}

			if idx := slices.Index(linkedMetadata.LinksToHere, normalizedOldPath); idx != -1 {
				linkedMetadata.LinksToHere = slices.Delete(linkedMetadata.LinksToHere, idx, idx+1)
				logging.LogInfo("removed %s from LinksToHere of %s", normalizedOldPath, linkedPath)
			}

			if !slices.Contains(linkedMetadata.LinksToHere, normalizedNewPath) {
				linkedMetadata.LinksToHere = append(linkedMetadata.LinksToHere, normalizedNewPath)
				logging.LogInfo("added %s to LinksToHere of %s", normalizedNewPath, linkedPath)
			}

			if err := MetaDataSaveRaw(linkedMetadata); err != nil {
				logging.LogWarning("failed to save LinksToHere updates for %s: %v", linkedPath, err)
			}
		}
	}

	logging.LogInfo("successfully completed link rebuilding for moved file %s -> %s", normalizedOldPath, normalizedNewPath)
	return nil
}

// updateLinksInFile updates links within a single file from oldPath to newPath.
func updateLinksInFile(filePath, oldPath, newPath string) error {
	fullPath := pathutils.ToFullPath(filePath)

	contentData, err := os.ReadFile(fullPath)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	content := string(contentData)
	originalContent := content

	handler := parser.GetParserRegistry().GetHandler(fullPath)
	if handler == nil {
		logging.LogWarning("no handler found for file %s, skipping link update", filePath)
		return nil
	}

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
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.Contains(line, oldPath) {
			start := strings.Index(line, "[["+oldPath)
			if start != -1 {
				end := strings.Index(line[start:], "]]")
				if end != -1 {
					end += start
					linkPart := line[start+2 : end]

					if strings.HasPrefix(linkPart, oldPath) {
						if len(linkPart) == len(oldPath) {
							lines[i] = strings.Replace(line, "[["+oldPath+"]]", "[["+newPath+"]]", 1)
							updated = true
						} else if linkPart[len(oldPath)] == '|' {
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

	if updated && content != originalContent {
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write updated content to %s: %w", filePath, err)
		}

		logging.LogInfo("updated links in file %s: %s -> %s", filePath, oldPath, newPath)

		if err := UpdateLinksForSingleFile(filePath); err != nil {
			logging.LogWarning("failed to rebuild links for modified file %s: %v", filePath, err)
		}
	}

	return nil
}

// moveFileMetadata moves metadata from old path to new path.
func moveFileMetadata(oldPath, newPath string) error {
	normalizedOldPath := pathutils.ToWithPrefix(oldPath)
	normalizedNewPath := pathutils.ToWithPrefix(newPath)

	metadata, err := MetaDataGet(normalizedOldPath)
	if err != nil {
		logging.LogDebug("no metadata found for %s, creating new metadata for %s", normalizedOldPath, normalizedNewPath)
		newMetadata := &Metadata{Path: normalizedNewPath}
		return MetaDataSave(newMetadata)
	}

	if metadata == nil {
		newMetadata := &Metadata{Path: normalizedNewPath}
		return MetaDataSave(newMetadata)
	}

	metadata.Path = normalizedNewPath

	if err := MetaDataSave(metadata); err != nil {
		return fmt.Errorf("failed to save metadata for new path %s: %w", normalizedNewPath, err)
	}

	if err := MetaDataDelete(normalizedOldPath); err != nil {
		logging.LogWarning("failed to delete old metadata for %s: %v", normalizedOldPath, err)
	}

	logging.LogInfo("moved metadata: %s -> %s", normalizedOldPath, normalizedNewPath)
	return nil
}

// updateTitle extracts the title from the first markdown header in the file.
func updateTitle(metadata *Metadata) {
	if strings.HasPrefix(metadata.Path, "media/") {
		return
	}

	fullPath := pathutils.ToFullPath(metadata.Path)

	logging.LogDebug("extracting title for %s", metadata.Path)

	file, err := os.Open(fullPath)
	if err != nil {
		logging.LogWarning("failed to open file %s: %v", fullPath, err)
		return
	}
	defer file.Close()

	buffer := make([]byte, 1024)
	n, err := file.Read(buffer)
	if err != nil && n == 0 {
		logging.LogWarning("failed to read file %s: %v", fullPath, err)
		return
	}

	content := string(buffer[:n])

	// strip YAML front matter before scanning for the title header
	body := parser.StripFrontMatter([]byte(content))
	lines := strings.Split(string(body), "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if strings.HasPrefix(trimmed, "# ") {
			title := strings.TrimSpace(trimmed[2:])
			if title != "" {
				metadata.Title = title
				logging.LogDebug("found title for %s: %s", metadata.Path, title)
				return
			}
		}

		break
	}

	metadata.Title = ""
	logging.LogDebug("no title found for %s", metadata.Path)
}

// updateParentChildRelationships updates parent-child relationships when parents change.
func updateParentChildRelationships(metadata *Metadata, oldParents []string) {
	logging.LogInfo("updating parent-child relationships for %s: old=%v, new=%v", metadata.Path, oldParents, metadata.Parents)

	for _, oldParent := range oldParents {
		if !slices.Contains(metadata.Parents, oldParent) {
			parentMetadata, err := MetaDataGet(oldParent)
			if err != nil || parentMetadata == nil {
				logging.LogWarning("failed to get metadata for former parent %s: %v", oldParent, err)
				continue
			}

			if idx := slices.Index(parentMetadata.Kids, metadata.Path); idx != -1 {
				parentMetadata.Kids = slices.Delete(parentMetadata.Kids, idx, idx+1)

				if err := MetaDataSaveRaw(parentMetadata); err != nil {
					logging.LogWarning("failed to update kids list for %s: %v", oldParent, err)
				} else {
					logging.LogInfo("removed %s from kids list of %s", metadata.Path, oldParent)
				}
			}
		}
	}

	for _, newParent := range metadata.Parents {
		if !slices.Contains(oldParents, newParent) {
			parentMetadata, err := MetaDataGet(newParent)
			if err != nil || parentMetadata == nil {
				logging.LogWarning("failed to get metadata for new parent %s: %v", newParent, err)
				continue
			}

			if !slices.Contains(parentMetadata.Kids, metadata.Path) {
				parentMetadata.Kids = append(parentMetadata.Kids, metadata.Path)

				if err := MetaDataSaveRaw(parentMetadata); err != nil {
					logging.LogWarning("failed to update kids list for %s: %v", newParent, err)
				} else {
					logging.LogInfo("added %s to kids list of %s", metadata.Path, newParent)
				}
			}
		}
	}
}

// UpdateLinksForSingleFile updates link metadata for a single file incrementally.
func UpdateLinksForSingleFile(filePath string) error {
	logging.LogInfo("updating links for file: %s", filePath)

	metadata, err := MetaDataGet(filePath)
	if err != nil || metadata == nil {
		logging.LogWarning("failed to get metadata for file %s: %v", filePath, err)
		return err
	}

	updateUsedLinks(metadata)

	if err := MetaDataSaveRaw(metadata); err != nil {
		logging.LogError("failed to save updated metadata for file %s: %v", filePath, err)
		return err
	}

	logging.LogInfo("updated links for file %s: %d outbound links", filePath, len(metadata.UsedLinks))
	return nil
}

// computeRelated scores candidate files by shared link co-occurrence with target.
// cache may be nil, in which case it falls back to MetaDataGet for each file.
func computeRelated(target *Metadata, cache map[string]*Metadata, limit int) []string {
	neighbors := make(map[string]struct{}, len(target.UsedLinks)+len(target.LinksToHere))
	for _, l := range target.UsedLinks {
		neighbors[l] = struct{}{}
	}
	for _, l := range target.LinksToHere {
		neighbors[l] = struct{}{}
	}
	if len(neighbors) == 0 {
		return []string{}
	}

	scores := make(map[string]int)
	if cache != nil {
		for path, other := range cache {
			if path == target.Path {
				continue
			}
			score := 0
			for _, l := range other.UsedLinks {
				if _, ok := neighbors[l]; ok {
					score++
				}
			}
			for _, l := range other.LinksToHere {
				if _, ok := neighbors[l]; ok {
					score++
				}
			}
			if score > 0 {
				scores[path] = score
			}
		}
	} else {
		allFiles, err := GetAllPhysicalFiles()
		if err != nil {
			return []string{}
		}
		for _, f := range allFiles {
			if f.Path == target.Path {
				continue
			}
			other, err := MetaDataGet(f.Path)
			if err != nil || other == nil {
				continue
			}
			score := 0
			for _, l := range other.UsedLinks {
				if _, ok := neighbors[l]; ok {
					score++
				}
			}
			for _, l := range other.LinksToHere {
				if _, ok := neighbors[l]; ok {
					score++
				}
			}
			if score > 0 {
				scores[f.Path] = score
			}
		}
	}

	type scored struct {
		path  string
		score int
	}
	ranked := make([]scored, 0, len(scores))
	for path, score := range scores {
		ranked = append(ranked, scored{path, score})
	}
	slices.SortFunc(ranked, func(a, b scored) int { return b.score - a.score })

	result := make([]string, 0, limit)
	for i, r := range ranked {
		if i >= limit {
			break
		}
		result = append(result, r.path)
	}
	return result
}

// StartMetaGetCounter activates MetaDataGet call counting.
func StartMetaGetCounter() {
	count := 0
	rebuildMetaGetCount = &count
}

// StopMetaGetCounter deactivates counting and returns the total.
func StopMetaGetCounter() {
	if rebuildMetaGetCount == nil {
		return
	}
	count := *rebuildMetaGetCount
	rebuildMetaGetCount = nil
	logging.LogBuilder("rebuild-metadata").Printf("total MetaDataGet calls: %d", count)
}

// UpdateLinksForMovedMedia updates all doc files that reference a moved media file.
// Instead of relying on LinksToHere (which may be stale), it scans all doc files'
// UsedLinks for the old media path — a safe reverse lookup.
// Must be called BEFORE MoveMediaMetadata.
func UpdateLinksForMovedMedia(oldMediaPath, newMediaPath string) error {
	normalizedOld := pathutils.ToWithPrefix(oldMediaPath)

	allFiles, err := GetAllFiles()
	if err != nil {
		return fmt.Errorf("failed to list files for media link update: %w", err)
	}

	var updated int
	for _, file := range allFiles {
		metadata, err := MetaDataGet(file.Path)
		if err != nil || metadata == nil {
			continue
		}
		if !slices.Contains(metadata.UsedLinks, normalizedOld) {
			continue
		}
		if err := updateLinksInFile(file.Path, oldMediaPath, newMediaPath); err != nil {
			logging.LogWarning("failed to update media links in %s: %v", file.Path, err)
		} else {
			updated++
			logging.LogInfo("updated media link in %s: %s -> %s", file.Path, oldMediaPath, newMediaPath)
		}
	}

	logging.LogInfo("updated media links in %d files: %s -> %s", updated, oldMediaPath, newMediaPath)
	return nil
}

// MoveMediaMetadata moves metadata from old media path to new media path.
func MoveMediaMetadata(oldPath, newPath string) error {
	return moveFileMetadata(oldPath, newPath)
}
