# Creating a New Theme

This guide explains how to create a custom theme for the Go theme system with multiple required templates.

## Required Files for Each Theme

Every theme **must** include these files:
- `base.gotmpl` - Main/home page template
- `history.gotmpl` - History page template  
- `fileview.gotmpl` - File browser page template
- `theme.json` - Theme metadata

## Step-by-Step Theme Creation

### 1. Create Theme Directory
Create a new folder in the `themes/` directory with your theme name:
```bash
mkdir themes/mytheme
```

### 2. Create Theme Metadata (`theme.json`)
Create a `theme.json` file in your theme directory:

```json
{
  "name": "My Custom Theme",
  "version": "1.0.0",
  "author": "Your Name",
  "description": "A beautiful custom theme"
}
```

### 3. Create Required Templates

You **must** create all three template files. Missing any template will cause the theme to fail loading.

#### `base.gotmpl` - Main Page Template
```html
<!DOCTYPE html>
<html>
<head>
    <title>{{.Title}} - {{.CurrentMetadata.Name}}</title>
    <style>/* Your styles here */</style>
</head>
<body>
    <div class="container">
        <h1>{{.Heading}} - Base Page</h1>
        
        <!-- Navigation (required for switching pages) -->
        <div class="nav">
            <a href="/?page=base">Base</a>
            <a href="/?page=history">History</a>
            <a href="/?page=fileview">File View</a>
        </div>
        
        <!-- Theme selector (required) -->
        <form method="POST">
            <select name="theme">
                {{range .AvailableThemes}}
                <option value="{{.}}" {{if eq . $.CurrentTheme}}selected{{end}}>{{.}}</option>
                {{end}}
            </select>
            <button type="submit">Change Theme</button>
        </form>

        <p>{{.Message}}</p>
        <p>Time: {{.Time.Format "2006-01-02 15:04:05"}}</p>
    </div>
</body>
</html>
```

#### `history.gotmpl` - History Page Template
```html
<!DOCTYPE html>
<html>
<head>
    <title>{{.Title}} - History - {{.CurrentMetadata.Name}}</title>
    <style>/* Your styles here */</style>
</head>
<body>
    <div class="container">
        <h1>{{.Heading}} - History</h1>
        
        <!-- Navigation with active state -->
        <div class="nav">
            <a href="/?page=base">Base</a>
            <a href="/?page=history" class="active">History</a>
            <a href="/?page=fileview">File View</a>
        </div>
        
        <!-- Theme selector -->
        <form method="POST">
            <select name="theme">
                {{range .AvailableThemes}}
                <option value="{{.}}" {{if eq . $.CurrentTheme}}selected{{end}}>{{.}}</option>
                {{end}}
            </select>
            <button type="submit">Change Theme</button>
        </form>

        <h2>Recent Activity</h2>
        {{range .Items}}
        <div class="history-item">
            <div class="timestamp">{{$.Time.Format "2006-01-02 15:04:05"}}</div>
            <div>{{.}}</div>
        </div>
        {{end}}
        
        <p>{{.Message}} - History view active</p>
    </div>
</body>
</html>
```

#### `fileview.gotmpl` - File View Template
```html
<!DOCTYPE html>
<html>
<head>
    <title>{{.Title}} - File View - {{.CurrentMetadata.Name}}</title>
    <style>/* Your styles here */</style>
</head>
<body>
    <div class="container">
        <h1>{{.Heading}} - File View</h1>
        
        <!-- Navigation with active state -->
        <div class="nav">
            <a href="/?page=base">Base</a>
            <a href="/?page=history">History</a>
            <a href="/?page=fileview" class="active">File View</a>
        </div>
        
        <!-- Theme selector -->
        <form method="POST">
            <select name="theme">
                {{range .AvailableThemes}}
                <option value="{{.}}" {{if eq . $.CurrentTheme}}selected{{end}}>{{.}}</option>
                {{end}}
            </select>
            <button type="submit">Change Theme</button>
        </form>

        <h2>File Browser</h2>
        <div class="file-browser">
            {{range .Items}}
            <div class="file-item">
                <span><span class="file-icon">📄</span>{{.}}</span>
                <span class="timestamp">{{$.Time.Format "15:04"}}</span>
            </div>
            {{end}}
        </div>
        
        <p>{{.Message}} - File view active</p>
    </div>
</body>
</html>
```

### 4. Test Your Theme

1. **Start the application:**
   ```bash
   go run .
   ```

2. **Check the console output:**
   - Look for successful theme loading messages
   - Check for any error messages about missing templates

3. **Test all pages:**
   - Base: `http://localhost:1325/?page=base`
   - History: `http://localhost:1325/?page=history`
   - File View: `http://localhost:1325/?page=fileview`

4. **Verify theme switching:**
   - Use the dropdown on each page
   - Ensure all templates work with your theme

## Error Handling & Debugging

### Missing Template Files
If you're missing any required template files, you'll see error messages like:
```
[ThemeManager] Failed to load theme 'mytheme': theme 'mytheme' validation failed: missing required file: history.gotmpl
```

### Template Parsing Errors
If your template syntax is invalid:
```
[ThemeManager] Failed to load theme 'mytheme': failed to parse templates: template: mytheme:5: unexpected "}" in command
```

### Runtime Template Errors
If a template exists but fails during rendering:
```
Render error: template 'history.gotmpl' not found in theme 'mytheme'
```

## Available Template Data

All templates receive this data structure:

```go
// Basic content data
.Title      // Page title
.Heading    // Main heading
.Message    // Dynamic message (changes per page)
.Time       // Current timestamp
.Items      // List of items to display

// Theme-specific data
.CurrentTheme          // Name of current theme
.CurrentMetadata.Name  // Theme display name
.CurrentMetadata.Version     // Theme version
.CurrentMetadata.Author      // Theme author
.CurrentMetadata.Description // Theme description
.AvailableThemes      // List of all available themes
```

## Best Practices

1. **Include navigation** on all pages for easy switching
2. **Show active page state** in navigation
3. **Keep consistent styling** across all templates
4. **Test all pages** before considering theme complete
5. **Use semantic CSS classes** for maintainability
6. **Include theme selector** on every page
7. **Handle responsive design** for all templates

## Troubleshooting

- **Theme not loading?** Check console for specific error messages
- **Missing templates?** Ensure all 3 .gotmpl files are present and named correctly
- **Parse errors?** Verify Go template syntax in all files
- **Styling issues?** Check CSS syntax within `<style>` tags
- **Navigation broken?** Ensure all page links use `?page=` parameter

The theme system now provides detailed error logging to help identify exactly what's wrong when themes fail to load.
