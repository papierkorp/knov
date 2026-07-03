package editorstest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"knov/internal/files"
	"knov/internal/pathutils"
	"knov/internal/test"
)

// renameCase seeds a target file plus a referencer linking to it, moves the target on disk
// (os.Rename, same as handleAPIRenameFile / handleAPIMoveFolderFile), then calls the same
// files.UpdateLinksForMovedFile the handlers use and verifies the referencer's markdown
// link was rewritten to the new path.
func renameCase(name, oldRel, newRel string) test.CaseResult {
	referencer := testPath(name + "-referencer.md")

	if err := writeFile(oldRel, "# rename/move target\n"); err != nil {
		return errCase(name, err)
	}
	if err := saveMetadata(oldRel, files.EditorTypeToastUI); err != nil {
		return errCase(name, err)
	}

	if err := writeFile(referencer, fmt.Sprintf("[link](%s)\n", oldRel)); err != nil {
		return errCase(name, err)
	}
	if err := saveMetadata(referencer, files.EditorTypeToastUI); err != nil {
		return errCase(name, err)
	}

	// full link rebuild so the target's LinksToHere is populated before the move -
	// UpdateLinksForSingleFile only updates a file's own outbound links, not who points at it.
	if err := files.MetaDataLinksRebuild(); err != nil {
		return errCase(name, err)
	}

	oldFull := pathutils.ToDocsPath(oldRel)
	newFull := pathutils.ToDocsPath(newRel)
	if err := os.MkdirAll(filepath.Dir(newFull), 0755); err != nil {
		return errCase(name, err)
	}
	if err := os.Rename(oldFull, newFull); err != nil {
		return errCase(name, err)
	}
	if err := files.UpdateLinksForMovedFile(oldRel, newRel); err != nil {
		return errCase(name, err)
	}

	got, err := readFile(referencer)
	if err != nil {
		return errCase(name, err)
	}

	success := strings.Contains(got, newRel) && !strings.Contains(got, oldRel)
	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("referencer link points at %s", newRel),
		Actual:   got,
		Success:  success,
	}
	if !success {
		cr.Error = "referencing file's link was not rewritten to the new path"
	}
	return cr
}

// caseFileRename covers a same-directory rename (handleAPIRenameFile).
func caseFileRename() test.CaseResult {
	return renameCase("file-rename", testPath("rename-target.md"), testPath("rename-target-renamed.md"))
}

// caseFileMove covers a cross-directory move (handleAPIMoveFolderFile's per-file link update).
func caseFileMove() test.CaseResult {
	return renameCase("file-move", testPath("move-src/movefile.md"), testPath("move-dst/movefile.md"))
}
