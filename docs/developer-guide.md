# Developer Guide

## Prerequisites

```bash
go install github.com/a-h/templ/cmd/templ@latest
go install github.com/swaggo/swag/cmd/swag@latest
go install golang.org/x/text/cmd/gotext@latest
```

## Environment Setup

Set these environment variables for development:

```bash
export KNOV_DATA_PATH="data"
export KNOV_SERVER_PORT="1324"
export KNOV_LOG_LEVEL="debug"
export KNOV_GIT_REPO_URL=""  # Empty for local development
export KNOV_METADATA_STORAGE="json"
```

## Development

```bash
make dev     # Start development server with test data
make prod    # Build production binary
```

Server runs on the port specified by `KNOV_SERVER_PORT` environment variable (default: `http://localhost:1324`)

## Configuration System

The application uses a two-tier configuration system:

### App Configuration

- Set via environment variables
- Read-only at runtime
- Affects core application behavior
- See `internal/configmanager/config.go`

### User Settings

- Stored in JSON files per user
- Changeable at runtime via API
- UI preferences and personalization
- See `internal/configmanager/settings.go`

## Config Manager

Access configuration through:

```go
// App config
appConfig := configmanager.GetAppConfig()
dataPath := appConfig.DataPath

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
