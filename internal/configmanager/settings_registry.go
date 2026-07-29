package configmanager

import (
	"fmt"
	"time"

	"knov/internal/fonts"
	"knov/internal/translation"
	"knov/internal/utils"
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
	ShowColumnFilters = register(&BoolSetting{
		key: "showColumnFilters", Default: true,
		Section: SectionTable,
		Label:   "Show Column Filters",
		Desc:    "display a value-picker dropdown below column headers that have repeated values",
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

	ShowHiddenFiles = register(&BoolSetting{
		key: "showHiddenFiles", Default: false,
		Section: SectionFileTypes, Group: GroupNone,
		Label: "Show Hidden Files",
		Desc:  "show files and folders starting with a dot",
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
	HomeDashboard = register(&StringSetting{
		key: "homeDashboard", Default: "home",
		Section: SectionGeneral, Group: GroupNone,
		Label: "Home Dashboard",
		Desc:  "set a dashboard ID to use as the home page",
	})
	// ── Appearance ────────────────────────────────────────────────────────────
	Theme = register(&StringSetting{
		key: "theme", Default: "builtin",
		Section: SectionAppearance, Group: GroupNone,
		Label:   "Theme",
		Desc:    "choose the visual appearance of the interface",
		DynURL:  "/api/themes/",
		Refresh: true,
	})
	CustomCSS = register(&StringSetting{
		key: "customCSS", Default: "",
		Section: SectionAppearance, Group: GroupNone,
		Label:     "Custom CSS",
		Desc:      "additional CSS applied on top of the active theme, across all themes",
		Multiline: true,
	})
	FontBody = register(&StringSetting{
		key: "fontBody", Default: "Roboto",
		Section: SectionAppearance, Group: GroupTypography,
		Label:       "Font (Body)",
		Desc:        "base font used for body text across the app",
		Options:     fontOptionList(false),
		FontPreview: true,
	})
	FontHeadings = register(&StringSetting{
		key: "fontHeadings", Default: "Roboto",
		Section: SectionAppearance, Group: GroupTypography,
		Label:       "Font (Headings)",
		Desc:        "font used for headings across the app",
		Options:     fontOptionList(false),
		FontPreview: true,
	})
	FontH1 = register(&StringSetting{
		key: "fontH1", Default: "Roboto",
		Section: SectionAppearance, Group: GroupTypography,
		Label:       "Font (H1 Headings)",
		Desc:        "font used for level-1 headings across the app, overriding the general heading font",
		Options:     fontOptionList(false),
		FontPreview: true,
	})
	FontCode = register(&StringSetting{
		key: "fontCode", Default: "Roboto Mono",
		Section: SectionAppearance, Group: GroupTypography,
		Label:       "Font (Code)",
		Desc:        "font used for code blocks and inline code across the app (monospace fonts only, so aligned characters stay aligned)",
		Options:     fontOptionList(true),
		FontPreview: true,
	})

	// ── PDF Export ────────────────────────────────────────────────────────────
	PDFShowFileButton = register(&BoolSetting{
		key: "pdfShowFileButton", Default: true,
		Section: SectionPDFExport,
		Label:   "Show File Export Button",
		Desc:    "display the pdf export button in the file actions panel",
	})
	PDFShowHeaderButton = register(&BoolSetting{
		key: "pdfShowHeaderButton", Default: true,
		Section: SectionPDFExport,
		Label:   "Show Section Export Buttons",
		Desc:    "display the pdf export button next to each heading in a rendered file",
	})
	PDFPageBreakBeforeHeadings = register(&BoolSetting{
		key: "pdfPageBreakBeforeHeadings", Default: true,
		Section: SectionPDFExport,
		Label:   "Page Break Before Headings",
		Desc:    "start a new page before each top-level heading when exporting to pdf",
	})
	PDFUseTaskIcons = register(&BoolSetting{
		key: "pdfUseTaskIcons", Default: true,
		Section: SectionPDFExport,
		Label:   "Use Icons for Task Checkboxes",
		Desc:    "draw task-list checkboxes as status icons instead of plain text marks ([ ], [x], [-], [O]) when exporting to pdf",
	})
	PDFSyntaxHighlighting = register(&BoolSetting{
		key: "pdfSyntaxHighlighting", Default: true,
		Section: SectionPDFExport,
		Label:   "Syntax Highlighting",
		Desc:    "color-highlight code blocks by language when exporting to pdf",
	})
	PDFHeaderRule = register(&BoolSetting{
		key: "pdfHeaderRule", Default: false,
		Section: SectionPDFExport,
		Label:   "Show Rule Below Header",
	})
	PDFFooterRule = register(&BoolSetting{
		key: "pdfFooterRule", Default: false,
		Section: SectionPDFExport,
		Label:   "Show Rule Above Footer",
	})
	PDFPageFormat = register(&StringSetting{
		key: "pdfPageFormat", Default: "A4",
		Section: SectionPDFExport,
		Label:   "Page Format",
		Desc:    "paper size used when exporting to pdf",
		Options: []SettingOption{{"A3", "A3"}, {"A4", "A4"}, {"A5", "A5"}, {"Letter", "Letter"}, {"Legal", "Legal"}},
	})
	PDFOrientation = register(&StringSetting{
		key: "pdfOrientation", Default: "P",
		Section: SectionPDFExport,
		Label:   "Orientation",
		Desc:    "page orientation used when exporting to pdf",
		Options: []SettingOption{{"P", "Portrait"}, {"L", "Landscape"}},
	})
	PDFMarginMM = register(&IntSetting{
		key: "pdfMarginMM", Default: 20,
		Section: SectionPDFExport,
		Label:   "Margin (mm)",
		Desc:    "page margin in millimeters on every side when exporting to pdf",
		Min:     intPtr(0), Max: intPtr(50),
	})
	PDFFontOverall = register(&StringSetting{
		key: "pdfFontOverall", Default: "Roboto",
		Section:     SectionPDFExport,
		Label:       "Font (Overall)",
		Desc:        "base font used for body text when exporting to pdf, and the fallback for headings",
		Options:     fontOptionList(false),
		FontPreview: true,
	})
	PDFFontCodeBlock = register(&StringSetting{
		key: "pdfFontCodeBlock", Default: "Roboto Mono",
		Section:     SectionPDFExport,
		Label:       "Font (Code Blocks)",
		Desc:        "font used for code blocks and inline code when exporting to pdf (monospace fonts only, so wrapped code lines stay aligned)",
		Options:     fontOptionList(true),
		FontPreview: true,
	})
	PDFFontHeadings = register(&StringSetting{
		key: "pdfFontHeadings", Default: "Roboto",
		Section:     SectionPDFExport,
		Label:       "Font (Headings)",
		Desc:        "font used for headings when exporting to pdf",
		Options:     fontOptionList(false),
		FontPreview: true,
	})
	PDFFontH1 = register(&StringSetting{
		key: "pdfFontH1", Default: "Roboto",
		Section:     SectionPDFExport,
		Label:       "Font (H1 Headings)",
		Desc:        "font used for level-1 headings when exporting to pdf, overriding the general heading font",
		Options:     fontOptionList(false),
		FontPreview: true,
	})
	PDFFooterLeft = register(&StringSetting{
		key: "pdfFooterLeft", Default: "",
		Section: SectionPDFExport, Group: GroupPDFFooterLeft,
		Label: "Text",
	})
	PDFFooterLeftFont = register(&StringSetting{
		key: "pdfFooterLeftFont", Default: "Roboto",
		Section: SectionPDFExport, Group: GroupPDFFooterLeft,
		Label:       "Font",
		Options:     fontOptionList(false),
		FontPreview: true,
	})
	PDFFooterLeftColor = register(&StringSetting{
		key: "pdfFooterLeftColor", Default: "#000000",
		Section: SectionPDFExport, Group: GroupPDFFooterLeft,
		Label:    "Color",
		IsColor:  true,
		Validate: utils.ValidateHexColor,
	})
	PDFFooterLeftSize = register(&IntSetting{
		key: "pdfFooterLeftSize", Default: 8,
		Section: SectionPDFExport, Group: GroupPDFFooterLeft,
		Label: "Size",
		Desc:  "font size in points",
		Min:   intPtr(6), Max: intPtr(24),
	})
	PDFFooterLeftBold = register(&BoolSetting{
		key: "pdfFooterLeftBold", Default: false,
		Section: SectionPDFExport, Group: GroupPDFFooterLeft,
		Label: "Bold",
	})
	PDFFooterLeftItalic = register(&BoolSetting{
		key: "pdfFooterLeftItalic", Default: false,
		Section: SectionPDFExport, Group: GroupPDFFooterLeft,
		Label: "Italic",
	})
	PDFFooterCenter = register(&StringSetting{
		key: "pdfFooterCenter", Default: "",
		Section: SectionPDFExport, Group: GroupPDFFooterCenter,
		Label: "Text",
	})
	PDFFooterCenterFont = register(&StringSetting{
		key: "pdfFooterCenterFont", Default: "Roboto",
		Section: SectionPDFExport, Group: GroupPDFFooterCenter,
		Label:       "Font",
		Options:     fontOptionList(false),
		FontPreview: true,
	})
	PDFFooterCenterColor = register(&StringSetting{
		key: "pdfFooterCenterColor", Default: "#000000",
		Section: SectionPDFExport, Group: GroupPDFFooterCenter,
		Label:    "Color",
		IsColor:  true,
		Validate: utils.ValidateHexColor,
	})
	PDFFooterCenterSize = register(&IntSetting{
		key: "pdfFooterCenterSize", Default: 8,
		Section: SectionPDFExport, Group: GroupPDFFooterCenter,
		Label: "Size",
		Desc:  "font size in points",
		Min:   intPtr(6), Max: intPtr(24),
	})
	PDFFooterCenterBold = register(&BoolSetting{
		key: "pdfFooterCenterBold", Default: false,
		Section: SectionPDFExport, Group: GroupPDFFooterCenter,
		Label: "Bold",
	})
	PDFFooterCenterItalic = register(&BoolSetting{
		key: "pdfFooterCenterItalic", Default: false,
		Section: SectionPDFExport, Group: GroupPDFFooterCenter,
		Label: "Italic",
	})
	PDFFooterRight = register(&StringSetting{
		key: "pdfFooterRight", Default: "",
		Section: SectionPDFExport, Group: GroupPDFFooterRight,
		Label: "Text",
	})
	PDFFooterRightFont = register(&StringSetting{
		key: "pdfFooterRightFont", Default: "Roboto",
		Section: SectionPDFExport, Group: GroupPDFFooterRight,
		Label:       "Font",
		Options:     fontOptionList(false),
		FontPreview: true,
	})
	PDFFooterRightColor = register(&StringSetting{
		key: "pdfFooterRightColor", Default: "#000000",
		Section: SectionPDFExport, Group: GroupPDFFooterRight,
		Label:    "Color",
		IsColor:  true,
		Validate: utils.ValidateHexColor,
	})
	PDFFooterRightSize = register(&IntSetting{
		key: "pdfFooterRightSize", Default: 8,
		Section: SectionPDFExport, Group: GroupPDFFooterRight,
		Label: "Size",
		Desc:  "font size in points",
		Min:   intPtr(6), Max: intPtr(24),
	})
	PDFFooterRightBold = register(&BoolSetting{
		key: "pdfFooterRightBold", Default: false,
		Section: SectionPDFExport, Group: GroupPDFFooterRight,
		Label: "Bold",
	})
	PDFFooterRightItalic = register(&BoolSetting{
		key: "pdfFooterRightItalic", Default: false,
		Section: SectionPDFExport, Group: GroupPDFFooterRight,
		Label: "Italic",
	})
	PDFHeaderLeft = register(&StringSetting{
		key: "pdfHeaderLeft", Default: "",
		Section: SectionPDFExport, Group: GroupPDFHeaderLeft,
		Label: "Text",
	})
	PDFHeaderLeftFont = register(&StringSetting{
		key: "pdfHeaderLeftFont", Default: "Roboto",
		Section: SectionPDFExport, Group: GroupPDFHeaderLeft,
		Label:       "Font",
		Options:     fontOptionList(false),
		FontPreview: true,
	})
	PDFHeaderLeftColor = register(&StringSetting{
		key: "pdfHeaderLeftColor", Default: "#000000",
		Section: SectionPDFExport, Group: GroupPDFHeaderLeft,
		Label:    "Color",
		IsColor:  true,
		Validate: utils.ValidateHexColor,
	})
	PDFHeaderLeftSize = register(&IntSetting{
		key: "pdfHeaderLeftSize", Default: 8,
		Section: SectionPDFExport, Group: GroupPDFHeaderLeft,
		Label: "Size",
		Desc:  "font size in points",
		Min:   intPtr(6), Max: intPtr(24),
	})
	PDFHeaderLeftBold = register(&BoolSetting{
		key: "pdfHeaderLeftBold", Default: false,
		Section: SectionPDFExport, Group: GroupPDFHeaderLeft,
		Label: "Bold",
	})
	PDFHeaderLeftItalic = register(&BoolSetting{
		key: "pdfHeaderLeftItalic", Default: false,
		Section: SectionPDFExport, Group: GroupPDFHeaderLeft,
		Label: "Italic",
	})
	PDFHeaderCenter = register(&StringSetting{
		key: "pdfHeaderCenter", Default: "",
		Section: SectionPDFExport, Group: GroupPDFHeaderCenter,
		Label: "Text",
	})
	PDFHeaderCenterFont = register(&StringSetting{
		key: "pdfHeaderCenterFont", Default: "Roboto",
		Section: SectionPDFExport, Group: GroupPDFHeaderCenter,
		Label:       "Font",
		Options:     fontOptionList(false),
		FontPreview: true,
	})
	PDFHeaderCenterColor = register(&StringSetting{
		key: "pdfHeaderCenterColor", Default: "#000000",
		Section: SectionPDFExport, Group: GroupPDFHeaderCenter,
		Label:    "Color",
		IsColor:  true,
		Validate: utils.ValidateHexColor,
	})
	PDFHeaderCenterSize = register(&IntSetting{
		key: "pdfHeaderCenterSize", Default: 8,
		Section: SectionPDFExport, Group: GroupPDFHeaderCenter,
		Label: "Size",
		Desc:  "font size in points",
		Min:   intPtr(6), Max: intPtr(24),
	})
	PDFHeaderCenterBold = register(&BoolSetting{
		key: "pdfHeaderCenterBold", Default: false,
		Section: SectionPDFExport, Group: GroupPDFHeaderCenter,
		Label: "Bold",
	})
	PDFHeaderCenterItalic = register(&BoolSetting{
		key: "pdfHeaderCenterItalic", Default: false,
		Section: SectionPDFExport, Group: GroupPDFHeaderCenter,
		Label: "Italic",
	})
	PDFHeaderRight = register(&StringSetting{
		key: "pdfHeaderRight", Default: "",
		Section: SectionPDFExport, Group: GroupPDFHeaderRight,
		Label: "Text",
	})
	PDFHeaderRightFont = register(&StringSetting{
		key: "pdfHeaderRightFont", Default: "Roboto",
		Section: SectionPDFExport, Group: GroupPDFHeaderRight,
		Label:       "Font",
		Options:     fontOptionList(false),
		FontPreview: true,
	})
	PDFHeaderRightColor = register(&StringSetting{
		key: "pdfHeaderRightColor", Default: "#000000",
		Section: SectionPDFExport, Group: GroupPDFHeaderRight,
		Label:    "Color",
		IsColor:  true,
		Validate: utils.ValidateHexColor,
	})
	PDFHeaderRightSize = register(&IntSetting{
		key: "pdfHeaderRightSize", Default: 8,
		Section: SectionPDFExport, Group: GroupPDFHeaderRight,
		Label: "Size",
		Desc:  "font size in points",
		Min:   intPtr(6), Max: intPtr(24),
	})
	PDFHeaderRightBold = register(&BoolSetting{
		key: "pdfHeaderRightBold", Default: false,
		Section: SectionPDFExport, Group: GroupPDFHeaderRight,
		Label: "Bold",
	})
	PDFHeaderRightItalic = register(&BoolSetting{
		key: "pdfHeaderRightItalic", Default: false,
		Section: SectionPDFExport, Group: GroupPDFHeaderRight,
		Label: "Italic",
	})
	PDFFTokensNote = register(&NoteSetting{
		key:     "pdfTokensNote",
		Section: SectionPDFExport,
		Group:   GroupPDFNotes,
		Text:    "tokens (all columns): {{date}}, {{page}}, {{pages}}, {{filename}}, {{filepath}}, {{folder}}, {{created}}, {{edited}}, {{tags}}, {{collection}}",
	})
)

// fontOptionList is called inline at each font setting's declaration (rather
// than shared via a package-level var) so that referencing it doesn't turn
// the setting into a dependent in Go's package-init dependency graph — that
// would defer its initialization (and thus its register() call, and thus
// its position in the settings UI) until after every setting that doesn't
// reference a shared var, scrambling the display order. Derived from the
// fonts manifest so the selectable values, the registered ttf files, and the
// settings preview can never drift apart. The code block settings only offer
// monospace families — their wrapping assumes a fixed character width.
func fontOptionList(monoOnly bool) []SettingOption {
	var opts []SettingOption
	for _, f := range fonts.Families {
		if monoOnly && !f.Monospace {
			continue
		}
		opts = append(opts, SettingOption{Value: f.Name, Label: f.Name})
	}
	return opts
}
