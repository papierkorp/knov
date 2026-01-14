# Configuration Guide

KNOV supports configuration through environment variables and configuration files.

## Environment Variables

### Server Configuration

- `KNOV_SERVER_PORT`: HTTP server port (default: 1324)
- `KNOV_LOG_LEVEL`: Logging level (debug, info, warning, error)

### Path Configuration

- `KNOV_DATA_PATH`: Directory containing your knowledge base files (default: "data")
- `KNOV_THEMES_PATH`: Directory containing theme .so files (default: "themes")
- `KNOV_CONFIG_PATH`: Directory for configuration and user settings (default: "config")

#### Path Normalization

The application automatically handles path normalization using utility functions:
- **Documents**: Stored in `{KNOV_DATA_PATH}/docs/`
- **Media**: Stored in `{KNOV_DATA_PATH}/media/`

**Utility Functions:**
- `utils.StripPathPrefix(path, prefix)` - Generic prefix removal
- `utils.NormalizeDocsPath(path)` - Removes "docs/" prefix
- `utils.NormalizeMediaPath(path)` - Removes "media/" prefix

When specifying file paths in API calls, you can use either:
- Relative paths: `"ai.md"`, `"projects/file.md"`
- Prefixed paths: `"docs/ai.md"`, `"media/image.jpg"`

The system automatically strips duplicate prefixes to prevent path issues like `data/docs/docs/file.md`.

### Storage Configuration

- `KNOV_STORAGE_METHOD`: Storage backend for metadata (default: "json")

Metadata files are created in `<knov executable>./metadata/`

User settings and dashboards are stored as JSON files in `{KNOV_CONFIG_PATH}/`

## Metadata Browsing

KNOV provides browsing capabilities for the following metadata fields on the overview page (`/overview`):

- **Tags**: Browse files by tags
- **Collections**: Browse files by collection
- **Folders**: Browse files by folder
- **Type**: Browse files by type (todo, fleeting, literature, moc, permanent, filter, journaling)
- **Priority**: Browse files by priority (low, medium, high)
- **Status**: Browse files by status (draft, published, archived)
- **PARA Organization**: Browse files by projects, areas, resources, and archive

Each browse link shows the count of files with that metadata value. Clicking a browse link displays all files with that specific metadata.

### Git Configuration

- `KNOV_GIT_REPOSITORY`: Git repository URL for your knowledge base

## Configuration Files

User settings are stored in JSON format at:
`{KNOV_CONFIG_PATH}/settings.json`

Example user settings:

```json
{
  "theme": "builtin",
  "language": "en",
  "themeSettings": {
    "builtin": {
      "darkMode": false,
      "colorScheme": "green",
      "customCSS": "",
      "fileView": "detailed"
    },
    "myCustomTheme": {
      "customColor": "#ff0000",
      "fontSize": 16,
      "enableFeature": true
    }
  }
}
```

### Theme Settings

Theme-specific settings are stored under `themeSettings` as key-value pairs, with the theme name as the key. Settings structure is completely generic - themes define their settings in `theme.json` and the app stores them as-is without any predefined structure. This allows theme creators to define any settings they need.

## Deployment Examples

### Development

```bash
export KNOV_LOG_LEVEL=debug
go run .
```

### Production with Custom Paths

```bash
export KNOV_DATA_PATH=/var/lib/knov/data
export KNOV_THEMES_PATH=/usr/local/knov/themes
export KNOV_CONFIG_PATH=/etc/knov
export KNOV_SERVER_PORT=8080
./knov
```

### Docker Deployment

```dockerfile
ENV KNOV_DATA_PATH=/app/data
ENV KNOV_CONFIG_PATH=/app/config
ENV KNOV_THEMES_PATH=/app/themes
VOLUME ["/app/data", "/app/config"]
```

## Theme Configuration

Themes are automatically discovered from `{KNOV_THEMES_PATH}/*.so` files.

The builtin theme is always available and embedded in the binary.

Upload new themes via the admin interface at `/admin`.

### Theme Overrides

You can override individual templates from any active theme by placing custom template files in the `themes/overwrite/` directory. This allows you to modify specific pages without creating a complete custom theme.

**Directory Structure:**
```
themes/
ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â€šÂ¬Ã‚ÂÃƒâ€¦Ã¢â‚¬Å“ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â€šÂ¬Ã‚ÂÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â€šÂ¬Ã‚ÂÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬ overwrite/
ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â€šÂ¬Ã‚ÂÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡   ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â€šÂ¬Ã‚ÂÃƒâ€¦Ã¢â‚¬Å“ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â€šÂ¬Ã‚ÂÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â€šÂ¬Ã‚ÂÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬ base.gohtml          # Override base template
ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â€šÂ¬Ã‚ÂÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡   ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â€šÂ¬Ã‚ÂÃƒâ€¦Ã¢â‚¬Å“ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â€šÂ¬Ã‚ÂÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â€šÂ¬Ã‚ÂÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬ settings.gohtml      # Override settings template
ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â€šÂ¬Ã‚ÂÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡   ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â€šÂ¬Ã‚ÂÃƒâ€¦Ã¢â‚¬Å“ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â€šÂ¬Ã‚ÂÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â€šÂ¬Ã‚ÂÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬ fileview.gohtml      # Override file view template
ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â€šÂ¬Ã‚ÂÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡   ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â€šÂ¬Ã‚ÂÃƒÂ¢Ã¢â€šÂ¬Ã‚ÂÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â€šÂ¬Ã‚ÂÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â€šÂ¬Ã‚ÂÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬ ...                  # Other template overrides
ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â€šÂ¬Ã‚ÂÃƒÂ¢Ã¢â€šÂ¬Ã‚ÂÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â€šÂ¬Ã‚ÂÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â€šÂ¬Ã‚ÂÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬ your-theme/
    ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â€šÂ¬Ã‚ÂÃƒÂ¢Ã¢â€šÂ¬Ã‚ÂÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â€šÂ¬Ã‚ÂÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â€šÂ¬Ã‚ÂÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬ ...
```

**How it works:**
1. Place your custom template files in `themes/overwrite/` using the same filename as the original template
2. Template files should have the `.gohtml` extension
3. When rendering a page, the system first checks for an overwrite template
4. If found and valid, the overwrite template is used instead of the theme's template
5. If the overwrite template has errors, the system falls back to the original theme template

**Example Override:**
Create `themes/overwrite/base.gohtml` to customize the base template:

```html
<!DOCTYPE html>
<html>
  <head>
    <title>{{.Title}} - Custom Override</title>
    <link href="/themes/{{.CurrentTheme}}/style.css" rel="stylesheet" />
  </head>
  <body>
    <header>My Custom Header</header>
    <main>
      <!-- Your custom content here -->
    </main>
  </body>
</html>
```

**Available Templates to Override:**
- `base.gohtml` - Base layout template
- `settings.gohtml` - Settings page
- `fileview.gohtml` - File viewing page
- `fileedit.gohtml` - File editing page
- `browsefiles.gohtml` - File browser
- `search.gohtml` - Search results
- `dashboard*.gohtml` - Dashboard templates
- And others as defined by your theme

Template overrides must follow the same structure and data expectations as the original templates.
