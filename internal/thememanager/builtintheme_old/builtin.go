// Package thememanager ...
package thememanager

import (
	"path/filepath"

	"knov/internal/dashboard"
	"knov/internal/files"
	"knov/internal/logging"

	"github.com/a-h/templ"
)

// -----------------------------------------------------------------------------
// ----------------------------- Builtin Theme ---------------------------------
// -----------------------------------------------------------------------------

// BuiltinTheme implements the builtin theme
type BuiltinTheme struct{}

var builtinTheme = BuiltinTheme{}

var builtinMetadata = ThemeMetadata{
	AvailableFileViews:          []string{"detailed", "compact", "reader"},
	AvailableHomeViews:          []string{"default"},
	AvailableSearchViews:        []string{"default"},
	AvailableOverviewViews:      []string{"default"},
	AvailableDashboardViews:     []string{"default"},
	AvailableSettingsViews:      []string{"default"},
	AvailableAdminViews:         []string{"default"},
	AvailablePlaygroundViews:    []string{"default"},
	AvailableHistoryViews:       []string{"default"},
	AvailableLatestChangesViews: []string{"default"},
	AvailableBrowseFilesViews:   []string{"default"},
	SupportsDarkMode:            true,
	AvailableColorSchemes: []ColorScheme{
		{Name: "default", Label: "Default Blue"},
		{Name: "green", Label: "Forest Green"},
		{Name: "red", Label: "Ruby Red"},
		{Name: "purple", Label: "Royal Purple"},
	},
}

func (t *BuiltinTheme) Home(viewName string) (templ.Component, error) {
	return Home(), nil
}

func (t *BuiltinTheme) Settings(viewName string) (templ.Component, error) {
	return Settings(), nil
}

func (t *BuiltinTheme) Admin(viewName string) (templ.Component, error) {
	return Admin(), nil
}

func (t *BuiltinTheme) Playground(viewName string) (templ.Component, error) {
	return Playground(), nil
}

func (t *BuiltinTheme) LatestChanges(viewName string) (templ.Component, error) {
	return LatestChanges(), nil
}

func (t *BuiltinTheme) History(viewName string) (templ.Component, error) {
	return History(), nil
}

func (t *BuiltinTheme) Overview(viewName string) (templ.Component, error) {
	return Overview(), nil
}

func (t *BuiltinTheme) Search(viewName string, query string) (templ.Component, error) {
	return Search(query), nil
}

func (t *BuiltinTheme) RenderFileView(viewName string, fileContent *files.FileContent, filePath string) (templ.Component, error) {
	filename := filepath.Base(filePath)

	switch viewName {
	case "compact":
		return FileViewCompact(fileContent, filePath, filename), nil
	case "reader":
		return FileViewReader(fileContent, filePath, filename), nil
	default:
		return FileViewDetailed(fileContent, filePath, filename), nil
	}
}

func (t *BuiltinTheme) FileEdit(viewName string, content string, filePath string) (templ.Component, error) {
	filename := filepath.Base(filePath)
	return FileEdit(content, filePath, filename), nil
}

func (t *BuiltinTheme) Dashboard(viewName string, id string, action string, dash *dashboard.Dashboard) (templ.Component, error) {
	if action == "new" {
		return DashboardNew(), nil
	}

	if action == "edit" {
		if dash == nil {
			logging.LogWarning("dashboard not found: %s", id)
			return t.Home("default")
		}
		return DashboardEdit(dash), nil
	}

	if dash == nil {
		logging.LogWarning("dashboard not found: %s", id)
		return t.Home("default")
	}

	return Dashboard(dash), nil
}

func (t *BuiltinTheme) BrowseFiles(viewName string, metadataType string, value string, query string) (templ.Component, error) {
	return BrowseFiles(metadataType, value, query), nil
}
