# Developer Guide

**Prerequisites**

- Go 1.21 or later
- Git
- Make
- Swag CLI: `go install github.com/swaggo/swag/cmd/swag@latest`
- gotext: `go install golang.org/x/text/cmd/gotext@latest`
- MinGW-w64 (for Windows cross-compilation): `sudo apt-get install mingw-w64` (Linux) or `brew install mingw-w64` (macOS)


## Quick Start

Clone and setup:

```bash
git clone https://github.com/papierkorp/knov.git
cd knov
go mod download
```

Install required tools:

```bash
go install github.com/swaggo/swag/cmd/swag@latest
go install golang.org/x/text/cmd/gotext@latest
```

Start development server:

```bash
# Start development server
make dev
make dev-fast # without fts5 search init

# Generate Swagger docs
make swaggo-api-init

# Generate translations
make translation

# Build for production
make prod

# Create and Run Docker image
make docker
```

## API Development

### Adding New Endpoints

1. Add handler function to appropriate `internal/server/api_*.go` file
2. Add route in `internal/server/server.go`
3. Add Swagger documentation comments


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

## Configuration Management


# Theme Creation Guide

## Quick Start

1. Create a new folder in `themes/` with the required files
2. Run the application: `make dev`
3. Navigate to `http://localhost:1324/settings`
4. Select your theme from the dropdown
5. Test all pages

## Required Files

Are determined in [thememanager.go](../internal/thememanager.go) in the `ThemeTemplates` struct.

### theme.json

Is defined in the [thememanager.go](../internal/thememanager.go) and allows to pass [Theme Settings](#theme_settings) through the app which can then be used via the [Template Data](#template_data)

```go
type ThemeMetadata struct {
	Name          string                  `json:"name"`
	Version       string                  `json:"version"`
	Author        string                  `json:"author"`
	Description   string                  `json:"description"`
	ThemeSettings map[string]ThemeSetting `json:"themeSettings,omitempty"`
}
```

**Example**

```json
{
  "name": "Builtin Theme",
  "version": "1.0.0",
  "author": "System",
  "description": "Default builtin theme",
}
```

## Theme Settings

Themes can define custom settings that users can configure. 
Is defined in the [thememanager.go](../internal/thememanager.go) in the ThemeSettings struct:

```go
type ThemeMetadata struct {
	Name          string                  `json:"name"`
	Version       string                  `json:"version"`
	Author        string                  `json:"author"`
	Description   string                  `json:"description"`
	ThemeSettings map[string]ThemeSetting `json:"themeSettings,omitempty"`
}

type ThemeSetting struct {
	Type        string      `json:"type"`
	Default     interface{} `json:"default"`
	Label       string      `json:"label"`
	Description string      `json:"description,omitempty"`
	Options     []string    `json:"options,omitempty"`
	Min         *int        `json:"min,omitempty"`
	Max         *int        `json:"max,omitempty"`
}
````

Add a `themeSettings` object to your theme.json:

```json
{
  "name": "Builtin Theme",
  "version": "1.0.0",
  "author": "System",
  "description": "Default builtin theme",
  "themeSettings": {
    "darkMode": {
      "type": "boolean",
      "default": true,
      "label": "Dark Mode",
      "description": "enable dark theme appearance for better readability in low light"
    },
    "colorScheme": {
      "type": "select",
      "options": ["blue", "green", "red", "purple"],
      "default": "green",
      "label": "Color Scheme",
      "description": "choose the color theme for the interface"
    },
    "fileView": {
      "type": "select",
      "options": ["detailed", "compact", "reader"],
      "default": "detailed",
      "label": "File View",
      "description": "choose how files are displayed - detailed shows metadata, compact saves space, reader optimizes for reading"
    },
  }
}
```

**Setting Types:**

- `boolean`: Checkbox input
- `select`: Dropdown with predefined options
- `range`: Slider with min/max values
- `textarea`: Multi-line text input
- `text`: Single-line text input

**Standard Settings (Recommended):**

Most themes should implement these standard settings for consistency:

- `darkMode` (boolean): Enable dark theme appearance
- `colorScheme` (select): Color scheme selection
- `fileView` (select): File view layout options (e.g., detailed, compact, reader)
- `customCSS` (textarea): Custom CSS input

**Required fields for each setting:**
- `type`: The input type
- `default`: Default value
- `label`: User-friendly display name

**Optional fields:**
- `description`: Help text for the setting
- `options`: Array of options for select type
- `min`/`max`: Range limits for range type

## example template

**Settings Example**

Which allows theme Switching:

```html
<!DOCTYPE html>
<html>
  <head>
    <title>{{.Title}}</title>
    <link href="/themes/{{.CurrentTheme}}/style.css" rel="stylesheet" />
  </head>
  <body>
    <h1>Settings</h1>

    <div class="nav">
      <a href="/base">Base</a>
      <a href="/settings">Settings</a>
    </div>

    <form method="post" action="/settings">
      <label for="theme">Select Theme:</label>
      <select name="theme" id="theme">
        {{range .Themes}}
        <option value="{{.Name}}" {{if eq .Name $.CurrentTheme}}selected{{end}}>
          {{.Metadata.Name}}
        </option>
        {{end}}
      </select>
      <input type="submit" value="Change Theme" />
    </form>
  </body>
</html>
```

## Template Data

Template Data is passed from the application to the theme.
Is defined in [template_data.go](../internal/template_data.go).

**Accessing Template Data**

In your templates, you can access template data like this:

```html
<!DOCTYPE html>
<html>
<head>
    <title>{{ .Title }}</title>
</head>
```

**Example: Using fileView setting for conditional rendering**

```html
{{ define "content" }}
{{if eq (index .ThemeSettings "fileView") "compact"}}
    {{ template "compact" . }}
{{else if eq (index .ThemeSettings "fileView") "reader"}}
    {{ template "reader" . }}
{{else}}
    {{ template "detailed" . }}
{{end}}
{{ end }}
```

## Validation

The system validates:

- All required files exist and are not empty
- theme.json contains all required fields (name, version, author, description)
- Theme settings (if defined) have valid types and required fields
- Templates parse correctly

If validation fails, check console output for error messages.
