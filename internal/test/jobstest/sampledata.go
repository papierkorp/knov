// Package jobstest - sample file/media helpers
package jobstest

import (
	"os"
	"path/filepath"

	"knov/internal/contentStorage"
	"knov/internal/files"
	"knov/internal/pathutils"
	"knov/internal/test"
)

// testDir is the docs-relative sample folder every case seeds into, wiped and reseeded at
// the start of each run so cases never see stale state from a previous run.
const testDir = "test/jobs-tests"

const (
	parentFile = "jobs-parent.md"
	childFile  = "jobs-child.md" // Parents=[parentFile], seeded raw (no cascade) for the rebuild case
	linkerFile = "jobs-linker.md"
	linkedFile = "jobs-linked.md"
	searchFile = "jobs-search.md"
)

const searchMarker = "JobsTestUniqueSearchMarker"

const (
	usedMediaFile   = "jobs-used.png"
	orphanMediaFile = "jobs-orphan.png"
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

func mediaPath(name string) string {
	return pathutils.ToSlash(filepath.Join(testDir, name))
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

// resetAndSeed wipes the sample folder/media and reseeds:
//   - parentFile/childFile: childFile's Parents is set via a raw save that bypasses
//     metaDataUpdate's cascade entirely, so Ancestor/Kids are left empty on purpose -
//     only job.RunFullRebuild (via files.MetaDataLinksRebuild) should populate them.
//   - linkerFile/linkedFile: linkerFile has a real markdown link but UsedLinks is left
//     empty (same raw-save bypass), for the same reason.
//   - searchFile: unique content, indexed only once job.RunSearchReindex runs.
//   - used/orphan media: usedMediaFile is linked from linkerFile's content so it isn't
//     orphaned; orphanMediaFile has no links pointing to it.
func resetAndSeed() error {
	full := pathutils.ToDocsPath(testDir)
	if err := os.RemoveAll(full); err != nil {
		return err
	}
	if err := os.MkdirAll(full, 0755); err != nil {
		return err
	}
	mediaFull := pathutils.ToMediaPath(testDir)
	if err := os.RemoveAll(mediaFull); err != nil {
		return err
	}

	if err := writeFile(testPath(parentFile), "# jobs-parent.md\n\ncontent\n"); err != nil {
		return err
	}
	if err := test.SeedMetadata(&files.Metadata{Path: withPrefix(parentFile), Editor: files.EditorTypeCodeMirror}); err != nil {
		return err
	}

	if err := writeFile(testPath(childFile), "# jobs-child.md\n\ncontent\n"); err != nil {
		return err
	}
	if err := test.SeedMetadataRaw(&files.Metadata{
		Path:    withPrefix(childFile),
		Editor:  files.EditorTypeCodeMirror,
		Parents: []string{withPrefix(parentFile)},
	}); err != nil {
		return err
	}

	if err := writeFile(testPath(linkedFile), "# jobs-linked.md\n\ncontent\n"); err != nil {
		return err
	}
	if err := test.SeedMetadata(&files.Metadata{Path: withPrefix(linkedFile), Editor: files.EditorTypeCodeMirror}); err != nil {
		return err
	}
	if err := writeFile(testPath(linkerFile), "# jobs-linker.md\n\nSee ["+linkedFile+"]("+testPath(linkedFile)+")\n"); err != nil {
		return err
	}
	if err := test.SeedMetadataRaw(&files.Metadata{Path: withPrefix(linkerFile), Editor: files.EditorTypeCodeMirror}); err != nil {
		return err
	}

	if err := writeFile(testPath(searchFile), "# jobs-search.md\n\n"+searchMarker+"\n"); err != nil {
		return err
	}
	if err := test.SeedMetadata(&files.Metadata{Path: withPrefix(searchFile), Editor: files.EditorTypeCodeMirror}); err != nil {
		return err
	}

	// used media is linked from a real doc file so it isn't orphaned
	if err := writeMediaFile(mediaPath(usedMediaFile), pngMagic); err != nil {
		return err
	}
	if err := test.SeedMetadata(&files.Metadata{Path: "media/" + mediaPath(usedMediaFile)}); err != nil {
		return err
	}
	usedMediaLinker := testPath("jobs-media-linker.md")
	if err := writeFile(usedMediaLinker, "# jobs-media-linker.md\n\n![img](media/"+mediaPath(usedMediaFile)+")\n"); err != nil {
		return err
	}
	if err := test.SeedMetadata(&files.Metadata{Path: withPrefix("jobs-media-linker.md"), Editor: files.EditorTypeCodeMirror}); err != nil {
		return err
	}
	if err := files.UpdateLinksForSingleFile(withPrefix("jobs-media-linker.md")); err != nil {
		return err
	}

	// orphan media has metadata (required for orphan detection, see
	// files.UpdateOrphanedMediaCache) but nothing links to it
	if err := writeMediaFile(mediaPath(orphanMediaFile), pngMagic); err != nil {
		return err
	}
	if err := test.SeedMetadata(&files.Metadata{Path: "media/" + mediaPath(orphanMediaFile)}); err != nil {
		return err
	}

	files.InvalidateFileListCache()
	return nil
}

func errCase(name string, err error) test.CaseResult {
	return test.CaseResult{Name: name, Success: false, Error: err.Error()}
}
