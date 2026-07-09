package chattest

import (
	"fmt"
	"strings"

	"knov/internal/chat"
	"knov/internal/contentStorage"
	"knov/internal/files"
	"knov/internal/pathutils"
	"knov/internal/test"
)

func caseAddGlobalMessage() test.CaseResult {
	name := "add-global-message"

	msg, err := chat.Add("chattest global message", "")
	if err != nil {
		return errCase(name, err)
	}
	defer chat.Delete(msg.ID)

	got, err := chat.GetByID(msg.ID)
	success := err == nil && got != nil && got.Content == "chattest global message" && got.FilePath == ""

	cr := test.CaseResult{
		Name:     name,
		Expected: `content="chattest global message" filePath=""`,
		Success:  success,
	}
	if got != nil {
		cr.Actual = fmt.Sprintf("content=%q filePath=%q", got.Content, got.FilePath)
	}
	if !success {
		cr.Error = "global message not stored/retrieved as expected"
	}
	return cr
}

func caseAddFileScopedMessage() test.CaseResult {
	name := "add-file-scoped-message"
	scopedPath := testPath(moveTargetFile)

	msg, err := chat.Add("chattest file-scoped message", scopedPath)
	if err != nil {
		return errCase(name, err)
	}
	defer chat.Delete(msg.ID)

	got, err := chat.GetByID(msg.ID)
	success := err == nil && got != nil && got.FilePath == scopedPath

	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("filePath=%q", scopedPath),
		Success:  success,
	}
	if got != nil {
		cr.Actual = fmt.Sprintf("filePath=%q", got.FilePath)
	}
	if !success {
		cr.Error = "file-scoped message did not retain its file path"
	}
	return cr
}

func caseDeleteMessage() test.CaseResult {
	name := "delete-message"

	msg, err := chat.Add("chattest to be deleted", "")
	if err != nil {
		return errCase(name, err)
	}

	if err := chat.Delete(msg.ID); err != nil {
		return errCase(name, err)
	}

	got, _ := chat.GetByID(msg.ID)
	success := got == nil

	cr := test.CaseResult{
		Name:     name,
		Expected: "message not found after delete",
		Actual:   fmt.Sprintf("message=%v", got),
		Success:  success,
	}
	if !success {
		cr.Error = "deleted message still retrievable"
	}
	return cr
}

// caseGetPagePagination adds more than chat.PageSize messages to a dedicated file-scoped
// path and checks that GetPage truncates to PageSize and the offset page returns the rest.
func caseGetPagePagination() test.CaseResult {
	name := "get-page-pagination"
	scopedPath := testPath(paginationFile)
	defer chat.DeleteForFile(scopedPath)

	const extra = 5
	total := chat.PageSize + extra
	for i := 0; i < total; i++ {
		if _, err := chat.Add(fmt.Sprintf("pagination message %d", i), scopedPath); err != nil {
			return errCase(name, err)
		}
	}

	firstPage, firstTotal, err := chat.GetPage(scopedPath, 0)
	if err != nil {
		return errCase(name, err)
	}
	secondPage, secondTotal, err := chat.GetPage(scopedPath, chat.PageSize)
	if err != nil {
		return errCase(name, err)
	}

	success := len(firstPage) == chat.PageSize && firstTotal == total &&
		len(secondPage) == extra && secondTotal == total

	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("offset=0 returns %d of %d total, offset=%d returns %d of %d total", chat.PageSize, total, chat.PageSize, extra, total),
		Actual:   fmt.Sprintf("offset=0 got %d/%d, offset=%d got %d/%d", len(firstPage), firstTotal, chat.PageSize, len(secondPage), secondTotal),
		Success:  success,
	}
	if !success {
		cr.Error = "GetPage did not truncate/paginate as expected"
	}
	return cr
}

// caseSingleMoveAppend mirrors handleAPIMoveChatMessage's append-mode branch: append the
// message content onto the existing target file, then delete the message.
func caseSingleMoveAppend() test.CaseResult {
	name := "single-move-append"
	targetPath := testPath(moveTargetFile)

	msg, err := chat.Add("chattest single append message", "")
	if err != nil {
		return errCase(name, err)
	}

	fullPath := pathutils.ToDocsPath(targetPath)
	existing, _ := contentStorage.ReadFile(fullPath)
	newContent := append(existing, []byte("\n\n"+msg.Content)...)
	if err := contentStorage.WriteFile(fullPath, newContent, 0644); err != nil {
		return errCase(name, err)
	}
	if err := chat.Delete(msg.ID); err != nil {
		return errCase(name, err)
	}

	got, err := readFile(targetPath)
	if err != nil {
		return errCase(name, err)
	}
	stillExists, _ := chat.GetByID(msg.ID)

	success := strings.Contains(got, "chattest single append message") && stillExists == nil
	cr := test.CaseResult{
		Name:     name,
		Expected: "target file contains appended message, message deleted",
		Actual:   fmt.Sprintf("content=%q messageStillExists=%v", got, stillExists != nil),
		Success:  success,
	}
	if !success {
		cr.Error = "single append move did not produce expected file content or cleanup"
	}
	return cr
}

// caseSingleMoveNewFile mirrors handleAPIMoveChatMessage's new-file branch with a todo
// editor target, using the same formatForEditor conversion the handler applies.
func caseSingleMoveNewFile() test.CaseResult {
	name := "single-move-new-file"

	msg, err := chat.Add("chattest single new-file message", "")
	if err != nil {
		return errCase(name, err)
	}

	target, newContent, resolvedEditor := formatForEditorReplica(testPath("chat-single-new"), msg.Content, files.EditorTypeTodo)
	fullPath := pathutils.ToDocsPath(target)
	if err := files.MetaDataSave(&files.Metadata{Path: pathutils.ToWithPrefix(target), Editor: resolvedEditor}); err != nil {
		return errCase(name, err)
	}
	if err := contentStorage.WriteFile(fullPath, newContent, 0644); err != nil {
		return errCase(name, err)
	}
	if err := chat.Delete(msg.ID); err != nil {
		return errCase(name, err)
	}

	got, err := readFile(target)
	if err != nil {
		return errCase(name, err)
	}
	meta, _ := files.MetaDataGet(pathutils.ToWithPrefix(target))
	stillExists, _ := chat.GetByID(msg.ID)

	success := strings.Contains(got, "- [ ] chattest single new-file message") &&
		strings.HasSuffix(target, ".todo") && meta != nil && meta.Editor == files.EditorTypeTodo &&
		stillExists == nil

	cr := test.CaseResult{
		Name:     name,
		Expected: "new .todo file with checkbox item, metadata editor=todo-editor, message deleted",
		Actual:   fmt.Sprintf("target=%q content=%q editor=%v messageStillExists=%v", target, got, metaEditor(meta), stillExists != nil),
		Success:  success,
	}
	if !success {
		cr.Error = "single new-file move did not produce expected file content, metadata, or cleanup"
	}
	return cr
}

// caseBulkMoveNewFile mirrors handleAPIBulkMoveChatMessages's new-file branch with a list
// editor target, concatenating several messages and converting them to list-item format.
func caseBulkMoveNewFile() test.CaseResult {
	name := "bulk-move-new-file"

	msg1, err := chat.Add("bulk new-file message one", "")
	if err != nil {
		return errCase(name, err)
	}
	msg2, err := chat.Add("bulk new-file message two", "")
	if err != nil {
		return errCase(name, err)
	}
	ids := []string{msg1.ID, msg2.ID}

	var parts []string
	for _, id := range ids {
		m, err := chat.GetByID(id)
		if err != nil || m == nil {
			return errCase(name, fmt.Errorf("message %s not found", id))
		}
		parts = append(parts, m.Content)
	}
	combined := strings.Join(parts, "\n\n")

	target, newContent, resolvedEditor := formatForEditorReplica(testPath("chat-bulk-new"), combined, files.EditorTypeList)
	fullPath := pathutils.ToDocsPath(target)
	if err := files.MetaDataSave(&files.Metadata{Path: pathutils.ToWithPrefix(target), Editor: resolvedEditor}); err != nil {
		return errCase(name, err)
	}
	if err := contentStorage.WriteFile(fullPath, newContent, 0644); err != nil {
		return errCase(name, err)
	}
	for _, id := range ids {
		if err := chat.Delete(id); err != nil {
			return errCase(name, err)
		}
	}

	got, err := readFile(target)
	if err != nil {
		return errCase(name, err)
	}
	stillExists := false
	for _, id := range ids {
		if m, _ := chat.GetByID(id); m != nil {
			stillExists = true
		}
	}

	success := strings.Contains(got, "- bulk new-file message one") &&
		strings.Contains(got, "- bulk new-file message two") &&
		strings.HasSuffix(target, ".list") && !stillExists

	cr := test.CaseResult{
		Name:     name,
		Expected: "new .list file with both messages as list items, messages deleted",
		Actual:   fmt.Sprintf("target=%q content=%q messagesStillExist=%v", target, got, stillExists),
		Success:  success,
	}
	if !success {
		cr.Error = "bulk new-file move did not produce expected file content or cleanup"
	}
	return cr
}

// caseBulkDelete mirrors handleAPIBulkDeleteChatMessages: delete each id in a comma-separated
// list, tolerating individually-missing ids.
func caseBulkDelete() test.CaseResult {
	name := "bulk-delete"
	scopedPath := testPath("chat-bulk-delete-scope.md")
	defer chat.DeleteForFile(scopedPath)

	msg1, err := chat.Add("bulk delete message one", scopedPath)
	if err != nil {
		return errCase(name, err)
	}
	msg2, err := chat.Add("bulk delete message two", scopedPath)
	if err != nil {
		return errCase(name, err)
	}
	ids := []string{msg1.ID, msg2.ID}

	deleted := 0
	for _, id := range ids {
		if err := chat.Delete(id); err != nil {
			continue
		}
		deleted++
	}

	_, total, err := chat.GetPage(scopedPath, 0)
	if err != nil {
		return errCase(name, err)
	}

	success := deleted == 2 && total == 0
	cr := test.CaseResult{
		Name:     name,
		Expected: "2 messages deleted, 0 remaining",
		Actual:   fmt.Sprintf("deleted=%d remaining=%d", deleted, total),
		Success:  success,
	}
	if !success {
		cr.Error = "bulk delete did not remove all messages"
	}
	return cr
}

// caseMoveFilePath exercises chat.MoveFilePath, used when a file is renamed/moved - messages
// should reattach from the old path to the new one.
func caseMoveFilePath() test.CaseResult {
	name := "move-file-path"
	oldPath := testPath(renameOldFile)
	newPath := testPath(renameNewFile)
	defer chat.DeleteForFile(oldPath)
	defer chat.DeleteForFile(newPath)

	if _, err := chat.Add("chattest rename message", oldPath); err != nil {
		return errCase(name, err)
	}

	if err := chat.MoveFilePath(oldPath, newPath); err != nil {
		return errCase(name, err)
	}

	_, oldTotal, err := chat.GetPage(oldPath, 0)
	if err != nil {
		return errCase(name, err)
	}
	newMessages, newTotal, err := chat.GetPage(newPath, 0)
	if err != nil {
		return errCase(name, err)
	}

	found := false
	for _, m := range newMessages {
		if m.Content == "chattest rename message" {
			found = true
		}
	}

	success := oldTotal == 0 && newTotal == 1 && found
	cr := test.CaseResult{
		Name:     name,
		Expected: "message reattached from old path to new path",
		Actual:   fmt.Sprintf("oldTotal=%d newTotal=%d found=%v", oldTotal, newTotal, found),
		Success:  success,
	}
	if !success {
		cr.Error = "MoveFilePath did not reattach the message as expected"
	}
	return cr
}

// caseDeleteForFile exercises chat.DeleteForFile, used when a file is deleted - all messages
// attached to that file path should be removed.
func caseDeleteForFile() test.CaseResult {
	name := "delete-for-file"
	scopedPath := testPath(deleteForFileTgt)

	if _, err := chat.Add("chattest delete-for-file message one", scopedPath); err != nil {
		return errCase(name, err)
	}
	if _, err := chat.Add("chattest delete-for-file message two", scopedPath); err != nil {
		return errCase(name, err)
	}

	if err := chat.DeleteForFile(scopedPath); err != nil {
		return errCase(name, err)
	}

	_, total, err := chat.GetPage(scopedPath, 0)
	if err != nil {
		return errCase(name, err)
	}

	success := total == 0
	cr := test.CaseResult{
		Name:     name,
		Expected: "0 messages remaining for file",
		Actual:   fmt.Sprintf("remaining=%d", total),
		Success:  success,
	}
	if !success {
		cr.Error = "DeleteForFile did not remove all messages attached to the file"
	}
	return cr
}

func metaEditor(m *files.Metadata) files.EditorType {
	if m == nil {
		return ""
	}
	return m.Editor
}
