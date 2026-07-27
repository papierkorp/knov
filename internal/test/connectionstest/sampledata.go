// Package connectionstest - sample file/metadata helpers
package connectionstest

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
const testDir = "test/connections-tests"

const (
	parentFile  = "conn-parent.md" // root of one parent-child chain
	childFile   = "conn-child.md"  // Parents=[parentFile]
	grandchild  = "conn-grandchild.md"
	parent2File = "conn-parent2.md" // root of a second, independent chain
	child2File  = "conn-child2.md"  // Parents=[parent2File]

	linkerFile = "conn-linker.md" // real markdown link to linkedFile, for used-links
	linkedFile = "conn-linked.md" // target of linkerFile's link

	relatedFile = "conn-related.md" // Related seeded via raw save

	conflictOriginal = "conn-conflict-original.md"
	conflictCopy     = "conn-conflict-original.conflict.md"
)

func testPath(name string) string {
	return pathutils.ToSlash(filepath.Join(testDir, name))
}

func withPrefix(name string) string {
	return pathutils.ToWithPrefix(testPath(name))
}

func writeFile(relPath, content string) error {
	full := pathutils.ToDocsPath(relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		return err
	}
	return contentStorage.WriteFile(full, []byte(content), 0644)
}

func saveMetadata(name string, m *files.Metadata) error {
	m.Path = withPrefix(name)
	return files.MetaDataSave(m)
}

// resetAndSeed wipes the sample folder and seeds two independent parent->child(->grandchild)
// chains (real MetaDataSave calls, so Kids/Ancestor are computed by the actual cascade rather
// than faked), a linker/linked pair for used-links, and standalone files for related/conflict.
func resetAndSeed() error {
	full := pathutils.ToDocsPath(testDir)
	if err := os.RemoveAll(full); err != nil {
		return err
	}
	if err := os.MkdirAll(full, 0755); err != nil {
		return err
	}

	for _, name := range []string{
		parentFile, childFile, grandchild, parent2File, child2File,
		linkedFile, relatedFile, conflictOriginal, conflictCopy,
	} {
		if err := writeFile(testPath(name), "# "+name+"\n\ncontent\n"); err != nil {
			return err
		}
	}
	// the link href must be the full docs-relative path (no "docs/" prefix) - a bare
	// filename wouldn't resolve to linkedFile's actual metadata path when UsedLinks/
	// LinksToHere are computed below, since link hrefs aren't resolved relative to the
	// linking file's own folder.
	if err := writeFile(testPath(linkerFile), "# conn-linker.md\n\nSee ["+linkedFile+"]("+testPath(linkedFile)+")\n"); err != nil {
		return err
	}

	if err := saveMetadata(parentFile, &files.Metadata{Editor: files.EditorTypeToastUI}); err != nil {
		return err
	}
	if err := saveMetadata(childFile, &files.Metadata{Editor: files.EditorTypeToastUI, Parents: []string{withPrefix(parentFile)}}); err != nil {
		return err
	}
	if err := saveMetadata(grandchild, &files.Metadata{Editor: files.EditorTypeToastUI, Parents: []string{withPrefix(childFile)}}); err != nil {
		return err
	}
	if err := saveMetadata(parent2File, &files.Metadata{Editor: files.EditorTypeToastUI}); err != nil {
		return err
	}
	if err := saveMetadata(child2File, &files.Metadata{Editor: files.EditorTypeToastUI, Parents: []string{withPrefix(parent2File)}}); err != nil {
		return err
	}

	if err := saveMetadata(linkedFile, &files.Metadata{Editor: files.EditorTypeToastUI}); err != nil {
		return err
	}
	if err := saveMetadata(linkerFile, &files.Metadata{Editor: files.EditorTypeToastUI}); err != nil {
		return err
	}
	if err := files.UpdateLinksForSingleFile(withPrefix(linkerFile)); err != nil {
		return err
	}

	if err := saveMetadata(relatedFile, &files.Metadata{Editor: files.EditorTypeToastUI}); err != nil {
		return err
	}
	relatedMeta, err := files.MetaDataGet(withPrefix(relatedFile))
	if err != nil {
		return err
	}
	relatedMeta.Related = []string{withPrefix(childFile), withPrefix(grandchild), withPrefix(parent2File)}
	if err := files.MetaDataSaveRaw(relatedMeta); err != nil {
		return err
	}

	if err := saveMetadata(conflictOriginal, &files.Metadata{Editor: files.EditorTypeToastUI}); err != nil {
		return err
	}
	if err := saveMetadata(conflictCopy, &files.Metadata{Editor: files.EditorTypeToastUI}); err != nil {
		return err
	}

	files.InvalidateFileListCache()
	return nil
}

func errCase(name string, err error) test.CaseResult {
	return test.CaseResult{Name: name, Success: false, Error: err.Error()}
}
