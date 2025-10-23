# thememanager flow explanation

comprehensive walkthrough of how the new thememanager works, starting from a user visiting `/home`:

## 🌟 complete flow: user visits `/home`

### 1. **app startup** (happens once)
```go
// main.go calls thememanager.Init()
func Init() {
    // 1. create thememanager with themes path
    globalThemeManager = NewThemeManager(configmanager.GetThemesPath())
    
    // 2. extract builtin theme if missing
    extractBuiltinTheme() // extracts from embedded files to themes/builtin/
    
    // 3. load all themes from filesystem
    LoadThemes() // scans themes/ directory
    
    // 4. set current theme (from config or default to builtin)
    SetTheme(configmanager.GetTheme() || "builtin")
}
```

### 2. **theme loading process** (for each theme found)
```go
func (tm *ThemeManager) loadTheme(name, path string) (*Theme, error) {
    // 1. create theme struct
    theme := &Theme{Name: name, Path: path}
    
    // 2. load metadata (theme.json) with defaults
    metadata := ThemeMetadata{...} // defaults applied
    json.Unmarshal(themeJsonData, &metadata)
    tm.setDefaultViews(&metadata.Views) // ensure "default" if empty
    
    // 3. validate theme structure
    ValidateTheme(name, path) // checks required files exist
    
    // 4. load templates with fallback system
    template := tm.loadTemplatesWithFallback(name, path)
    // - tries to load from theme directory first
    // - falls back to builtin for missing templates
    // - adds FuncMap with translation functions
    
    // 5. store completed theme
    tm.themes[name] = theme
}
```

### 3. **user visits `/home`** 
```
GET /home HTTP/1.1
Host: localhost:1324
```

### 4. **chi router matches** 
```go
// server.go
r.Get("/home", handleHome)
```

### 5. **handleHome executes**
```go
func handleHome(w http.ResponseWriter, r *http.Request) {
    // 1. get theme manager instance
    tm := thememanager.GetThemeManager()
    
    // 2. create structured content for home page
    content := thememanager.HomeContent{
        Title:       "home",
        WelcomeText: "welcome to your knowledge management system",
        QuickActions: []thememanager.QuickAction{
            {Name: "browse files", URL: "/overview", Icon: "📁"},
            {Name: "search", URL: "/search", Icon: "🔍"},
            {Name: "dashboard", URL: "/dashboard", Icon: "📊"},
        },
    }
    
    // 3. render with theme manager
    tm.Render(w, "home.gotmpl", "default", content)
}
```

### 6. **thememanager.Render executes**
```go
func (tm *ThemeManager) Render(w http.ResponseWriter, templateName, viewName string, content interface{}) error {
    // 1. get current theme (with mutex protection)
    tm.mutex.RLock()
    theme := tm.themes[tm.currentTheme] // e.g., "builtin"
    tm.mutex.RUnlock()
    
    // 2. determine actual template to use
    actualTemplateName := templateName // "home.gotmpl"
    if viewName != "default" {
        // try view-specific template: "home_compact.gotmpl"
        viewTemplate := "home_" + viewName + ".gotmpl"
        if tm.HasTemplate(viewTemplate) {
            actualTemplateName = viewTemplate
        }
    }
    
    // 3. check template exists
    if !theme.Template.Lookup(actualTemplateName) {
        return TemplateNotFoundError{...}
    }
    
    // 4. prepare template data
    templateData := TemplateData{
        Theme:    "builtin",
        Language: "en", // from config
        DarkMode: false, // from config
        Content:  content, // HomeContent struct
    }
    
    // 5. execute template
    w.Header().Set("Content-Type", "text/html")
    return theme.Template.ExecuteTemplate(w, actualTemplateName, templateData)
}
```

### 7. **template execution process**
```go
// builtin/home.gotmpl gets executed with templateData
{{template "base.gotmpl" .}}
{{define "content"}}
<div id="page-home">
    <section class="hero">
        <h1>{{.T "welcome to knov"}}</h1> <!-- translation function -->
        <p>{{.Content.WelcomeText}}</p>    <!-- from HomeContent -->
        {{range .Content.QuickActions}}    <!-- iterate QuickActions -->
            <a href="{{.URL}}">{{.Name}} {{.Icon}}</a>
        {{end}}
    </section>
</div>
{{end}}
```

### 8. **base template renders**
```html
<!-- builtin/base.gotmpl -->
<!DOCTYPE html>
<html lang="en">
<head>
    <title>home - knov</title>
    <link href="/static/css/style.css" rel="stylesheet"> <!-- theme CSS -->
</head>
<body data-theme="builtin" data-dark-mode="false">
    <header>...</header>
    <main>
        <!-- content from home.gotmpl gets inserted here -->
        <div id="page-home">
            <section class="hero">
                <h1>welcome to knov</h1>
                <p>welcome to your knowledge management system</p>
                <a href="/overview">browse files 📁</a>
                <a href="/search">search 🔍</a>
                <a href="/dashboard">dashboard 📊</a>
            </section>
        </div>
    </main>
</body>
</html>
```

### 9. **css loading process** (when browser requests `/static/css/style.css`)
```go
func handleStatic(w http.ResponseWriter, r *http.Request) {
    if filePath == "css/style.css" {
        // 1. get current theme
        tm := thememanager.GetThemeManager()
        theme := tm.GetCurrentTheme() // "builtin"
        
        // 2. read theme's style.css
        cssPath := filepath.Join(theme.Path, "style.css")
        // e.g., "themes/builtin/style.css"
        
        data, err := os.ReadFile(cssPath)
        w.Write(data) // serve theme-specific CSS
    }
}
```

---

## 🔄 **key features in action**

### **theme switching** (user changes theme in settings)
```go
// 1. user selects new theme in settings
POST /api/themes/setTheme
Content-Type: application/x-www-form-urlencoded
theme=mytheme

// 2. API handler updates theme
func handleAPISetTheme(w http.ResponseWriter, r *http.Request) {
    theme := r.FormValue("theme")
    tm.SetTheme(theme) // atomic switch with mutex
    configmanager.SetTheme(theme) // persist choice
}

// 3. next page load uses new theme
// - no restart required
// - templates reload automatically
// - CSS switches immediately
```

### **template overwrite system**
```
themes/
├── builtin/           (complete theme)
│   ├── home.gotmpl
│   ├── fileview.gotmpl
│   ├── settings.gotmpl
│   └── style.css
└── mytheme/           (partial theme - only overrides)
    ├── home.gotmpl    (custom home page)
    └── style.css      (custom styles)
    
    # missing templates fall back to builtin automatically
```

### **view variants**
```go
// theme supports multiple file views
"views": {
    "file": ["default", "detailed", "compact", "reader"]
}

// user can switch between views
configmanager.SetFileView("detailed")

// render uses view-specific template if available
tm.Render(w, "fileview.gotmpl", "detailed", content)
// tries: fileview_detailed.gotmpl → fileview.gotmpl (fallback)
```

### **function availability in templates**
```go
// functions added via FuncMap
tmpl.Funcs(template.FuncMap{
    "T": translation.Sprintf, // translation
})

// usage in templates
{{.T "welcome"}}           // → "welcome" (en) / "willkommen" (de)
{{.T "files: %d" .Count}}  // → "files: 42"
```

---

## 🏗️ **architecture benefits**

### **simplicity**
- ✅ no compilation required for themes
- ✅ standard go html/template syntax
- ✅ file-based theme loading
- ✅ runtime theme switching

### **flexibility** 
- ✅ partial theme overrides
- ✅ multiple view variants per template
- ✅ structured content types
- ✅ theme-specific features

### **safety**
- ✅ always working builtin theme
- ✅ validation on theme load
- ✅ fallback to builtin for missing templates
- ✅ thread-safe theme switching

### **maintainability**
- ✅ clean separation of concerns
- ✅ typed content structures
- ✅ proper error handling
- ✅ embedded default theme

the new system is much simpler than the old templ+plugin approach while being more powerful and flexible!
