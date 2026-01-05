// Package dashboard - Widget operations
package dashboard

import (
	"knov/internal/types"
)

// WidgetPosition represents widget position on dashboard
type WidgetPosition struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// Widget represents a dashboard widget
type Widget struct {
	ID       string         `json:"id"`
	Type     WidgetType     `json:"type"`
	Title    string         `json:"title"`
	Position WidgetPosition `json:"position"`
	Config   WidgetConfig   `json:"config"`
}

// WidgetType represents widget types
type WidgetType string

const (
	WidgetTypeFilter        WidgetType = "filter"
	WidgetTypeFilterForm    WidgetType = "filterForm"
	WidgetTypeFileContent   WidgetType = "fileContent"
	WidgetTypeStatic        WidgetType = "static"
	WidgetTypeTags          WidgetType = "tags"
	WidgetTypeCollections   WidgetType = "collections"
	WidgetTypeFolders       WidgetType = "folders"
	WidgetTypeParaProjects  WidgetType = "para_projects"
	WidgetTypeParaAreas     WidgetType = "para_areas"
	WidgetTypeParaResources WidgetType = "para_resources"
	WidgetTypeParaArchive   WidgetType = "para_archive"
)

// FilterConfig represents filter configuration for widgets
type FilterConfig struct {
	Criteria []types.Criteria `json:"criteria"`
	Logic    string           `json:"logic"`
	Display  string           `json:"display"` // list, cards, dropdown, content
	Limit    int              `json:"limit"`
}

// StaticConfig represents static content configuration
type StaticConfig struct {
	Content string `json:"content"`
	Format  string `json:"format"` // html, markdown, text
}

// FileContentConfig represents file content configuration
type FileContentConfig struct {
	FilePath string `json:"filePath"`
}

// WidgetConfig represents widget-specific configuration
type WidgetConfig struct {
	Filter      *FilterConfig      `json:"filter,omitempty"`
	Static      *StaticConfig      `json:"static,omitempty"`
	FileContent *FileContentConfig `json:"fileContent,omitempty"`
}
