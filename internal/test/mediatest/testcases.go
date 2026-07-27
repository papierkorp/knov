package mediatest

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"

	"knov/internal/contentStorage"
	"knov/internal/files"
	"knov/internal/pathutils"
	"knov/internal/test"
)

// buildMultipartFile builds a real *multipart.FileHeader/multipart.File pair by encoding
// then re-parsing an actual multipart form body, the same shape r.FormFile("file") hands
// handleAPIMediaUpload - so files.UploadMedia (which takes those two types directly) is
// exercised exactly as the real handler calls it.
func buildMultipartFile(filename string, content []byte) (multipart.File, *multipart.FileHeader, error) {
	buf := new(bytes.Buffer)
	writer := multipart.NewWriter(buf)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, nil, err
	}
	if _, err := part.Write(content); err != nil {
		return nil, nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, nil, err
	}

	reader := multipart.NewReader(buf, writer.Boundary())
	form, err := reader.ReadForm(int64(len(content)) + 1024)
	if err != nil {
		return nil, nil, err
	}
	header := form.File["file"][0]
	file, err := header.Open()
	if err != nil {
		return nil, nil, err
	}
	return file, header, nil
}

func caseUpload() test.CaseResult {
	name := "upload"

	file, _, err := buildMultipartFile("upload-test.png", pngMagic)
	if err != nil {
		return errCase(name, err)
	}
	_, header, err := buildMultipartFile("upload-test.png", pngMagic)
	if err != nil {
		return errCase(name, err)
	}

	result, err := files.UploadMedia(file, header, withPrefix(contextFile))
	if err != nil {
		return errCase(name, err)
	}

	expectedPath := mediaTestPath("upload-test.png")
	_, statErr := os.Stat(pathutils.ToMediaPath(result.Path))
	fileExists := statErr == nil
	meta, err := files.MetaDataGet("media/" + result.Path)

	success := result.Path == expectedPath && fileExists && err == nil && meta != nil
	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("uploaded to %s, mirroring the context file's folder, with metadata created", expectedPath),
		Actual:   fmt.Sprintf("path=%q fileExists=%v metaFound=%v", result.Path, fileExists, meta != nil),
		Success:  success,
	}
	if !success {
		cr.Error = "UploadMedia did not write the file/metadata to the expected mirrored path"
	}
	return cr
}

func containsMediaPath(list []files.File, mediaPathWithPrefix string) bool {
	for _, f := range list {
		if f.Path == mediaPathWithPrefix {
			return true
		}
	}
	return false
}

func caseListPartition() test.CaseResult {
	name := "list-partition"

	mediaFiles, err := files.GetAllMediaFiles()
	if err != nil {
		return errCase(name, err)
	}
	visible := files.FilterByVisibility(mediaFiles)

	orphaned, err := files.GetOrphanedMediaFromCache()
	if err != nil {
		return errCase(name, err)
	}

	used := files.FilterMediaFiles(visible, orphaned, "used")
	orphanedOnly := files.FilterMediaFiles(visible, orphaned, "orphaned")

	usedPath := "media/" + mediaTestPath(usedMediaFile)
	orphanPath := "media/" + mediaTestPath(orphanMediaFile)

	success := containsMediaPath(used, usedPath) && !containsMediaPath(orphanedOnly, usedPath) &&
		containsMediaPath(orphanedOnly, orphanPath) && !containsMediaPath(used, orphanPath)

	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("%s in used (not orphaned), %s in orphaned (not used)", usedPath, orphanPath),
		Actual:   fmt.Sprintf("used=%d orphaned=%d", len(used), len(orphanedOnly)),
		Success:  success,
	}
	if !success {
		cr.Error = "FilterMediaFiles did not partition used/orphaned media as expected"
	}
	return cr
}

// caseRename replicates handleAPIMediaRename's exact sequence (internal/server/api_media.go) -
// update links in referencing docs, move the file, then move its metadata.
func caseRename() test.CaseResult {
	name := "rename"

	oldRel := mediaTestPath(renameMediaFile)
	newRel := mediaTestPath("media-renamed.png")
	oldMediaPath := "media/" + oldRel
	newMediaPath := "media/" + newRel

	if err := files.UpdateLinksForMovedMedia(oldMediaPath, newMediaPath); err != nil {
		return errCase(name, err)
	}

	oldFull := pathutils.ToMediaPath(oldRel)
	newFull := pathutils.ToMediaPath(newRel)
	if err := os.MkdirAll(filepath.Dir(newFull), 0755); err != nil {
		return errCase(name, err)
	}
	if err := os.Rename(oldFull, newFull); err != nil {
		return errCase(name, err)
	}
	if err := files.MoveMediaMetadata(oldMediaPath, newMediaPath); err != nil {
		return errCase(name, err)
	}

	referencerContent, err := os.ReadFile(pathutils.ToDocsPath(testPath(renameReferencerFile)))
	if err != nil {
		return errCase(name, err)
	}
	linkRewritten := bytes.Contains(referencerContent, []byte(newRel)) && !bytes.Contains(referencerContent, []byte(oldRel))

	newMeta, err := files.MetaDataGet(newMediaPath)
	if err != nil {
		return errCase(name, err)
	}
	oldMeta, _ := files.MetaDataGet(oldMediaPath)

	success := linkRewritten && newMeta != nil && oldMeta == nil
	cr := test.CaseResult{
		Name:     name,
		Expected: "referencer's link rewritten to the new path, metadata moved from old to new path",
		Actual:   fmt.Sprintf("linkRewritten=%v newMetaFound=%v oldMetaGone=%v", linkRewritten, newMeta != nil, oldMeta == nil),
		Success:  success,
	}
	if !success {
		cr.Error = "media rename did not update the referencing link/metadata as expected"
	}
	return cr
}

// caseDelete replicates handleAPIDeleteMedia's success path (unreferenced media).
func caseDelete() test.CaseResult {
	name := "delete"

	fullMediaPath := "media/" + mediaTestPath(deleteMediaFile)
	fullPath := pathutils.ToMediaPath(mediaTestPath(deleteMediaFile))

	exists, err := contentStorage.FileExists(fullPath)
	if err != nil || !exists {
		return errCase(name, fmt.Errorf("sample media file missing before delete"))
	}

	if err := contentStorage.DeleteFile(fullPath); err != nil {
		return errCase(name, err)
	}
	if err := files.MetaDataDelete(fullMediaPath); err != nil {
		return errCase(name, err)
	}

	stillExists, _ := contentStorage.FileExists(fullPath)
	meta, _ := files.MetaDataGet(fullMediaPath)

	success := !stillExists && meta == nil
	cr := test.CaseResult{
		Name:     name,
		Expected: "unreferenced media file removed from disk and metadata",
		Actual:   fmt.Sprintf("stillExists=%v metaFound=%v", stillExists, meta != nil),
		Success:  success,
	}
	if !success {
		cr.Error = "media delete did not remove the file/metadata as expected"
	}
	return cr
}

// caseDeleteBlockedWhenReferenced covers handleAPIDeleteMedia's referenced-file guard
// (metadata.LinksToHere > 0 blocks deletion) - checks the same condition the handler does,
// without HTML rendering, and confirms the file is untouched.
func caseDeleteBlockedWhenReferenced() test.CaseResult {
	name := "delete-blocked-when-referenced"

	fullMediaPath := "media/" + mediaTestPath(blockedMediaFile)
	metadata, err := files.MetaDataGet(fullMediaPath)
	if err != nil {
		return errCase(name, err)
	}

	guardBlocks := metadata != nil && len(metadata.LinksToHere) > 0

	fullPath := pathutils.ToMediaPath(mediaTestPath(blockedMediaFile))
	exists, err := contentStorage.FileExists(fullPath)
	if err != nil {
		return errCase(name, err)
	}

	success := guardBlocks && exists
	cr := test.CaseResult{
		Name:     name,
		Expected: "referenced media file's LinksToHere is non-empty (delete guard would block it), file still present",
		Actual:   fmt.Sprintf("linksToHere=%v exists=%v", metadata.LinksToHere, exists),
		Success:  success,
	}
	if !success {
		cr.Error = "the referenced-media delete guard's condition did not hold as expected"
	}
	return cr
}

func caseStats() test.CaseResult {
	name := "stats"

	stats, err := files.GetMediaStorageStats()
	if err != nil {
		return errCase(name, err)
	}

	// GetMediaStorageStats scans every media file in the vault, not just this suite's
	// sample folder, so assert lower bounds rather than exact counts.
	success := stats.TotalFiles >= 2 && stats.UsedFiles >= 1 && stats.OrphanedFiles >= 1
	cr := test.CaseResult{
		Name:     name,
		Expected: "totalFiles/usedFiles/orphanedFiles all at least reflect this suite's sample media",
		Actual:   fmt.Sprintf("total=%d used=%d orphaned=%d", stats.TotalFiles, stats.UsedFiles, stats.OrphanedFiles),
		Success:  success,
	}
	if !success {
		cr.Error = "GetMediaStorageStats did not report the expected counts"
	}
	return cr
}
