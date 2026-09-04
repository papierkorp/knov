# Features

## The Binary

- Ships as a single executable for all major operating systems - can be carried on a USB stick
- The built-in `builtin` theme is bundled inside and unpacked on first start
- All static assets and templates are embedded - the binary is everything you need
- Docker deployment via `docker compose` (the image is built locally - no prebuilt image is published)
- Configuration via a single `.env` file; take a look at `.env.example` for all options

## Files & Storage

All your data lives in plain files on disk - readable and editable with any text editor or IDE.

- **Flat file content** - markdown and plain text
- **Git versioning** - every save is automatically committed; full history is accessible in the app
- **Separate storage layer** - metadata, search index, chat and config are stored in SQLite databases alongside your files; these are not mixed into your content
- Files can be added via the app UI, by dropping them into the data folder directly, or via git push

## Git & Sync

- Every file save triggers an automatic local commit in the background
- Connect a git remote (GitHub, Gitea, Gitlab, bare repo) to sync between machines or share with others
- Each user runs their own binary - there is no server-side user management
- **Conflict handling** - if two users save the same file simultaneously, your version is preserved as a conflict copy, the file resets to the remote version, and a diff banner appears in the UI so you can review and merge manually

## Metadata

KNOV tracks metadata automatically and lets you enrich it manually.

**Automatic:**
- Collection (derived from the top-level folder)
- Folders, title, file size, created/edited dates
- Ancestor/parent/child chain (from manually set parents)
- Inbound and outbound links (parsed from markdown link syntax)
- Related files (computed via SQLite similarity)
- Kanban column assignment (derived from status tags, within a configured board's folder)

**Manual:**
- Tags, parent links, editor type
- External references (URL + description, stored in metadata - not cluttering the file content) appended to a file
- autocomplete for both internal wiki links with `[[<filelink>]]` and for default markdown links `[]()` including header

All metadata is browsable on the overview page (`/browse/files`) grouped by metadata

**Broken link scan & repair:**
- Scan every file for internal links that no longer resolve
- Each broken link comes with a suggested target; apply the repairs you pick and the links are rewritten in place

## Filter System

Filters are saved metadata queries that produce a live file list.

- Build a filter with any combination of metadata fields, operators and AND/OR logic
- Saving a filter generates a paired index file that shows the current results
- The index updates automatically whenever metadata changes - no manual refresh needed
- Filters are usable as dashboard widgets, as browse targets and in the kanban advanced filter panel
- **Bulk metadata update** - apply a metadata change to every file matching a filter in one go

## Kanban

- Boards are explicitly configured folders (`KNOV_KANBAN_BOARDS=folder/path:Display Name`), each covering that folder and its subfolders
- A file gets a kanban status tag to appear in a column - e.g. `kb-status-inbox`
- Columns are configurable per instance (`KNOV_KANBAN_COLUMNS`)
- Drag cards between columns to update status - saves automatically
- Optional folder-sync (`KNOV_KANBAN_FOLDERSYNC`) - moving a card physically relocates the file into a status-named subfolder, and moving a file into such a folder updates its status tag
- Optional archive drop zone (`KNOV_KANBAN_ARCHIVE_STATUS`) that isn't shown as a column
- Every card move is recorded in an event log, queryable via `GET /api/kanban/{board}/events`
- Quick filters (ancestor, tag, search) always visible in the toolbar
- Advanced filter panel (same system as saved filters) available via the filter button
- Tag chips on cards can be colour-coded per tag (`KNOV_KANBAN_TAG_COLORS`); cards can be styled per column (`KNOV_KANBAN_CARD_STYLES`)

## Dashboard System

- Customisable dashboards with multiple widget types to surface your data
- The home page (`/`) shows a configurable dashboard
- Widget types: filters, filter forms, file content, static text, tags, collections and folders

## Search

- Full-text search powered by SQLite FTS5 with BM25 ranking
- Falls back to grep-based search if configured
- Indexed in the background after every save - always up to date
- Trigram fallback for queries that return no FTS results
- Optional "search history" toggle also searches the content of files deleted in git history

## Chat

- A simple stream-of-consciousness note area available globally
- Stored in SQLite; paginated and always shown newest-first
- Useful for quick notes, comments or context attached to a specific file
- move each chat entry directly into new or existing files

## Media

- Upload images and attachments directly in the app
- Browse all media files with usage information
- Orphan detection - identifies media files not referenced by any document
- Orphaned files can be cleaned up from the admin page

## PDF Export

- Export any file to PDF, from the file actions panel or per-heading
- Configurable page format, orientation and margins
- Header and footer zones (left/centre/right) with page number and date tokens, optional rule lines, skippable on the first page
- Separate fonts for body text, headings, the title and code blocks
- Per-language syntax highlighting for code blocks; task checkboxes rendered as status icons

## Books

A book is an ordered list of whole files and individual sections that composes into one document.

- Built with a dedicated editor - reorder entries, and optionally pull in a section's subheaders
- Stored as a plain markdown file, so git, search, links and backups treat it like any other file
- The file view shows the composed document; export it from there as markdown or PDF
- Unresolvable references are marked inline instead of breaking the view or the export

## Notifications

- Brief toast notifications for save confirmations, errors and git conflicts
- Each theme exposes a view of the last 100 notifications
- Visible duration is configurable (`KNOV_NOTIFY_DURATION`)

## Import / Export

- Per-file DokuWiki -> Markdown conversion on download
- Export all files as a zip, optionally converting DokuWiki files to Markdown in the process
- Export metadata as JSON or CSV
- Export/import the settings file (JSON) for backup or migrating between instances

## Backup & Restore

- `/system/backup` snapshots the internal storage (metadata, chat, kanban, notifications, config, search) into one set - your docs/media are already versioned by git and stay out of it
- Snapshot everything or just a subset (e.g. only metadata)
- Every backup and every restore is logged; download any set as a `.tar.gz`, or restore it
- Restoring snapshots the current state first, so a bad restore can always be undone
- Old sets are rotated automatically by age and count; lock a set to keep it forever
- Automatic scheduled backups via cron-style profiles (`KNOV_BACKUP_AUTO_PROFILES`)

## Admin & System Jobs

- `/system/jobs` runs and monitors maintenance jobs
- `/system/logs` shows application logs, including per-job run logs
- Jobs run automatically on a configurable interval or can be triggered manually
- `/system/changelog`, `/system/environment` and `/system/version` show the in-app changelog, every recognised `KNOV_*` variable and build info
- Admin actions: rebuild metadata, invalidate cache, run the sync job, pull/push the git remote, restart the app

## Theme System

- Bundled `builtin` theme, plus an `example` theme to use as a starting point for your own
- Drop additional theme folders into `themes/` to add more
- Per-user appearance settings: dark mode, colour scheme, sidebar configuration, timezone and date format
- Separate font settings for body text, headings, the title and code
- Custom CSS settable per theme in the UI - no restart needed
- Template overrides: place `.gohtml` files in `themes/overwrite/` to override specific pages without touching the theme itself

## Organisation Methods

- **Tags** - free-form, fully customisable
- **Collections** - automatic grouping by top-level folder, overridable per file
- **Parent/child hierarchy** - set a parent to build a tree; ancestors and children are computed automatically
- **Editor/File types** - a general-purpose CodeMirror editor (WYSIWYG, syntax highlighting, live rendering) plus dedicated editors for todo, list, filter, index/MOC and book files, with every output stored as a viable markdown file
- **Table editor** - edit any individual markdown table inside a file in a dedicated grid editor

## Architecture

- **Backend** - Go with Chi router
- **Frontend** - HTMX + Go HTML templates; no JavaScript framework
- **Content storage** - flat files on the local filesystem
- **Metadata & config storage** - SQLite (default) with JSON fallback options
- **Search** - SQLite FTS5 with BM25 ranking; grep mode available
- **Internationalisation** - English and German; additional languages addable via translation files
