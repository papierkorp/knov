package thememanager

import (
	"html/template"
	"net/http"
	"path/filepath"
	"strings"

	"knov/internal/configmanager"
)

const systemPageContent = `{{ define "content" }}
<div class="system-page">
    <h1 class="system-page-title">{{ .SystemTitle }}</h1>
    <div class="system-page-content">{{ .Content }}</div>
</div>
{{ end }}`

// SystemPageData is the template data for all /system/* pages.
type SystemPageData struct {
	BaseTemplateData
	SystemTitle string
	Content     template.HTML
}

// RenderSystemPage renders an app-controlled system page using the current theme's base
func (tm *ThemeManager) RenderSystemPage(w http.ResponseWriter, title string, content template.HTML) error {
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

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return err
	}

	_, err = w.Write([]byte(buf.String()))
	return err
}
