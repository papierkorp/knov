package configmanager

// SettingSection is a top-level group shown as a <section> on the settings page.
type SettingSection struct {
	Key         string
	Label       string
	Description string
}

var (
	SectionGeneral   = SettingSection{Key: "general", Label: "General Settings"}
	SectionEditor    = SettingSection{Key: "editor", Label: "Editor Settings"}
	SectionTable     = SettingSection{Key: "table", Label: "Table Settings"}
	SectionMedia     = SettingSection{Key: "media", Label: "Media Upload Settings"}
	SectionFileTypes = SettingSection{Key: "file-types", Label: "File Type Visibility", Description: "Control which file types are visible in file listings and browsing"}
)

// AllSections returns sections in display order.
func AllSections() []SettingSection {
	return []SettingSection{SectionGeneral, SectionEditor, SectionTable, SectionMedia, SectionFileTypes}
}

// SettingGroup is a named sub-section rendered as <div class="setting-group"> inside a section.
type SettingGroup struct {
	Key         string
	Label       string
	Description string
}

var (
	GroupNone            = SettingGroup{}
	GroupFiles           = SettingGroup{Key: "files", Label: "Files"}
	GroupToastUI         = SettingGroup{Key: "toastui", Label: "ToastUI Editor"}
	GroupCodeMirror      = SettingGroup{Key: "code-mirror", Label: "Code / Text Editor (CodeMirror)"}
	GroupAllEditors      = SettingGroup{Key: "all-editors", Label: "All Editors"}
	GroupSectionEditing  = SettingGroup{Key: "section-editing", Label: "Section Editing & Display"}
	GroupFileExtensions  = SettingGroup{Key: "file-extensions", Label: "File Extensions", Description: "Use dedicated file extensions instead of .md for these editor types"}
	GroupPreviewSettings = SettingGroup{Key: "preview-settings", Label: "Preview Settings"}
	GroupEditorTypes     = SettingGroup{Key: "editor-types", Label: "Editor Types"}
	GroupMediaTypes      = SettingGroup{Key: "media-types", Label: "Media Types"}
)

// SettingOption is a single entry in a select input.
type SettingOption struct {
	Value string
	Label string // display text; falls back to Value when empty
}

// SettingsBySection returns renderable settings for a section in registration order.
func SettingsBySection(section SettingSection) []RenderableSetting {
	var out []RenderableSetting
	for _, s := range renderSettings {
		if s.GetMeta().Section == section {
			out = append(out, s)
		}
	}
	return out
}
