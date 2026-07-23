// Package files - Link management for metadata
package files

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
func MetaDataLinksRebuild(key logging.Key) error {
	logging.LogInfo(key, "metadata links rebuild started")

	paths, err := contentStorage.ListFiles()
	if err != nil {
		return err
	}
	logging.LogInfo(key, "docs files to process: %d", len(paths))

	// load media files once — used in zeroth pass and final media pass
	allMediaFiles, err := GetAllMediaFiles()
	if err != nil {
		logging.LogWarning(key, "failed to get media files for link rebuild: %v", err)
		allMediaFiles = nil
	}
	logging.LogInfo(key, "media files found: %d", len(allMediaFiles))

	// zeroth pass: clear LinksToHere on all media files so stale references don't persist
	for _, file := range allMediaFiles {
		normalizedPath := pathutils.ToWithPrefix(file.Path)
		metadata, err := MetaDataGet(normalizedPath)
		if err != nil || metadata == nil {
			continue
		}
		metadata.LinksToHere = []string{}
		if err := MetaDataSaveRaw(metadata); err != nil {
			logging.LogWarning(key, "failed to clear media linkstohere for %s: %v", normalizedPath, err)
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
					cleanLink := resolveMediaLink(utils.CleanLink(link))
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
			logging.LogWarning(key, "failed to save metadata for %s: %v", metadata.Path, err)
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
			logging.LogWarning(key, "failed to save metadata for %s: %v", normalizedPath, err)
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
			logging.LogWarning(key, "failed to save related for %s: %v", normalizedPath, err)
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
			logging.LogWarning(key, "failed to save media linkstohere for %s: %v", normalizedPath, err)
			continue
		}
		mediaCount++
	}
	logging.LogInfo(key, "media files with linkstohere updated: %d", mediaCount)
	logging.LogInfo(key, "media refs found in docs usedlinks: %d media files referenced", mediaCount)

	logging.LogInfo(key, "metadata links rebuild completed")
	if OnMetadataRebuild != nil {
		OnMetadataRebuild()
	}
	return nil
}

// MetaDataLinksRebuildForFile rebuilds link metadata for a single file.
func MetaDataLinksRebuildForFile(filePath string) error {
	normalizedPath := pathutils.ToWithPrefix(filePath)
	logging.LogInfo(logging.KeyApp, "rebuilding metadata links for file: %s", normalizedPath)

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

	logging.LogInfo(logging.KeyApp, "metadata links rebuild completed for file: %s", normalizedPath)
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
		logging.LogWarning(logging.KeyApp, "cycle detected in parent chain for %s", filePath)
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
			logging.LogWarning(logging.KeyApp, "cannot find metadata for parent %s", filePath)
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

// resolveMediaLink promotes a link lacking the "media/" prefix to its prefixed
// form when it matches an existing media file. The media detail page displays
// paths without that prefix, so links copied from there would otherwise never
// match stored media metadata and silently fail to register in LinksToHere.
func resolveMediaLink(link string) string {
	if link == "" || strings.HasPrefix(link, "media/") || strings.HasPrefix(link, "docs/") {
		return link
	}
	if _, err := os.Stat(pathutils.ToMediaPath(link)); err == nil {
		return "media/" + link
	}
	return link
}

func updateUsedLinks(metadata *Metadata) {
	// skip link extraction for media files
	if strings.HasPrefix(metadata.Path, "media/") {
		return
	}

	fullPath := pathutils.ToFullPath(metadata.Path)

	logging.LogInfo(logging.KeyApp, "processing file for links: %s", fullPath)

	contentData, err := os.ReadFile(fullPath)
	if err != nil {
		logging.LogWarning(logging.KeyApp, "failed to read file %s: %v", fullPath, err)
		return
	}

	handler := parser.GetParserRegistry().GetHandler(fullPath)
	if handler == nil {
		logging.LogWarning(logging.KeyApp, "no handler found for file %s", fullPath)
		return
	}

	links := handler.ExtractLinks(contentData)
	logging.LogInfo(logging.KeyApp, "extracted %d links from %s", len(links), metadata.Path)

	// store old links to detect removals
	oldUsedLinks := make([]string, len(metadata.UsedLinks))
	copy(oldUsedLinks, metadata.UsedLinks)

	metadata.UsedLinks = []string{}

	for _, link := range links {
		cleanLink := resolveMediaLink(utils.CleanLink(link))

		if cleanLink == "" || cleanLink == metadata.Path {
			continue
		}

		if !slices.Contains(metadata.UsedLinks, cleanLink) {
			metadata.UsedLinks = append(metadata.UsedLinks, cleanLink)
		}
	}

	logging.LogDebug(logging.KeyApp, "cleaned used links for %s: %v", metadata.Path, metadata.UsedLinks)

	// update linkstohere in the found files
	updateLinksToHere(metadata, oldUsedLinks)
}

func updateLinksToHere(metadata *Metadata, oldUsedLinks []string) {
	logging.LogInfo(logging.KeyApp, "updating linkstohere for linked files from %s", metadata.Path)

	// add current file to linkstohere for new links
	for _, usedLink := range metadata.UsedLinks {
		linkedMetadata, err := MetaDataGet(usedLink)
		if err != nil || linkedMetadata == nil {
			logging.LogDebug(logging.KeyApp, "skipping linkstohere update for %s: metadata not found", usedLink)
			continue
		}

		if !slices.Contains(linkedMetadata.LinksToHere, metadata.Path) {
			linkedMetadata.LinksToHere = append(linkedMetadata.LinksToHere, metadata.Path)

			if err := MetaDataSaveRaw(linkedMetadata); err != nil {
				logging.LogWarning(logging.KeyApp, "failed to save linkstohere for %s: %v", usedLink, err)
			} else {
				logging.LogInfo(logging.KeyApp, "added %s to linkstohere of %s", metadata.Path, usedLink)
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
					logging.LogWarning(logging.KeyApp, "failed to save linkstohere for %s: %v", oldLink, err)
				} else {
					logging.LogInfo(logging.KeyApp, "removed %s from linkstohere of %s", metadata.Path, oldLink)
				}
			}
		}
	}
}

func updateKidsAndLinksToHere(metadata *Metadata) {
	files, err := GetAllPhysicalFiles()
	if err != nil {
		logging.LogWarning(logging.KeyApp, "failed to get all files for updating kids and links: %v", err)
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

// UpdateLinksForMovedFile updates all files that link to a moved file with the
// new path, then refreshes the aggregate caches. For moving/renaming many
// files in one request (e.g. a folder move), call UpdateLinksForMovedFileNoRefresh
// in the loop and RefreshCaches() once afterwards instead - otherwise each file
// kicks off its own full background cache rebuild.
func UpdateLinksForMovedFile(key logging.Key, oldPath, newPath string) error {
	if err := UpdateLinksForMovedFileNoRefresh(key, oldPath, newPath); err != nil {
		return err
	}
	RefreshCaches()
	return nil
}

// UpdateLinksForMovedFileNoRefresh is UpdateLinksForMovedFile without the
// aggregate cache refresh. See UpdateLinksForMovedFile.
func UpdateLinksForMovedFileNoRefresh(key logging.Key, oldPath, newPath string) error {
	logging.LogInfo(key, "updating links for moved file: %s -> %s", oldPath, newPath)

	normalizedOldPath := pathutils.ToWithPrefix(oldPath)
	normalizedNewPath := pathutils.ToWithPrefix(newPath)

	oldMetadata, err := MetaDataGet(normalizedOldPath)
	if err != nil {
		logging.LogWarning(key, "could not get metadata for moved file %s: %v", normalizedOldPath, err)
		return err
	}

	if err := moveFileMetadata(key, oldPath, newPath); err != nil {
		logging.LogError(key, "failed to move metadata for %s: %v", oldPath, err)
		return err
	}

	if err := chat.MoveFilePath(normalizedOldPath, normalizedNewPath); err != nil {
		logging.LogWarning(key, "failed to move chat messages for %s -> %s: %v", normalizedOldPath, normalizedNewPath, err)
	}

	movedMetadata, err := MetaDataGet(normalizedNewPath)
	if err != nil || movedMetadata == nil {
		logging.LogWarning(key, "could not get moved metadata for %s: %v", normalizedNewPath, err)
	}

	// step 1: rebuild outbound links for the moved file
	if movedMetadata != nil {
		logging.LogInfo(key, "rebuilding outbound links for moved file %s", normalizedNewPath)
		updateUsedLinks(movedMetadata)
		if err := MetaDataSaveRaw(movedMetadata); err != nil {
			logging.LogWarning(key, "failed to save rebuilt links for moved file %s: %v", normalizedNewPath, err)
		}
	}

	// step 2: update file content in files that linked to the old path
	if oldMetadata != nil && len(oldMetadata.LinksToHere) > 0 {
		logging.LogInfo(key, "found %d files linking to %s, updating their content", len(oldMetadata.LinksToHere), normalizedOldPath)

		updatedFiles := 0
		for _, linkingFilePath := range oldMetadata.LinksToHere {
			ok, err := updateLinksInFile(key, linkingFilePath, oldPath, newPath)
			if err != nil {
				logging.LogError(key, "failed to update links in file %s: %v", linkingFilePath, err)
				continue
			}
			if !ok {
				logging.LogWarning(key, "no literal link occurrence found in %s for %s -> %s", linkingFilePath, oldPath, newPath)
				continue
			}
			updatedFiles++
		}
		logging.LogInfo(key, "updated links in %d files", updatedFiles)
	}

	// step 3: update LinksToHere in files the moved file links to
	if movedMetadata != nil && len(movedMetadata.UsedLinks) > 0 {
		logging.LogInfo(key, "updating LinksToHere in %d files that moved file links to", len(movedMetadata.UsedLinks))

		for _, linkedPath := range movedMetadata.UsedLinks {
			linkedMetadata, err := MetaDataGet(linkedPath)
			if err != nil || linkedMetadata == nil {
				logging.LogWarning(key, "could not get metadata for linked file %s: %v", linkedPath, err)
				continue
			}

			if idx := slices.Index(linkedMetadata.LinksToHere, normalizedOldPath); idx != -1 {
				linkedMetadata.LinksToHere = slices.Delete(linkedMetadata.LinksToHere, idx, idx+1)
				logging.LogInfo(key, "removed %s from LinksToHere of %s", normalizedOldPath, linkedPath)
			}

			if !slices.Contains(linkedMetadata.LinksToHere, normalizedNewPath) {
				linkedMetadata.LinksToHere = append(linkedMetadata.LinksToHere, normalizedNewPath)
				logging.LogInfo(key, "added %s to LinksToHere of %s", normalizedNewPath, linkedPath)
			}

			if err := MetaDataSaveRaw(linkedMetadata); err != nil {
				logging.LogWarning(key, "failed to save LinksToHere updates for %s: %v", linkedPath, err)
			}
		}
	}

	logging.LogInfo(key, "successfully completed link rebuilding for moved file %s -> %s", normalizedOldPath, normalizedNewPath)
	return nil
}

// rebuildLinkTarget reconstructs a link target for newPath, preserving whether
// the original link used an absolute "/files/..." view URL or a bare relative path.
func rebuildLinkTarget(originalTarget, newPath string) string {
	if strings.HasPrefix(strings.TrimPrefix(originalTarget, "/"), "files/") {
		return pathutils.ToFileURL(pathutils.ToWithPrefix(newPath))
	}
	return newPath
}

// updateLinksInFile updates links within a single file from oldPath to newPath.
// The returned bool reports whether a matching link was actually found and rewritten.
func updateLinksInFile(key logging.Key, filePath, oldPath, newPath string) (bool, error) {
	fullPath := pathutils.ToFullPath(filePath)

	contentData, err := os.ReadFile(fullPath)
	if err != nil {
		return false, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	content := string(contentData)
	originalContent := content

	handler := parser.GetParserRegistry().GetHandler(fullPath)
	if handler == nil {
		logging.LogWarning(key, "no handler found for file %s, skipping link update", filePath)
		return false, nil
	}

	updated := false

	// markdown-style links: [text](oldPath) -> [text](newPath). The link target may
	// also carry an anchor and/or be an absolute "/files/docs/..." view URL rather than
	// a bare relative path, so each target is cleaned the same way metadata links are
	// before comparing, and the original absolute/relative style is preserved on write.
	markdownLinkRe := regexp.MustCompile(`\]\(([^)]+)\)`)
	content = markdownLinkRe.ReplaceAllStringFunc(content, func(match string) string {
		raw := match[2 : len(match)-1]
		base, anchor, hasAnchor := strings.Cut(raw, "#")
		if utils.CleanLink(base) != oldPath {
			return match
		}
		updated = true
		logging.LogDebug(key, "updated markdown link in %s", filePath)
		newTarget := rebuildLinkTarget(base, newPath)
		if hasAnchor {
			newTarget += "#" + anchor
		}
		return "](" + newTarget + ")"
	})

	// wiki-style links: [[oldPath]] and [[oldPath|text]] -> same with newPath.
	// Also tries the extensionless form ([[note]] for note.md), since that's
	// how wiki links are normally typed.
	oldWikiTargets := []string{oldPath}
	newWikiTargets := []string{newPath}
	if noExt := strings.TrimSuffix(oldPath, ".md"); noExt != oldPath {
		oldWikiTargets = append(oldWikiTargets, noExt)
		newWikiTargets = append(newWikiTargets, strings.TrimSuffix(newPath, ".md"))
	}

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		for t, oldTarget := range oldWikiTargets {
			newTarget := newWikiTargets[t]
			start := strings.Index(line, "[["+oldTarget)
			if start == -1 {
				continue
			}
			end := strings.Index(line[start:], "]]")
			if end == -1 {
				continue
			}
			end += start
			linkPart := line[start+2 : end]

			if linkPart == oldTarget {
				line = strings.Replace(line, "[["+oldTarget+"]]", "[["+newTarget+"]]", 1)
				updated = true
				logging.LogDebug(key, "updated wiki link in %s", filePath)
			} else if strings.HasPrefix(linkPart, oldTarget) && linkPart[len(oldTarget)] == '|' {
				textPart := linkPart[len(oldTarget):]
				line = strings.Replace(line, "[["+oldTarget+textPart+"]]", "[["+newTarget+textPart+"]]", 1)
				updated = true
				logging.LogDebug(key, "updated wiki link in %s", filePath)
			}
		}
		lines[i] = line
	}

	if updated {
		content = strings.Join(lines, "\n")
	}

	if updated && content != originalContent {
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return false, fmt.Errorf("failed to write updated content to %s: %w", filePath, err)
		}

		logging.LogInfo(key, "updated links in file %s: %s -> %s", filePath, oldPath, newPath)

		if err := UpdateLinksForSingleFile(filePath); err != nil {
			logging.LogWarning(key, "failed to rebuild links for modified file %s: %v", filePath, err)
		}
	}

	return updated, nil
}

// moveFileMetadata moves metadata from old path to new path.
func moveFileMetadata(key logging.Key, oldPath, newPath string) error {
	normalizedOldPath := pathutils.ToWithPrefix(oldPath)
	normalizedNewPath := pathutils.ToWithPrefix(newPath)

	metadata, err := MetaDataGet(normalizedOldPath)
	if err != nil {
		logging.LogDebug(key, "no metadata found for %s, creating new metadata for %s", normalizedOldPath, normalizedNewPath)
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
		logging.LogWarning(key, "failed to delete old metadata for %s: %v", normalizedOldPath, err)
	}

	logging.LogInfo(key, "moved metadata: %s -> %s", normalizedOldPath, normalizedNewPath)
	return nil
}

// updateTitle extracts the title from the first markdown header in the file.
func updateTitle(metadata *Metadata) {
	if strings.HasPrefix(metadata.Path, "media/") {
		return
	}

	fullPath := pathutils.ToFullPath(metadata.Path)

	logging.LogDebug(logging.KeyApp, "extracting title for %s", metadata.Path)

	file, err := os.Open(fullPath)
	if err != nil {
		logging.LogWarning(logging.KeyApp, "failed to open file %s: %v", fullPath, err)
		return
	}
	defer file.Close()

	buffer := make([]byte, 1024)
	n, err := file.Read(buffer)
	if err != nil && n == 0 {
		logging.LogWarning(logging.KeyApp, "failed to read file %s: %v", fullPath, err)
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
				logging.LogDebug(logging.KeyApp, "found title for %s: %s", metadata.Path, title)
				return
			}
		}

		break
	}

	metadata.Title = ""
	logging.LogDebug(logging.KeyApp, "no title found for %s", metadata.Path)
}

// updateParentChildRelationships updates parent-child relationships when parents change.
func updateParentChildRelationships(metadata *Metadata, oldParents []string) {
	logging.LogInfo(logging.KeyApp, "updating parent-child relationships for %s: old=%v, new=%v", metadata.Path, oldParents, metadata.Parents)

	for _, oldParent := range oldParents {
		if !slices.Contains(metadata.Parents, oldParent) {
			parentMetadata, err := MetaDataGet(oldParent)
			if err != nil || parentMetadata == nil {
				logging.LogWarning(logging.KeyApp, "failed to get metadata for former parent %s: %v", oldParent, err)
				continue
			}

			if idx := slices.Index(parentMetadata.Kids, metadata.Path); idx != -1 {
				parentMetadata.Kids = slices.Delete(parentMetadata.Kids, idx, idx+1)

				if err := MetaDataSaveRaw(parentMetadata); err != nil {
					logging.LogWarning(logging.KeyApp, "failed to update kids list for %s: %v", oldParent, err)
				} else {
					logging.LogInfo(logging.KeyApp, "removed %s from kids list of %s", metadata.Path, oldParent)
				}
			}
		}
	}

	for _, newParent := range metadata.Parents {
		if !slices.Contains(oldParents, newParent) {
			parentMetadata, err := MetaDataGet(newParent)
			if err != nil || parentMetadata == nil {
				logging.LogWarning(logging.KeyApp, "failed to get metadata for new parent %s: %v", newParent, err)
				continue
			}

			if !slices.Contains(parentMetadata.Kids, metadata.Path) {
				parentMetadata.Kids = append(parentMetadata.Kids, metadata.Path)

				if err := MetaDataSaveRaw(parentMetadata); err != nil {
					logging.LogWarning(logging.KeyApp, "failed to update kids list for %s: %v", newParent, err)
				} else {
					logging.LogInfo(logging.KeyApp, "added %s to kids list of %s", metadata.Path, newParent)
				}
			}
		}
	}
}

// UpdateLinksForSingleFile updates link metadata for a single file incrementally.
func UpdateLinksForSingleFile(filePath string) error {
	logging.LogInfo(logging.KeyApp, "updating links for file: %s", filePath)

	metadata, err := MetaDataGet(filePath)
	if err != nil || metadata == nil {
		logging.LogWarning(logging.KeyApp, "failed to get metadata for file %s: %v", filePath, err)
		return err
	}

	updateUsedLinks(metadata)

	if err := MetaDataSaveRaw(metadata); err != nil {
		logging.LogError(logging.KeyApp, "failed to save updated metadata for file %s: %v", filePath, err)
		return err
	}

	logging.LogInfo(logging.KeyApp, "updated links for file %s: %d outbound links", filePath, len(metadata.UsedLinks))
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
	logging.LogInfo(logging.KeyFullRebuild, "total MetaDataGet calls: %d", count)
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
		ok, err := updateLinksInFile(logging.KeyApp, file.Path, oldMediaPath, newMediaPath)
		if err != nil {
			logging.LogWarning(logging.KeyApp, "failed to update media links in %s: %v", file.Path, err)
		} else if ok {
			updated++
			logging.LogInfo(logging.KeyApp, "updated media link in %s: %s -> %s", file.Path, oldMediaPath, newMediaPath)
		}
	}

	logging.LogInfo(logging.KeyApp, "updated media links in %d files: %s -> %s", updated, oldMediaPath, newMediaPath)
	return nil
}

// MoveMediaMetadata moves metadata from old media path to new media path.
func MoveMediaMetadata(oldPath, newPath string) error {
	return moveFileMetadata(logging.KeyApp, oldPath, newPath)
}

// BrokenLink is an outbound link whose target no longer exists.
type BrokenLink struct {
	SourceFile string `json:"sourceFile"`
	Target     string `json:"target"`
	Suggested  string `json:"suggested,omitempty"`
}

// FindBrokenLinks scans link metadata (no file content is read) for outbound
// links pointing to paths that no longer exist. A repair is suggested when
// exactly one existing file shares the broken link's basename.
func FindBrokenLinks() ([]BrokenLink, error) {
	docFiles, err := GetAllPhysicalFiles()
	if err != nil {
		return nil, err
	}
	mediaFiles, err := GetAllMediaFiles()
	if err != nil {
		return nil, err
	}

	validPaths := make(map[string]bool, len(docFiles)+len(mediaFiles))
	byBasename := make(map[string][]string)
	for _, f := range docFiles {
		// links may be written either relative ("note.md") or with the docs/
		// prefix ("docs/note.md", as produced by the app's own file-view URLs)
		validPaths[f.Path] = true
		validPaths[pathutils.ToWithPrefix(f.Path)] = true
		byBasename[filepath.Base(f.Path)] = append(byBasename[filepath.Base(f.Path)], f.Path)
	}
	for _, f := range mediaFiles {
		validPaths[f.Path] = true
		byBasename[filepath.Base(f.Path)] = append(byBasename[filepath.Base(f.Path)], f.Path)
	}

	var broken []BrokenLink
	for _, f := range docFiles {
		metadata, err := MetaDataGet(f.Path)
		if err != nil || metadata == nil {
			continue
		}
		for _, target := range metadata.UsedLinks {
			if validPaths[target] {
				continue
			}
			bl := BrokenLink{SourceFile: metadata.Path, Target: target}
			if candidates := byBasename[filepath.Base(target)]; len(candidates) == 1 {
				bl.Suggested = candidates[0]
			}
			broken = append(broken, bl)
		}
	}

	return broken, nil
}

// RepairBrokenLink rewrites a single broken link occurrence in sourceFile
// from oldTarget to newTarget and resyncs link metadata for that file.
// Returns false (with no error) if no matching link occurrence was found,
// e.g. because the link was written in a form updateLinksInFile doesn't match.
func RepairBrokenLink(sourceFile, oldTarget, newTarget string) (bool, error) {
	return updateLinksInFile(logging.KeyRepairLinks, sourceFile, oldTarget, newTarget)
}
