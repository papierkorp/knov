// Package render - chat component rendering
package render

import (
	"fmt"
	"strings"

	"knov/internal/chat"
	"knov/internal/configmanager"
	"knov/internal/translation"
)

// RenderChatComponent renders the full chat component (history + input)
func RenderChatComponent(messages []chat.Message, total, offset int, filePath string, short bool) string {
	var html strings.Builder

	filePathAttr := ""
	if filePath != "" {
		filePathAttr = fmt.Sprintf(` data-file-path="%s"`, filePath)
	}

	html.WriteString(fmt.Sprintf(`<div id="component-chat"%s>`, filePathAttr))

	// history — newest on top, load-more at bottom for older messages
	html.WriteString(`<div id="component-chat-history">`)
	for _, m := range messages {
		html.WriteString(renderMessage(m, short))
	}
	html.WriteString(renderLoadMoreButton(total, offset, len(messages), filePath, short))
	html.WriteString(`</div>`)

	// input
	inputURL := "/api/chat/messages"
	sep := "?"
	if filePath != "" {
		inputURL += sep + fmt.Sprintf(`file=%s`, filePath)
		sep = "&"
	}
	if short {
		inputURL += sep + "short=true"
	}
	fmt.Fprintf(&html, `<div id="component-chat-input">
	<textarea id="chat-input" name="chat-input" class="chat-textarea" placeholder="%s"
		hx-post="%s"
		hx-target="#component-chat-history"
		hx-swap="afterbegin"
		hx-trigger="keydown[key=='Enter'&&!shiftKey]"
		onkeydown="if(event.key==='Enter'&&!event.shiftKey)event.preventDefault()"
		hx-on--after-request="this.value=''"></textarea>
</div>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "type a message, enter to send"),
		inputURL)

	html.WriteString(`</div>`)
	return html.String()
}

// RenderChatLoadMore renders older messages + a new load-more button if needed.
// Replaces only the load-more button element (hx-swap outerHTML on the button div).
func RenderChatLoadMore(messages []chat.Message, total, offset int, filePath string, short bool) string {
	var html strings.Builder
	for _, m := range messages {
		html.WriteString(renderMessage(m, short))
	}
	html.WriteString(renderLoadMoreButton(total, offset, len(messages), filePath, short))
	return html.String()
}

func renderLoadMoreButton(total, offset, count int, filePath string, short bool) string {
	older := total - offset - count
	if older <= 0 {
		return ""
	}
	loadMoreURL := fmt.Sprintf(`/api/chat/messages?offset=%d`, offset+count)
	if filePath != "" {
		loadMoreURL += fmt.Sprintf(`&file=%s`, filePath)
	}
	if short {
		loadMoreURL += "&short=true"
	}
	return fmt.Sprintf(`<div class="chat-load-more" id="chat-load-more">
	<button class="btn-secondary"
		hx-get="%s"
		hx-target="#chat-load-more"
		hx-swap="outerHTML">↓ %s (%d)</button>
</div>`,
		loadMoreURL,
		translation.SprintfForRequest(configmanager.GetLanguage(), "load older messages"),
		older)
}

// RenderChatMessage renders a single message (used after POST)
func RenderChatMessage(m chat.Message, short bool) string {
	return renderMessage(m, short)
}

func renderMessage(m chat.Message, short bool) string {
	msgDivID := fmt.Sprintf("chat-message-%s", m.ID)

	if short {
		newFileURL := fmt.Sprintf(`/api/chat/messages/%s/move?mode=new&short=true`, m.ID)
		appendURL := fmt.Sprintf(`/api/chat/messages/%s/move?mode=append&short=true`, m.ID)
		deleteURL := fmt.Sprintf(`/api/chat/messages/%s`, m.ID)
		lang := configmanager.GetLanguage()
		return fmt.Sprintf(`<div class="chat-message chat-message-short" id="%s">
	<div class="chat-message-actions">
		<button class="btn-small btn-secondary"
			hx-get="%s" hx-target="#%s" hx-swap="outerHTML">%s</button>
		<button class="btn-small btn-secondary"
			hx-get="%s" hx-target="#%s" hx-swap="outerHTML">%s</button>
		<button class="btn-small btn-danger"
			hx-delete="%s" hx-target="#%s" hx-swap="outerHTML"
			hx-confirm="%s">%s</button>
	</div>
	<div class="chat-message-content">%s</div>
</div>`,
			msgDivID,
			newFileURL, msgDivID, translation.SprintfForRequest(lang, "to new file"),
			appendURL, msgDivID, translation.SprintfForRequest(lang, "append"),
			deleteURL, msgDivID,
			translation.SprintfForRequest(lang, "delete this message?"),
			translation.SprintfForRequest(lang, "delete"),
			m.Content)
	}

	newFileURL := fmt.Sprintf(`/api/chat/messages/%s/move?mode=new`, m.ID)
	appendURL := fmt.Sprintf(`/api/chat/messages/%s/move?mode=append`, m.ID)
	deleteURL := fmt.Sprintf(`/api/chat/messages/%s`, m.ID)
	timestamp := m.CreatedAt.Format("2006-01-02 15:04")
	lang := configmanager.GetLanguage()

	return fmt.Sprintf(`<div class="chat-message" id="%s">
	<div class="chat-message-actions">
		<button class="btn-small btn-secondary"
			hx-get="%s" hx-target="#%s" hx-swap="outerHTML">%s</button>
		<button class="btn-small btn-secondary"
			hx-get="%s" hx-target="#%s" hx-swap="outerHTML">%s</button>
		<button class="btn-small btn-danger"
			hx-delete="%s" hx-target="#%s" hx-swap="outerHTML"
			hx-confirm="%s">%s</button>
	</div>
	<div class="chat-message-content">%s</div>
	<div class="chat-message-meta">
		<span class="chat-timestamp">%s</span>
	</div>
</div>`,
		msgDivID,
		newFileURL, msgDivID, translation.SprintfForRequest(lang, "to new file"),
		appendURL, msgDivID, translation.SprintfForRequest(lang, "append"),
		deleteURL, msgDivID, translation.SprintfForRequest(lang, "delete this message?"), translation.SprintfForRequest(lang, "delete"),
		m.Content, timestamp)
}

// RenderChatNewFileForm renders the new-file move form
func RenderChatNewFileForm(m chat.Message) string {
	msgDivID := fmt.Sprintf("chat-message-%s", m.ID)
	moveURL := fmt.Sprintf(`/api/chat/messages/%s/move`, m.ID)
	cancelURL := fmt.Sprintf(`/api/chat/messages/%s`, m.ID)
	newInputID := fmt.Sprintf("chat-move-new-%s", m.ID)
	editorInputID := fmt.Sprintf("chat-move-editor-%s", m.ID)
	editorListID := fmt.Sprintf("chat-move-editors-%s", m.ID)
	lang := configmanager.GetLanguage()

	return fmt.Sprintf(`<div class="chat-message chat-message-moving" id="%s">
	<div class="chat-message-content">%s</div>
	<div class="chat-move-form">
		<input type="text" id="%s" name="target" class="form-input" placeholder="%s" autocomplete="off"/>
		<input type="text" id="%s" name="editor" class="form-input" placeholder="%s" list="%s" autocomplete="off"/>
		<datalist id="%s" hx-get="/api/metadata/editors?format=options&context=chat" hx-trigger="load" hx-target="this" hx-swap="innerHTML"></datalist>
		<div class="chat-move-actions">
			<button class="btn-small btn-primary"
				hx-post="%s"
				hx-include="#%s,#%s"
				hx-vals='{"mode":"new"}'
				hx-target="#%s"
				hx-swap="outerHTML">%s</button>
			<button class="btn-small btn-secondary"
				hx-get="%s" hx-target="#%s" hx-swap="outerHTML">%s</button>
		</div>
	</div>
</div>`,
		msgDivID, m.Content,
		newInputID, translation.SprintfForRequest(lang, "filename"),
		editorInputID, translation.SprintfForRequest(lang, "select editor type"), editorListID,
		editorListID,
		moveURL, newInputID, editorInputID, msgDivID,
		translation.SprintfForRequest(lang, "create"),
		cancelURL, msgDivID,
		translation.SprintfForRequest(lang, "cancel"))
}

// RenderChatAppendForm renders the append-to-existing-file move form
func RenderChatAppendForm(m chat.Message) string {
	msgDivID := fmt.Sprintf("chat-message-%s", m.ID)
	moveURL := fmt.Sprintf(`/api/chat/messages/%s/move`, m.ID)
	cancelURL := fmt.Sprintf(`/api/chat/messages/%s`, m.ID)
	appendInputID := fmt.Sprintf("chat-move-append-%s", m.ID)
	filesListID := fmt.Sprintf("chat-move-files-%s", m.ID)
	lang := configmanager.GetLanguage()

	return fmt.Sprintf(`<div class="chat-message chat-message-moving" id="%s">
	<div class="chat-message-content">%s</div>
	<div class="chat-move-form">
		<input type="text" id="%s" name="target" class="form-input" placeholder="%s" list="%s" autocomplete="off"/>
		<datalist id="%s" hx-get="/api/files/list?format=options" hx-trigger="load" hx-target="this" hx-swap="innerHTML"></datalist>
		<div class="chat-move-actions">
			<button class="btn-small btn-primary"
				hx-post="%s"
				hx-include="#%s"
				hx-vals='{"mode":"append"}'
				hx-target="#%s"
				hx-swap="outerHTML">%s</button>
			<button class="btn-small btn-secondary"
				hx-get="%s" hx-target="#%s" hx-swap="outerHTML">%s</button>
		</div>
	</div>
</div>`,
		msgDivID, m.Content,
		appendInputID, translation.SprintfForRequest(lang, "select file"), filesListID,
		filesListID,
		moveURL, appendInputID, msgDivID,
		translation.SprintfForRequest(lang, "append"),
		cancelURL, msgDivID,
		translation.SprintfForRequest(lang, "cancel"))
}

// RenderChatMoveSuccess renders a confirmation with a link to the target file
func RenderChatMoveSuccess(filePath string) string {
	return fmt.Sprintf(`<div class="chat-message chat-message-moved">
	<span>%s</span> <a href="/files/%s">%s</a>
</div>`,
		translation.SprintfForRequest(configmanager.GetLanguage(), "moved to"),
		filePath, filePath)
}
