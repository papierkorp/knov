# Developer Guide

## Prerequisites

```bash
go install github.com/a-h/templ/cmd/templ@latest
go install github.com/swaggo/swag/cmd/swag@latest
go install golang.org/x/text/cmd/gotext@latest
```

## Environment Setup

Set these environment variables for development:

````bash
export KNOV_DATA_PATH="data"
export KNOV_SERVER_PORT="1324"
export KNOV_LOG_LEVEL="debug"
export KNOV_GIT_REPO_URL=""  # Empty for local development
export KNOV_STORAGE="json"   # Changed from KNOV_METADATA_STORAGE
export KNOV_CRONJOB_INTERVAL="5m"  # File processing interval
export KNOV_SEARCH_INDEX_INTERVAL="15m"  # Search index rebuild interval
```

## Development

```bash
make dev     # Start development server with test data
make prod    # Build production binary
````

Server runs on the port specified by `KNOV_SERVER_PORT` environment variable (default: `http://localhost:1324`)

## Configuration System

The application uses a two-tier configuration system:

### App Configuration

- Set via environment variables
- Read-only at runtime
- Affects core application behavior
- See `internal/configmanager/config.go`

### User Settings

- Stored via the unified storage system
- Changeable at runtime via API
- UI preferences and personalization
- See `internal/configmanager/settings.go`

## Storage System

The app uses a unified key-based storage system:

```go
// Access storage
storage := storage.GetStorage()

// Save data
storage.Set("datatype/identifier", jsonData)

// Load data
data, err := storage.Get("datatype/identifier")

// List keys
keys, err := storage.List("datatype/")

// Delete data
storage.Delete("datatype/identifier")
```

### Key Patterns

- `metadata/filepath` - File metadata
- `dashboard/id` - Global dashboards
- `user/userid/dashboard/id` - User dashboards
- `user/userid/settings` - User settings

## Config Manager

Access configuration through:

```go
// App config
appConfig := configmanager.GetAppConfig()
dataPath := appConfig.DataPath
storageMethod := configmanager.GetStorageMethod()  // New function name

// User settings
userSettings := configmanager.GetUserSettings()
theme := userSettings.Theme

// Helper functions
language := configmanager.GetLanguage()
configmanager.SetLanguage("de")
```

## Git Integration

- Git operations only work if `KNOV_GIT_REPO_URL` is configured
- If empty, the system operates in local-only mode
- Repository is cloned/initialized on startup
- See `internal/testdata/testdata.go` for git setup examples

## Theme Development

For creating custom themes, see the [Theme Creator Guide](theme-creator-guide.md).

The theme system uses Go plugins for maximum flexibility while maintaining a clean interface.
