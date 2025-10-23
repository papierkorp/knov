# theme creator guide

minimal guide for creating knov themes.

## quick start

1. **copy builtin theme**:
   ```bash
   cp -r themes/builtin themes/mytheme
   ```

2. **edit metadata** (`themes/mytheme/theme.json`):
   ```json
   {
     "name": "mytheme",
     "author": "your name",
     "version": "1.0.0"
   }
   ```

3. **customize templates** - edit any `.gotmpl` file
4. **customize styles** - edit `style.css`
5. **restart app** or upload via admin panel

## theme structure

```
themes/mytheme/
├── theme.json          # metadata (required)
├── style.css           # styles (required)
├── base.gotmpl         # layout (required)
├── home.gotmpl         # pages (required)
├── fileview.gotmpl     # ...
├── overview.gotmpl     #
├── settings.gotmpl     #
└── (optional pages)    # other .gotmpl files
```

## partial themes

**override only what you want** - missing templates fall back to builtin:

```
themes/minimal/
├── theme.json     # metadata
├── style.css      # custom styles  
└── home.gotmpl    # custom home page only
                   # all other pages use builtin
```

## metadata options

see `ThemeMetadata` struct in `internal/thememanager/thememanager.go` for all options:

- **views**: different variants per page type (`file: ["default", "compact"]`)
- **features**: dark mode, color schemes, responsive css
- **categories**: theme classification (see Categories struct)

## template data

all templates receive `TemplateData` struct with:
- `.Theme` - current theme name
- `.Language` - current language  
- `.DarkMode` - dark mode enabled
- `.Content` - page-specific data (see content types in thememanager.go)

## template functions

available in all templates:
- `{{.T "text"}}` - translation
- `{{.T "format %s" .Value}}` - translation with formatting

## css structure

use css variables for theming:
```css
:root {
  --primary: #2563eb;
  --bg: #ffffff;
}

[data-dark-mode="true"] {
  --bg: #0f172a;
}

[data-theme*="green"] {
  --primary: #16a34a;
}
```

## view variants

support multiple views per page type:
```json
{
  "views": {
    "file": ["default", "detailed", "compact", "reader"]
  }
}
```

create templates: `fileview.gotmpl`, `fileview_detailed.gotmpl`, etc.

## validation

themes are validated on load:
- required templates exist
- valid json metadata  
- css file present
- template syntax correct

check logs for validation errors.

## tips

- **start simple** - copy builtin and modify gradually
- **test frequently** - restart app to reload themes
- **use browser devtools** - inspect css variables and structure
- **check validation** - watch logs for errors
- **minimal overrides** - only customize what you need

## examples

- **color theme**: only change css variables
- **layout theme**: override base.gotmpl + style.css
- **page theme**: customize specific pages like home.gotmpl
- **view theme**: add new view variants for file display

for complete reference, see builtin theme structure and code documentation.
