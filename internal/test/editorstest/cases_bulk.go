package editorstest

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"knov/internal/chat"
	"knov/internal/contentStorage"
	"knov/internal/files"
	"knov/internal/filter"
	"knov/internal/pathutils"
	"knov/internal/test"
)

// caseBulkDeleteFiles mirrors handleAPIDeleteFilesBulk's tag-matching branch: find all files
// tagged with a marker tag and delete each (file + metadata).
func caseBulkDeleteFiles() test.CaseResult {
	name := "bulk-delete-files"
	tag := "edtest-bulk-delete-tag"
	paths := []string{testPath("bulk1.md"), testPath("bulk2.md"), testPath("bulk3.md")}

	for _, p := range paths {
		if err := writeFile(p, "# bulk delete fixture\n"); err != nil {
			return errCase(name, err)
		}
		if err := files.MetaDataSave(&files.Metadata{
			Path:   pathutils.ToWithPrefix(p),
			Editor: files.EditorTypeToastUI,
			Tags:   []string{tag},
		}); err != nil {
			return errCase(name, err)
		}
	}

	allFiles, err := files.GetAllFiles()
	if err != nil {
		return errCase(name, err)
	}

	deleted := 0
	for _, f := range allFiles {
		meta, err := files.MetaDataGet(f.Path)
		if err != nil || meta == nil || !slices.Contains(meta.Tags, tag) {
			continue
		}
		fullPath := pathutils.ToDocsPath(pathutils.ToRelative(f.Path))
		if err := os.Remove(fullPath); err != nil {
			continue
		}
		files.MetaDataDelete(f.Path)
		deleted++
	}

	stillExists := false
	for _, p := range paths {
		if _, err := os.Stat(pathutils.ToDocsPath(p)); err == nil {
			stillExists = true
		}
	}

	success := deleted == len(paths) && !stillExists
	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("%d files deleted, none remaining on disk", len(paths)),
		Actual:   fmt.Sprintf("%d files deleted, still on disk: %v", deleted, stillExists),
		Success:  success,
	}
	if !success {
		cr.Error = "bulk delete did not remove all tagged files"
	}
	return cr
}

// caseBulkMetadataPatch mirrors handleAPIBulkUpdateMetadata + applyBulkPatch's tag-add
// branch: filter files by tag, then add a new tag to each match via files.MetaDataSave.
func caseBulkMetadataPatch() test.CaseResult {
	name := "bulk-metadata-patch"
	paths := []string{testPath("patch1.md"), testPath("patch2.md")}

	for _, p := range paths {
		if err := writeFile(p, "# bulk patch fixture\n"); err != nil {
			return errCase(name, err)
		}
		if err := files.MetaDataSave(&files.Metadata{
			Path:   pathutils.ToWithPrefix(p),
			Editor: files.EditorTypeToastUI,
			Tags:   []string{"edtest-patch-before"},
		}); err != nil {
			return errCase(name, err)
		}
	}

	criteria := []filter.Criteria{{Metadata: "tags", Operator: "contains", Value: "edtest-patch-before", Action: "include"}}
	matched, err := filter.FilterFiles(criteria, "and")
	if err != nil {
		return errCase(name, err)
	}

	for _, f := range matched {
		if slices.Contains(f.Metadata.Tags, "edtest-patch-after") {
			continue
		}
		tags := append(append([]string{}, f.Metadata.Tags...), "edtest-patch-after")
		if err := files.MetaDataSave(&files.Metadata{Path: f.Metadata.Path, Tags: tags}); err != nil {
			return errCase(name, err)
		}
	}

	allTagged := true
	for _, p := range paths {
		meta, err := files.MetaDataGet(p)
		if err != nil || meta == nil || !slices.Contains(meta.Tags, "edtest-patch-after") {
			allTagged = false
		}
	}

	success := len(matched) == len(paths) && allTagged
	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("%d files matched and tagged edtest-patch-after", len(paths)),
		Actual:   fmt.Sprintf("%d files matched, all tagged=%v", len(matched), allTagged),
		Success:  success,
	}
	if !success {
		cr.Error = "bulk metadata patch did not apply to all matching files"
	}
	return cr
}

// caseBulkChatMoveDelete mirrors handleAPIBulkMoveChatMessages's append-mode branch
// (concatenate messages onto an existing file) followed by handleAPIBulkDeleteChatMessages.
func caseBulkChatMoveDelete() test.CaseResult {
	name := "bulk-chat-move-delete"
	targetPath := testPath("chat-target.md")

	if err := writeFile(targetPath, "# Chat target\n"); err != nil {
		return errCase(name, err)
	}
	if err := saveMetadata(targetPath, files.EditorTypeToastUI); err != nil {
		return errCase(name, err)
	}

	msg1, err := chat.Add("first message", "")
	if err != nil {
		return errCase(name, err)
	}
	msg2, err := chat.Add("second message", "")
	if err != nil {
		return errCase(name, err)
	}
	ids := []string{msg1.ID, msg2.ID}

	var parts []string
	for _, id := range ids {
		msg, err := chat.GetByID(id)
		if err != nil || msg == nil {
			return errCase(name, fmt.Errorf("message %s not found", id))
		}
		parts = append(parts, msg.Content)
	}
	combined := strings.Join(parts, "\n\n")

	fullPath := pathutils.ToDocsPath(targetPath)
	existing, _ := contentStorage.ReadFile(fullPath)
	newContent := append(existing, []byte("\n\n"+combined)...)
	if err := contentStorage.WriteFile(fullPath, newContent, 0644); err != nil {
		return errCase(name, err)
	}

	for _, id := range ids {
		if err := chat.Delete(id); err != nil {
			return errCase(name, err)
		}
	}

	got, err := readFile(targetPath)
	if err != nil {
		return errCase(name, err)
	}

	stillExists := false
	for _, id := range ids {
		if m, _ := chat.GetByID(id); m != nil {
			stillExists = true
		}
	}

	success := strings.Contains(got, "first message") && strings.Contains(got, "second message") && !stillExists
	cr := test.CaseResult{
		Name:     name,
		Expected: "target file contains both messages, messages deleted",
		Actual:   fmt.Sprintf("content=%q messagesStillExist=%v", got, stillExists),
		Success:  success,
	}
	if !success {
		cr.Error = "bulk chat move/delete did not produce expected file content or cleanup"
	}
	return cr
}
