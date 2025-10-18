# Theme Creator Guide

Create custom themes for KNOV using HTML templates and Go's `html/template` engine. Package themes as `.tar.gz` archives for easy distribution.

## Quick Start

```bash
mkdir -p mytheme/templates mytheme/static/css
# Create your theme files (see structure below)
cd mytheme
# Package as .tar.gz
tar -czf mytheme.tar.gz .
# Upload via admin interface or place in config/themes/
```

## Theme Structure

```
mytheme/
├── theme.json              # Theme metadata (required)
├── templates/              # HTML templates (required)
│   ├── base.html          # Base layout template
│   ├── home.html          # Home page
│   ├── fileview.html      # File viewing
│   ├── search.html        # Search page
│   ├── settings.html      # Settings page
│   ├── admin.html         # Admin page
│   ├── dashboard.html     # Dashboard page
│   ├── playground.html    # Playground page
│   ├── overview.html      # Overview page
│   ├── history.html       # History page
│   ├── latestchanges.html # Latest changes page
│   ├── browsefiles.html   # Browse files page
│   └── fileedit.html      # File editor
└── static/                 # Static assets (optional)
    ├── css/
    │   └── theme.css
    ├── js/
    │   └── theme.js
    └── fonts/
```

## Theme Metadata (theme.json)

Required configuration file:

```json
{
  "name": "mytheme",
  "version": "1.0.0",
  "author": "Your Name",
  "description": "My custom theme for KNOV",
  "views": {
    "home": ["default"],
    "fileview": ["detailed", "compact", "reader"],
    "search": ["default"],
    "overview": ["default"],
    "dashboard": ["default"],
    "settings": ["default"],
    "admin": ["default"],
    "playground": ["default"],
    "history": ["default"],
    "latestchanges": ["default"],
    "browsefiles": ["default"],
    "fileedit": ["default"]
  },
  "features": {
    "darkMode": true,
    "colorSchemes": [
      {"name": "default", "label": "Ocean Blue"},
      {"name": "green", "label": "Forest Green"}
    ]
  },
  "templates": {
    "base": "templates/base.html",
    "home": "templates/home.html",
    "fileview": "templates/fileview.html",
    "search": "templates/search.html",
    "overview": "templates/overview.html",
    "dashboard": "templates/dashboard.html",
    "settings": "templates/settings.html",
    "admin": "templates/admin.html",
    "playground": "templates/playground.html",
    "history": "templates/history.html",
    "latestchanges": "templates/latestchanges.html",
    "browsefiles": "templates/browsefiles.html",
    "fileedit": "templates/fileedit.html"
  }
}
```

## Template System

KNOV uses Go's `html/template` package. All templates have access to common data and helper functions.

### Template File Format

**Use `.html` file extensions** (recommended)

**Why `.html`?**
- ✅ Better IDE support (syntax highlighting, auto-completion)
- ✅ Clear intent - it's HTML with template directives
- ✅ Frontend developer friendly
- ✅ Better tooling support (formatters, linters)

**Alternative: `.tmpl` extension**
- Also acceptable, common in Go projects
- Less IDE support for HTML syntax

**Not recommended: `.go` files**
- ❌ No HTML tooling support
- ❌ Templates as strings are harder to maintain
- ❌ KNOV's thememanager expects template files, not Go code

All templates are plain HTML files with Go template directives (`{{ }}`) embedded.

### Available Template Data

Every page template receives a `TemplateData` struct with these fields:

```go
{
    Title              string              // Page title
    ThemeName          string              // Current theme name
    DarkMode           bool                // Dark mode enabled
    ColorScheme        string              // Current color scheme
    Language           string              // Current language code
    T                  func(string) string // Translation function
    View               string              // Current view variant
    CustomCSSEditor    template.HTML       // Custom CSS editor HTML
    AvailableLanguages []Language          // Available language options
    CurrentLanguage    string              // Current language
    GitRepoURL         string              // Git repository URL
    DataPath           string              // Data directory path
    
    // Page-specific fields
    Query              string              // For search page
    FileContent        *FileContent        // For file view
    FilePath           string              // For file view/edit
    Dashboard          *Dashboard          // For dashboard
    MetadataType       string              // For browse files
    Value              string              // For browse files
}
```

### Template Functions

#### Built-in Go Template Functions

Go's `html/template` provides these out of the box:

- `{{eq .A .B}}` - Check equality (also works with multiple args: `eq .A .B .C`)
- `{{ne .A .B}}` - Check inequality
- `{{lt .A .B}}` - Less than
- `{{le .A .B}}` - Less than or equal
- `{{gt .A .B}}` - Greater than
- `{{ge .A .B}}` - Greater than or equal
- `{{and .A .B}}` - Logical AND
- `{{or .A .B}}` - Logical OR
- `{{not .A}}` - Logical NOT

#### KNOV Custom Functions

Additional functions provided by KNOV:

- `{{.T "translation.key"}}` - Translate text (REQUIRED for i18n)
- `{{add .A .B}}` - Add numbers (e.g., `{{add .Index 1}}`)
- `{{sub .A .B}}` - Subtract numbers (e.g., `{{sub .Level 1}}`)
- `{{mul .A .B}}` - Multiply numbers (e.g., `{{mul .Indent 12}}`)

**Example usage:**
```html
<!-- Calculate nested indentation -->
<div style="padding-left: {{mul (sub .Level 1) 20}}px;">

<!-- Translation -->
<h1>{{.T "welcome.title"}}</h1>

<!-- Comparisons -->
{{if eq .Status "active"}}Active{{end}}
{{if gt .Count 0}}{{.Count}} items{{end}}
```

## Base Template (base.html)

The base template defines the overall page structure:

```html
<!DOCTYPE html>
<html>
<head>
    <title>{{.Title}}</title>
    <meta charset="UTF-8"/>
    <meta name="viewport" content="width=device-width, initial-scale=1.0"/>
    <link href="/static/css/style.css" rel="stylesheet"/>
    <link href="/static/css/custom.css" rel="stylesheet"/>
    <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/7.0.1/css/all.min.css"/>
</head>
<body
    data-theme="{{.ThemeName}}"
    data-dark-mode="{{.DarkMode}}"
    data-color-scheme="{{.ColorScheme}}"
    data-language="{{.Language}}"
>
    <div id="wrapper">
        {{template "header" .}}
        <main>
            {{template "content" .}}
        </main>
    </div>
    <script src="/static/htmx.min.js"></script>
</body>
</html>

{{define "header"}}
<!-- Your header content -->
{{end}}

{{define "content"}}
<!-- Overridden by page templates -->
{{end}}
```

## Page Templates

Each page template must define a `content` block:

### Example: home.html

```html
{{define "content"}}
<div id="page-home">
    <h1>{{.T "Welcome"}}</h1>
    <section class="overview-browse">
        <h2>{{.T "Browse by Metadata"}}</h2>
        <div class="browse-grid">
            <div class="browse-section">
                <h3>{{.T "Tags"}}</h3>
                <div hx-get="/api/metadata/tags" hx-trigger="load">
                    loading tags...
                </div>
            </div>
        </div>
    </section>
</div>
{{end}}
```

### Example: fileview.html with View Variants

```html
{{define "content"}}
{{if eq .View "detailed"}}
    {{template "fileview-detailed" .}}
{{else if eq .View "compact"}}
    {{template "fileview-compact" .}}
{{else if eq .View "reader"}}
    {{template "fileview-reader" .}}
{{else}}
    {{template "fileview-detailed" .}}
{{end}}
{{end}}

{{define "fileview-detailed"}}
<div id="view-fileview-detailed">
    <div class="fileview-layout">
        <article class="file-content">
            {{.FileContent.HTML}}
        </article>
        <aside class="file-sidebar">
            <h3>metadata</h3>
            <span hx-get="/api/metadata/file/tags?filepath={{.FilePath | urlquery}}" hx-trigger="load">...</span>
        </aside>
    </div>
</div>
{{end}}

{{define "fileview-compact"}}
<!-- Compact view layout -->
{{end}}

{{define "fileview-reader"}}
<!-- Reader view layout -->
{{end}}
```

## HTMX Integration

KNOV extensively uses HTMX for dynamic content. Your theme should leverage HTMX attributes:

```html
<!-- Load content on page load -->
<div hx-get="/api/metadata/tags" hx-trigger="load">loading...</div>

<!-- Submit form on change -->
<select name="theme" hx-post="/api/themes/setTheme" hx-trigger="change">
    <option>builtin</option>
</select>

<!-- Search with debounce -->
<input
    hx-get="/api/search"
    hx-trigger="keyup changed delay:300ms"
    hx-target="#results"
    name="q"
/>
```

## Styling

### Using CSS Variables

Access theme settings via data attributes:

```css
body[data-dark-mode="true"] {
    background: #1a1a1a;
    color: #ffffff;
}

body[data-color-scheme="green"] {
    --primary: #2d5016;
    --accent: #4a7c2e;
}

body[data-theme="mytheme"] {
    /* Theme-specific styles */
}
```

### Including Theme CSS

Reference your CSS files:

```html
<link href="/themes/mytheme/static/css/theme.css" rel="stylesheet"/>
```

## Translation Support

Use the `T` function for all user-facing text:

```html
<h1>{{.T "Welcome"}}</h1>
<button>{{.T "Save"}}</button>
<p>{{.T "Are you sure?"}}</p>
```

## Packaging Your Theme

### Create the Archive

```bash
cd mytheme
tar -czf mytheme.tar.gz theme.json templates/ static/
```

### Installation Methods

#### Method 1: Upload via Admin Interface

1. Navigate to `/admin`
2. Scroll to "Theme Management"
3. Click "Choose File" and select `mytheme.tar.gz`
4. Click "Upload Theme"
5. Theme will be extracted to `config/themes/mytheme/`
6. Switch to your theme in Settings

#### Method 2: Manual Installation

```bash
# Extract directly to themes directory
mkdir -p config/themes/mytheme
tar -xzf mytheme.tar.gz -C config/themes/mytheme/
# Restart KNOV to load the theme
```

#### Method 3: Command Line Upload

```bash
curl -F "file=@mytheme.tar.gz" http://localhost:1324/api/themes/upload
```

### Theme Location

All themes (including builtin) are stored in:
```
config/themes/
  ├── builtin/          # Automatically extracted from binary
  ├── mytheme/          # Your custom theme
  └── another-theme/    # Another custom theme
```

## Best Practices

### 1. **Keep Templates Simple**
- Focus on structure, not heavy logic
- Use HTMX for dynamic content instead of complex template logic

### 2. **Support Dark Mode**
- Always check `{{.DarkMode}}` and style accordingly
- Use CSS variables for easy color switching

### 3. **Maintain Accessibility**
- Use semantic HTML elements
- Include ARIA labels where needed
- Ensure keyboard navigation works

### 4. **Performance**
- Minimize inline styles
- Optimize images and assets
- Use CSS for animations instead of JavaScript

### 5. **Mobile Responsive**
- Include viewport meta tag
- Use responsive CSS (flexbox, grid)
- Test on multiple screen sizes

## Example: Minimal Theme

**theme.json:**
```json
{
  "name": "minimal",
  "version": "1.0.0",
  "author": "KNOV Team",
  "description": "Minimal clean theme",
  "views": {
    "home": ["default"],
    "fileview": ["default"],
    "search": ["default"],
    "overview": ["default"],
    "dashboard": ["default"],
    "settings": ["default"],
    "admin": ["default"],
    "playground": ["default"],
    "history": ["default"],
    "latestchanges": ["default"],
    "browsefiles": ["default"],
    "fileedit": ["default"]
  },
  "features": {
    "darkMode": true,
    "colorSchemes": [{"name": "default", "label": "Default"}]
  }
}
```

**Note**: The `templates` field in `theme.json` is optional. KNOV automatically loads all `.html` files from the `templates/` directory.

**templates/base.html:**
```html
<!DOCTYPE html>
<html>
<head>
    <title>{{.Title}}</title>
    <meta charset="UTF-8"/>
    <meta name="viewport" content="width=device-width, initial-scale=1.0"/>
    <style>
        body { font-family: sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }
        body[data-dark-mode="true"] { background: #1a1a1a; color: #fff; }
    </style>
</head>
<body data-dark-mode="{{.DarkMode}}">
    {{template "content" .}}
</body>
</html>
```

**templates/home.html:**
```html
{{define "content"}}
<h1>{{.T "welcome.title"}}</h1>
<p>{{.T "minimal.description"}}</p>
{{end}}
```

Package it:
```bash
tar -czf minimal.tar.gz theme.json templates/
```

## Troubleshooting

### Theme Won't Load

- Verify `theme.json` is valid JSON
- Ensure all template paths in `theme.json` match actual files
- Check that all required templates are present

### Templates Not Rendering

- Verify each template defines a `{{define "content"}}` block
- Check for syntax errors in templates
- Review server logs for specific error messages

### Styles Not Applying

- Confirm CSS files are in the correct directory
- Check CSS file paths in templates
- Verify CSS selectors match rendered HTML

## Support

For questions or issues:
- Check the KNOV documentation
- Review the builtin theme source code as a reference
- Report bugs at the project repository

## Advanced Topics

### Custom JavaScript

Add interactivity beyond HTMX:

```html
<script>
function customFeature() {
    // Your custom code
}
</script>
```

### External Fonts

Include web fonts:

```html
<link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;600&display=swap" rel="stylesheet"/>
```

### Custom Layouts

Create multiple layout variants:

```json
"views": {
    "fileview": ["default", "compact", "reader", "custom"]
}
```

Then handle in your template:

```html
{{if eq .View "custom"}}
    {{template "fileview-custom" .}}
{{end}}
```

## Theme Gallery

Visit the KNOV theme gallery to see examples and download community themes: [link to gallery]

---

**Happy theming!** 🎨
