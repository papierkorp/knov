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
  },
  "colorSchemes": [
    {
      "value": "default",
      "label": "Default"
    },
    {
      "value": "dark",
      "label": "Dark Theme"
    }
  ]
}
```

**Fields:**

- `name`: Display name of your theme
- `version`: Theme version (use semantic versioning)
- `author`: Your name or organization
- `description`: Brief theme description
- `views`: Define available views for each template
- `colorSchemes`: Array of color scheme options for your theme (required)

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

## Color Schemes

Color schemes allow users to switch between different color variations of your theme. **Every theme must define at least one color scheme.**

### Defining Color Schemes

Add a `colorSchemes` array to your theme.json with at least one scheme:

```json
{
  "name": "My Theme",
  "version": "1.0.0",
  "author": "Me",
  "description": "My custom theme",
  "views": {
    "base": [""],
    "settings": [""]
  },
  "colorSchemes": [
    {
      "value": "light",
      "label": "Light Mode"
    },
    {
      "value": "dark",
      "label": "Dark Mode"
    },
    {
      "value": "ocean",
      "label": "Ocean Blue"
    }
  ]
}
```

**Required fields for each color scheme:**
- `value`: The CSS selector value (used in `data-color-scheme="value"`)
- `label`: User-friendly display name

### Implementing in CSS

Use the `data-color-scheme` attribute in your CSS:

```css
/* Default styles */
:root {
  --primary: #3b82f6;
  --background: #ffffff;
}

/* Light color scheme */
body[data-color-scheme="light"] {
  --primary: #2563eb;
  --background: #f8fafc;
}

/* Dark color scheme */
body[data-color-scheme="dark"] {
  --primary: #60a5fa;
  --background: #1e293b;
}

/* Ocean color scheme */
body[data-color-scheme="ocean"] {
  --primary: #0891b2;
  --background: #f0f9ff;
}
```

The color scheme value is automatically set on the `<body>` element as `data-color-scheme="value"`.

## Testing Your Theme

1. Place your theme folder in `themes/`
2. Run the application: `make dev`
3. Navigate to `http://localhost:1325/settings`
4. Select your theme from the dropdown
5. Test all pages and views

## Validation

The system validates:

- All required files exist and are not empty
- theme.json contains all required fields (name, version, author, description, views, colorSchemes)
- colorSchemes array has at least one color scheme
- Each color scheme has required `value` and `label` fields
- All views defined in theme.json exist as templates
- Templates parse correctly

If validation fails, check console output for error messages.
