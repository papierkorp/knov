# Developer Guide

## Development Setup

### Prerequisites

- Go 1.21+
- Swag CLI: `go install github.com/swaggo/swag/cmd/swag@latest`
- MinGW-w64 (for Windows cross-compilation): `sudo apt-get install mingw-w64` (Linux) or `brew install mingw-w64` (macOS)

### Development Commands

```bash
# Start development server with hot reload
make dev

# Generate Swagger docs
make docs

# Generate translations
make translate

# Build for production
make prod

# Clean build artifacts
make clean
```

## Project Structure

```
├── cmd/                    # Application entry point (legacy, moved to root)
├── internal/
│   ├── configmanager/     # Configuration and user settings
│   ├── dashboard/         # Dashboard and widget system
│   ├── files/            # File handling and metadata
│   ├── filetype/         # File type processors (Markdown, DokuWiki)
│   ├── logging/          # Logging utilities
│   ├── search/           # Search engine implementations
│   ├── server/           # HTTP server and API routes
│   ├── storage/          # Data persistence layer
│   ├── thememanager/     # Theme system and builtin theme
│   ├── translation/      # Internationalization
│   └── utils/            # Utility functions
├── static/               # Static assets (embedded)
├── themes/               # Plugin themes
├── main.go              # Application entry point
└── docs/                # Documentation
```

## Building Themes

### Builtin Theme

The builtin theme is embedded in the binary:

- Templates in `themes/builtin/*.gohtml`
- CSS in `themes/builtin/style/*.css`
- Automatically embedded via `//go:embed themes/builtin` in main.go

### Plugin Themes

Plugin themes are Go plugins with embedded assets:

```go
// themes/mytheme/main.go
package main

import (
    "embed"
    "knov/internal/thememanager"
    // ...
)

//go:embed templates/*.css
var cssFiles embed.FS

type MyTheme struct{}
var Theme MyTheme

func GetCSS(filename string) string {
    cssPath := "templates/" + filename
    if data, err := cssFiles.ReadFile(cssPath); err == nil {
        return string(data)
    }
    return ""
}

// Implement ITheme interface methods...
```

Build theme:

```bash
cd themes/mytheme
go build -buildmode=plugin -o mytheme.so .
```

## File Path Handling

### Content Storage Paths

The application uses standardized path handling to manage files:

- **Docs Path**: `data/docs/` - for document files
- **Media Path**: `data/media/` - for media files

### Path Utility Functions

**Core utilities in `internal/utils/cleanse.go`:**
- `StripPathPrefix(path, prefix)` - Generic function to strip any prefix from a path
- `NormalizeDocsPath(path)` - Strips "docs/" prefix if present
- `NormalizeMediaPath(path)` - Strips "media/" prefix if present

**Storage utilities in `internal/contentStorage/contentStorage.go`:**
- `ToDocsPath(relativePath)` - Converts relative paths to full docs paths
  - Input: `"ai.md"` or `"docs/ai.md"` â†’ Output: `"/data/docs/ai.md"`
  - Uses `NormalizeDocsPath()` internally
- `ToMediaPath(relativePath)` - Converts relative paths to full media paths
  - Input: `"image.jpg"` or `"media/image.jpg"` â†’ Output: `"/data/media/image.jpg"`
  - Uses `NormalizeMediaPath()` internally
- `ToRelativePath(fullPath)` - Strips data directory prefixes to get relative paths

### API Path Parameters

When handling file paths in API endpoints:
- Accept paths with or without directory prefixes ("docs/", "media/")
- Use `ToDocsPath()` and `ToMediaPath()` for automatic normalization
- For custom normalization, use `utils.StripPathPrefix(path, "custom/")`
- This prevents path duplication issues like `data/docs/docs/file.md`

**Important: Metadata Path Handling**
- Metadata paths are stored WITH prefixes: `docs/folder/file.md` for documents, `media/folder/image.png` for media
- Use `getFilePathForMetadata()` helper function to get correct filesystem paths from metadata paths
- The `metaDataUpdate()` function automatically handles both docs and media files based on path prefix

### Example Usage

```go
// In API handlers
normalizedPath := utils.NormalizeDocsPath("docs/projects/file.md") // -> "projects/file.md"
fullPath := contentStorage.ToDocsPath(normalizedPath) // -> "/data/docs/projects/file.md"

// Custom prefix stripping
cleanPath := utils.StripPathPrefix("temp/uploads/file.txt", "temp/") // -> "uploads/file.txt"
```

## API Development

### Adding New Endpoints

1. Add handler function to appropriate `internal/server/api_*.go` file
2. Add route in `internal/server/server.go`
3. Add Swagger documentation comments
4. Run `make docs` to regenerate Swagger documentation

### Editor System

The application uses a dynamic editor system that selects the appropriate editor based on file type:

- **Location**: `internal/server/api_editor.go`
- **Routes**: `/api/editor/*`
- **Editor Types**:
  - `markdown-editor`: For markdown files (detected via MarkdownHandler)
  - `textarea-editor`: For dokuwiki and plaintext files (detected via respective handlers)
  - `list-editor`: For todo and journaling filetypes (TODO)
  - `filter-editor`: For filter filetypes (TODO)
  - `index-editor`: For MOC (maps of content) filetypes (TODO)

The `GetEditor()` function determines the appropriate editor based on:
1. File `FileType` from metadata (todo, fleeting, literature, permanent, moc, filter, journaling)
2. Syntax detected by filetype handlers using `CanHandle()` methods

**Syntax Detection**:
- Syntax is always dynamically detected using the filetype handler registry
- The system uses `CanHandle()` methods from registered handlers:
  - **MarkdownHandler**: Detects `.md` and `.markdown` files â†’ markdown-editor
  - **DokuwikiHandler**: Detects `.txt` files with DokuWiki syntax (headers like `====== title ======`) â†’ textarea-editor
  - **PlaintextHandler**: Detects plain `.txt` files â†’ textarea-editor
- Detection happens at request time based on file extension and content
- No syntax metadata is stored - always uses fresh detection

### Metadata Fields

Files have the following metadata fields:
- **name**: Manual filename
- **path**: Automatic file path
- **collection**: Collection name (auto-detected from folder structure)
- **filetype**: Type of file (fleeting, literature, permanent, moc, filter, todo, journaling)
- **status**: Status (draft, published, archived)
- **priority**: Priority level (low, medium, high)
- **tags**: Manual tags
- **folders**: Automatic folder hierarchy
- **PARA**: Projects, Areas, Resources, Archive organization
- Timestamps: createdAt, lastEdited, targetDate
- Links: parents, kids, ancestors, usedLinks, linksToHere

Note: Syntax is NOT stored in metadata - it is dynamically detected using filetype handlers. Regenerate docs with `make docs`

### Filter Testing System

The application includes a comprehensive filter testing system to ensure filter functionality works correctly:

- **Location**: `internal/testdata/testfilter.go`
- **API Endpoint**: `/api/testdata/filtertest`
- **Admin Interface**: Available in the Test Data Management section

**Filter Test Features**:
- Creates 12 test metadata objects (filterTestA through filterTestL)
- Tests various filter scenarios including:
  - Single tag filtering (`experimental`, `basic`, etc.)
  - AND/OR logic combinations
  - Collection, status, priority filtering
  - File type and folder filtering
  - PARA organization filtering (projects, areas, resources)
  - Exclusion filters
  - Complex multi-criteria filters

**Test Metadata Structure**:
The test metadata objects are carefully designed with different:
- Collections: `filter-tests`, `advanced-tests`, `basic-tests`, `integration-tests`, `performance-tests`, `special-tests`
- Tags: Various combinations for testing different scenarios
- File types: `fleeting`, `literature`, `permanent`, `journaling`, `moc`
- Statuses: `draft`, `published`, `archived`
- Priorities: `high`, `medium`, `low`
- PARA organization: Different projects, areas, resources, and archive values

**Running Filter Tests**:
1. Via Admin Interface: Go to Admin â†’ Test Data Management â†’ "Run Filter Tests"
2. Via API: `POST /api/testdata/filtertest`
3. Returns detailed results including passed/failed tests and expected vs actual counts

The testing system is essential for validating filter functionality during development and ensures that the filter system remains reliable as a cornerstone feature of the application.

### Theme-Friendly APIs

- Use form data instead of JSON for consistency
- Add Swagger comments for documentation
- Keep APIs generic and theme-agnostic
- Return HTMX-compatible responses

## UI/UX Design

### Admin & Settings Pages

The admin and settings pages use a unified card-based design system that provides:

- **Consistent Layout**: Both pages use the same section and card structure for visual uniformity
- **Configurable Help Text**: Support for both tooltip and help-text description display modes
  - `help-text`: Always-visible descriptions below form elements (used for settings pages)
  - `tooltips`: Hover-only tooltips for compact views
- **Responsive Design**: Mobile-friendly layouts that adapt to different screen sizes
- **Visual Hierarchy**: Clear headings, sections, and card-based organization

### Styling Guidelines

- Use ID selectors for page-specific styling (`#page-admin`, `#page-settings`)
- Keep global styles in `style.css`
- Use component-specific files for detailed styling (e.g., `settings-admin.css`)
- Follow theme-agnostic design patterns using CSS custom properties

### Accessibility Improvements

- Configurable description display: choose between always-visible help text or tooltips based on context
- Proper form labeling and structure
- Keyboard navigation support
- ARIA-compliant interactive elements

### Theme Settings Rendering

Theme settings can use different description display modes via the `DescriptionType` enum:

```go
// Import the render package for the enum
import "knov/internal/server/render"

// For settings pages (always visible help text)
html := render.RenderThemeSettingsForm(schema, values, render.DescriptionTypeHelpText)

// For compact views (hover tooltips)
html := render.RenderThemeSettingsForm(schema, values, render.DescriptionTypeTooltips)
```

**Available types:**
- `render.DescriptionTypeHelpText`: Always-visible descriptions below form elements
- `render.DescriptionTypeTooltips`: Hover-only tooltips for compact views
- Proper form labeling and structure
- Keyboard navigation support
- ARIA-compliant interactive elements

## Translation

Add translatable strings in templates:

```go
{{T "Your translatable text"}}
```

Add translatable strings in Go code (global):

```go
translation.Sprintf("Your translatable text")
```

Add translatable strings in HTMX handlers (user-specific):

```go
func handleSomeHTMX(w http.ResponseWriter, r *http.Request) {
    // Use user's current language setting
    userLang := configmanager.GetLanguage()
    text := translation.SprintfForRequest(userLang, "Your translatable text")
    html := fmt.Sprintf(`<div>%s</div>`, text)
    w.Write([]byte(html))
}
```

Generate translations:

```bash
make translation
```

Translation files in `internal/translation/locales/{lang}/messages.gotext.json`

## Embedded Assets

### Static Files

Static files are embedded from the project root:

```go
//go:embed static/*
var staticFS embed.FS
```

### Theme Assets

Builtin theme assets are embedded in main.go:

```go
//go:embed themes/builtin
var builtinThemeFS embed.FS
```

Plugin themes embed their own assets:

```go
//go:embed templates/*.css
var cssFiles embed.FS
```

## Configuration System

Configuration uses a layered approach:

1. Environment variables (highest priority)
2. Configuration files
3. Defaults (lowest priority)

Add new config options in `internal/configmanager/config.go`:

```go
func getNewOption() string {
    if val := os.Getenv("KNOV_NEW_OPTION"); val != "" {
        return val
    }
    return "default_value"
}
```
