# Configuration

# App Configuration (Environment Variables)

App configuration is read from environment variables on startup and cannot be changed at runtime.

- `KNOV_DATA_PATH` (string) - Path to data directory (default: `"data"`)
- `KNOV_SERVER_PORT` (string) - Server port (default: `"1324"`)
- `KNOV_LOG_LEVEL` (string) - How much logs do you want to receive from most to lowest (default: `"info"`)
  - Available Options: `"debug"`, `"info"`, `"warning"`, `"error"`
- `KNOV_GIT_REPO_URL` (string) - Git repository URL to clone. If empty, a new repository is initialized (default: `""`)
- `KNOV_STORAGE` (string) - Storage method for all data including metadata and dashboards (default: `"json"`)
  - Available options: `"json"`, `"sqlite"`, `"postgres"`
- `KNOV_SEARCH_ENGINE` (string) - Search engine type (default: `"sqlite"`)
  - Available options: `"sqlite"`, `"memory"`, `"grep"`
- `KNOV_CRONJOB_INTERVAL` (string) - Interval for file processing tasks (default: `"5m"`)
  - Format: Duration string (e.g., `"5m"`, `"1h"`, `"30s"`)
  - Tasks: Add new files to git, initialize metadata
- `KNOV_SEARCH_INDEX_INTERVAL` (string) - Interval for search index rebuild (default: `"15m"`)
  - Format: Duration string (e.g., `"15m"`, `"30m"`, `"1h"`)
  - Rebuilds the entire search index to ensure accuracy

## Storage System

The application uses a unified key-based storage system for all data types:

| Data Type       | Key Pattern                | Example                        |
| --------------- | -------------------------- | ------------------------------ |
| Metadata        | `metadata/filepath`        | `metadata/projects/backend.md` |
| Dashboards      | `dashboard/id`             | `dashboard/home`               |
| User Dashboards | `user/userid/dashboard/id` | `user/john/dashboard/work`     |
| User Settings   | `user/userid/settings`     | `user/john/settings`           |

### Storage Methods

| Method     | Best For                    | Advantages                                 | Disadvantages              |
| ---------- | --------------------------- | ------------------------------------------ | -------------------------- |
| `json`     | Development, small datasets | Simple, human-readable, no dependencies    | Slower for large datasets  |
| `sqlite`   | Production, medium datasets | Fast queries, ACID compliance, single file | Requires SQLite            |
| `postgres` | Enterprise, large datasets  | Full SQL features, concurrent access       | Requires PostgreSQL server |

### File Structure (JSON Storage)

```
config/
├── .storage/
│   ├── metadata/
│   │   ├── project.md.json
│   │   └── guides/
│   │       └── setup.md.json
│   ├── dashboard/
│   │   └── home.json
│   └── user/
│       └── john/
│           ├── settings.json
│           └── dashboard/
│               └── work.json
└── users/  ← legacy user settings (will be migrated)
```

## Supported File Formats

KNOV supports multiple file formats with automatic syntax highlighting:

- **Markdown** (`.md`) - Full CommonMark support with extensions
  - Fenced code blocks: ` ```language `
- **DokuWiki** (`.txt`, `.dokuwiki`) - DokuWiki syntax with automatic conversion
  - Standard code blocks: `<code language>...</code>`
  - SyntaxHighlighter4: `<sxh language>...</sxh>`
  - Codify plugin: `<codify language>...</codify>`
- **PDF** (`.pdf`) - Direct viewing
- **Other** - Raw content display

### Syntax Highlighting

Code blocks are highlighted using Prism.js with support for 200+ languages:

**Markdown example:**
\`\`\`go
func main() {
fmt.Println("Hello")
}
\`\`\`

**DokuWiki examples:**

### Tables

DokuWiki tables are enhanced with metadata attributes for future functionality:

**Alignment Detection:**

```
^ Left    ^  Center  ^    Right ^
| Data    |  Data    |     Data |
```

**Type Detection:**

- Numbers: `123`, `45.67`, `-10`
- Dates: `2024-01-01`, `01.01.2024`
- Currency: `$100`, `€50`, `£25`
- Text: everything else

**Features:**

- Sticky headers (stay visible while scrolling)
- Responsive wrapper (horizontal scroll on small screens)
- Zebra striping (alternating row colors)
- Hover highlighting
- Type-specific formatting (numbers right-aligned, monospace for currency)
- `data-editable="true"` attribute for future editing functionality
- `data-align` and `data-type` attributes for enhanced styling

Tables are wrapped in `.table-wrapper` for horizontal scrolling on narrow screens.
