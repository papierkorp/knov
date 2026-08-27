# Configuration

A deeper look at each system. For the initial setup and key env variables see `quickstart.md`.  
All env variables go in your `.env` file - changes require a restart.

---

## Git & Conflict Handling

Knov uses git to version every file change automatically. You do not interact with git directly.

**Local only** - the default. No remote needed, full history available via the file history view.

**Remote sync** - set `KNOV_GIT_REMOTE` to enable. On every save knov commits and pushes in the background without blocking you. A background cronjob retries anything that failed (e.g. during a network hiccup).

**Multiple users on one remote:**
- Each user runs their own binary pointing to the same remote
- If two users edit different files - no problem, both changes apply
- If two users edit the **same file** at the same time:
  - the slower version is saved as `filename.conflict.YYYYMMDD-HHMMSS.md`
  - The file resets to the remote version
  - A warning notification appears and a diff banner shows above the file
  - You manually copy your changes back and save - no work is ever lost
  - Only one conflict copy is kept per file at a time

**Auth options:** SSH key (`KNOV_GIT_SSH_KEY`) or HTTPS with a personal access token (`KNOV_GIT_TOKEN`). Token takes priority over password if both are set.

---

## Kanban

The kanban board organises files into columns based on status tags.

**Boards:**
- Each board is a folder you configure explicitly via `KNOV_KANBAN_BOARDS` - format: `folder/path:Display Name`, comma-separated, e.g. `KNOV_KANBAN_BOARDS=projects/work:Work Board,personal/todo:Personal Todo`
- A board covers that folder and all its subfolders (recursive)
- The board's URL is a slug derived from its folder path (e.g. `projects/work` → `/kanban/projects-work`); duplicate slugs are automatically disambiguated with a numeric suffix
- `/kanban` with no boards configured shows an empty picker; each configured board appears there under its display name

**Placing files on a board:**
- Add one status tag to a file inside a configured board's folder to place it in a column - e.g. `kb-status-inbox`
- Only one status tag per file is valid; if you add two the last one wins

**Configuring columns:**
- Default columns: `inbox`, `inprogress`, `blocked`, `archive`
- Change them with `KNOV_KANBAN_COLUMNS` (comma-separated) - these apply to every board
- The tag prefix defaults to `kb-status` - change it with `KNOV_KANBAN_PREFIX`
- Tags that start with the prefix but are not in the allowed column list are rejected
- add a "archive" status which is not displayed as a column but activated as a separate drop zone with `KNOV_KANBAN_ARCHIVE_STATUS` - leave it empty to disable this feature

**Card colours:**
- Non-status tags appear as chips on each card
- Give specific tags a colour with `KNOV_KANBAN_TAG_COLORS` - e.g. `urgent:red,user1:green`
- Give specific column cards a different style with `KNOV_KANBAN_CARD_STYLES` - e.g `archive:deleted, inprogress:highlighted, waiting:italic`
- Any valid CSS colour name or hex value works

**Event log:**
- Every time a card moves between columns an event is recorded (file, board folder, from/to status, timestamp)
- Query events via `GET /api/kanban/{board}/events` (board = the URL slug) — supports `?file=`, `?from=`, `?to=`, `?limit=` parameters
- `KNOV_KANBAN_EVENTS_ENABLED=true` (default) — set to `false` to disable event logging entirely
- `KNOV_KANBAN_EVENTS_STORAGE_PROVIDER=sqlite` (default) — storage backend for events

**Filtering on the board:**
- Quick filters (ancestor, tag, search) are always visible in the toolbar
- An advanced filter panel (same system as saved filters) can be opened with the filter button

---

## Themes & Appearance

### Switching themes
- Drop a theme folder into `themes/` and select it under **Settings => Theme**
- The builtin theme is always available as a fallback

### Overwrite templates
- Create `themes/overwrite/` and place `.gohtml` files there
- Any template in that folder takes precedence over the active theme on every request - no restart needed
- Only put the templates you actually want to change there; everything else falls back normally
- as a base you can copy the template from your current theme

### custom.css

- is supported by default
- use this: `@import url("/themes/xxx.css");` in the setting to use a viable css file with proper syntax highlighting

---

## File Visibility

Configured under **Settings => File Types / Folders**.

**Hide Paths** - comma-separated folder path patterns to exclude from file listings, browse, search, filter and the kanban board.
- Patterns are `/`-separated; `*` matches any single segment, e.g. `*/todo` hides every folder named `todo`, while `test/todo` only hides the `todo` folder inside `test`
- By default a pattern hides its folder everywhere
- Append `::tree`, `::browse`, `::overview`, `::search`, `::filter` and/or `::kanban` to a pattern (combine with `|`, e.g. `::search|filter`) to hide it in only those scopes, leaving it visible everywhere else, e.g. `projects/archive::search|filter` is hidden from search results and saved filters but still shows up in the file tree, browse, overview and kanban
- An unrecognized scope in a pattern (e.g. a typo like `::serach`) is rejected on save rather than silently doing nothing
- Media has no per-scope override - a `::tag` suffix never hides a path from `/media`; only a pattern with no suffix does

---

## Filters

Filters are saved queries that produce a live list of matching files.

- Create a filter via **New File => Filter**
- Configure criteria (field, operator, value) and logic (AND / OR)
- Saving a filter stores the config and immediately generates a paired index file showing the current results
- The index file updates automatically whenever metadata changes - you do not need to re-save the filter
- Filters are available as dashboard widgets and as browse targets

**Supported fields:** title, collection, tags, folders, editor type, created/edited date, PARA fields, ancestry, references and more - the field list in the filter editor is the authoritative list.

---

## File Auto-Tagging

Useful for kanban setups where every new file in a given folder should land in a default column.

- `KNOV_AUTOCREATE_TAGS` - comma-separated list of `folder/path:tag` entries (recursive - also covers subfolders); a bare tag with no `folder/path:` prefix applies to every newly created file everywhere
- Multiple entries can target the same folder (or the same tag can be repeated across folders) - each entry is independent

Example: `KNOV_AUTOCREATE_TAGS=projects/work:kb-status-inbox,personal/todo:kb-status-inbox` puts every new file created under either of those two kanban board folders straight into the inbox column of its board.

---

## Metadata & Search

Knov tracks metadata (tags, collection, dates, relationships, PARA fields) for every file automatically. You do not configure this - it runs in the background.

**What you can influence:**
- tags, parent relationships and references set manually per file in the sidebar
- The metadata rebuild runs on a background cronjob and also after every save - you can trigger it manually from the admin page if something looks out of sync

**Search** is full-text and indexed in the background after each save. It covers file content as well as metadata fields.

**Search history** - the search page has a "search history" toggle that searches deleted files in git history. Useful when you want to remember content from a file you deleted. (can be slower in huge git repository)

---

## Notifications

Knov shows brief toast notifications for save confirmations, errors and git conflicts.

- `KNOV_NOTIFY_DURATION` - how long notifications stay visible in milliseconds (default: 3500)

Each theme supports a way to take a look at the last 100 notifications.

---

## PDF Export

Configured under **Settings => PDF Export**.

- Page format, orientation, margin, fonts and task-icon rendering are all configurable
- A header and a footer can each be set per zone (left, center, right), shown on every exported page
- Header/footer text supports the tokens `{{date}}`, `{{page}}`, `{{pages}}`, `{{filename}}`, `{{filepath}}`, `{{folder}}`, `{{created}}`, `{{edited}}`, `{{tags}}` and `{{collection}}`; any other text is shown literally
- `{{created}}`/`{{edited}}`/`{{tags}}`/`{{collection}}` come from the file's metadata
- A zone can instead hold a markdown image link, e.g. `![logo](media/logo.png)`, to embed a small local image instead of text - external URLs aren't supported
- Zone images are scaled to 8mm tall (width follows the image's aspect ratio), capped to the zone's width for very wide images, and vertically centered in the zone
- Leave a zone empty to omit it
- Each zone has its own font, size, color, bold and italic setting
- Optional rule lines can be shown above the footer / below the header
- The header and footer can each be hidden on the first page independently, useful for a cover page

## Backup & Restore

Available at **Admin => Backups** (`/system/backup`).

- Snapshots storages under `KNOV_STORAGE_PATH` (metadata, chat, kanban, notifications, config, search) into one backup set, stored locally under `KNOV_BACKUPS_PATH` (default `./backups`, next to `storage/` - point it elsewhere, e.g. a separate disk, to keep backups off the same volume as what they're backing up) - your docs/media (`KNOV_DATA_PATH`) are already versioned by git, so they're not included, and neither is the cache - it only holds derived data rebuilt from files/git, and a restore refreshes it automatically
- Untick any storage before clicking "Create backup" to snapshot only that subset (e.g. just metadata) instead of everything - each row shows that set's actual contents
- The page is a log, not just a current listing: every backup created and every restore applied gets its own row, newest first, and stays there even after the set behind it is gone (rotated away, or a restore whose source set was later deleted) - such a row just loses its actions and is marked "no longer available", the historical fact of it happening is kept
- Click "Download" on any available row to get that set's raw `.tar.gz` archive
- Click "Restore" on any available row to roll back to it
- Restoring is destructive but safe: your current state is snapshotted first (a full backup, regardless of what the restored set contains), then the app restarts to apply the restored data - a bad restore can always be undone by restoring that automatic pre-restore snapshot
- If you've changed a storage's provider (e.g. `KNOV_METADATA_STORAGE_PROVIDER`) since a backup was taken, restoring it converts that storage's data into the currently configured provider automatically instead of applying the old format as-is
- Exception: if metadata is on the `yaml` provider (front matter stored inside docs files), restoring a backup never touches it, even for an old backup also made on `yaml` - that data is already covered by git, so use a file's history view (see Git & Conflict Handling) to roll back its tags/dates/etc. instead. A backup taken while on `yaml` still snapshots your front matter though (into a small sqlite file inside that backup's metadata folder) even though *restoring* it back onto a live `yaml` setup is a no-op - that snapshot exists specifically so it can be applied via the provider-conversion path below. If you specifically need to pull metadata out of an old *backup* while yaml is active (not from git history), temporarily switch `KNOV_METADATA_STORAGE_PROVIDER` to `sqlite` or `json` and restart first - that both migrates your current front matter out and makes the following restore go through the provider-conversion path above, which does apply an old yaml-era backup. Switch back to `yaml` and restart again afterwards to fold the restored data back into front matter
- Old backups are trimmed automatically after every backup. A set survives if it matches any of three independent rules:
  - it's within `KNOV_BACKUP_ROTATION_KEEP_DAYS` days (default 7) - any kind, full or partial
  - it's a full backup and among the `KNOV_BACKUP_ROTATION_KEEP_FULL` most recent full backups (default 10) - a long-term floor so coming back after months away still leaves something restorable, even if daily backups lapsed. Partial backups (e.g. "just metadata") get no long-term floor of their own - once they age out of the days window, they're deleted
  - it's locked - click "Lock" on any set to keep it forever regardless of the two settings above, until "Unlock" is clicked
- **Automatic backups** - off by default. Set `KNOV_BACKUP_AUTO_ENABLED=true` and `KNOV_BACKUP_AUTO_INTERVAL` (e.g. `24h`) to create a full backup periodically in the background, on top of manual ones. Requires a restart to take effect, like every other env var

## Logging

- `KNOV_LOG_LEVEL` - controls verbosity (`debug`, `info`, `warning`, `error`)
- Logs rotate automatically; old log files are kept in `logs/`
- For production use `info` or `warning` - `debug` is verbose

## favicon

To upload/use a favicon:
- use the settings of either the builtin or the rail theme
- in the storage folder create a favicon folder and in there copy your favicon.ico, favicon.png or favicon.svg
