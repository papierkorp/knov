# Developer Guide

## Development Setup

### Prerequisites

- Go 1.21+
- Swag CLI: `go install github.com/swaggo/swag/cmd/swag@latest`

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
  - `filter-editor`: For filter filetypes - JSON-based filter configuration with live preview
  - `index-editor`: For MOC (maps of content) filetypes (TODO)

The `GetEditor()` function determines the appropriate editor based on:
1. File `FileType` from metadata (todo, fleeting, literature, permanent, moc, filter, journaling)
2. Syntax detected by filetype handlers using `CanHandle()` methods

**Syntax Detection**:
- Syntax is always dynamically detected using the filetype handler registry
- The system uses `CanHandle()` methods from registered handlers:
  - **MarkdownHandler**: Detects `.md` and `.markdown` files → markdown-editor
  - **DokuwikiHandler**: Detects `.txt` files with DokuWiki syntax (headers like `====== title ======`) → textarea-editor
  - **FilterHandler**: Detects `.filter` files → filter-editor
  - **PlaintextHandler**: Detects plain `.txt` files → textarea-editor
- Detection happens at request time based on file extension and content
- No syntax metadata is stored - always uses fresh detection

**Filter Editor**:
- Edit mode: Shows JSON configuration editor with live preview
- View mode: Displays filtered results based on saved configuration
- Configuration format: JSON with `criteria`, `logic`, `display`, and `limit` fields
- Uses existing `/api/filter` endpoint for previewing and applying filters

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

### Theme-Friendly APIs

- Use form data instead of JSON for consistency
- Add Swagger comments for documentation
- Keep APIs generic and theme-agnostic
- Return HTMX-compatible responses

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
