# Theme Creator Guide

The theme system uses Go plugins to provide customizable UI themes. Each theme is compiled as a .so plugin file and loaded dynamically at runtime.

## Quick Start

1. Create theme directory: themes/mytheme/
2. Implement ITheme interface: Create main.go with theme struct
3. Add metadata: Export Metadata variable with theme configuration
4. Add templates: Create templates/ folder with .templ files
5. Add styles: Create templates/style.css for theme styling
6. Build: Run make dev to compile and load your theme

## Theme Structure

```
themes/
└── mytheme/
    ├── main.go              # Theme implementation
    └── templates/
        ├── style.css        # Theme styles
        ├── *.templ          # Template files
        └── *_templ.go       # Generated template files
```

## Theme Implementation

Your theme's main.go must export two symbols:

### 1. Theme Variable

Export a variable named Theme that implements the ITheme interface:

```go
    package main
    type MyTheme struct{}
    var Theme MyTheme
```
`
See internal/thememanager/thememanager.go for the complete ITheme interface definition.

### 2. Metadata Variable

Export a Metadata variable containing static theme configuration:

```go
    var Metadata = thememanager.ThemeMetadata{
        AvailableFileViews: []string{"detailed", "compact", "minimal"},
    }
```

**Important: Use thememanager.ThemeMetadata type, not a local type definition.**

## Theme Metadata Fields

- AvailableFileViews: List of file view types the theme supports (e.g., "markdown", "detailed", "compact")

## Minimal Example

```go
    package main

    import (
        "knov/internal/thememanager"
        "knov/themes/mytheme/templates"
        "github.com/a-h/templ"
    )

    type MyTheme struct{}

    var Theme MyTheme

    var Metadata = thememanager.ThemeMetadata{
        AvailableFileViews: []string{"detailed"},
    }

    func (t *MyTheme) Home() (templ.Component, error) {
        tm := thememanager.GetThemeManager()
        data := thememanager.TemplateData{
            ThemeToUse:      tm.GetCurrentThemeName(),
            AvailableThemes: tm.GetAvailableThemes(),
        }
        return templates.Home(data), nil
    }

    // Implement remaining ITheme methods...
```
`
## Template Data

Access dynamic data in your templates via thememanager.TemplateData:

```go
    type TemplateData struct {
        ThemeToUse      string
        AvailableThemes []string
        Dashboard       *dashboard.Dashboard
        ShowCreateForm  bool
    }
```

## Building and Loading

Themes are automatically compiled and loaded during initialization:

1. Compile: go build -buildmode=plugin -o themes/mytheme.so themes/mytheme/
2. Load: Theme manager automatically loads all .so files from themes/ directory
3. Switch: Set active theme via settings page or config

## Reference Implementation

See themes/builtin/ for a complete reference implementation demonstrating all required methods and best practices.

## Interface Definitions

- ITheme: See internal/thememanager/thememanager.go
- IThemeManager: See internal/thememanager/thememanager.go
- TemplateData: See internal/thememanager/thememanager.go
