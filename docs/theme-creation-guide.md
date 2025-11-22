# Theme Creation Guide

## Quick Start

Create a new folder in `themes/` with the following required files:

```
themes/your-theme-name/
â”œâ”€â”€ theme.json
â”œâ”€â”€ base.gotmpl
â”œâ”€â”€ settings.gotmpl
â””â”€â”€ style.css
```

## Required Files

### 1. theme.json

Contains theme metadata and settings.

```json
{
  "name": "Your Theme Name",
  "version": "1.0.0",
  "author": "Your Name",
  "description": "A brief description",
  "themeSettings": {
    "darkMode": {
      "type": "boolean",
      "default": false,
      "label": "Dark Mode"
    }
  }
}
```

**Fields:**

- `name`: Display name of your theme
- `version`: Theme version (use semantic versioning)
- `author`: Your name or organization
- `description`: Brief theme description
- `themeSettings`: Optional theme-specific settings schema

### Theme Settings (Optional)

Themes can define custom settings that users can configure. Add a `themeSettings` object to your theme.json:

```json
{
  "name": "My Theme",
  "themeSettings": {
    "sidebarWidth": {
      "type": "range",
      "min": 200,
      "max": 400,
      "default": 250,
      "label": "Sidebar Width",
      "description": "Adjust the width of the sidebar in pixels"
    },
    "enableAnimations": {
      "type": "boolean",
      "default": true,
      "label": "Enable Animations",
      "description": "Turn animations on or off"
    },
    "layoutStyle": {
      "type": "select",
      "options": ["compact", "spacious", "minimal"],
      "default": "compact",
      "label": "Layout Style"
    },
    "customCSS": {
      "type": "textarea",
      "default": "",
      "label": "Custom CSS",
      "description": "Add your custom styles"
    }
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

### 2. base.gotmpl

Main template file. Must exist and contain valid HTML.

**Example:**

```html
<!DOCTYPE html>
<html>
  <head>
    <title>{{.Title}}</title>
    <link href="/themes/{{.CurrentTheme}}/style.css" rel="stylesheet" />
  </head>
  <body>
    <h1>{{.Title}}</h1>
    <div class="nav">
      <a href="/base">Base</a>
      <a href="/settings">Settings</a>
    </div>
  </body>
</html>
```

### 3. settings.gotmpl

Settings page template. Must exist and contain valid HTML.

**Example:**

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

### 4. style.css

Your theme's CSS file.

**Example:**

```css
body {
  background: #ffffff;
  color: #000000;
  font-family: Arial, sans-serif;
}
```

## Available Template Data

Templates receive the following data:

- `{{.Title}}` - Current page title
- `{{.Themes}}` - List of all themes
- `{{.CurrentTheme}}` - Name of active theme
- `{{.ThemeSettings}}` - Current theme's setting values (available in all templates)
- `{{.Language}}` - Current language code

### Accessing Theme Settings in Templates

In your templates, you can access theme settings like this:

```html
{{if index .ThemeSettings "enableAnimations"}}
  <div class="animated">Content with animations</div>
{{else}}
  <div class="static">Content without animations</div>
{{end}}

<div style="width: {{index .ThemeSettings "sidebarWidth"}}px;">
  Sidebar with custom width
</div>
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

## Testing Your Theme

1. Place your theme folder in `themes/`
2. Run the application: `make dev`
3. Navigate to `http://localhost:1325/settings`
4. Select your theme from the dropdown
5. Test all pages

## Validation

The system validates:

- All required files exist and are not empty
- theme.json contains all required fields (name, version, author, description)
- Theme settings (if defined) have valid types and required fields
- Templates parse correctly

If validation fails, check console output for error messages.
