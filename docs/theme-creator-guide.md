# Theme Creator Guide

Create custom themes using Go plugins for maximum flexibility with minimal complexity.

## Quick Start

```bash
mkdir -p themes/mytheme/templates
# create files (see below)
make dev
```

## Theme Structure

```
themes/mytheme/
├── main.go                 # theme implementation
└── templates/
    ├── *.templ            # your templates
    └── style.css          # your styles
```

## Minimal Theme

**themes/mytheme/main.go:**

```go
package main

import (
    "knov/internal/dashboard"
    "knov/internal/thememanager"
    "knov/themes/mytheme/templates"
    "github.com/a-h/templ"
)

type MyTheme struct{}

var Theme MyTheme

var Metadata = thememanager.ThemeMetadata{
    AvailableFileViews: []string{"default"},
    SupportsDarkMode: true,
    AvailableColorSchemes: []thememanager.ColorScheme{
        {
            Name:  "blue",
            Label: "Ocean Blue",
            Colors: map[string]string{
                "primary": "#0ea5e9",
                "accent":  "#38bdf8",
            },
        },
        {
            Name:  "green",
            Label: "Forest Green",
            Colors: map[string]string{
                "primary": "#65a30d",
                "accent":  "#a3e635",
            },
        },
    },
}

func (t *MyTheme) Home(viewName string) (templ.Component, error) {
    return templates.Home(), nil
}

func (t *MyTheme) Settings(viewName string) (templ.Component, error) {
    return templates.Settings(), nil
}

// implement all other required methods...

func (t *MyTheme) RenderFileView(viewName string, content string, filePath string) (templ.Component, error) {
    switch viewName {
    case "compact":
        return templates.FileViewCompact(content, filePath), nil
    default:
        return templates.FileView(content, filePath), nil
    }
}

func (t *MyTheme) Dashboard(viewName string, id string, action string, dash *dashboard.Dashboard) (templ.Component, error) {
    if action == "new" {
        return templates.DashboardNew(), nil
    }
    if action == "edit" {
        return templates.DashboardEdit(dash), nil
    }
    return templates.Dashboard(dash), nil
}
```

## Required Methods

Implement all methods from `ITheme` interface:

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
    RenderFileView(viewName string, content string, filePath string) (templ.Component, error)
    Dashboard(viewName string, id string, action string, dash *dashboard.Dashboard) (templ.Component, error)
    BrowseFiles(viewName string, metadataType string, value string, query string) (templ.Component, error)
}
```

## Theme Metadata

Define theme capabilities and view variants:

```go
type ThemeMetadata struct {
    // view variants for each page type
    AvailableFileViews          []string
    AvailableHomeViews          []string
    AvailableSearchViews        []string
    AvailableOverviewViews      []string
    AvailableDashboardViews     []string
    AvailableSettingsViews      []string
    AvailableAdminViews         []string
    AvailablePlaygroundViews    []string
    AvailableHistoryViews       []string
    AvailableLatestChangesViews []string
    AvailableBrowseFilesViews   []string

    // theme capabilities
    SupportsDarkMode      bool
    AvailableColorSchemes []ColorScheme
}

type ColorScheme struct {
    Name   string            // e.g. "green", "blue"
    Label  string            // e.g. "Forest Green", "Ocean Blue"
    Colors map[string]string // e.g. {"primary": "#65a30d"}
}
```

## Color Schemes

Define pre-made color schemes users can select from:

```go
var Metadata = thememanager.ThemeMetadata{
    SupportsDarkMode: true,
    AvailableColorSchemes: []thememanager.ColorScheme{
        {
            Name:  "blue",
            Label: "Ocean Blue",
            Colors: map[string]string{
                "primary": "#0ea5e9",
                "accent":  "#38bdf8",
                "neutral": "#64748b",
            },
        },
        {
            Name:  "green",
            Label: "Forest Green",
            Colors: map[string]string{
                "primary": "#65a30d",
                "accent":  "#a3e635",
                "neutral": "#475569",
            },
        },
    },
}
```

## Styling with CSS

**themes/mytheme/templates/style.css:**

```css
/* Define color schemes using data attributes */

/* Blue scheme */
:root,
body[data-color-scheme="blue"] {
    --primary: #0ea5e9;
    --accent: #38bdf8;
    --neutral: #64748b;
}

/* Green scheme */
body[data-color-scheme="green"] {
    --primary: #65a30d;
    --accent: #a3e635;
    --neutral: #475569;
}

/* Light mode (default) */
body {
    --bg: #ffffff;
    --text: #1a1a1a;
    --border: var(--neutral);

    background: var(--bg);
    color: var(--text);
}

/* Dark mode */
body[data-dark-mode="true"] {
    --bg: #0f172a;
    --text: #f1f5f9;
    --border: #334155;
}

/* Use color variables throughout */
.button {
    background: var(--primary);
    color: var(--bg);
}

a {
    color: var(--primary);
}

a:hover {
    color: var(--accent);
}

/* Use ID selectors for specificity */
#page-home { }
#component-navbar { }
#view-fileview-compact { }
```

## Available Body Attributes

Your CSS can read these data attributes:

- `data-theme` - current theme name
- `data-dark-mode` - "true" or "false"
- `data-color-scheme` - selected color scheme name
- `data-language` - current language code

## User Preferences

User settings are automatically stored in `config/users/{userid}/settings.json`:

```json
{
  "theme": "mytheme",
  "language": "en",
  "fileView": "default",
  "darkMode": true,
  "colorScheme": "green"
}
```

No additional configuration needed!

## Building

```bash
make dev  # auto-compiles all themes
```

Manual:
```bash
cd themes/mytheme
go build -buildmode=plugin -o ../mytheme.so .
```

## Reference

See `themes/builtin/` for complete implementation.
