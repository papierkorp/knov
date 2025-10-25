# Theme Manager

A simple Go-based theme system with template management.

## Features

**Theme System**

- Load multiple themes from `themes/` directory
- Switch themes via settings page
- Each theme requires: `theme.json`, `base.gotmpl`, `settings.gotmpl`, `style.css`
- Theme validation on load (required files, metadata, template structure)

**Views**

- Define multiple views per template in `theme.json`
- Example: `base.gotmpl` can have "basic" and "advanced" views
- Views use Go template `{{define}}` blocks
- First view in array is rendered as default

**Overwrite System**

- Place templates in `themes/overwrite/` to override current theme
- Does not require full theme structure
- Validates overwrite templates before applying
- Falls back to theme template if overwrite is invalid

**Builtin Theme**

- Embedded into binary at compile time
- Automatically extracted on startup
- Set as default theme

**Template Data**

- `{{.Title}}` - page title
- `{{.Themes}}` - all available themes
- `{{.CurrentTheme}}` - active theme name

## Usage

```bash
make dev   # run development server
make prod  # build binaries
```

Server runs on `http://localhost:1325`

# Theme Creation Guide

## Quick Start

Create a new folder in `themes/` with the following required files:

```
themes/your-theme-name/
├── theme.json
├── base.gotmpl
├── settings.gotmpl
└── style.css
```

## Required Files

### 1. theme.json

Contains theme metadata and view definitions.

```json
{
  "name": "Your Theme Name",
  "version": "1.0.0",
  "author": "Your Name",
  "description": "A brief description",
  "views": {
    "base": [""],
    "settings": [""]
  }
}
```

**Fields:**

- `name`: Display name of your theme
- `version`: Theme version (use semantic versioning)
- `author`: Your name or organization
- `description`: Brief theme description
- `views`: Define available views for each template

### 2. base.gotmpl

Main template file. Must exist and contain valid HTML.

**Simple example (no named views):**

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

**With named views:**

```html
{{define "basic"}}
<!DOCTYPE html>
<html>
  <head>
    <title>{{.Title}}</title>
    <link href="/themes/{{.CurrentTheme}}/style.css" rel="stylesheet" />
  </head>
  <body>
    <h1>Basic View</h1>
  </body>
</html>
{{end}} {{define "advanced"}}
<!DOCTYPE html>
<html>
  <head>
    <title>{{.Title}}</title>
    <link href="/themes/{{.CurrentTheme}}/style.css" rel="stylesheet" />
  </head>
  <body>
    <h1>Advanced View</h1>
    <p>More features here</p>
  </body>
</html>
{{end}}
```

If using named views, update `theme.json`:

```json
"views": {
  "base": ["basic", "advanced"],
  "settings": [""]
}
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

## Views System

### Simple Theme (No Named Views)

Use empty string in views array:

```json
"views": {
  "base": [""],
  "settings": [""]
}
```

Template needs no `{{define}}` blocks.

### Multiple Views

Define named views in theme.json:

```json
"views": {
  "base": ["basic", "advanced"],
  "settings": [""]
}
```

Then create corresponding `{{define}}` blocks in your template.

## Testing Your Theme

1. Place your theme folder in `themes/`
2. Run the application: `make dev`
3. Navigate to `http://localhost:1325/settings`
4. Select your theme from the dropdown
5. Test all pages and views

## Validation

The system validates:

- All required files exist and are not empty
- theme.json contains all required fields
- All views defined in theme.json exist as templates
- Templates parse correctly

If validation fails, check console output for error messages.
