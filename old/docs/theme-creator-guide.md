# Theme Creator Guide

Create custom themes for KNOV using Go plugins with embedded assets.

## Quick Start
```bash
mkdir -p themes/mytheme/templates
# Create your theme files
cd themes/mytheme
go build -buildmode=plugin -o mytheme.so .
# Upload mytheme.so via admin interface
```

## Theme Structure
```
themes/mytheme/
├── main.go                 # Theme implementation
└── templates/
    ├── *.templ            # Templ templates
    └── *.css              # CSS files
```

## Self-Contained Theme Template

**themes/mytheme/main.go:**
```go
package main

import (
    "embed"
    "knov/internal/dashboard"
    "knov/internal/files"
    "knov/internal/thememanager"
    "knov/themes/mytheme/templates"
    "github.com/a-h/templ"
)

//go:embed templates/*.css
var cssFiles embed.FS

type MyTheme struct{}
var Theme MyTheme

var Metadata = thememanager.ThemeMetadata{
    AvailableFileViews: []string{"default", "compact"},
    SupportsDarkMode: true,
    AvailableColorSchemes: []thememanager.ColorScheme{
        {Name: "blue", Label: "Ocean Blue"},
        {Name: "green", Label: "Forest Green"},
    },
}

// GetCSS returns embedded CSS for any filename
func GetCSS(filename string) string {
    cssPath := "templates/" + filename
    if data, err := cssFiles.ReadFile(cssPath); err == nil {
        return string(data)
    }
    return ""
}

// Implement all ITheme interface methods
func (t *MyTheme) Home(viewName string) (templ.Component, error) {
    return templates.Home(), nil
}

func (t *MyTheme) Settings(viewName string) (templ.Component, error) {
    return templates.Settings(), nil
}

// ... implement all other required methods
```

## Required Methods

Implement all methods from the `ITheme` interface:
```go
type ITheme interface {
    Home(viewName string) (templ.Component, error)
    Settings(viewName string) (templ.Component, error)
    Admin(viewName string) (templ.Component, error)
    Playground(viewName string) (templ.Component, error)
    LatestChanges(viewName string) (templ.Component, error)
    History(viewName string) (templ.Component, error)
    Search(viewName string, query string) (templ.Component, error)
    Overview(viewName string) (templ.Component, error)
    RenderFileView(viewName string, fileContent *files.FileContent, filePath string) (templ.Component, error)
    FileEdit(viewName string, content string, filePath string) (templ.Component, error)
    Dashboard(viewName string, id string, action string, dash *dashboard.Dashboard) (templ.Component, error)
    BrowseFiles(viewName string, metadataType string, value string, query string) (templ.Component, error)
}
```

## CSS Management

### Embedded CSS Files
All CSS files in `templates/*.css` are automatically embedded:
```go
//go:embed templates/*.css
var cssFiles embed.FS

func GetCSS(filename string) string {
    cssPath := "templates/" + filename
    if data, err := cssFiles.ReadFile(cssPath); err == nil {
        return string(data)
    }
    return ""
}
```

### Required CSS
- `style.css` - Main theme styles (required)

### Optional CSS
- `components.css` - Component-specific styles
- `dark.css` - Dark mode styles
- `mobile.css` - Mobile-specific styles
- Any other `.css` files will be automatically available

## Theme Metadata

Configure your theme capabilities:
```go
var Metadata = thememanager.ThemeMetadata{
    AvailableFileViews:          []string{"default", "compact", "reader"},
    AvailableHomeViews:          []string{"default", "grid"},
    AvailableSearchViews:        []string{"default", "compact"},
    // ... other view types
    SupportsDarkMode:            true,
    AvailableColorSchemes: []thememanager.ColorScheme{
        {Name: "blue", Label: "Ocean Blue"},
        {Name: "green", Label: "Forest Green"},
    },
}
```

## Color Schemes and Dark Mode

### CSS Variables
Use CSS variables for themeable colors:
```css
:root {
    --primary: #0ea5e9;
    --accent: #38bdf8;
    --bg: #ffffff;
    --text: #1a1a1a;
}

/* Dark mode */
body[data-dark-mode="true"] {
    --bg: #0f172a;
    --text: #f1f5f9;
}

/* Color schemes */
body[data-color-scheme="green"] {
    --primary: #65a30d;
    --accent: #a3e635;
}
```

### Available Data Attributes
Your CSS can read these from the `<body>` element:
- `data-theme` - Current theme name
- `data-dark-mode` - "true" or "false"
- `data-color-scheme` - Selected color scheme
- `data-language` - Current language code

## Building and Deployment

### Development Build
```bash
cd themes/mytheme
go build -buildmode=plugin -o mytheme.so .
```

### Upload via Admin Interface
1. Go to `/admin`
2. Upload your `.so` file
3. Select your theme in `/settings`

### Production Deployment
Themes can be:
1. Uploaded via admin interface
2. Copied to `{KNOV_THEMES_PATH}/` directory
3. Built during deployment process

## Styling Guidelines

### ID Selectors
Use ID selectors for theme-specific styles:
- `#page-{pagename}` - Page-specific styles
- `#component-{name}` - Component-specific styles
- `#view-{viewtype}-{viewname}` - View-specific styles

### Global Styles
Only put truly global styles in `style.css`. Use separate CSS files for:
- Component styles
- Page-specific styles
- View-specific styles

### CSS Organization
```css
/* style.css - Global styles only */
body { ... }
.global-utility { ... }

/* components.css - Component styles */
#component-header { ... }
#component-sidebar { ... }

/* pages.css - Page-specific styles */
#page-home { ... }
#page-settings { ... }
```

## Best Practices

1. **Self-contained**: Embed all assets in the .so file
2. **Theme-agnostic APIs**: Don't depend on specific API formats
3. **Responsive design**: Support mobile and desktop
4. **Accessibility**: Use semantic HTML and proper contrast
5. **Performance**: Optimize CSS and minimize HTTP requests
6. **Fallbacks**: Provide reasonable defaults for all views

## Example: Complete Minimal Theme

See `themes/test/` for a complete working example of a self-contained theme with embedded CSS.

The key points:
- Single `.so` file deployment
- All CSS embedded and served via `GetCSS()`
- No external dependencies
- Works in any deployment environment
