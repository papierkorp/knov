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
    <h1 class="system-page-title">{{T .SystemTitle}}</h1>
    <div class="system-page-content">{{ .Content }}</div>
</div>
{{ end }}`

// SystemPageData is the template data for all /system/* pages.
type SystemPageData struct {
	BaseTemplateData
	SystemTitle string
	Content     htmltemplate.HTML
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

	data := SystemPageData{
		BaseTemplateData: NewBaseTemplateData(title),
		SystemTitle:      title,
		Content:          content,
	}
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
