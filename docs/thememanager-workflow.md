# ThemeManager Workflow Documentation

This document explains how the thememanager system works in knov, from application startup through template rendering.

## Table of Contents
1. [Application Startup](#application-startup)
2. [ThemeManager Initialization](#thememanager-initialization)
3. [Theme Loading Process](#theme-loading-process)
4. [Template Parsing](#template-parsing)
5. [Request Handling & Rendering](#request-handling--rendering)
6. [Template Execution Flow](#template-execution-flow)

---

## 1. Application Startup

**File**: `main.go:26-43`

The application initializes in this specific order:

```go
func main() {
    // 1. Set embedded filesystems
    server.SetStaticFiles(staticFS)                           // static/* files
    thememanager.SetBuiltinThemeArchive(builtinThemeArchive) // themes/builtin.tar.gz
    
    // 2. Initialize configuration
    configmanager.InitAppConfig()      // Loads .env, sets config paths
    translation.Init()                 // Initializes translation system
    storage.Init(...)                  // Initializes metadata storage
    configmanager.InitUserSettings("default")
    translation.SetLanguage(...)       // Sets user language
    
    // 3. Initialize theme manager (this is where theme loading happens)
    thememanager.Init()
    
    // 4. Initialize search
    search.InitSearch()
    
    // 5. Start cron jobs and server
    cronjob.Start()
    server.StartServerChi()
}
```

### Key Points:
- **Line 26**: Embedded `themes/builtin.tar.gz` is registered globally
- **Line 37**: `thememanager.Init()` triggers theme loading and initialization
- **Line 35**: User's preferred theme is loaded from config (defaults to "builtin")

---

## 2. ThemeManager Initialization

**File**: `internal/thememanager/thememanager.go:94-141`

### Step 1: Create ThemeManager Instance

```go
// thememanager.go:88-92
func Init() {
    tm := NewThemeManager()  // Creates new instance with funcMap
    tm.Initialize()          // Loads themes
    globalThemeManager = tm  // Sets global singleton
}
```

The `NewThemeManager()` constructor creates a ThemeManager with a funcMap:

```go
// thememanager.go:100-108
func NewThemeManager() *ThemeManager {
    return &ThemeManager{
        themes: make(map[string]*Theme),
        funcMap: template.FuncMap{
            "T":   translation.Sprintf,     // Translation function for {{T "text"}}
            "add": func(a, b int) int { ... },
            "sub": func(a, b int) int { ... },
            "mul": func(a, b int) int { ... },
        },
    }
}
```

### Step 2: Initialize - Extract Builtin Theme

```go
// thememanager.go:94-141
func (tm *ThemeManager) Initialize() {
    themesDir := getThemesPath()  // Returns config/themes
    builtinPath := filepath.Join(themesDir, "builtin")
    
    // Check if builtin theme exists
    if _, err := os.Stat(filepath.Join(builtinPath, "theme.json")); os.IsNotExist(err) {
        // Extract from embedded archive
        archiveData, _ := builtinThemeArchive.ReadFile("themes/builtin.tar.gz")
        reader := bytes.NewReader(archiveData)
        tm.LoadThemeFromArchive("builtin", reader)  // Extracts to config/themes/builtin
    }
    
    // Load all themes from directory
    tm.loadAllThemes()
    
    // Ensure builtin exists
    if _, ok := tm.themes["builtin"]; !ok {
        panic("builtin theme is required but not found")
    }
    
    // Set current theme from user config
    currentTheme := configmanager.GetTheme()  // e.g., "builtin" or "simple"
    if currentTheme == "" {
        currentTheme = "builtin"
    }
    
    tm.SetCurrentTheme(currentTheme)
}
```

### Key Points:
- **First run**: Builtin theme is extracted from embedded `builtin.tar.gz` to `config/themes/builtin/`
- **Subsequent runs**: Builtin theme is already on disk and loaded directly
- **Theme switching**: User's theme preference is loaded from config (settings.json)

---

## 3. Theme Loading Process

**File**: `internal/thememanager/thememanager.go:143-179`

### Step 1: Load All Themes from Directory

```go
// thememanager.go:143-166
func (tm *ThemeManager) loadAllThemes() error {
    themesDir := getThemesPath()  // config/themes
    
    // Create directory if needed
    os.MkdirAll(themesDir, 0755)
    
    // Read all directories in config/themes/
    entries, _ := os.ReadDir(themesDir)
    
    for _, entry := range entries {
        if !entry.IsDir() {
            continue
        }
        
        themeName := entry.Name()  // e.g., "builtin", "simple"
        themePath := filepath.Join(themesDir, themeName)
        
        tm.loadTheme(themeName, themePath)
    }
    
    return nil
}
```

### Step 2: Load Individual Theme

```go
// thememanager.go:168-200
func (tm *ThemeManager) loadTheme(name string, path string) error {
    tm.mutex.Lock()
    defer tm.mutex.Unlock()
    
    // 1. Read theme.json
    metadataPath := filepath.Join(path, "theme.json")
    metadataBytes, _ := os.ReadFile(metadataPath)
    
    var metadata ThemeMetadata
    json.Unmarshal(metadataBytes, &metadata)
    
    // 2. Parse all templates
    templatesDir := filepath.Join(path, "templates")
    templates, _ := tm.parseTemplates(templatesDir)
    
    // 3. Create Theme object
    theme := &Theme{
        Name:      metadata.Name,
        Path:      path,
        Metadata:  metadata,
        Templates: templates,
    }
    
    // 4. Store in themes map
    tm.themes[metadata.Name] = theme
    
    return nil
}
```

### Theme Directory Structure:
```
config/themes/
├── builtin/
│   ├── theme.json          ← Metadata
│   ├── templates/
│   │   ├── base.html       ← Base wrapper
│   │   ├── home.html       ← Page templates
│   │   ├── fileview.html
│   │   ├── settings.html
│   │   └── ...
│   └── static/
│       └── css/style.css
└── simple/                 ← Additional themes
    └── ...
```

---

## 4. Template Parsing

**File**: `internal/thememanager/thememanager.go:202-252`

### How Templates are Parsed

Each page template (e.g., `home.html`, `fileview.html`) is combined with `base.html`:

```go
// thememanager.go:202-252
func (tm *ThemeManager) parseTemplates(templatesDir string) (ThemeTemplates, error) {
    var templates ThemeTemplates
    
    // 1. Read base template first
    baseContent, _ := os.ReadFile(filepath.Join(templatesDir, "base.html"))
    
    // 2. Helper function to parse page template with base
    parseTemplate := func(filename string) (*template.Template, error) {
        content, _ := os.ReadFile(filepath.Join(templatesDir, filename))
        
        // Create template with funcMap (includes T for translation)
        tmpl := template.New(filename).Funcs(tm.funcMap)
        
        // Add base.html to template
        tmpl.New("base.html").Parse(string(baseContent))
        
        // Add page-specific template
        tmpl.Parse(string(content))
        
        return tmpl, nil
    }
    
    // 3. Parse all templates
    templates.Base = template.Must(template.New("base.html").Funcs(tm.funcMap).Parse(string(baseContent)))
    templates.Home = template.Must(parseTemplate("home.html"))
    templates.FileView = template.Must(parseTemplate("fileview.html"))
    templates.Settings = template.Must(parseTemplate("settings.html"))
    // ... all 13 page templates
    
    return templates, nil
}
```

### Template Structure

Each page template defines a `content` block that gets inserted into `base.html`:

**base.html**:
```html
<!DOCTYPE html>
<html lang="{{.Language}}">
<head>
    <title>{{.Title}}</title>
    <script src="https://unpkg.com/htmx.org@2.0.4"></script>
</head>
<body>
    <div id="app-wrapper">
        {{template "header" .}}
        <main id="app-main">
            {{template "content" .}}  ← Page content inserted here
        </main>
    </div>
</body>
</html>
```

**fileview.html**:
```html
{{define "content"}}
<div id="page-fileview">
    <article>{{.FileContent.HTML}}</article>
    <aside>
        <span hx-get="/api/metadata/file/collection?filepath={{.FilePath | urlquery}}">
            {{T "Loading..."}}
        </span>
    </aside>
</div>
{{/define}}
```

### FuncMap Functions

The funcMap provides custom functions available in all templates:

- **`T`**: Translation function - `{{T "Settings"}}` → "Einstellungen" (in German)
- **`urlquery`**: URL encoding (built-in Go template function)
- **`add`, `sub`, `mul`**: Math operations
- **`eq`**: Equality comparison (built-in Go template function)

---

## 5. Request Handling & Rendering

**File**: `internal/server/server.go` (various handlers)

### HTTP Request Flow

```
Browser Request
    ↓
Chi Router
    ↓
Handler Function (e.g., handleFileView)
    ↓
isHTMXRequest() check
    ↓
┌─────────────┴─────────────┐
│                           │
HTMX Request              Normal Request
│                           │
tm.RenderContent()        tm.RenderPage()
│                           │
Returns <div id="page-*">  Returns full HTML with <html>, <head>, <body>
```

### Example Handler

```go
// server.go
func handleFileView(w http.ResponseWriter, r *http.Request) {
    tm := thememanager.GetThemeManager()
    
    // Prepare template data
    data := &FileViewData{
        TemplateData: NewTemplateData("File View"),
        FilePath:     "/notes/example.md",
        FileContent:  renderedContent,
    }
    
    // Render based on request type
    if err := renderPage(w, r, tm, "fileview.html", data, "fileview"); err != nil {
        http.Error(w, "failed to render page", http.StatusInternalServerError)
    }
}

// Helper function
func renderPage(w http.ResponseWriter, r *http.Request, tm thememanager.IThemeManager, 
                page string, data interface{}, pageName string) error {
    if isHTMXRequest(r) {
        return tm.RenderContent(w, page, data)  // Returns fragment
    }
    return tm.RenderPage(w, page, data)         // Returns full page
}

// HTMX detection
func isHTMXRequest(r *http.Request) bool {
    return r.Header.Get("HX-Request") == "true" || 
           r.Header.Get("X-Requested-With") == "HTMX"
}
```

---

## 6. Template Execution Flow

**File**: `internal/thememanager/thememanager.go:355-411`

### Full Page Rendering (Normal Requests)

```go
// thememanager.go:355-401
func (tm *ThemeManager) RenderPage(w io.Writer, page string, data interface{}) error {
    tm.mutex.RLock()
    defer tm.mutex.RUnlock()
    
    // 1. Get template by page name
    var tmpl *template.Template
    switch page {
    case "home.html":
        tmpl = tm.currentTheme.Templates.Home
    case "fileview.html":
        tmpl = tm.currentTheme.Templates.FileView
    case "settings.html":
        tmpl = tm.currentTheme.Templates.Settings
    // ... all 13 templates
    }
    
    // 2. Execute base.html template (includes content)
    var buf bytes.Buffer
    tmpl.ExecuteTemplate(&buf, "base.html", data)
    
    // 3. Write to response
    w.Write(buf.Bytes())
    return nil
}
```

**What gets rendered**:
```html
<!DOCTYPE html>
<html lang="en">
<head>
    <title>File View</title>
    <script src="https://unpkg.com/htmx.org@2.0.4"></script>
</head>
<body>
    <div id="app-wrapper">
        <nav>...</nav>
        <main id="app-main">
            <div id="page-fileview">
                <article>Content here...</article>
            </div>
        </main>
    </div>
</body>
</html>
```

### Content Fragment Rendering (HTMX Requests)

```go
// thememanager.go:403-450
func (tm *ThemeManager) RenderContent(w io.Writer, page string, data interface{}) error {
    tm.mutex.RLock()
    defer tm.mutex.RUnlock()
    
    // 1. Get template by page name (same as RenderPage)
    var tmpl *template.Template
    switch page {
    case "fileview.html":
        tmpl = tm.currentTheme.Templates.FileView
    // ... all templates
    }
    
    // 2. Execute ONLY the "content" template (not base.html)
    var buf bytes.Buffer
    tmpl.ExecuteTemplate(&buf, "content", data)
    
    // 3. Write to response
    w.Write(buf.Bytes())
    return nil
}
```

**What gets rendered** (only the fragment):
```html
<div id="page-fileview">
    <article>Content here...</article>
    <aside>
        <span hx-get="/api/metadata/...">Metadata</span>
    </aside>
</div>
```

### HTMX Swapping Process

When a user clicks an HTMX-enabled link:

```html
<a href="/files/example.md" 
   hx-get="/files/example.md" 
   hx-target="#app-main" 
   hx-push-url="true" 
   hx-swap="innerHTML swap:0.2s">
   example.md
</a>
```

1. **Browser**: Intercepts click, sends AJAX GET with `HX-Request: true` header
2. **Server**: `handleFileView()` detects HTMX, calls `RenderContent()`
3. **Response**: Returns only `<div id="page-fileview">...</div>` (5KB instead of 14KB)
4. **HTMX**: Swaps content into `#app-main`, updates URL
5. **User**: Sees new page without full reload

---

## Summary

The thememanager workflow follows this sequence:

1. **Startup**: Embedded `builtin.tar.gz` is registered
2. **Init**: ThemeManager extracts builtin theme (if needed) and loads all themes
3. **Parse**: Each theme's templates are parsed with `base.html` and funcMap
4. **Load**: User's preferred theme is set as current
5. **Request**: HTTP handler prepares template data
6. **Render**: 
   - Normal request → `RenderPage()` → Full HTML with `<html>`, `<head>`, `<body>`
   - HTMX request → `RenderContent()` → Fragment with `<div id="page-*">`
7. **Execute**: Template engine processes `{{T "text"}}`, `{{.Data}}`, and custom functions
8. **Response**: HTML is written to response writer

This architecture provides:
- **Separation of concerns**: Templates, logic, and data are separate
- **Theme switching**: Easy to change themes at runtime
- **Progressive enhancement**: Works with and without JavaScript
- **Performance**: HTMX reduces payload by 65% (14KB → 5KB)
- **Internationalization**: `{{T "text"}}` automatically translates based on user language
