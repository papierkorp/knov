# Setup

## Requirements

- A place to run the binary or .exe (server, NAS, your own machine)

## The Binary

Knov ships as a single self-contained binary (or `.exe` on Windows) — no installer, no dependencies, no separate database to set up.

- Both built-in themes (`builtin` and `rail`) are bundled inside the binary and unpacked into `themes/` on first start
- Static assets, default templates and all required files are embedded the same way — the binary is all you need
- To update: stop knov, replace the binary, start again — your data and settings are untouched
- To move to another machine: copy the binary and your data folder, done

## First Run

1. Copy `.env.example` to `.env` and adjust the values
2. Run the binary or .exe - it creates all neccessary folders and a git repository automatically (if no one is configured)
3. Open your browser at `http://localhost:8080` (or your configured port)

## Data Folder

Everything knov stores lives in a folder and is created automatically:

- `data/docs/` - your files
- `data/media/` - uploaded images and attachments
- `themes/` - custom or downloaded themes 
- `storage/` - configurable databases (sqlite, json) for all the systems (e.g. metadata, cache, chat, config)
- `storage/config/` - settings and filter configs
- `.git/` - version history (managed automatically)
- `logs/` - logging for different executions (can be configured)

Back the data and storage folders up to keep everything safe.

## Home Dashboard

- The home page (`/`) shows a dashboard
- Set `KNOV_HOME_DASHBOARD=your-dashboard-name` to choose which one
- Defaults to a dashboard named `home`

## Git & Sync

**Local only (default)** - leave `KNOV_GIT_REMOTE` empty. All your changes are versioned locally, no network required.

**With a remote** - point `KNOV_GIT_REMOTE` to any git remote (GitHub, Gitea, Gitlab, a bare repo on your NAS, etc.):

- Every file save automatically commits and pushes in the background
- Multiple users can share one remote - each runs their own knov binary
- If two people edit the **same file** at the same time, your version is saved as a conflict copy and a warning appears - no work is lost

## Kanban

- A file needs to be inside a **subfolder** to appear on a board - the folder name becomes the board name (a collection)
- Add a status tag (can be configured, defaults to `kb-status`) to a file to put it on the board: `kb-status-inbox`, `kb-status-inprogress`, `kb-status-blocked`, `kb-status-archive`
- Go to `/kanban` to open a board

## Themes

- Drop a theme folder into `themes/` and select it under **Settings → Theme**
- Small CSS tweaks: **Settings → Theme → Custom CSS** - no restart needed

# Configuration

All settings go in your `.env` file. Copy `.env.example` to get started — every option is listed there with a description.

## Key Settings

| Variable | Default | What it does |
|---|---|---|
| `KNOV_DATA_PATH` | `./data` | Where your files, themes and config are stored |
| `KNOV_PORT` | `8080` | Port the app listens on |
| `KNOV_LANGUAGE` | `en` | Interface language |
| `KNOV_THEME` | `builtin` | Active theme name |
| `KNOV_HOME_DASHBOARD` | `home` | Dashboard shown at `/` |

## Git Sync

| Variable | Notes |
|---|---|
| `KNOV_GIT_REMOTE` | Leave empty for local-only mode |
| `KNOV_GIT_REMOTE_BRANCH` | Branch to push/pull (default: `main`) |
| `KNOV_GIT_SSH_KEY` | Path to your SSH private key |
| `KNOV_GIT_TOKEN` | Personal access token (HTTPS auth) |
| `KNOV_GIT_AUTO_PUSH` | `true` to push on every save |

## Kanban

| Variable | Notes |
|---|---|
| `KNOV_KANBAN_PREFIX` | Tag prefix for status tags (default: `kb-status`) |
| `KNOV_KANBAN_COLUMNS` | Comma-separated list of status columns |
| `KNOV_KANBAN_TAG_COLORS` | Color chips per tag — e.g. `urgent:red,markus:green` |

## File Behaviour

| Variable | Notes |
|---|---|
| `KNOV_AUTOCREATE_TAGS` | Tags added to every new file automatically |
| `KNOV_AUTOCREATE_COLLECTIONS` | Limit auto-tags to these collections only |
| `KNOV_USE_EXTENSION_INDEX` | Use `.index` extension instead of `.md` for index/filter files |

## Notes

- Changes to `.env` require a restart
- Theme settings (dark mode, color scheme, custom CSS) are saved per user in the UI — no `.env` needed
- The conflict copy filename is `filename.conflict.YYYYMMDD-HHMMSS.md` — always check for these if you share a remote with others
