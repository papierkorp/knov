package thememanager

import (
	htmltemplate "html/template"
	"net/http"
	"path/filepath"
	"strings"
	"text/template"

	"knov/internal/configmanager"
)

const systemPageContent = `{{ define "content" }}
<div class="system-page">
    <h1 class="system-page-title">{{T .Data.SystemTitle}}</h1>
    <div class="system-page-content">{{ .Data.Content }}</div>
</div>
{{ end }}`

// SystemData is the page payload for all /system/* pages.
type SystemData struct {
	SystemTitle string            // page title shown in the system-page header (translated via T)
	Content     htmltemplate.HTML // pre-rendered HTML body for the system page
}

// RenderSystemPage renders an app-controlled system page using the current theme's base
func (tm *ThemeManager) RenderSystemPage(w http.ResponseWriter, title string, content htmltemplate.HTML) error {
	currentTheme := tm.GetCurrentTheme()
	themesDir := configmanager.GetThemesPath()
	baseFilePath := filepath.Join(themesDir, currentTheme.Name, "base.gohtml")

	funcMap := CreateFuncMap()
	tmpl, err := template.New("base.gohtml").Funcs(funcMap).ParseFiles(baseFilePath)
	if err != nil {
		return err
	}
	tmpl, err = tmpl.Parse(systemPageContent)
	if err != nil {
		return err
	}

	data := newPageData(title, SystemData{
		SystemTitle: title,
		Content:     content,
	})
	data.SystemPage = true

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return err
	}

	html := injectDefaultCSS(buf.String())
	html = injectDefaultJS(html)

	_, err = w.Write([]byte(html))
	return err
}
