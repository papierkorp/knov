package configmanager

import (
	"fmt"
	"time"

	"knov/internal/translation"
)

//nolint:gochecknoglobals
var (
	// ── Editor / ToastUI ─────────────────────────────────────────────────────
	ToastuiInitialView = register(&StringSetting{
		key: "toastuiInitialView", Default: "markdown",
		Section: SectionEditor, Group: GroupToastUI,
		Label:   "Default View",
		Desc:    "which editing mode opens when you first open a markdown file",
		Options: []SettingOption{{"markdown", "Markdown source"}, {"wysiwyg", "WYSIWYG"}},
	})
	ToastuiPreviewStyle = register(&StringSetting{
		key: "toastuiPreviewStyle", Default: "tab",
		Section: SectionEditor, Group: GroupToastUI,
		Label:   "Preview Layout",
		Desc:    "how the live preview is shown in markdown mode",
		Options: []SettingOption{{"tab", "Tab (switch between editor and preview)"}, {"vertical", "Vertical (side-by-side)"}},
	})
	ToastuiShowToolbar = register(&BoolSetting{
		key: "toastuiShowToolbar", Default: true,
		Section: SectionEditor, Group: GroupToastUI,
		Label: "Show Toolbar",
		Desc:  "display the formatting toolbar above the editor",
	})
	ToastuiShowModeSwitch = register(&BoolSetting{
		key: "toastuiShowModeSwitch", Default: true,
		Section: SectionEditor, Group: GroupToastUI,
		Label: "Show Mode Switch Bar",
		Desc:  "display the markdown / WYSIWYG switch tab at the bottom of the editor",
	})

	// ── Editor / Code / Text Editor (CodeMirror) ─────────────────────────────
	CodeMirrorVimMode = register(&BoolSetting{
		key: "codeMirrorVimMode", Default: false,
		Section: SectionEditor, Group: GroupCodeMirror,
		Label: "Vim Keybindings",
		Desc:  "enable vim normal / insert / visual mode in the code editor",
	})
	CodeMirrorLineNumbers = register(&BoolSetting{
		key: "codeMirrorLineNumbers", Default: true,
		Section: SectionEditor, Group: GroupCodeMirror,
		Label: "Line Numbers",
		Desc:  "show line numbers in the gutter",
	})
	CodeMirrorRelativeLineNumbers = register(&BoolSetting{
		key: "codeMirrorRelativeLineNumbers", Default: false,
		Section: SectionEditor, Group: GroupCodeMirror,
		Label: "Relative Line Numbers",
		Desc:  "show line numbers relative to the cursor — useful with vim mode",
	})
	CodeMirrorFoldGutter = register(&BoolSetting{
		key: "codeMirrorFoldGutter", Default: true,
		Section: SectionEditor, Group: GroupCodeMirror,
		Label: "Code Folding",
		Desc:  "show fold indicators in the gutter and enable folding keybindings",
	})
	CodeMirrorBracketMatching = register(&BoolSetting{
		key: "codeMirrorBracketMatching", Default: true,
		Section: SectionEditor, Group: GroupCodeMirror,
		Label: "Bracket Matching",
		Desc:  "highlight matching brackets, parentheses and braces",
	})
	CodeMirrorAutoBrackets = register(&BoolSetting{
		key: "codeMirrorAutoBrackets", Default: true,
		Section: SectionEditor, Group: GroupCodeMirror,
		Label: "Auto-Close Brackets",
		Desc:  "automatically insert closing brackets, parentheses and quotes",
	})
	CodeMirrorHighlightSelection = register(&BoolSetting{
		key: "codeMirrorHighlightSelection", Default: true,
		Section: SectionEditor, Group: GroupCodeMirror,
		Label: "Highlight Selection Matches",
		Desc:  "highlight all other occurrences of the currently selected text",
	})
	CodeMirrorHighlightSelectionWholeWord = register(&BoolSetting{
		key: "codeMirrorHighlightSelectionWholeWord", Default: true,
		Section: SectionEditor, Group: GroupCodeMirror,
		Label: "Whole-Word Matches Only",
		Desc:  "only highlight when the selection matches a complete word boundary",
	})

	// ── Editor / All Editors ──────────────────────────────────────────────────
	DefaultMarkdownEditor = register(&StringSetting{
		key: "defaultMarkdownEditor", Default: "toastui-editor",
		Section: SectionEditor, Group: GroupAllEditors,
		Label: "Default Markdown Editor",
		Desc:  "which editor opens by default for new and unassigned markdown files",
		Options: []SettingOption{
			{"toastui-editor", "ToastUI (rich markdown editor)"},
			{"codemirror-editor", "CodeMirror (plain text editor)"},
			{"textarea-editor", "Textarea (simple text area)"},
		},
	})
	SpellCheck = register(&BoolSetting{
		key: "spellCheck", Default: false,
		Section: SectionEditor, Group: GroupAllEditors,
		Label: "Spell Checking",
		Desc:  "let the browser underline misspelled words while editing",
	})
	WikiLinkCursorEnd = register(&BoolSetting{
		key: "wikiLinkCursorEnd", Default: false,
		Section: SectionEditor, Group: GroupAllEditors,
		Label: "Wiki Link Autocomplete: Jump Cursor Past ]]",
		Desc:  "when off, the cursor lands before ]] after autocomplete (between the path and the closing brackets)",
	})

	// ── Editor / Section Editing ──────────────────────────────────────────────
	SectionEditIncludeSubheaders = register(&BoolSetting{
		key: "sectionEditIncludeSubheaders", Default: false,
		Section: SectionEditor, Group: GroupSectionEditing,
		Label: "Include Sub-Headers When Editing Sections",
		Desc:  "when enabled, editing a section also selects content from all nested sub-level headers",
	})
	CodeBlockWrap = register(&BoolSetting{
		key: "codeBlockWrap", Default: false,
		Section: SectionEditor, Group: GroupSectionEditing,
		Label: "Wrap Long Lines in Code Blocks",
		Desc:  "when enabled, long lines in code blocks wrap instead of scrolling horizontally",
	})

	// ── Editor / File Extensions ──────────────────────────────────────────────
	UseExtensionTodo = register(&BoolSetting{
		key: "useExtensionTodo", Default: false,
		Section: SectionEditor, Group: GroupFileExtensions,
		Label: "Use .todo Extension",
		Desc:  "save todo files as .todo instead of .md",
	})
	UseExtensionList = register(&BoolSetting{
		key: "useExtensionList", Default: false,
		Section: SectionEditor, Group: GroupFileExtensions,
		Label: "Use .list Extension",
		Desc:  "save list files as .list instead of .md",
	})
	UseExtensionIndex = register(&BoolSetting{
		key: "useExtensionIndex", Default: false,
		Section: SectionEditor, Group: GroupFileExtensions,
		Label: "Use .index Extension",
		Desc:  "save index files as .index instead of .md",
	})

	// ── Table ─────────────────────────────────────────────────────────────────
	PageSize = register(&IntSetting{
		key: "pageSize", Default: 25,
		Section: SectionTable,
		Label:   "Rows Per Page",
		Desc:    "how many rows to show per page in interactive tables",
		Min:     intPtr(5), Max: intPtr(200),
		Trigger: "change delay:500ms",
	})
	ShowSearch = register(&BoolSetting{
		key: "showSearch", Default: true,
		Section: SectionTable,
		Label:   "Show Search Input",
		Desc:    "display a search box above interactive tables",
	})
	ShowInfo = register(&BoolSetting{
		key: "showInfo", Default: true,
		Section: SectionTable,
		Label:   "Show Row Count Info",
		Desc:    "display the 'showing X-Y of Z rows' summary line",
	})
	ShowPaging = register(&BoolSetting{
		key: "showPaging", Default: true,
		Section: SectionTable,
		Label:   "Show Pagination Buttons",
		Desc:    "display first / prev / next / last navigation buttons below the table",
	})

	// ── Media ─────────────────────────────────────────────────────────────────
	MaxUploadSizeMB = register(&IntSetting{
		key: "maxUploadSizeMB", Default: 10,
		Section: SectionMedia,
		Label:   "Max Upload Size (MB)",
		Desc:    "maximum file size allowed for media uploads in megabytes",
		Min:     intPtr(1), Max: intPtr(100),
		Trigger: "change delay:500ms",
	})
	AllowedMimeTypes = register(&StringSliceSetting{
		key: "allowedMimeTypes",
		Default: []string{
			"image/jpeg", "image/gif", "image/png", "image/webp",
			"image/vnd.microsoft.icon", "image/svg+xml",
			"audio/mpeg", "audio/ogg", "audio/wav",
			"video/webm", "video/ogg", "video/mp4",
			"application/pdf", "text/vtt",
		},
		Section: SectionMedia,
		Label:   "Allowed MIME Types",
		Desc:    "comma-separated MIME types accepted for upload (e.g. image/*, application/pdf)",
		Trigger: "change delay:1s",
	})
	EnablePreviews = register(&BoolSetting{
		key: "enablePreviews", Default: true,
		Section: SectionMedia, Group: GroupPreviewSettings,
		Label: "Enable Media Previews",
		Desc:  "show images and other media inline in file view with configurable sizing",
	})
	DefaultPreviewSize = register(&IntSetting{
		key: "defaultPreviewSize", Default: 300,
		Section: SectionMedia, Group: GroupPreviewSettings,
		Label: "Default Preview Size (px)",
		Desc:  "default max-width and max-height for inline media previews in pixels",
		Min:   intPtr(50), Max: intPtr(1000),
		Trigger: "change delay:500ms",
	})
	DisplayMode = register(&StringSetting{
		key: "displayMode", Default: "center",
		Section: SectionMedia, Group: GroupPreviewSettings,
		Label:   "Display Mode",
		Desc:    "how inline media previews are aligned in the document",
		Options: []SettingOption{{"left", "Left aligned"}, {"center", "Center aligned"}, {"right", "Right aligned"}, {"inline", "Inline with text"}},
	})
	BorderStyle = register(&StringSetting{
		key: "borderStyle", Default: "simple",
		Section: SectionMedia, Group: GroupPreviewSettings,
		Label:   "Border Style",
		Desc:    "visual border style applied to inline media previews",
		Options: []SettingOption{{"none", "No border"}, {"simple", "Simple border"}, {"rounded", "Rounded border"}, {"shadow", "Shadow border"}},
	})
	ShowCaption = register(&BoolSetting{
		key: "showCaption", Default: false,
		Section: SectionMedia, Group: GroupPreviewSettings,
		Label: "Show Captions",
		Desc:  "display the filename as a caption below each media preview",
	})
	ClickToEnlarge = register(&BoolSetting{
		key: "clickToEnlarge", Default: true,
		Section: SectionMedia, Group: GroupPreviewSettings,
		Label: "Click to Enlarge",
		Desc:  "make preview images clickable to open the full-size version",
	})

	// ── File Types / Editor Types ─────────────────────────────────────────────
	HideMarkdown = register(&BoolSetting{
		key: "hideMarkdown", Default: false,
		Section: SectionFileTypes, Group: GroupEditorTypes,
		Label: "Hide Markdown Files",
		Desc:  "exclude markdown files from file listings and browse views",
	})
	HideText = register(&BoolSetting{
		key: "hideText", Default: false,
		Section: SectionFileTypes, Group: GroupEditorTypes,
		Label: "Hide Text Files",
		Desc:  "exclude plain-text files from file listings and browse views",
	})
	HideList = register(&BoolSetting{
		key: "hideList", Default: false,
		Section: SectionFileTypes, Group: GroupEditorTypes,
		Label: "Hide List Files",
		Desc:  "exclude list files from file listings and browse views",
	})
	HideTodo = register(&BoolSetting{
		key: "hideTodo", Default: false,
		Section: SectionFileTypes, Group: GroupEditorTypes,
		Label: "Hide Todo Files",
		Desc:  "exclude todo files from file listings and browse views",
	})
	HideFilter = register(&BoolSetting{
		key: "hideFilter", Default: false,
		Section: SectionFileTypes, Group: GroupEditorTypes,
		Label: "Hide Filter Files",
		Desc:  "exclude filter files from file listings and browse views",
	})
	HideIndex = register(&BoolSetting{
		key: "hideIndex", Default: false,
		Section: SectionFileTypes, Group: GroupEditorTypes,
		Label: "Hide Index Files",
		Desc:  "exclude index files from file listings and browse views",
	})

	// ── File Types / Media Types ──────────────────────────────────────────────
	HideImage = register(&BoolSetting{
		key: "hideImage", Default: false,
		Section: SectionFileTypes, Group: GroupMediaTypes,
		Label: "Hide Image Files",
		Desc:  "exclude image files from file listings and browse views",
	})
	HideVideo = register(&BoolSetting{
		key: "hideVideo", Default: false,
		Section: SectionFileTypes, Group: GroupMediaTypes,
		Label: "Hide Video Files",
		Desc:  "exclude video files from file listings and browse views",
	})
	HidePDF = register(&BoolSetting{
		key: "hidePDF", Default: false,
		Section: SectionFileTypes, Group: GroupMediaTypes,
		Label: "Hide PDF Files",
		Desc:  "exclude PDF files from file listings and browse views",
	})
	HideOfficeDocuments = register(&BoolSetting{
		key: "hideOfficeDocuments", Default: false,
		Section: SectionFileTypes, Group: GroupMediaTypes,
		Label: "Hide Office Documents",
		Desc:  "exclude .docx, .xlsx, .pptx, .ods and similar files from listings",
	})
	HideArchives = register(&BoolSetting{
		key: "hideArchives", Default: false,
		Section: SectionFileTypes, Group: GroupMediaTypes,
		Label: "Hide Archives",
		Desc:  "exclude .zip, .rar, .7z and similar archives from file listings",
	})
	HideExecutables = register(&BoolSetting{
		key: "hideExecutables", Default: false,
		Section: SectionFileTypes, Group: GroupMediaTypes,
		Label: "Hide Executables",
		Desc:  "exclude .exe, .jar, .pfx and similar executable files from listings",
	})
	HideScripts = register(&BoolSetting{
		key: "hideScripts", Default: false,
		Section: SectionFileTypes, Group: GroupMediaTypes,
		Label: "Hide Scripts",
		Desc:  "exclude .sh, .bat and similar script files from file listings",
	})

	// ── Theme settings ────────────────────────────────────────────────────────
	// MapSetting: persisted but not renderable — mutated via SetThemeSetting.
	ThemeSettingsStore = register(&MapSetting[AllThemeSettings]{
		key:     "themeSettings",
		Default: make(AllThemeSettings),
	})

	// ── General ───────────────────────────────────────────────────────────────
	Theme = register(&StringSetting{
		key: "theme", Default: "builtin",
		Section: SectionGeneral, Group: GroupNone,
		Label:   "Theme",
		Desc:    "choose the visual appearance of the interface",
		DynURL:  "/api/themes/",
		Refresh: true,
	})

	Language = register(&StringSetting{
		key: "language", Default: "en",
		Section: SectionGeneral, Group: GroupNone,
		Label:   "Language",
		Desc:    "choose your preferred interface language",
		DynURL:  "/api/config/languages",
		Refresh: true,
		Validate: func(v string) error {
			for _, lang := range GetAvailableLanguages() {
				if lang.Code == v {
					return nil
				}
			}
			return fmt.Errorf("unsupported language %q", v)
		},
		OnChange: func(v interface{}) {
			if s, ok := v.(string); ok {
				translation.SetLanguage(s)
			}
		},
	})
	DateFormat = register(&StringSetting{
		key: "dateFormat", Default: "DD.MM.YYYY",
		Section: SectionGeneral, Group: GroupNone,
		Label: "Date Format",
		Desc:  "choose how dates are displayed throughout the app",
		Options: []SettingOption{
			{"DD.MM.YYYY", "DD.MM.YYYY (31.12.2026)"},
			{"YYYY-MM-DD", "YYYY-MM-DD (2026-12-31)"},
			{"MM/DD/YYYY", "MM/DD/YYYY (12/31/2026)"},
			{"DD/MM/YYYY", "DD/MM/YYYY (31/12/2026)"},
		},
	})
	Timezone = register(&StringSetting{
		key: "timezone", Default: time.Local.String(),
		Section: SectionGeneral, Group: GroupNone,
		Label: "Timezone",
		Desc:  "IANA timezone name for displaying timestamps (e.g. Local, Europe/Berlin, America/New_York)",
		Validate: func(v string) error {
			_, err := time.LoadLocation(v)
			return err
		},
	})
	ShowHiddenFiles = register(&BoolSetting{
		key: "showHiddenFiles", Default: false,
		Section: SectionGeneral, Group: GroupFiles,
		Label: "Show Hidden Files",
		Desc:  "show files and folders starting with a dot",
	})
	HomeDashboard = register(&StringSetting{
		key: "homeDashboard", Default: "home",
		Section: SectionGeneral, Group: GroupFiles,
		Label: "Home Dashboard",
		Desc:  "set a dashboard ID to use as the home page",
	})
)
