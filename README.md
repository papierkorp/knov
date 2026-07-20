# KNOV - Knowledge Management System

KNOV (Knowledge Vault) is a local-first knowledge management system that runs as a single self-contained binary. No cloud, no account, no dependencies.

---

## The Binary

- Ships as a single executable for all major operating systems - can be carried on a USB stick
- Both built-in themes (`builtin` and `rail`) are bundled inside and unpacked on first start
- All static assets and templates are embedded - the binary is everything you need
- Docker image available for server deployments
- Configuration via a single `.env` file; take a look at `.env.example` for all options

---

## Files & Storage

All your data lives in plain files on disk - readable and editable with any text editor or IDE.

- **Flat file content** - markdown and plain text
- **Git versioning** - every save is automatically committed; full history is accessible in the app
- **Separate storage layer** - metadata, search index, chat and config are stored in SQLite databases alongside your files; these are not mixed into your content
- Files can be added via the app UI, by dropping them into the data folder directly, or via git push

---

## Git & Sync

- Every file save triggers an automatic local commit in the background
- Connect a git remote (GitHub, Gitea, Gitlab, bare repo) to sync between machines or share with others
- Each user runs their own binary - there is no server-side user management
- **Conflict handling** - if two users save the same file simultaneously, your version is preserved as a conflict copy, the file resets to the remote version, and a diff banner appears in the UI so you can review and merge manually

---

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
- easy way to add internal links with a `[[<filelink>]]` syntax

All metadata is browsable on the overview page (`/browse/files`) grouped by metadata

---

## Filter System

Filters are saved metadata queries that produce a live file list.

- Build a filter with any combination of metadata fields, operators and AND/OR logic
- Saving a filter generates a paired index file that shows the current results
- The index updates automatically whenever metadata changes - no manual refresh needed
- Filters are usable as dashboard widgets, as browse targets and in the kanban advanced filter panel

---

## Kanban

- Boards are explicitly configured folders (`KNOV_KANBAN_BOARDS=folder/path:Display Name`), each covering that folder and its subfolders
- A file gets a kanban status tag to appear in a column - e.g. `kb-status-inbox`
- Drag cards between columns to update status - saves automatically
- Quick filters (ancestor, tag, search) always visible in the toolbar
- Advanced filter panel (same system as saved filters) available via the filter button
- Tag chips on cards can be colour-coded per tag via config

---

## Dashboard System

- Customisable dashboards with multiple widget types to surface your data
- The home page (`/`) shows a configurable dashboard
- Dashboards support filter widgets, file lists, recent changes and more

---

## Search

- Full-text search powered by SQLite FTS5 with BM25 ranking
- Falls back to grep-based search if configured
- Indexed in the background after every save - always up to date
- Trigram fallback for queries that return no FTS results

---

## Chat

- A simple stream-of-consciousness note area available globally
- Stored in SQLite; paginated and always shown newest-first
- Useful for quick notes, comments or context attached to a specific file
- move each chat entry directly into new or existing files

---

## Media

- Upload images and attachments directly in the app
- Browse all media files with usage information
- Orphan detection - identifies media files not referenced by any document
- Orphaned files can be cleaned up from the admin page

---

## Theme System

- Two themes bundled: `builtin` (classic layout) and `rail` (sidebar panel layout)
- Drop additional theme folders into `themes/` to add more
- Per-user appearance settings: dark mode, colour scheme, font family, sidebar configuration
- Custom CSS settable per theme in the UI - no restart needed
- Template overrides: place `.gohtml` files in `themes/overwrite/` to override specific pages without touching the theme itself

---

## Organisation Methods

- **Tags** - free-form, fully customisable
- **Collections** - automatic grouping by top-level folder, overridable per file
- **Parent/child hierarchy** - set a parent to build a tree; ancestors and children are computed automatically
- **Editor/File types** - custom editors for: todo's, list's, filter, index/MOC files with every output being stored as a viable markdown file

---

## Architecture

- **Backend** - Go with Chi router
- **Frontend** - HTMX + Go HTML templates; no JavaScript framework
- **Content storage** - flat files on the local filesystem
- **Metadata & config storage** - SQLite (default) with JSON fallback options
- **Search** - SQLite FTS5 with BM25 ranking; grep mode available
- **Internationalisation** - English and German; additional languages addable via translation files
