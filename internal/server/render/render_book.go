package render

import (
	"fmt"
	"html/template"

	"knov/internal/book"
	"knov/internal/configmanager"
	"knov/internal/translation"
)

// RenderBookViewPrefix builds the banner prepended to a book in the file view: an info
// notice when it has no entries, otherwise the markdown/pdf export toolbar. Re-reads the
// `.book` for the empty check (small file, book view only); an unreadable one just falls
// through to the toolbar since the caller already rendered the composed body.
func RenderBookViewPrefix(bookPath string) string {
	if entries, err := book.Read(bookPath); err == nil && len(entries) == 0 {
		// a freshly created `.book` composes to nothing - avoid a blank-looking page
		return RenderStatusMessage(StatusInfo, translation.SprintfForRequest(configmanager.GetLanguage(),
			"this book has no entries yet - add a file reference in the editor to compose and export it."))
	}
	return bookExportToolbar(bookPath)
}

// bookExportToolbar renders the markdown/pdf export links. Both hit the generic file
// export endpoints, which compose the book by themselves (see files.IsBook).
func bookExportToolbar(bookPath string) string {
	lang := configmanager.GetLanguage()
	t := func(key string, args ...any) string {
		return translation.SprintfForRequest(lang, key, args...)
	}
	esc := template.URLQueryEscaper(bookPath)
	return fmt.Sprintf(`<div id="view-book-toolbar">`+
		`<a class="btn-secondary" href="/api/files/export/markdown?filepath=%s" download>%s</a>`+
		`<a class="btn-secondary" href="/api/files/export/pdf?filepath=%s" download>%s</a>`+
		`</div>`,
		esc, t("export composed markdown"), esc, t("export composed pdf"))
}
