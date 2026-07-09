// Package chattest - sample folder and per-case sample file/metadata helpers
package chattest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"knov/internal/chat"
	"knov/internal/contentStorage"
	"knov/internal/files"
	"knov/internal/pathutils"
	"knov/internal/test"
)

// testDir is the docs-relative sample folder every case seeds into, wiped at the start of
// each run so cases never see stale state from a previous run.
const testDir = "test/chat-tests"

const (
	moveTargetFile   = "chat-move-target.md"
	paginationFile   = "chat-pagination.md"
	renameOldFile    = "chat-rename-old.md"
	renameNewFile    = "chat-rename-new.md"
	deleteForFileTgt = "chat-delete-target.md"
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

func readFile(relPath string) (string, error) {
	full := pathutils.ToDocsPath(relPath)
	b, err := contentStorage.ReadFile(full)
	return string(b), err
}

func saveMetadata(relPath string, editor files.EditorType) error {
	return files.MetaDataSave(&files.Metadata{
		Path:   pathutils.ToWithPrefix(relPath),
		Editor: editor,
	})
}

// resetAndSeed clears the sample folder on disk, writes the append-move target file, and
// clears out any chat messages left over from a previous (possibly failed) run under the
// fixed file-scoped paths cases use - chat messages aren't removed by wiping testDir since
// they're keyed by path string in a separate store, not tied to the file's existence.
func resetAndSeed() error {
	full := pathutils.ToDocsPath(testDir)
	if err := os.RemoveAll(full); err != nil {
		return err
	}
	if err := os.MkdirAll(full, 0755); err != nil {
		return err
	}

	if err := writeFile(testPath(moveTargetFile), "# Chat target\n"); err != nil {
		return err
	}
	if err := saveMetadata(testPath(moveTargetFile), files.EditorTypeToastUI); err != nil {
		return err
	}

	for _, name := range []string{paginationFile, renameOldFile, renameNewFile, deleteForFileTgt} {
		if err := chat.DeleteForFile(testPath(name)); err != nil {
			return err
		}
	}

	return nil
}

func errCase(name string, err error) test.CaseResult {
	return test.CaseResult{Name: name, Success: false, Error: err.Error()}
}

// formatForEditorReplica mirrors internal/server/api_chat.go's unexported formatForEditor -
// converts concatenated message content into the target editor's on-disk format. Kept as its
// own copy since the original lives in package server and can't be imported here.
func formatForEditorReplica(target, content string, editor files.EditorType) (string, []byte, files.EditorType) {
	parts := strings.Split(content, "\n\n")

	switch editor {
	case files.EditorTypeTodo:
		target = strings.TrimSuffix(target, filepath.Ext(target)) + ".todo"
		var b strings.Builder
		for _, p := range parts {
			if strings.TrimSpace(p) != "" {
				fmt.Fprintf(&b, "- [ ] %s\n", strings.TrimSpace(p))
			}
		}
		return target, []byte(b.String()), files.EditorTypeTodo
	case files.EditorTypeList:
		target = strings.TrimSuffix(target, filepath.Ext(target)) + ".list"
		var b strings.Builder
		for _, p := range parts {
			if strings.TrimSpace(p) != "" {
				fmt.Fprintf(&b, "- %s\n", strings.TrimSpace(p))
			}
		}
		return target, []byte(b.String()), files.EditorTypeList
	case files.EditorTypeIndex:
		target = strings.TrimSuffix(target, filepath.Ext(target)) + ".index"
		var b strings.Builder
		for _, p := range parts {
			if strings.TrimSpace(p) != "" {
				fmt.Fprintf(&b, "## %s\n", strings.TrimSpace(p))
			}
		}
		return target, []byte(b.String()), files.EditorTypeIndex
	case files.EditorTypeTextarea:
		target = strings.TrimSuffix(target, filepath.Ext(target)) + ".txt"
		return target, []byte(content), files.EditorTypeTextarea
	default:
		if !strings.Contains(target, ".") {
			target = target + ".md"
		}
		return target, []byte(content), files.EditorTypeToastUI
	}
}
