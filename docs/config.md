# Configuration

# App Configuration (Environment Variables)

App configuration is read from environment variables on startup and cannot be changed at runtime.

- `KNOV_DATA_PATH` (string) - Path to data directory (default: `"data"`)
- `KNOV_SERVER_PORT` (string) - Server port (default: `"1324"`)
- `KNOV_LOG_LEVEL` (string) - How much logs do you want to receive from most to lowest (default: `"info"`)
  - Available Options: `"debug"`, `"info"`, `"warning"`, `"error"`
- `KNOV_GIT_REPO_URL` (string) - Git repository URL to clone. If empty, a new repository is initialized (default: `""`)
- `KNOV_METADATA_STORAGE` (string) - Storage method for file metadata (default: `"json"`)
  - Available options: `"json"`, `"sqlite"`, `"postgres"`, `"yaml"`
- `KNOV_SEARCH_ENGINE` (string) - Search engine type (default: `"sqlite"`)
  - Available options: `"sqlite"`, `"memory"`, `"grep"`

## Search Engine Comparison

| Engine   | Best For                      | File Count  | Advantages                                                       | Disadvantages                                       |
| -------- | ----------------------------- | ----------- | ---------------------------------------------------------------- | --------------------------------------------------- |
| `sqlite` | Production, persistent search | 1000+ files | - Persistent index<br>- Fast queries<br>- Handles large datasets | - Disk space usage<br>- Index rebuild time          |
| `memory` | Development, fast iteration   | <500 files  | - Extremely fast search<br>- No disk usage<br>- Instant startup  | - Rebuilds on restart<br>- High memory usage        |
| `grep`   | Simple setups, no indexing    | <100 files  | - No indexing needed<br>- Uses system tools<br>- Low memory      | - Slow on large datasets<br>- Requires grep command |

# User Settings (JSON Files)

User settings are stored in JSON files and can be changed at runtime. Each user has their own settings file.

Settings are stored in: `config/users/{userID}/settings.json`
