# Developer Guide

**Prerequisites**

- Go 1.21 or later
- Git
- Make
- Swag CLI: `go install github.com/swaggo/swag/cmd/swag@latest`
- gotext: `go install golang.org/x/text/cmd/gotext@latest`

## Quick Start

Clone and setup:

```bash
git clone https://github.com/papierkorp/knov.git
cd knov
go mod download
```

Install required tools:

```bash
go install github.com/swaggo/swag/cmd/swag@latest
go install golang.org/x/text/cmd/gotext@latest
```

Start development server:

```bash
# Start development server
make dev
make dev-fast # without fts5 search init

# Generate Swagger docs
make swaggo-api-init

# Generate translations
make translation

# Build for production
make prod

# Create and Run Docker image
make docker
```

## API Development

### Adding New Endpoints

1. Add handler function to appropriate `internal/server/api_*.go` file
2. Add route in `internal/server/server.go`
3. Add Swagger documentation comments


## Translation

Add translatable strings in templates:

```go
{{T "Your translatable text"}}
```

Add translatable strings in Go code (global):

```go
translation.Sprintf("Your translatable text")
```

Add translatable strings in HTMX handlers (user-specific):

```go
func handleSomeHTMX(w http.ResponseWriter, r *http.Request) {
    // Use user's current language setting
    userLang := configmanager.GetLanguage()
    text := translation.SprintfForRequest(userLang, "Your translatable text")
    html := fmt.Sprintf(`<div>%s</div>`, text)
    w.Write([]byte(html))
}
```

Generate translations:

```bash
make translation
```

Translation files in `internal/translation/locales/{lang}/messages.gotext.json`


## Embedded Assets

### Static Files

Static files are embedded from the project root:

```go
//go:embed static/*
var staticFS embed.FS
```


### Theme Assets

Builtin theme assets are embedded in main.go:

```go
//go:embed themes/builtin
var builtinThemeFS embed.FS
```

Plugin themes embed their own assets:

```go
//go:embed templates/*.css
var cssFiles embed.FS
```

## Configuration Management

# dbmigration

Tiny version-based schema migrations for sqlite. No external tools, no SQL files — migrations are plain Go functions.

## How it works

- A `schema_version` table holds a single integer: the db's current version.
- Each storage keeps its own ordered `[]Migration` and a `schemaVersion` const.
- On startup, `Migrate` compares the stored version to the target and runs the gap: missing `Up` steps when behind, `Down` steps in reverse when ahead.
- Each step plus its version bump run in one transaction — a failure rolls back cleanly and the version never lands in a half-applied state.

Version `N` is reached by applying `migrations[0..N-1]`. The slice index is the "from" version: `migrations[0]` is 0→1, `migrations[1]` is 1→2, etc.

## Usage

```go
const schemaVersion = 2

var migrations = []dbmigration.Migration{
    {Up: migrateV0toV1, Down: migrateV1toV0},
    {Up: migrateV1toV2, Down: migrateV2toV1},
}

func (ss *sqliteStorage) initialize() error {
    return dbmigration.Migrate(ss.db, schemaVersion, migrations)
}

func migrateV1toV2(tx *sql.Tx) error {
    _, err := tx.Exec(`ALTER TABLE metadata ADD COLUMN related TEXT`)
    return err
}

func migrateV2toV1(tx *sql.Tx) error {
    _, err := tx.Exec(`ALTER TABLE metadata DROP COLUMN related`)
    return err
}
```

## Rules

- Migrations are **append-only**. Never edit a shipped step — add a new one.
- Each storage owns its own version counter; they advance independently.
- Always bump `schemaVersion` when appending a migration.
- `Down` may be `nil` for irreversible steps; downgrading past one returns an error.
- Dropping a column requires sqlite ≥ 3.35 (2021). For older sqlite, use the create-new/copy/rename pattern.

## Migration test

**setup test folder**

```bash
mkdir /home/markus/develop/privat/migration-test-knov
cd /home/markus/develop/privat/migration-test-knov
cp /home/markus/develop/privat/knov/bin/knov .
./knov  # start once to let it initialize all sqlite DBs, then stop it
```

**change metadataStorage_sqlite.go**

Either use functions or put the migration directly in:

```go
func (ss *sqliteStorage) initialize() error {
	const version = 1
	steps := []dbmigration.Migration{
  	{
  		Up: func(tx *sql.Tx) error {
  			_, err := tx.Exec(`
  			CREATE TABLE IF NOT EXISTS metadata (
  				path TEXT PRIMARY KEY,
  				title TEXT,
  				created_at DATETIME,
  				last_edited DATETIME,
  				collection TEXT,
  				folders TEXT,
  				tags TEXT,
  				ancestor TEXT,
  				parents TEXT,
  				kids TEXT,
  				used_links TEXT,
  				links_to_here TEXT,
  				related TEXT,
  				editor TEXT,
  				size INTEGER,
  				"references" TEXT
  			);
  			CREATE INDEX IF NOT EXISTS idx_collection ON metadata(collection);
  			CREATE INDEX IF NOT EXISTS idx_editor ON metadata(editor);
  			`)
  			return err
  		},
  		Down: func(tx *sql.Tx) error {
  			_, err := tx.Exec(`DROP TABLE IF EXISTS metadata`)
  			return err
  		},
  	},
  	{Up: metaV1toV2, Down: metaV2toV1},
		{
			Up: func(tx *sql.Tx) error {
				_, err := tx.Exec(`UPDATE metadata SET test_col = 'migrated' WHERE test_col IS NULL`)
				return err
			},
			Down: func(tx *sql.Tx) error {
				_, err := tx.Exec(`UPDATE metadata SET test_col = NULL`)
				return err
			},
		},
		{
    Up: func(tx *sql.Tx) error {
        if _, err := tx.Exec(`ALTER TABLE metadata ADD COLUMN test_col TEXT`); err != nil {
            return err
        }
        return fmt.Errorf("intentional failure")
    },
    Down: nil,
},
	}
	if err := dbmigration.Migrate(ss.db, version, steps); err != nil {
		return fmt.Errorf("metadata storage migration failed: %w", err)
	}
	logging.LogDebug("metadata sqlite storage ready at version %d", version)
	return nil
}

func metaV1toV2(tx *sql.Tx) error {
	_, err := tx.Exec(`ALTER TABLE metadata ADD COLUMN test_col TEXT`)
	return err
}

func metaV2toV1(tx *sql.Tx) error {
	_, err := tx.Exec(`ALTER TABLE metadata DROP COLUMN test_col`)
	return err
}
```

**start application with different version**

```bash
# first check current version
sqlite3 storage/metadata/metadata.db "SELECT version FROM schema_version"
sqlite3 storage/metadata/metadata.db ".schema"

# set const version = 2
make dev   # stop immediately after "storage ready" log
sqlite3 storage/metadata/metadata.db "SELECT version FROM schema_version" # → 2
sqlite3 storage/metadata/metadata.db "PRAGMA table_info(metadata)" | grep test_col # → test_col should appear

# set const version = 1
make dev   # stop after startup
sqlite3 storage/metadata/metadata.db "SELECT version FROM schema_version" # → 1
sqlite3 storage/metadata/metadata.db "PRAGMA table_info(metadata)" | grep test_col # → nothing (column dropped)

# set const version = 3
# seed some rows first
sqlite3 storage/metadata/metadata.db "INSERT OR IGNORE INTO metadata (path) VALUES ('docs/test.md')"
make dev   # stop after startup
sqlite3 storage/metadata/metadata.db "SELECT path, test_col FROM metadata WHERE path = 'docs/test.md'" # → docs/test.md|migrated

# set const version = 4
make dev   # should log "migration 3→4 failed" and refuse to start
sqlite3 storage/metadata/metadata.db "SELECT version FROM schema_version" # → 3  (did not advance)
```

# Theme Creation Guide

## Quick Start

1. Create a new folder in `themes/` with the required files
2. Run the application: `make dev`
3. Navigate to `http://localhost:1324/settings`
4. Select your theme from the dropdown
5. Test all pages

## Required Files

Are determined in [thememanager.go](../internal/thememanager.go) in the `ThemeTemplates` struct.

### theme.json

Is defined in the [thememanager.go](../internal/thememanager.go) and allows to pass [Theme Settings](#theme_settings) through the app which can then be used via the [Template Data](#template_data)

```go
type ThemeMetadata struct {
	Name          string                  `json:"name"`
	Version       string                  `json:"version"`
	Author        string                  `json:"author"`
	Description   string                  `json:"description"`
	ThemeSettings map[string]ThemeSetting `json:"themeSettings,omitempty"`
}
```

**Example**

```json
{
  "name": "Builtin Theme",
  "version": "1.0.0",
  "author": "System",
  "description": "Default builtin theme",
}
```

## Theme Settings

Themes can define custom settings that users can configure. 
Is defined in the [thememanager.go](../internal/thememanager.go) in the ThemeSettings struct:

```go
type ThemeMetadata struct {
	Name          string                  `json:"name"`
	Version       string                  `json:"version"`
	Author        string                  `json:"author"`
	Description   string                  `json:"description"`
	ThemeSettings map[string]ThemeSetting `json:"themeSettings,omitempty"`
}

type ThemeSetting struct {
	Type        string      `json:"type"`
	Default     interface{} `json:"default"`
	Label       string      `json:"label"`
	Description string      `json:"description,omitempty"`
	Options     []string    `json:"options,omitempty"`
	Min         *int        `json:"min,omitempty"`
	Max         *int        `json:"max,omitempty"`
}
````

Add a `themeSettings` object to your theme.json:

```json
{
  "name": "Builtin Theme",
  "version": "1.0.0",
  "author": "System",
  "description": "Default builtin theme",
  "themeSettings": {
    "darkMode": {
      "type": "boolean",
      "default": true,
      "label": "Dark Mode",
      "description": "enable dark theme appearance for better readability in low light"
    },
    "colorScheme": {
      "type": "select",
      "options": ["blue", "green", "red", "purple"],
      "default": "green",
      "label": "Color Scheme",
      "description": "choose the color theme for the interface"
    },
    "fileView": {
      "type": "select",
      "options": ["detailed", "compact", "reader"],
      "default": "detailed",
      "label": "File View",
      "description": "choose how files are displayed - detailed shows metadata, compact saves space, reader optimizes for reading"
    },
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

## example template

**Settings Example**

Which allows theme Switching:

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

## Template Data

Template Data is passed from the application to the theme.
Is defined in [template_data.go](../internal/template_data.go).

**Accessing Template Data**

In your templates, you can access template data like this:

```html
<!DOCTYPE html>
<html>
<head>
    <title>{{ .Title }}</title>
</head>
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

## Validation

The system validates:

- All required files exist and are not empty
- theme.json contains all required fields (name, version, author, description)
- Theme settings (if defined) have valid types and required fields
- Templates parse correctly

If validation fails, check console output for error messages.
