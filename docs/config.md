# Configuration

Configuration is split between app configuration (environment variables) and user settings (JSON files).

## App Configuration (Environment Variables)

App configuration is read from environment variables on startup and cannot be changed at runtime.

- `KNOV_DATA_PATH` (string) - Path to data directory (default: `"data"`)
- `KNOV_SERVER_PORT` (string) - Server port (default: `"1324"`)
- `KNOV_LOG_LEVEL` (string) - Log level: `"debug"`, `"info"`, `"warning"`, `"error"` (default: `"info"`)
- `KNOV_GIT_REPO_URL` (string) - Git repository URL to clone. If empty, no git operations are performed (default: `""`)
- `KNOV_METADATA_STORAGE` (string) - Storage method for file metadata (default: `"json"`)
  - Available options: `"json"`, `"sqlite"`, `"postgres"`, `"yaml"`

## User Settings (JSON Files)

User settings are stored in JSON files and can be changed at runtime. Each user has their own settings file.

Settings are stored in: `config/users/{userID}/settings.json`

Available settings:

- `theme` (string) - Active theme name (default: `"builtin"`)
- `language` (string) - UI language code (default: `"en"`)
  - Available languages: `"en"` (English), `"de"` (German)

## Multi-User Support

The system supports multiple users, each with their own settings:

- Default user: `config/users/default/settings.json`
- Additional users: `config/users/{userID}/settings.json`

Use `configmanager.SwitchUser(userID)` to change the active user.

## Environment Setup

Set these environment variables before starting the application:

```bash
export KNOV_DATA_PATH="data"
export KNOV_SERVER_PORT="1324"
export KNOV_LOG_LEVEL="info"
export KNOV_GIT_REPO_URL="https://github.com/your-repo/notes.git"
export KNOV_METADATA_STORAGE="json"
```

## Git Integration

The system automatically initializes a git repository in the data directory:

- **With git URL configured**: Repository is cloned from the remote URL
- **Without git URL**: Local git repository is initialized automatically
- This ensures version control is always available for your files
- Git URL cannot be changed after startup (restart required)
