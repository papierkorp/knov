# Theme Creator Guide

The theme system uses Go plugins to provide customizable UI themes. Each theme is compiled as a `.so` plugin file and loaded dynamically at runtime.

## Quick Start

1. **Create theme directory**: `themes/mytheme/`
2. **Implement ITheme interface**: Create `main.go` with theme struct
3. **Add templates**: Create `templates/` folder with `.templ` files
4. **Add styles**: Create `templates/style.css` for theme styling
5. **Build**: Run `make dev` to compile and load your theme

## Theme Structure

```
themes/
└── mytheme/
    ├── main.go              # Theme implementation
    └── templates/
        ├── style.css        # Theme styles
        ├── base.templ       # Optional: custom base template
        ├── home.templ       # Required templates
        ├── settings.templ
        ├── admin.templ
        ├── playground.templ
        ├── latestchanges.templ
        ├── history.templ
        ├── search.templ
        ├── overview.templ
        ├── fileview.templ
        └── dashboard.templ
```

## ITheme Interface

Your theme must implement all methods in the ITheme interface:

```go
type ITheme interface {
    Home() (templ.Component, error)
    Settings() (templ.Component, error)
    Admin() (templ.Component, error)
    Playground() (templ.Component, error)
    LatestChanges() (templ.Component, error)
    History() (templ.Component, error)
    Search(query string) (templ.Component, error)
    Overview() (templ.Component, error)
    GetAvailableFileViews() []string
    RenderFileView(viewName string, content string, filePath string) (templ.Component, error)
    Dashboard(id string) (templ.Component, error)
}
```

## Basic Implementation

```go
// themes/mytheme/main.go
package main

import (
    "knov/internal/thememanager"
    "knov/themes/mytheme/templates"
    "github.com/a-h/templ"
)

type MyTheme struct{}

// Required: Export as Theme variable
var Theme MyTheme

func (t *MyTheme) Home() (templ.Component, error) {
    tm := thememanager.GetThemeManager()
    data := thememanager.TemplateData{
        ThemeToUse:      tm.GetCurrentThemeName(),
        AvailableThemes: tm.GetAvailableThemes(),
    }
    return templates.Home(data), nil
}

// Implement other required methods...

func (t *MyTheme) GetAvailableFileViews() []string {
    return []string{"detailed", "minimal"}
}

func (t *MyTheme) RenderFileView(viewName string, content string, filePath string) (templ.Component, error) {
    // Return appropriate file view component
    switch viewName {
    case "minimal":
        return templates.FileViewMinimal(content, filePath), nil
    default:
        return templates.FileViewDetailed(content, filePath), nil
    }
}
```

## Template Data

All templates receive `TemplateData` with common information:

```go
type TemplateData struct {
    ThemeToUse       string              // Current theme name
    AvailableThemes  []string            // All available themes
    Dashboard        *dashboard.Dashboard // Dashboard data (when applicable)
    ShowCreateForm   bool                // Show creation form flag
}
```

## Special Route Handling

### Dashboard Routes
- `/dashboard` - Shows default dashboard
- `/dashboard/{id}` - Shows specific dashboard
- `/dashboard?action=new` - Shows dashboard creation form

Handle the "new" action in your Dashboard method:

```go
func (t *MyTheme) Dashboard(id string) (templ.Component, error) {
    data := thememanager.TemplateData{...}

    if id == "new" {
        data.ShowCreateForm = true
        return templates.Dashboard(data), nil
    }

    // Handle normal dashboard logic
}
```

### File Views

Implement multiple file view options:

```go
func (t *MyTheme) GetAvailableFileViews() []string {
    return []string{"detailed", "compact", "minimal", "reader"}
}

func (t *MyTheme) RenderFileView(viewName string, content string, filePath string) (templ.Component, error) {
    switch viewName {
    case "compact":
        return templates.FileViewCompact(content, filePath), nil
    case "minimal":
        return templates.FileViewMinimal(content, filePath), nil
    case "reader":
        return templates.FileViewReader(content, filePath), nil
    default:
        return templates.FileViewDetailed(content, filePath), nil
    }
}
```

## Styling Guidelines

### CSS Organization
- Use ID selectors for page-specific styles: `#page-home`, `#page-settings`
- Use ID selectors for component styles: `#component-header`, `#component-search`
- Use ID selectors for view styles: `#view-fileview-detailed`, `#view-fileview-compact`
- Global styles go in the main `style.css`

### CSS Variables
Use these standard CSS variables for consistency:

```css
:root {
    --primary: #65a30d;     /* Main brand color */
    --accent: #a3e635;      /* Accent/highlight color */
    --neutral: #475569;     /* Neutral gray */
    --black: #374151;       /* Dark background */
    --white: #d1d5db;       /* Light text */
}
```

### Example Theme Styles

```css
/* themes/mytheme/templates/style.css */
@import url("header.css");
@import url("fileview.css");

:root {
    --primary: #3b82f6;     /* Custom blue theme */
    --accent: #60a5fa;
    --neutral: #6b7280;
    --black: #1f2937;
    --white: #f9fafb;
}

#page-home {
    background: linear-gradient(135deg, var(--primary), var(--accent));
    padding: 20px;
}

#component-header {
    border-bottom: 3px solid var(--accent);
}
```

## Available APIs

Your templates can use these API endpoints:

### Files
- `GET /api/files/list` - List all files
- `GET /api/files/content/{filepath}` - Get file content
- `POST /api/files/filter` - Filter files by metadata

### Search
- `GET /api/search?q={query}&format={format}` - Search files

### Dashboards
- `GET /api/dashboards` - List dashboards
- `POST /api/dashboards` - Create dashboard
- `POST /api/dashboards/widget/{id}` - Render widget

### Metadata & Links
- `GET /api/metadata?filepath={path}` - Get file metadata
- `GET /api/links/parents?filepath={path}` - Get parent links

See the builtin theme for complete API usage examples.

## Building and Testing

1. **Development**: Run `make dev` to compile themes and start server
2. **Theme switching**: Use settings page or API: `POST /api/themes/setTheme`
3. **Debugging**: Set `KNOV_LOG_LEVEL=debug` for detailed theme loading logs

## Best Practices

1. **Follow the builtin theme structure** as a reference
2. **Keep components modular** - separate complex logic into smaller templates
3. **Use semantic HTML** with proper accessibility attributes
4. **Test responsive design** on mobile devices
5. **Provide meaningful file view options** beyond just "detailed"
6. **Use HTMX** for dynamic content loading where appropriate
7. **Handle errors gracefully** - always return valid components
8. **Document custom features** in your theme's README

## Troubleshooting

### Common Issues

1. **Theme not loading**: Check Go plugin compilation errors in logs
2. **Missing methods**: Ensure all ITheme interface methods are implemented
3. **Template errors**: Verify templ file syntax and imports
4. **CSS not applying**: Check file paths and CSS selector specificity

### Debug Steps

1. Enable debug logging: `KNOV_LOG_LEVEL=debug`
2. Check theme compilation: Look for `.so` files in themes directory
3. Verify plugin symbols: Ensure `var Theme MyTheme` is exported
4. Test individual templates: Use playground route for testing

## Advanced Features

### Custom Widget Types

You can extend dashboard widgets by handling them in your templates:

```go
// Handle custom widget rendering in dashboard template
if widget.Type == "custom-calendar" {
    @CustomCalendarWidget(widget.Config)
}
```

### Theme-Specific Configuration

Add theme-specific settings through user configuration:

```go
// Access user settings in templates
userSettings := configmanager.GetUserSettings()
// Add custom fields to settings
```

### Integration with External APIs

Themes can integrate with external services:

```go
// Example: Weather widget
func (t *MyTheme) Dashboard(id string) (templ.Component, error) {
    // Fetch weather data
    weather := fetchWeatherData()
    data.Weather = weather
    return templates.Dashboard(data), nil
}
```

## Contributing

When contributing new themes:

1. Follow the established patterns from the builtin theme
2. Document any new features or requirements
3. Test with different file types and dashboard configurations
4. Ensure responsive design works across devices
5. Add appropriate error handling and loading states
