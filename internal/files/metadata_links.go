// Package files - Link management for metadata
package files

import (
	"context"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"knov/internal/chat"
	"knov/internal/contentStorage"
	"knov/internal/logging"
	"knov/internal/parser"
	"knov/internal/pathutils"
	"knov/internal/pathutils/crosspath"
	"knov/internal/utils"
)

var rebuildMetaGetCount *int

// OnMetadataRebuild is called after every full or single-file metadata rebuild.
// Register filter.RegenerateAllIndexes here at startup to keep filter indexes in sync.
var OnMetadataRebuild func()

// MetaDataLinksRebuild rebuilds all link metadata from scratch. Cancellation is cooperative:
// if ctx is canceled it stops before the next file and returns ctx.Err(), leaving the
// metadata half-rebuilt (a re-run is needed to get back to a consistent state).
func MetaDataLinksRebuild(ctx context.Context, key logging.Key, report func(done, total int)) error {
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

	// progress spans the four sequential passes over paths plus the two media passes (zeroth
	// and final), so the bar keeps advancing to the end instead of pinning at N/N during the
	// media tail; bump() is called once at the top of each per-file loop body. It reports the
	// count completed so far (before incrementing), so the bar trails the item in flight rather
	// than jumping to N/N while the last file is still processing; a final report on success
	// closes it out at N/N.
	progressTotal := 4*len(paths) + 2*len(allMediaFiles)
	progressDone := 0
	bump := func() {
		if report != nil {
			report(progressDone, progressTotal)
		}
		progressDone++
	}

	// zeroth pass: clear LinksToHere on all media files so stale references don't persist.
	// Re-gets current metadata under Mutate rather than writing the pre-rebuild snapshot
	// back, so a concurrent edit landing mid-rebuild isn't reverted (see metadata.go godoc).
	for _, file := range allMediaFiles {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		bump()
		normalizedPath := pathutils.ToWithPrefix(file.Path)
		if err := MetaDataMutate(normalizedPath, func(m *Metadata, existed bool) (bool, error) {
			if !existed {
				return false, nil
			}
			m.LinksToHere = []string{}
			return true, nil
		}); err != nil {
			logging.LogWarning(key, "failed to clear media linkstohere for %s: %v", normalizedPath, err)
		}
	}

	// pre-populate cache so findTopAncestor never hits storage during pass 1. Used only to
	// COMPUTE the whole-graph view (ancestors/related need it) - every write below re-gets
	// the current record under MetaDataMutate and patches only rebuild-owned fields, so a
	// MoveCard/tag edit landing mid-rebuild is never reverted by this stale snapshot.
	metaCache := make(map[string]*Metadata, len(paths))
	for _, rawPath := range paths {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		bump()
		normalizedPath := pathutils.DocsPath(rawPath).String()
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
		if ctx.Err() != nil {
			return ctx.Err()
		}
		bump()
		normalizedPath := pathutils.DocsPath(rawPath).String()

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
				for _, target := range handler.ExtractLinks(contentData, metadata.Path) {
					if target != metadata.Path && !slices.Contains(metadata.UsedLinks, target) {
						metadata.UsedLinks = append(metadata.UsedLinks, target)
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

		ancestor, usedLinks, title := metadata.Ancestor, metadata.UsedLinks, metadata.Title
		if err := MetaDataMutate(normalizedPath, func(m *Metadata, existed bool) (bool, error) {
			m.Ancestor = ancestor
			m.UsedLinks = usedLinks
			m.Title = title
			return true, nil
		}); err != nil {
			logging.LogWarning(key, "failed to save metadata for %s: %v", metadata.Path, err)
		}
	}

	// second pass: apply reverse maps from cache, re-getting the current record per write
	for _, rawPath := range paths {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		bump()
		normalizedPath := pathutils.DocsPath(rawPath).String()

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

		kids, linksToHere := metadata.Kids, metadata.LinksToHere
		if err := MetaDataMutate(normalizedPath, func(m *Metadata, existed bool) (bool, error) {
			m.Kids = kids
			m.LinksToHere = linksToHere
			return true, nil
		}); err != nil {
			logging.LogWarning(key, "failed to save metadata for %s: %v", normalizedPath, err)
		}
	}

	// third pass: compute related files from cache — no I/O
	for _, rawPath := range paths {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		bump()
		normalizedPath := pathutils.DocsPath(rawPath).String()
		metadata := metaCache[normalizedPath]
		if metadata == nil {
			continue
		}
		related := computeRelated(metadata, metaCache, 5)
		metadata.Related = related
		if err := MetaDataMutate(normalizedPath, func(m *Metadata, existed bool) (bool, error) {
			m.Related = related
			return true, nil
		}); err != nil {
			logging.LogWarning(key, "failed to save related for %s: %v", normalizedPath, err)
		}
	}

	// apply media LinksToHere from the same reverse map
	mediaCount := 0
	for _, file := range allMediaFiles {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		bump()
		normalizedPath := pathutils.ToWithPrefix(file.Path)
		linksToHere := linksToHereMap[normalizedPath]
		if linksToHere == nil {
			linksToHere = []string{}
		}
		found := false
		err := MetaDataMutate(normalizedPath, func(m *Metadata, existed bool) (bool, error) {
			if !existed {
				return false, nil
			}
			found = true
			m.LinksToHere = linksToHere
			return true, nil
		})
		if err != nil {
			logging.LogWarning(key, "failed to save media linkstohere for %s: %v", normalizedPath, err)
			continue
		}
		if found {
			mediaCount++
		}
	}
	logging.LogInfo(key, "media files with linkstohere updated: %d", mediaCount)
	logging.LogInfo(key, "media refs found in docs usedlinks: %d media files referenced", mediaCount)

	logging.LogInfo(key, "metadata links rebuild completed")
	if report != nil {
		report(progressTotal, progressTotal)
	}
	if OnMetadataRebuild != nil {
		OnMetadataRebuild()
	}
	return nil
}

// MetaDataLinksRebuildForFile rebuilds link metadata for a single file.
func MetaDataLinksRebuildForFile(filePath string) error {
	normalizedPath := pathutils.ToWithPrefix(filePath)
	logging.LogInfo(logging.KeyApp, "rebuilding metadata links for file: %s", normalizedPath)

	unlock := lockMetaPath(normalizedPath)

	metadata, err := MetaDataGet(normalizedPath)
	if err != nil {
		unlock()
		return err
	}
	if metadata == nil {
		unlock()
		return fmt.Errorf("metadata not found for %s", normalizedPath)
	}

	metadata.Ancestor = []string{}
	metadata.Kids = []string{}
	metadata.UsedLinks = []string{}
	metadata.LinksToHere = []string{}

	updateAncestors(metadata, nil)
	fanOut := updateUsedLinks(metadata)
	updateTitle(metadata)

	if err := metaDataSaveRaw(metadata); err != nil {
		unlock()
		return err
	}

	updateKidsAndLinksToHere(metadata)
	metadata.Related = computeRelated(metadata, nil, 5)

	err = metaDataSaveRaw(metadata)
	unlock()
	if err != nil {
		return err
	}

	// runs after normalizedPath's own lock is released - see updateUsedLinks
	for _, apply := range fanOut {
		apply()
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

// updateUsedLinks recomputes metadata.UsedLinks from file content (in-memory only, no other
// path is touched here) and returns closures that push the resulting add/remove onto the
// linked files' LinksToHere. The caller must run those closures only after releasing
// metadata.Path's own write lock - see updateLinksToHereFanOut.
func updateUsedLinks(metadata *Metadata) []func() {
	// skip link extraction for media files
	if strings.HasPrefix(metadata.Path, "media/") {
		return nil
	}

	fullPath := pathutils.ToFullPath(metadata.Path)

	logging.LogInfo(logging.KeyApp, "processing file for links: %s", fullPath)

	contentData, err := os.ReadFile(fullPath)
	if err != nil {
		logging.LogWarning(logging.KeyApp, "failed to read file %s: %v", fullPath, err)
		return nil
	}

	handler := parser.GetParserRegistry().GetHandler(fullPath)
	if handler == nil {
		logging.LogWarning(logging.KeyApp, "no handler found for file %s", fullPath)
		return nil
	}

	links := handler.ExtractLinks(contentData, metadata.Path)
	logging.LogInfo(logging.KeyApp, "extracted %d links from %s", len(links), metadata.Path)

	// store old links to detect removals
	oldUsedLinks := make([]string, len(metadata.UsedLinks))
	copy(oldUsedLinks, metadata.UsedLinks)

	metadata.UsedLinks = []string{}

	for _, target := range links {
		if target != metadata.Path && !slices.Contains(metadata.UsedLinks, target) {
			metadata.UsedLinks = append(metadata.UsedLinks, target)
		}
	}

	logging.LogDebug(logging.KeyApp, "cleaned used links for %s: %v", metadata.Path, metadata.UsedLinks)

	return updateLinksToHereFanOut(metadata.Path, oldUsedLinks, metadata.UsedLinks)
}

// updateLinksToHereFanOut builds the closures that add/remove sourcePath from the LinksToHere
// of files it newly links to / no longer links to. Each closure takes a blocking lock on its
// target path via MetaDataMutate - safe only once sourcePath's own lock has been released,
// otherwise a concurrent mirror-image update (the target file being saved at the same time)
// could deadlock against it.
func updateLinksToHereFanOut(sourcePath string, oldLinks, newLinks []string) []func() {
	var fanOut []func()

	for _, usedLink := range newLinks {
		usedLink := usedLink
		fanOut = append(fanOut, func() {
			changed, err := addLinksToHere(usedLink, sourcePath)
			if err != nil {
				logging.LogWarning(logging.KeyApp, "failed to save linkstohere for %s: %v", usedLink, err)
			} else if changed {
				logging.LogInfo(logging.KeyApp, "added %s to linkstohere of %s", sourcePath, usedLink)
			}
		})
	}

	for _, oldLink := range oldLinks {
		if slices.Contains(newLinks, oldLink) {
			continue
		}
		oldLink := oldLink
		fanOut = append(fanOut, func() {
			changed, err := removeLinksToHere(oldLink, sourcePath)
			if err != nil {
				logging.LogWarning(logging.KeyApp, "failed to save linkstohere for %s: %v", oldLink, err)
			} else if changed {
				logging.LogInfo(logging.KeyApp, "removed %s from linkstohere of %s", sourcePath, oldLink)
			}
		})
	}

	return fanOut
}

// addLinksToHere adds sourcePath to path's LinksToHere if not already present. changed is
// false (with a nil error) when path has no metadata yet or already lists sourcePath.
func addLinksToHere(path, sourcePath string) (changed bool, err error) {
	err = MetaDataMutate(path, func(m *Metadata, existed bool) (bool, error) {
		if !existed || slices.Contains(m.LinksToHere, sourcePath) {
			return false, nil
		}
		m.LinksToHere = append(m.LinksToHere, sourcePath)
		changed = true
		return true, nil
	})
	return changed, err
}

// removeLinksToHere removes sourcePath from path's LinksToHere if present.
func removeLinksToHere(path, sourcePath string) (changed bool, err error) {
	err = MetaDataMutate(path, func(m *Metadata, existed bool) (bool, error) {
		if !existed {
			return false, nil
		}
		idx := slices.Index(m.LinksToHere, sourcePath)
		if idx == -1 {
			return false, nil
		}
		m.LinksToHere = slices.Delete(m.LinksToHere, idx, idx+1)
		changed = true
		return true, nil
	})
	return changed, err
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
	return withRefresh(func() error { return UpdateLinksForMovedFileNoRefresh(key, oldPath, newPath) })
}

// UpdateLinksForMovedFileNoRefresh is UpdateLinksForMovedFile without the
// aggregate cache refresh. See UpdateLinksForMovedFile.
func UpdateLinksForMovedFileNoRefresh(key logging.Key, oldPath, newPath string) error {
	return updateLinksForMovedFile(key, oldPath, newPath, nil)
}

// updateLinksForMovedFile is UpdateLinksForMovedFileNoRefresh for a file moved along with others
// (folder move): movedAlong maps the old metadata path of each other moved file to its new path,
// so a file linking to this one is read where it is now.
func updateLinksForMovedFile(key logging.Key, oldPath, newPath string, movedAlong map[string]string) error {
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

	relinkMovedDoc(key, oldPath, newPath)

	// step 1: rebuild outbound links for the moved file
	var movedMetadata *Metadata
	var movedFanOut []func()
	if err := MetaDataMutate(normalizedNewPath, func(m *Metadata, existed bool) (bool, error) {
		if !existed {
			return false, nil
		}
		logging.LogInfo(key, "rebuilding outbound links for moved file %s", normalizedNewPath)
		movedFanOut = updateUsedLinks(m)
		movedMetadata = m
		return true, nil
	}); err != nil {
		logging.LogWarning(key, "could not rebuild outbound links for moved file %s: %v", normalizedNewPath, err)
	}
	// fan-out runs after normalizedNewPath's own lock is released - see updateUsedLinks
	for _, apply := range movedFanOut {
		apply()
	}

	// step 2: update file content in files that linked to the old path
	if oldMetadata != nil && len(oldMetadata.LinksToHere) > 0 {
		logging.LogInfo(key, "found %d files linking to %s, updating their content", len(oldMetadata.LinksToHere), normalizedOldPath)

		updatedFiles := 0
		for _, linkingFilePath := range oldMetadata.LinksToHere {
			moved, along := movedAlong[linkingFilePath]
			if along {
				linkingFilePath = moved
			}
			ok, err := updateLinksInFile(key, linkingFilePath, oldPath, newPath)
			if err != nil {
				logging.LogError(key, "failed to update links in file %s: %v", linkingFilePath, err)
				continue
			}
			// a relative link between two files moved along still reads right
			if !ok && along {
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
			// a folder has no metadata, a file moved along has none yet - MoveFolder resyncs it afterwards
			if strings.HasSuffix(linkedPath, "/") || slices.Contains(slices.Collect(maps.Values(movedAlong)), linkedPath) {
				continue
			}
			linkedPath := linkedPath
			err := MetaDataMutate(linkedPath, func(m *Metadata, existed bool) (bool, error) {
				if !existed {
					logging.LogWarning(key, "could not get metadata for linked file %s", linkedPath)
					return false, nil
				}

				changed := false
				if idx := slices.Index(m.LinksToHere, normalizedOldPath); idx != -1 {
					m.LinksToHere = slices.Delete(m.LinksToHere, idx, idx+1)
					logging.LogInfo(key, "removed %s from LinksToHere of %s", normalizedOldPath, linkedPath)
					changed = true
				}

				if !slices.Contains(m.LinksToHere, normalizedNewPath) {
					m.LinksToHere = append(m.LinksToHere, normalizedNewPath)
					logging.LogInfo(key, "added %s to LinksToHere of %s", normalizedNewPath, linkedPath)
					changed = true
				}

				return changed, nil
			})
			if err != nil {
				logging.LogWarning(key, "failed to save LinksToHere updates for %s: %v", linkedPath, err)
			}
		}
	}

	logging.LogInfo(key, "successfully completed link rebuilding for moved file %s -> %s", normalizedOldPath, normalizedNewPath)
	return nil
}

// rebuildLinkTarget writes the path of the markdown or html link l in the doc docPath pointing at
// newPath, keeping how l was written: a "/media/" or "/files/" url (html always gets one, a bare
// path would resolve against the page), "media/" for a media file linked without leading "/",
// "/" + the docs-root path for a "/" link and the path from docPath's folder for a bare one.
// Returns the decoded link path, RewriteLinks encodes it (so it's no url built by hand).
func rebuildLinkTarget(docPath string, l parser.Link, newPath string) string {
	newPath = pathutils.ToWithPrefix(newPath)
	rel, media := strings.CutPrefix(newPath, "media/")
	switch {
	case media && (l.Kind == parser.LinkHTML || strings.HasPrefix(l.Path, "/")):
		return "/media/" + rel
	case l.Kind == parser.LinkHTML || strings.HasPrefix(l.Path, "/files/"):
		return parser.FilesLinkPath(newPath)
	case media:
		return newPath
	case strings.HasPrefix(l.Path, "/"):
		if rel := strings.TrimPrefix(newPath, "docs/"); !parser.WrittenAsMedia(rel) {
			return "/" + rel
		}
		return parser.FilesLinkPath(newPath)
	}
	// bare like it was written - unless that would read as the media folder
	relative := relativeDocLink(docPath, newPath)
	if bare := strings.TrimPrefix(relative, "./"); !strings.HasPrefix(bare, "media/") {
		return bare
	}
	return relative
}

// relativeDocLink is pathutils.RelativeLink to the target, except a docs file whose relative link
// would read as the media folder (docs/media/x.md from the docs root: "./media/x.md") is linked
// with its /files/ url.
func relativeDocLink(docPath, target string) string {
	if pathutils.IsMedia(target) {
		// keep the media/ segment, so it doesn't depend on the media fallback of LinkTarget
		return pathutils.RelativeLink(docPath, pathutils.DocsPath(target).String())
	}
	relative := pathutils.RelativeLink(docPath, target)
	if !pathutils.IsMedia(target) && parser.WrittenAsMedia(relative) {
		return parser.FilesLinkPath(target)
	}
	return relative
}

// relinkMovedDoc rewrites the bare, "./" and "../" links of the doc moved from oldPath to newPath (already
// on disk there) to a new relative path, so they keep pointing at their target - only those valid
// at the old location and reading as another file from the new one, so a target moved along
// (folder move) or an already missing one stays as written.
func relinkMovedDoc(key logging.Key, oldPath, newPath string) {
	fullPath := pathutils.ToFullPath(newPath)
	if !parser.IsMarkdownExtension(fullPath) {
		return
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		logging.LogWarning(key, "failed to read moved file %s: %v", newPath, err)
		return
	}
	content, changed := parser.RewriteLinks(string(data), func(l parser.Link) (string, bool) {
		target := parser.LinkTarget(oldPath, l)
		if !pathutils.IsRelativeLink(l.Path) && !parser.IsBareLink(l) || parser.LinkTarget(newPath, l) == target || !fileExists(pathutils.ToFullPath(target)) {
			return "", false
		}
		// a "./" or "../" link gets a new relative path (its docs-root path has no media/ prefix, a
		// relative link to it reads as media anyway), html src/href a url, like rename writes them
		if pathutils.IsRelativeLink(l.Path) && l.Kind != parser.LinkHTML {
			target := parser.LinkTarget(oldPath, l)
			if !strings.HasSuffix(l.Path, ".md") {
				target = strings.TrimSuffix(target, ".md") // keep an extensionless link extensionless
			}
			return relativeDocLink(newPath, target), true
		}
		return rebuildLinkTarget(newPath, l, target), true
	})
	if !changed {
		return
	}
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		logging.LogError(key, "failed to write relative links of moved file %s: %v", newPath, err)
		return
	}
	logging.LogInfo(key, "rewrote relative links of moved file %s", newPath)
}

// renameLinkFunc returns the parser.RewriteLinks callback pointing the links of filePath to oldPath
// at newPath. each link's parser.LinkTarget and oldPath are compared as metadata paths (docs/ or
// media/ prefixed), and the original absolute/relative style is preserved on write - a bare
// markdown link gets a new path from filePath's folder, a markdown or wiki "./" or "../" link a
// new relative path (rebuildLinkTarget). wiki links keep their
// extensionless form ([[note]] for note.md), since that's how they're normally typed - unless
// the new name would then read as another file ([[v1.2]] isn't v1.2.md)
func renameLinkFunc(filePath, oldPath, newPath string) func(l parser.Link) (string, bool) {
	oldPath = pathutils.ToWithPrefix(oldPath)
	return func(l parser.Link) (string, bool) {
		if parser.LinkTarget(filePath, l) != oldPath {
			return "", false
		}
		relative := pathutils.IsRelativeLink(l.Path)
		if l.Kind == parser.LinkHTML || (l.Kind == parser.LinkMarkdown && !relative) {
			return rebuildLinkTarget(filePath, l, newPath), true
		}
		target := newPath
		if bare := strings.TrimSuffix(newPath, ".md"); l.Kind == parser.LinkWiki && !strings.HasSuffix(l.Path, ".md") && utils.WithDefaultLinkExt(bare) == newPath {
			target = bare
		}
		if relative {
			return relativeDocLink(filePath, target), true
		}
		if rel, ok := strings.CutPrefix(target, "docs/"); ok && l.Kind == parser.LinkWiki {
			target = parser.DocsWikiPath(rel) // a wikilink reads docs files from the docs root
		}
		return target, true
	}
}

// updateLinksInFile updates links within a single file from oldPath to newPath.
// The returned bool reports whether a matching link was actually found and rewritten.
func updateLinksInFile(key logging.Key, filePath, oldPath, newPath string) (bool, error) {
	fullPath := pathutils.ToFullPath(filePath)

	contentData, err := os.ReadFile(fullPath)
	if err != nil {
		return false, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	handler := parser.GetParserRegistry().GetHandler(fullPath)
	if handler == nil {
		logging.LogWarning(key, "no handler found for file %s, skipping link update", filePath)
		return false, nil
	}

	content, updated := parser.RewriteLinks(string(contentData), renameLinkFunc(filePath, oldPath, newPath))

	if updated {
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

// moveFileMetadata moves metadata from old path to new path: the user-owned fields carry over
// from the old record via Mutate, then every derived field is recomputed fresh at the new
// location via Sync (same "Mutate then Sync" two-lock pattern as any other
// user-field-plus-derived-fields update).
//
// The initial read of oldPath below is deliberately NOT held under oldPath's lock across the
// rest of this function: doing so would mean acquiring newPath's lock while still holding
// oldPath's, and two goroutines moving in opposite directions at once (a path swap) could then
// deadlock against each other - see the no-two-locks-at-once rule in the package godoc. The
// accepted trade-off is a narrow window in which a concurrent writer of oldPath, landing
// between this read and MetaDataDelete(oldPath) below, has its update silently dropped instead
// of carried over to newPath. Given how infrequently a file move races a metadata edit on the
// very same file, this is left as a known limitation rather than restructured.
func moveFileMetadata(key logging.Key, oldPath, newPath string) error {
	normalizedOldPath := pathutils.ToWithPrefix(oldPath)
	normalizedNewPath := pathutils.ToWithPrefix(newPath)

	oldMetadata, err := MetaDataGet(normalizedOldPath)
	if err != nil {
		logging.LogDebug(key, "no metadata found for %s, creating new metadata for %s", normalizedOldPath, normalizedNewPath)
	}

	if oldMetadata != nil {
		if err := MetaDataMutate(normalizedNewPath, func(m *Metadata, existed bool) (bool, error) {
			m.Tags = oldMetadata.Tags
			m.Parents = oldMetadata.Parents
			m.Editor = oldMetadata.Editor
			m.CreatedAt = oldMetadata.CreatedAt
			m.References = oldMetadata.References
			m.KanbanAddedAt = oldMetadata.KanbanAddedAt
			m.KanbanMovedAt = oldMetadata.KanbanMovedAt
			return true, nil
		}); err != nil {
			return fmt.Errorf("failed to save metadata for new path %s: %w", normalizedNewPath, err)
		}
	}

	if err := MetaDataSyncNoRefresh(normalizedNewPath); err != nil {
		return fmt.Errorf("failed to sync metadata for new path %s: %w", normalizedNewPath, err)
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
	if err == io.EOF && n == 0 {
		// empty file — no title to extract
		metadata.Title = ""
		return
	}
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

// SetParents normalizes and sets path's parent links, updates the ancestor chain, and fans
// out the resulting Kids add/remove onto the old and new parents' own metadata.
func SetParents(path string, parents []string) error {
	return withRefresh(func() error { return SetParentsNoRefresh(path, parents) })
}

// SetParentsNoRefresh is SetParents without the aggregate cache refresh - for callers that set
// several fields or loop over many files in one request, so use SetParents and call
// RefreshCaches() once afterwards instead of paying for a full cache rebuild per field/file.
func SetParentsNoRefresh(path string, parents []string) error {
	normalized := pathutils.ToWithPrefix(path)

	var oldParents, newParents []string
	err := MetaDataMutate(normalized, func(m *Metadata, existed bool) (bool, error) {
		oldParents = append(oldParents, m.Parents...)
		for _, parent := range parents {
			newParents = append(newParents, utils.NormalizeLinkPath(strings.TrimSpace(crosspath.ToSlash(parent))))
		}
		m.Parents = newParents
		updateAncestors(m, nil)
		return true, nil
	})
	if err != nil {
		return err
	}

	// runs after normalized's own lock is released - see parentChildFanOut
	fanOut := parentChildFanOut(normalized, oldParents, newParents)
	for _, apply := range fanOut {
		apply()
	}
	return nil
}

// SetMetadataNoRefresh applies patch's user-owned fields (editor, tags, parents, createdAt,
// references) together with a derived-field resync in a single MetaDataMutate call, so the
// whole request-level update happens under one lock acquisition instead of one per field - a
// concurrent kanban move or file-sync on the same path can no longer interleave between, say,
// the tags and parents writes. Fan-out onto other paths (parents' Kids, linked files'
// LinksToHere) still runs after this path's own lock is released - see parentChildFanOut and
// updateLinksToHereFanOut.
func SetMetadataNoRefresh(path string, patch *Metadata) error {
	normalized := pathutils.ToWithPrefix(path)

	var oldParents, newParents []string
	var fanOut []func()
	err := MetaDataMutate(normalized, func(m *Metadata, existed bool) (bool, error) {
		if !existed {
			m.CreatedAt = time.Now()
		}

		if patch.Editor != "" {
			m.Editor = patch.Editor
		}
		if len(patch.Tags) > 0 {
			oldKanbanStatus := kanbanStatusFromTags(m.Tags)
			cleaned, err := SanitizeKanbanTags(m.Tags, patch.Tags)
			if err != nil {
				logging.LogWarning(logging.KeyApp, "tag sanitization for %s: %v", normalized, err)
			}
			m.Tags = cleaned
			applyKanbanTimestamps(m, oldKanbanStatus)
		}
		if len(patch.Parents) > 0 {
			oldParents = append(oldParents, m.Parents...)
			for _, parent := range patch.Parents {
				newParents = append(newParents, utils.NormalizeLinkPath(strings.TrimSpace(crosspath.ToSlash(parent))))
			}
			m.Parents = newParents
		}
		if !patch.CreatedAt.IsZero() {
			m.CreatedAt = patch.CreatedAt
		}
		if patch.References != nil {
			m.References = patch.References
		}

		fanOut = recomputeDerivedFields(m)
		return true, nil
	})
	if err != nil {
		return err
	}

	for _, apply := range fanOut {
		apply()
	}
	if newParents != nil {
		for _, apply := range parentChildFanOut(normalized, oldParents, newParents) {
			apply()
		}
	}
	return nil
}

// parentChildFanOut builds the closures that add/remove sourcePath from the Kids of files it
// newly claims as parents / no longer claims as parents. Same post-unlock-only rule as
// updateLinksToHereFanOut: each closure blocks on its target path via MetaDataMutate, so it
// must only run after sourcePath's own lock is released.
func parentChildFanOut(sourcePath string, oldParents, newParents []string) []func() {
	logging.LogInfo(logging.KeyApp, "updating parent-child relationships for %s: old=%v, new=%v", sourcePath, oldParents, newParents)

	var fanOut []func()

	for _, oldParent := range oldParents {
		if oldParent == sourcePath || slices.Contains(newParents, oldParent) {
			continue
		}
		oldParent := oldParent
		fanOut = append(fanOut, func() {
			changed, err := removeKid(oldParent, sourcePath)
			if err != nil {
				logging.LogWarning(logging.KeyApp, "failed to update kids list for %s: %v", oldParent, err)
			} else if changed {
				logging.LogInfo(logging.KeyApp, "removed %s from kids list of %s", sourcePath, oldParent)
			}
		})
	}

	for _, newParent := range newParents {
		if newParent == sourcePath || slices.Contains(oldParents, newParent) {
			continue
		}
		newParent := newParent
		fanOut = append(fanOut, func() {
			changed, err := addKid(newParent, sourcePath)
			if err != nil {
				logging.LogWarning(logging.KeyApp, "failed to update kids list for %s: %v", newParent, err)
			} else if changed {
				logging.LogInfo(logging.KeyApp, "added %s to kids list of %s", sourcePath, newParent)
			}
		})
	}

	return fanOut
}

// addKid adds sourcePath to path's Kids if not already present.
func addKid(path, sourcePath string) (changed bool, err error) {
	err = MetaDataMutate(path, func(m *Metadata, existed bool) (bool, error) {
		if !existed || slices.Contains(m.Kids, sourcePath) {
			return false, nil
		}
		m.Kids = append(m.Kids, sourcePath)
		changed = true
		return true, nil
	})
	return changed, err
}

// removeKid removes sourcePath from path's Kids if present.
func removeKid(path, sourcePath string) (changed bool, err error) {
	err = MetaDataMutate(path, func(m *Metadata, existed bool) (bool, error) {
		if !existed {
			return false, nil
		}
		idx := slices.Index(m.Kids, sourcePath)
		if idx == -1 {
			return false, nil
		}
		m.Kids = slices.Delete(m.Kids, idx, idx+1)
		changed = true
		return true, nil
	})
	return changed, err
}

// UpdateLinksForSingleFile updates link metadata for a single file incrementally.
func UpdateLinksForSingleFile(filePath string) error {
	logging.LogInfo(logging.KeyApp, "updating links for file: %s", filePath)

	unlock := lockMetaPath(filePath)

	metadata, err := MetaDataGet(filePath)
	if err != nil {
		unlock()
		logging.LogWarning(logging.KeyApp, "failed to get metadata for file %s: %v", filePath, err)
		return err
	}
	if metadata == nil {
		unlock()
		return nil
	}

	fanOut := updateUsedLinks(metadata)

	err = metaDataSaveRaw(metadata)
	unlock()
	if err != nil {
		logging.LogError(logging.KeyApp, "failed to save updated metadata for file %s: %v", filePath, err)
		return err
	}

	// runs after filePath's own lock is released - see updateUsedLinks
	for _, apply := range fanOut {
		apply()
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
	AboveRoot  bool   `json:"aboveRoot,omitempty"`
}

// FindBrokenLinks scans link metadata for outbound
// links pointing to paths that no longer exist - a folder link (trailing "/") is
// checked with os.Stat, all other targets against the listed files. A repair is
// suggested when an existing media file's path, read as link text, is the broken
// link (the old upload inserted the still-encoded media path raw,
// `media/x%20(1)/pic.png`), or else when exactly one existing file shares the
// broken link's basename. A "../" link climbing above the docs root to an existing target
// (read from the docs' content) is listed as AboveRoot with its target as suggestion, the repair
// writes it without the extra "../".
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
	byLinkText := make(map[string]string)
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
		byLinkText[parser.LinkTarget(parser.PathlessRender, parser.ParseLink(f.Path, parser.LinkMarkdown))] = f.Path
	}

	var broken []BrokenLink
	for _, f := range docFiles {
		metadata, err := MetaDataGet(f.Path)
		if err != nil || metadata == nil {
			continue
		}
		for _, target := range aboveRootTargets(metadata.Path) {
			if validPaths[target] || (strings.HasSuffix(target, "/") && isDir(pathutils.ToFullPath(target))) {
				broken = append(broken, BrokenLink{SourceFile: metadata.Path, Target: target, Suggested: target, AboveRoot: true})
			}
		}
		for _, target := range metadata.UsedLinks {
			if validPaths[target] || (strings.HasSuffix(target, "/") && isDir(pathutils.ToFullPath(target))) {
				continue
			}
			bl := BrokenLink{SourceFile: metadata.Path, Target: target}
			if p, ok := byLinkText[target]; ok {
				bl.Suggested = p
			} else if candidates := byBasename[filepath.Base(target)]; len(candidates) == 1 {
				bl.Suggested = candidates[0]
			}
			broken = append(broken, bl)
		}
	}

	return broken, nil
}

// aboveRootTargets returns the target of every link in the doc docPath that climbs above the docs
// root (pathutils.LinkClimbsAboveRoot), once each.
func aboveRootTargets(docPath string) []string {
	if !parser.IsMarkdownExtension(docPath) {
		return nil
	}
	data, err := os.ReadFile(pathutils.ToFullPath(docPath))
	if err != nil {
		return nil
	}
	var targets []string
	parser.RewriteLinks(string(data), func(l parser.Link) (string, bool) {
		p := l.Path
		if parser.IsBareLink(l) {
			p = "./" + p
		}
		if target := parser.LinkTarget(docPath, l); pathutils.LinkClimbsAboveRoot(docPath, p) && target != "" && !slices.Contains(targets, target) {
			targets = append(targets, target)
		}
		return "", false
	})
	return targets
}

// isDir reports whether fullPath is an existing folder, the target of a folder link ("../", "/files/").
func isDir(fullPath string) bool {
	info, err := os.Stat(fullPath)
	return err == nil && info.IsDir()
}

// RepairBrokenLink rewrites a single broken link occurrence in sourceFile
// from oldTarget to newTarget and resyncs link metadata for that file.
// Returns false (with no error) if no matching link occurrence was found,
// e.g. because the link was written in a form updateLinksInFile doesn't match.
func RepairBrokenLink(sourceFile, oldTarget, newTarget string) (bool, error) {
	return updateLinksInFile(logging.KeyRepairLinks, sourceFile, oldTarget, newTarget)
}
