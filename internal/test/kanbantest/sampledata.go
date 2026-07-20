// Package kanbantest - sample folder/card helpers
package kanbantest

import (
	"os"
	"path/filepath"
	"time"

	"knov/internal/configmanager"
	"knov/internal/contentStorage"
	"knov/internal/files"
	"knov/internal/kanban"
	"knov/internal/pathutils"
	"knov/internal/test"
)

// testDir is the docs-relative sample folder every case seeds into, wiped at the start of
// each run so cases never see stale state from a previous run. It also doubles as the kanban
// board's folder scope (kanban boards are now scoped by folder path, not by top-level collection).
const testDir = "test/kanban-tests"

const testFolder = testDir

const (
	alphaFile   = "kanban-alpha.md"   // inbox, older
	betaFile    = "kanban-beta.md"    // inbox, newer
	gammaFile   = "kanban-gamma.md"   // inprogress
	deltaFile   = "kanban-delta.md"   // blocked
	moveFile    = "kanban-move.md"    // unstatused, for the move case
	excerptFile = "kanban-excerpt.md" // for the excerpt case
)

const gammaExtraTag = "kanbantest-marker"

// alpha sorts first alphabetically but is seeded with the later CreatedAt, and beta the
// earlier one, so SortCreatedAt (oldest first) and SortAlphabetical disagree on order -
// proving each sort mode is actually applied rather than one happening to match by luck.
var (
	alphaCreatedAt = time.Date(2020, 1, 10, 0, 0, 0, 0, time.UTC)
	betaCreatedAt  = time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)
)

func testPath(name string) string {
	return filepath.Join(testDir, name)
}

func writeFile(relPath, content string) error {
	full := pathutils.ToDocsPath(relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		return err
	}
	return contentStorage.WriteFile(full, []byte(content), 0644)
}

// writeCard seeds a file with the given tags and pins CreatedAt to a fixed value (via
// MetaDataSave then a MetaDataGet/MetaDataSaveRaw round-trip - MetaDataSave always stamps
// LastEdited/CreatedAt from the save call itself, so a directly-set CreatedAt only sticks
// through the raw save) so sort-order cases don't depend on wall-clock timing.
func writeCard(relPath, title string, tags []string, createdAt time.Time) error {
	if err := writeFile(relPath, "# "+title+"\n"); err != nil {
		return err
	}
	if err := files.MetaDataSave(&files.Metadata{
		Path:   pathutils.ToWithPrefix(relPath),
		Editor: files.EditorTypeToastUI,
		Tags:   tags,
	}); err != nil {
		return err
	}

	meta, err := files.MetaDataGet(pathutils.ToWithPrefix(relPath))
	if err != nil {
		return err
	}
	meta.CreatedAt = createdAt
	return files.MetaDataSaveRaw(meta)
}

func kanbanTag(status string) string {
	return configmanager.KanbanStatusTag(status)
}

// clearKanbanStatus strips any kanban status tag from a file's metadata via a raw save,
// bypassing metaDataUpdate's "empty Tags means unchanged" merge semantics.
func clearKanbanStatus(relPath string) error {
	meta, err := files.MetaDataGet(pathutils.ToWithPrefix(relPath))
	if err != nil {
		return err
	}
	filtered := meta.Tags[:0:0]
	for _, t := range meta.Tags {
		if !configmanager.IsKanbanTag(t) {
			filtered = append(filtered, t)
		}
	}
	meta.Tags = filtered
	return files.MetaDataSaveRaw(meta)
}

// resetAndSeed wipes the sample folder, resets its kanban card order (kanban-order/<folder>,
// config-store backed - not touched by wiping the folder), and seeds a fixed set of cards
// across statuses plus one unstatused card for the move case.
func resetAndSeed() error {
	full := pathutils.ToDocsPath(testDir)
	if err := os.RemoveAll(full); err != nil {
		return err
	}
	if err := os.MkdirAll(full, 0755); err != nil {
		return err
	}

	if err := kanban.SaveOrder(testFolder, kanban.Order{}); err != nil {
		return err
	}

	if err := writeCard(testPath(alphaFile), "Alpha Card", []string{kanbanTag("inbox")}, alphaCreatedAt); err != nil {
		return err
	}
	if err := writeCard(testPath(betaFile), "Beta Card", []string{kanbanTag("inbox")}, betaCreatedAt); err != nil {
		return err
	}
	if err := writeCard(testPath(gammaFile), "Gamma Card", []string{kanbanTag("inprogress"), gammaExtraTag}, time.Now()); err != nil {
		return err
	}
	if err := writeCard(testPath(deltaFile), "Delta Card", []string{kanbanTag("blocked")}, time.Now()); err != nil {
		return err
	}
	if err := writeCard(testPath(moveFile), "Move Card", nil, time.Now()); err != nil {
		return err
	}
	// writeCard's tags=nil above leaves any pre-existing kanban status tag untouched -
	// MetaDataSave only overwrites Tags when the new value is non-empty (metaDataUpdate
	// treats an empty Tags field as "not specified", not "clear it"). caseMoveCard needs
	// moveFile to genuinely start unstatused even after a previous run already moved it,
	// so strip any kanban tag directly via MetaDataSaveRaw, mirroring MoveCard's own approach.
	if err := clearKanbanStatus(testPath(moveFile)); err != nil {
		return err
	}
	if err := writeFile(testPath(excerptFile), "---\ntitle: excerpt\n---\n# Excerpt Card\n\nThis is the **excerpt** body text.\n"); err != nil {
		return err
	}
	if err := files.MetaDataSave(&files.Metadata{
		Path:   pathutils.ToWithPrefix(testPath(excerptFile)),
		Editor: files.EditorTypeToastUI,
	}); err != nil {
		return err
	}

	files.InvalidateFileListCache()
	return nil
}

func errCase(name string, err error) test.CaseResult {
	return test.CaseResult{Name: name, Success: false, Error: err.Error()}
}
