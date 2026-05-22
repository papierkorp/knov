package server

import (
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"knov/internal/chat"
	"knov/internal/configmanager"
	"knov/internal/contentStorage"
	"knov/internal/files"
	"knov/internal/logging"
	"knov/internal/pathutils"
	"knov/internal/server/render"
	"knov/internal/translation"

	"github.com/go-chi/chi/v5"
)

// @Summary Get chat component HTML
// @Description Returns the full chat component with message history and input
// @Tags chat
// @Param file query string false "File path to scope chat to (empty = global chat)"
// @Param offset query int false "Pagination offset (default 0)"
// @Produce json,html
// @Router /api/chat/messages [get]
func handleAPIGetChat(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("file")
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	messages, total, err := chat.GetPage(filePath, offset)
	if err != nil {
		logging.LogError("failed to get chat messages: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to load chat"), http.StatusInternalServerError)
		return
	}

	var html string
	if offset > 0 {
		html = render.RenderChatLoadMore(messages, total, offset, filePath)
	} else {
		html = render.RenderChatComponent(messages, total, offset, filePath)
	}

	writeResponse(w, r, messages, html)
}

// @Summary Post a new chat message
// @Description Creates a new message in the global or file-scoped chat
// @Tags chat
// @Accept application/x-www-form-urlencoded
// @Param file query string false "File path to scope message to (empty = global chat)"
// @Param chat-input formData string true "Message content"
// @Produce json,html
// @Router /api/chat/messages [post]
func handleAPIPostChatMessage(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form"), http.StatusBadRequest)
		return
	}

	content := strings.TrimSpace(r.FormValue("chat-input"))
	if content == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "message cannot be empty"), http.StatusBadRequest)
		return
	}

	filePath := r.URL.Query().Get("file")

	msg, err := chat.Add(content, filePath)
	if err != nil {
		logging.LogError("failed to add chat message: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to save message"), http.StatusInternalServerError)
		return
	}

	logging.LogDebug("added chat message: %s", msg.ID)
	writeResponse(w, r, msg, render.RenderChatMessage(*msg))
}

// @Summary Delete a chat message
// @Tags chat
// @Param id path string true "Message ID"
// @Produce json,html
// @Success 200 {string} string "empty — element removed via hx-swap outerHTML"
// @Router /api/chat/messages/{id} [delete]
func handleAPIDeleteChatMessage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := chat.Delete(id); err != nil {
		logging.LogError("failed to delete chat message %s: %v", id, err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to delete message"), http.StatusInternalServerError)
		return
	}

	logging.LogDebug("deleted chat message: %s", id)
	writeResponse(w, r, map[string]string{"id": id}, "")
}

// @Summary Get a single chat message
// @Description Used to restore the message element after cancelling the move form
// @Tags chat
// @Param id path string true "Message ID"
// @Produce json,html
// @Router /api/chat/messages/{id} [get]
func handleAPIGetChatByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	msg, err := chat.GetByID(id)
	if err != nil || msg == nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "message not found"), http.StatusNotFound)
		return
	}

	writeResponse(w, r, msg, render.RenderChatMessage(*msg))
}

// @Summary Get move form for a chat message
// @Description Returns either the new-file form or append form depending on mode
// @Tags chat
// @Param id path string true "Message ID"
// @Param mode query string true "Form mode: new or append"
// @Produce json,html
// @Router /api/chat/messages/{id}/move [get]
func handleAPIGetChatMoveForm(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	mode := r.URL.Query().Get("mode")

	msg, err := chat.GetByID(id)
	if err != nil || msg == nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "message not found"), http.StatusNotFound)
		return
	}

	var html string
	if mode == "append" {
		html = render.RenderChatAppendForm(*msg)
	} else {
		html = render.RenderChatNewFileForm(*msg)
	}

	writeResponse(w, r, msg, html)
}

// @Summary Move a chat message to a file
// @Description Creates a new file or appends to an existing one, then deletes the message
// @Tags chat
// @Accept application/x-www-form-urlencoded
// @Param id path string true "Message ID"
// @Param mode formData string true "Mode: new or append"
// @Param target formData string true "Target filename (new) or existing file path (append)"
// @Param editor formData string false "Editor type for new files (e.g. markdown-editor, todo-editor)"
// @Produce json,html
// @Router /api/chat/messages/{id}/move [post]
func handleAPIMoveChatMessage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := r.ParseForm(); err != nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to parse form"), http.StatusBadRequest)
		return
	}

	target := strings.TrimSpace(r.FormValue("target"))
	mode := r.FormValue("mode")

	if target == "" {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "target is required"), http.StatusBadRequest)
		return
	}

	msg, err := chat.GetByID(id)
	if err != nil || msg == nil {
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "message not found"), http.StatusNotFound)
		return
	}

	var newContent []byte
	var fullPath string

	if mode == "append" {
		if !strings.Contains(target, ".") {
			target = target + ".md"
		}
		fullPath = pathutils.ToDocsPath(target)
		existing, _ := contentStorage.ReadFile(fullPath)
		if len(existing) > 0 {
			newContent = append(existing, []byte("\n\n"+msg.Content)...)
		} else {
			newContent = []byte(msg.Content)
		}
		// initialize metadata if file is new
		if existingMeta, _ := files.MetaDataGet(pathutils.ToWithPrefix(target)); existingMeta == nil {
			metadata := &files.Metadata{
				Path:   pathutils.ToWithPrefix(target),
				Editor: files.EditorFromExtension(target),
			}
			if err := files.MetaDataSave(metadata); err != nil {
				logging.LogWarning("failed to save metadata after append: %v", err)
			}
		}
	} else {
		editor := files.EditorType(r.FormValue("editor"))
		var resolvedEditor files.EditorType
		target, newContent, resolvedEditor = formatForEditor(target, msg.Content, editor)
		fullPath = pathutils.ToDocsPath(target)
		metadata := &files.Metadata{
			Path:   pathutils.ToWithPrefix(target),
			Editor: resolvedEditor,
		}
		if err := files.MetaDataSave(metadata); err != nil {
			logging.LogWarning("failed to save metadata for moved chat message: %v", err)
		}
	}

	if err := contentStorage.WriteFile(fullPath, newContent, 0644); err != nil {
		logging.LogError("failed to write file during chat move: %v", err)
		http.Error(w, translation.SprintfForRequest(configmanager.GetLanguage(), "failed to move message"), http.StatusInternalServerError)
		return
	}

	if err := chat.Delete(id); err != nil {
		logging.LogError("failed to delete message after move: %v", err)
	}

	logging.LogInfo("moved chat message %s to %s (mode: %s)", id, target, mode)
	writeResponse(w, r, map[string]string{"target": target}, render.RenderChatMoveSuccess(target))
}

func formatForEditor(target, content string, editor files.EditorType) (string, []byte, files.EditorType) {
	switch editor {
	case files.EditorTypeTodo:
		target = strings.TrimSuffix(target, filepath.Ext(target)) + ".todo"
		return target, []byte("- [ ] " + content + "\n"), files.EditorTypeTodo
	case files.EditorTypeList:
		target = strings.TrimSuffix(target, filepath.Ext(target)) + ".list"
		return target, []byte("- " + content + "\n"), files.EditorTypeList
	case files.EditorTypeIndex:
		target = strings.TrimSuffix(target, filepath.Ext(target)) + ".index"
		return target, []byte("## " + content + "\n"), files.EditorTypeIndex
	case files.EditorTypeTextarea:
		target = strings.TrimSuffix(target, filepath.Ext(target)) + ".txt"
		return target, []byte(content), files.EditorTypeTextarea
	default: // markdown-editor and anything else
		if !strings.Contains(target, ".") {
			target = target + ".md"
		}
		return target, []byte(content), files.EditorTypeMarkdown
	}
}
