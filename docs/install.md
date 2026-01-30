# Installation

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

# Configuration

## Environment Variables

KNOV is configured via ENV Variables - you can get a look at all available variables at the .env.example file

Some important variables are:

|        ENV         |                           description                            |
|--------------------|------------------------------------------------------------------|
| `KNOV_LOG_LEVEL`   | Logging level (debug, info, warning, error)                      |
| `KNOV_DATA_PATH`   | Directory containing your knowledge base files (default: "data") |
| `KNOV_THEMES_PATH` | Directory containing theme .so files (default: "themes")         |
| `KNOV_CONFIG_PATH` | Directory for configuration and user settings (default: "config" |
| `KNOV_GIT_REPOSITORY`| Git repository URL for your knowledge base, there will be a empty one per default |



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

## Theme Settings

Theme-specific settings are stored under `themeSettings` in the `settings.json` file, with the theme name as the key. 
The builtin Theme is packed into the binary and unpacked on the first start of the app. Per Default its unpacked into the same folder as the application but can be changed per [env Var](#Environment_Variables).
New Themes just have to be copied in the themes Path folder.

Themes define their settings in a `theme.json` file and the app stores them as-is without any predefined structure. This allows theme creators to define any settings they need.


### Theme Overrides

You can override individual templates from any active theme by placing custom template files in the `themes/overwrite/` directory. This allows you to modify specific pages without creating a complete custom theme.
As a template Engine [go html](https://pkg.go.dev/html/template) is used. As for available Variables take a look into the [template_data.go](../internal/thememanager/template_data.go) file.

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

- admin.gohtml
- base.gohtml
- browse.gohtml
- browsemetadata.gohtml
- browsefiles.gohtml
- dashboardview.gohtml
- dashboardedit.gohtml
- dashboardnew.gohtml
- fileedit.gohtml
- filedittable.gohtml
- filenew.gohtml
- fileview.gohtml
- filesoverview.gohtml
- help.gohtml
- history.gohtml
- home.gohtml
- latestchanges.gohtml
- playground.gohtml
- search.gohtml
- settings.gohtml
- mediaview.gohtml
- mediaoverview.gohtml
