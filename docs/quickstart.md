# Quickstart

## Requirements

- A place to run the binary or .exe (server, NAS, your own machine)

## The Binary

Knov ships as a single self-contained binary (or `.exe` on Windows) — no installer, no dependencies, no separate database to set up.

- The builtin theme is bundled inside the binary and unpacked into `themes/` on first start
- Static assets, default templates and all required files are embedded the same way — the binary is all you need
- To update: stop knov, replace the binary, start again — your data and settings are untouched
- To move to another machine: copy the binary and your data folder, done
- to build the binary yourself use: `make prod`

## Docker

Alternative to running the binary directly:

- Copy `.env.example` to `tools/docker_deployment/` as `.env` first, same as the binary setup
- go to `tools/docker_deployment/`
- use `docker compose up` to locally build and run a production image (at the moment there is no prebuild image)
- Data, themes, storage, logs and backups are mounted as volumes so they persist outside the container

## Mobile (Termux)

- Android only - iOS isn't feasible for a self-hosted Go binary
- Install [Termux](https://termux.dev) from F-Droid or GitHub releases, not the Play Store version (outdated)
- `make mobile` builds a Linux arm64 binary (`bin/knov-arm64`) - Termux runs it as a regular Linux binary, no Android-specific build needed
- Copy the binary and optionally your `.env` to the phone, run it like any other Linux binary, then open `http://localhost:1324` in the phone's browser
- Keep it running: `termux-wake-lock` stops Android from suspending the CPU while knov runs in the background; run knov inside `tmux` so it survives closing the Termux app; install the Termux:Boot add-on to auto-start it on device boot

## First Run

1. Run the binary or .exe - without a `.env` file it starts with defaults; to customize settings, copy `.env.example` to `.env` first and adjust the values
2. Open your browser at `http://localhost:1324` (or your configured port)
3. Check `/system/environment` any time to see every recognized `KNOV_*` variable with its description, default and current value

Nothing else needs to be set up by hand. On the first start knov creates every folder it needs (see below), initialises a git repository if none is configured, and unpacks the builtin theme.

## Data Folder

Everything knov stores lives in folders next to the binary, **all created automatically on first start**:

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
- Choose which one under **Settings → Home Dashboard** (a UI setting, not an env var)
- Defaults to a dashboard named `home`

## Git & Sync

**Local only (default)** - leave `KNOV_GIT_REMOTE` empty. All your changes are versioned locally, no network required.

**With a remote** - point `KNOV_GIT_REMOTE` to any git remote (GitHub, Gitea, Gitlab, a bare repo on your NAS, etc.):

- Every file save automatically commits and pushes in the background
- Multiple users can share one remote - each runs their own knov binary
- If two people edit the **same file** at the same time, your version is saved as a conflict copy and a warning appears - no work is lost

## Kanban

- Configure boards explicitly with `KNOV_KANBAN_BOARDS=folder/path:Display Name` (comma-separated) - each board covers that folder and its subfolders
- Add a status tag to a file in a board's folder to put it on the board: `kb-status-inbox`, `kb-status-inprogress`, `kb-status-blocked`, `kb-status-archive` (the `kb` prefix is configurable, `-status-` is fixed)
- Go to `/kanban` to see your configured boards and open one

## Themes

- Drop a theme folder into `themes/` and select it under **Settings → Theme**
- Small CSS tweaks: **Settings → Theme → Custom CSS** - no restart needed

# Configuration

All settings go in an optional `.env` file - without one, defaults are used. Copy `.env.example` to `.env` to get started — every option is listed there with a description, and `/system/environment` shows the full live list. For a per-system breakdown of every `KNOV_*` variable see [configuration.md](configuration.md).

## Notes

- Changes to `.env` require a restart
- Language, theme and home dashboard are UI settings, not env vars - see **Settings**
- Theme settings (dark mode, color scheme, custom CSS) are saved per user in the UI — no `.env` needed
- The conflict copy filename is `filename.conflict.YYYYMMDD-HHMMSS.md` — always check for these if you share a remote with others
