// Package mediatest - sample file/media helpers
package mediatest

import (
	"os"
	"path/filepath"

	"knov/internal/contentStorage"
	"knov/internal/files"
	"knov/internal/pathutils"
	"knov/internal/test"
)

// testDir is the docs-relative sample folder every case seeds into, wiped and reseeded at
// the start of each run so cases never see stale state from a previous run. mediaDir is its
// media-side mirror.
const testDir = "test/media-tests"
const mediaDir = testDir

const (
	contextFile           = "media-context.md"    // upload's context_path target
	usedMediaFile         = "media-used.png"      // linked from a real doc
	orphanMediaFile       = "media-orphan.png"    // no links, for list-partition/stats
	renameMediaFile       = "media-rename-me.png" // linked from renameReferencerFile
	renameReferencerFile  = "media-rename-referencer.md"
	deleteMediaFile       = "media-delete-me.png" // no links, safe to delete
	blockedMediaFile      = "media-blocked.png"   // linked from blockedReferencerFile
	blockedReferencerFile = "media-blocked-referencer.md"
)

// pngMagic is enough of a PNG signature for http.DetectContentType/real decoders to
// recognize the file as image/png without needing a fully valid image payload.
var pngMagic = []byte("\x89PNG\r\n\x1a\n")

func testPath(name string) string {
	return pathutils.ToSlash(filepath.Join(testDir, name))
}

func withPrefix(name string) string {
	return pathutils.ToWithPrefix(testPath(name))
}

func mediaTestPath(name string) string {
	return pathutils.ToSlash(filepath.Join(mediaDir, name))
}

func writeFile(relPath, content string) error {
	full := pathutils.ToDocsPath(relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		return err
	}
	return contentStorage.WriteFile(full, []byte(content), 0644)
}

func writeMediaFile(relPath string, content []byte) error {
	full := pathutils.ToMediaPath(relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		return err
	}
	return contentStorage.WriteFile(full, content, 0644)
}

func saveMediaMetadata(relPath string) error {
	return test.SeedMetadata(&files.Metadata{Path: "media/" + relPath})
}

// linkDoc writes a docs file embedding an image link to the given media path and updates
// its own link metadata (real content-based link extraction, same as connectionstest/
// jobstest use for used-links/linkstohere - not faked).
func linkDoc(relDocPath, mediaRelPath string) error {
	if err := writeFile(relDocPath, "# "+relDocPath+"\n\n![img](media/"+mediaRelPath+")\n"); err != nil {
		return err
	}
	if err := test.SeedMetadata(&files.Metadata{Path: pathutils.ToWithPrefix(relDocPath), Editor: files.EditorTypeCodeMirror}); err != nil {
		return err
	}
	return files.UpdateLinksForSingleFile(pathutils.ToWithPrefix(relDocPath))
}

// resetAndSeed wipes the sample docs/media folders and reseeds: a context file for upload,
// a used/orphan media pair for list-partition and stats, a media file with a real referencing
// doc for the rename case, a plain unreferenced media file for the delete case, and a
// referenced one for the delete-blocked case.
func resetAndSeed() error {
	docsFull := pathutils.ToDocsPath(testDir)
	if err := os.RemoveAll(docsFull); err != nil {
		return err
	}
	if err := os.MkdirAll(docsFull, 0755); err != nil {
		return err
	}
	mediaFull := pathutils.ToMediaPath(mediaDir)
	if err := os.RemoveAll(mediaFull); err != nil {
		return err
	}

	if err := writeFile(testPath(contextFile), "# media-context.md\n\ncontent\n"); err != nil {
		return err
	}
	if err := test.SeedMetadata(&files.Metadata{Path: withPrefix(contextFile), Editor: files.EditorTypeCodeMirror}); err != nil {
		return err
	}

	if err := writeMediaFile(mediaTestPath(usedMediaFile), pngMagic); err != nil {
		return err
	}
	if err := saveMediaMetadata(mediaTestPath(usedMediaFile)); err != nil {
		return err
	}
	if err := linkDoc(testPath("media-used-referencer.md"), mediaTestPath(usedMediaFile)); err != nil {
		return err
	}

	if err := writeMediaFile(mediaTestPath(orphanMediaFile), pngMagic); err != nil {
		return err
	}
	if err := saveMediaMetadata(mediaTestPath(orphanMediaFile)); err != nil {
		return err
	}

	if err := writeMediaFile(mediaTestPath(renameMediaFile), pngMagic); err != nil {
		return err
	}
	if err := saveMediaMetadata(mediaTestPath(renameMediaFile)); err != nil {
		return err
	}
	if err := linkDoc(testPath(renameReferencerFile), mediaTestPath(renameMediaFile)); err != nil {
		return err
	}

	if err := writeMediaFile(mediaTestPath(deleteMediaFile), pngMagic); err != nil {
		return err
	}
	if err := saveMediaMetadata(mediaTestPath(deleteMediaFile)); err != nil {
		return err
	}

	if err := writeMediaFile(mediaTestPath(blockedMediaFile), pngMagic); err != nil {
		return err
	}
	if err := saveMediaMetadata(mediaTestPath(blockedMediaFile)); err != nil {
		return err
	}
	if err := linkDoc(testPath(blockedReferencerFile), mediaTestPath(blockedMediaFile)); err != nil {
		return err
	}

	files.InvalidateFileListCache()
	return files.UpdateOrphanedMediaCache()
}

func errCase(name string, err error) test.CaseResult {
	return test.CaseResult{Name: name, Success: false, Error: err.Error()}
}
