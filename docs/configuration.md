# Configuration

A deeper look at each system. For the initial setup see `quickstart.md`.  
All env variables go in an optional `.env` file, starting from `.env.example` - without one, every variable falls back to its default. Changes require a restart (exceptions noted below). See `/system/environment` for every recognized `KNOV_*` variable with its description, default and current value.

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

**Auth options:** SSH key (`KNOV_GIT_SSH_KEY`) or HTTPS with a username + `KNOV_GIT_TOKEN` or `KNOV_GIT_PASSWORD` (token wins if both are set). If `KNOV_GIT_REMOTE` is set and no local repo exists yet, knov clones it on first start instead of init'ing an empty one.

---

## Kanban

The kanban board organises files into columns based on status tags. Everything below except the tag prefix and the event log is configured in the **Kanban Settings** section on `/settings` (no restart needed).

**Boards:**
- Each board is a folder you configure explicitly via **Boards** - format: `folder/path:Display Name`, comma-separated, e.g. `projects/work:Work Board, personal/todo:Personal Todo`
- A board covers that folder and all its subfolders (recursive)
- The board's URL is a slug derived from its folder path (e.g. `projects/work` → `/kanban/projects-work`); duplicate slugs are automatically disambiguated with a numeric suffix
- `/kanban` with no boards configured shows an empty picker; each configured board appears there under its display name
- Renaming a board's folder doesn't update **Boards** - the Kanban Settings section warns about a board folder that doesn't exist, fix the entry there

**Placing files on a board:**
- Add one status tag to a file inside a configured board's folder to place it in a column - e.g. `kb-status-inbox`
- Only one status tag per file is valid; if you add two the last one wins
- The tag is `<prefix>-status-<status>`; the prefix is `KNOV_KANBAN_PREFIX` (default `kb`, letters, digits and _ only, needs a restart), the `-status-` middle is fixed
- Changing the prefix or removing/renaming a status never deletes existing tags - those cards just drop off the board until retagged, and the Kanban Settings section lists them as a warning

**Statuses and columns:**
- **Statuses** (default `inbox, inprogress, blocked, archive`) defines every valid status - adding a new status tag outside this list is rejected
- **Columns** (default `inbox, inprogress, blocked`) is the subset shown as columns on the board; applies to every board
- **Archive Status** (default `archive`) is shown as a separate drop zone while dragging rather than a column - leave empty to disable it
- **Ancestor Filter Statuses** (default empty = no restriction) - only show an ancestor in the ancestor filter when a descendant card has one of these statuses

**Folder-sync (opt-in):**
- **Folder Sync** - comma-separated board folder paths (subset of **Boards**) where card position and on-disk location are kept in sync
- Moving a card moves the file into `folder/path/<status>/`; moving a file on disk into such a folder sets its status tag (picked up by the file-sync cronjob)
- If you move files by hand outside the app, run a manual file-sync before dragging those cards in the UI - the board doesn't know about an external move until the cronjob has run

**Card colours:**
- Non-status tags appear as chips on each card
- Give specific tags a colour with **Tag Colors** - e.g. `urgent:red,user1:green`
- Style cards per status with **Card Styles** (`status:style`, comma-separated) - styles: `normal`, `italic`, `highlighted`, `deleted`; e.g. `archive:deleted, inprogress:highlighted, waiting:italic`
- Any valid CSS colour name or hex value works for tag colours

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

### MOTD banner

- Set `KNOV_MOTD` to show a banner across the top of every page - handy for telling a test/copy setup apart from the real one at a glance
- Injected server-side into every theme (`#site-motd`), not something a theme opts into - it can only be hidden with `#site-motd { display: none }`
- Empty (the default) shows nothing

---

## File Visibility

Configured under **Settings => File Types / Folders**.

**Hide Paths** - comma-separated folder path patterns to exclude from file listings, browse, search, filter and the kanban board.
- Patterns are `/`-separated; `*` matches any single segment, e.g. `*/todo` hides every folder named `todo`, while `test/todo` only hides the `todo` folder inside `test`
- By default a pattern hides its folder everywhere
- Append `::tree`, `::browse`, `::overview`, `::search`, `::filter`, `::kanban`, `::detail` and/or `::dashboard` to a pattern (combine with `|`, e.g. `::search|filter`) to hide it in only those scopes, leaving it visible everywhere else, e.g. `projects/archive::search|filter` is hidden from search results and saved filters but still shows up in the file tree, browse, overview and kanban
- An unrecognized scope in a pattern (e.g. a typo like `::serach`) is rejected on save rather than silently doing nothing
- An invalid regular expression in a pattern segment (e.g. `[unclosed`) is rejected on save as well
- Media has no per-scope override - a `::tag` suffix never hides a path from `/media`; only a pattern with no suffix does

**Hide Files By Tag** - comma-separated tag patterns; every file carrying a matching tag is hidden, like Hide Paths but by tag.
- Case-insensitive; `*` matches any run of characters, e.g. `private*` hides files tagged `private` or `private-notes`, while `private` without a `*` only matches that exact tag
- Uses the same scopes as Hide Paths - append e.g. `::search|filter` to hide the files only there
- Careful with the kanban status tags (e.g. `kb-status*`): every kanban card carries one, so such a pattern hides all cards of the affected columns - the kanban board shows a warning in each column hidden this way

---

## Filters

Filters are saved queries that produce a live list of matching files.

- Create a filter via **New File => Filter**
- Configure criteria (field, operator, value) and logic (AND / OR)
- Saving a filter stores the config and immediately generates a paired index file showing the current results
- The index file updates automatically whenever metadata changes - you do not need to re-save the filter
- Filters are available as dashboard widgets and as browse targets

**Supported fields:** title, collection, tags, folders, editor type, created/edited date, kanban added/moved date, parent/child/ancestor relationships, references - the field list in the filter editor is the authoritative list.

---

## Trackers

Trackers are named counters (e.g. habit or hit-count tracking) with +/- buttons and a generated markdown stats table.

- Create a tracker via **New File => Tracker**
- `KNOV_TRACKER_ENABLED=true` (default) - set to `false` to disable the tracker editor entirely (hidden from **New File**, no storage is created)
- disabling it doesn't delete existing counter history - the day-deltas stay on disk untouched, but read as empty while disabled, so an existing tracker's stats markdown regenerates to all-zero until it's turned back on

---

## File Auto-Tagging

Useful for kanban setups where every new file in a given folder should land in a default column.

- `KNOV_AUTOCREATE_TAGS` - comma-separated list of `folder/path:tag` entries (recursive - also covers subfolders); a bare tag with no `folder/path:` prefix applies to every newly created file everywhere
- Multiple entries can target the same folder (or the same tag can be repeated across folders) - each entry is independent

Example: `KNOV_AUTOCREATE_TAGS=projects/work:kb-status-inbox,personal/todo:kb-status-inbox` puts every new file created under either of those two kanban board folders straight into the inbox column of its board.

---

## Storage Providers

Knov's own internal data (not your files - `docs/`/`media/` are plain files on disk, versioned by git) lives in pluggable per-system backends under `KNOV_STORAGE_PATH` (default `storage/`), each configured independently:

| Variable | Options | Default | Backs |
|---|---|---|---|
| `KNOV_CONFIG_STORAGE_PROVIDER` | `json` | `json` | app settings, filter configs and tracker titles/columns |
| `KNOV_METADATA_STORAGE_PROVIDER` | `json`, `yaml`, `sqlite` | `sqlite` | tags, dates, relationships per file |
| `KNOV_CACHE_STORAGE_PROVIDER` | `json`, `sqlite` | `sqlite` | rendered-content cache |
| `KNOV_SEARCH_STORAGE_PROVIDER` | `sqlite` | `sqlite` | full-text search index |
| `KNOV_TRACKER_STORAGE_PROVIDER` | `sqlite` | `sqlite` | tracker counter day-deltas |
| `KNOV_KANBAN_EVENTS_STORAGE_PROVIDER` | `json`, `sqlite` | `sqlite` | kanban card move history |

- `yaml` for metadata stores each file's tags/dates/etc. as front matter inside the file itself instead of a separate database - the only provider whose data is already covered by git rather than a backup (see Backup & Restore)
- Changing a provider takes effect after a restart; restoring an older backup converts its data into whichever provider is currently configured automatically (see Backup & Restore)

---

## Metadata & Search

Knov tracks metadata (tags, collection, dates, relationships) for every file automatically. You do not configure this - it runs in the background.

**What you can influence:**
- tags, parent relationships and references set manually per file in the sidebar
- The metadata rebuild also runs after every save, on top of its background schedule (see Background Jobs) - you can trigger it manually from the admin page if something looks out of sync

**Search** is full-text and indexed in the background after each save, on top of its own background schedule (see Background Jobs). It covers file content as well as metadata fields.
- `KNOV_SEARCH_ENGINE` picks how a query runs: `repository` (default) hits the sqlite full-text index; `grep` reads files on disk directly instead - slower, but always current even if the index hasn't caught up yet

**Search history** - the search page has a "search history" toggle that searches deleted files in git history. Useful when you want to remember content from a file you deleted. (can be slower in huge git repository)

---

## Background Jobs

Knov runs a handful of jobs on their own timers, independently of anything you trigger by saving a file. Every current run and its outcome is visible in the admin page's job history.

| Job | Interval | Configurable | Runs on startup too |
|---|---|---|---|
| File Sync | `KNOV_CRONJOB_INTERVAL` (default `5m`) | yes | yes |
| Search Reindex | `KNOV_SEARCH_INDEX_INTERVAL` (default `15m`) | yes | yes |
| Metadata Rebuild | `KNOV_METADATA_REBUILD_INTERVAL` (default `60m`) | yes | yes, once, 2 minutes after start |
| Git Repack | fixed `24h` | no | yes (cheap no-op unless already over the loose-object threshold) |
| Automatic Backup check | fixed `15m` | no | yes (catches a backup missed while the app was down) |

**File Sync** - the workhorse job, on `KNOV_CRONJOB_INTERVAL`:
- Pulls the remote (`KNOV_GIT_REMOTE`, if set) and commits any pending local changes
- Diffs against the last commit it processed to find files changed, deleted or renamed since
- For renamed files, updates every link pointing at the old path (see Filters/links) and applies kanban foldersync (see Kanban) if the move landed the file in a status folder
- Removes metadata for deleted files, (re)builds metadata for new/changed files - filling only empty fields (e.g. editor type), so it never overwrites something you set by hand
- Rebuilds the file/folder cache and regenerates saved-filter indexes as a final sub-step

**Search Reindex** - rebuilds the full-text search index from every file, on `KNOV_SEARCH_INDEX_INTERVAL`. Skipped entirely when `KNOV_SEARCH_ENGINE=grep`, since there's no index to keep current.

**Metadata Rebuild** - rebuilds cross-file relationships (parent/child links, ancestry) from what's on disk, on `KNOV_METADATA_REBUILD_INTERVAL`. This is the "link graph" rebuild, distinct from File Sync's per-file metadata sync above.

**Git Repack** - packs loose git objects once their count crosses an internal threshold, keeping repo size and git operations fast on a long-running instance. Shares its lock with File Sync, so a repack in progress simply delays the next tick's git work rather than racing it.

**Automatic Backup check** - evaluates every `KNOV_BACKUP_AUTO_PROFILES` entry and runs any that are due (see Backup & Restore).

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
  - it's a full backup and among the `KNOV_BACKUP_ROTATION_KEEP_DEFAULT` most recent full backups (default 10) - a long-term floor so coming back after months away still leaves something restorable, even if daily backups lapsed. Partial backups (e.g. "just metadata") get no long-term floor of their own - once they age out of the days window, they're deleted
  - it's locked - click "Lock" on any set to keep it forever regardless of the two settings above, until "Unlock" is clicked
- **S3 storage** - by default backup sets are `.tar.gz` files on the local filesystem under `KNOV_BACKUPS_PATH`. Set `KNOV_BACKUP_S3_BUCKET` to store them in an S3 bucket instead (works with AWS S3 and any S3-compatible service: MinIO, Cloudflare R2, Backblaze B2, DigitalOcean Spaces). `KNOV_BACKUPS_PATH` is then unused. Everything else (rotation, locking, the log page, download, restore, auto-profiles) works identically.
  - `KNOV_BACKUP_S3_ENDPOINT` - endpoint host, no scheme, e.g. `s3.amazonaws.com`, `nyc3.digitaloceanspaces.com`, `localhost:9000`. Required whenever the bucket is set
  - `KNOV_BACKUP_S3_ACCESS_KEY` / `KNOV_BACKUP_S3_SECRET_KEY` - credentials (redacted on `/system/environment`)
  - `KNOV_BACKUP_S3_REGION` - e.g. `us-east-1`; leave empty for providers that don't need it (Cloudflare R2 wants `auto`)
  - `KNOV_BACKUP_S3_PREFIX` - optional key prefix inside the bucket, e.g. `knov/backups/`; empty = bucket root
  - `KNOV_BACKUP_S3_USE_SSL` - `true` (default) connects over HTTPS; set `false` only for a plain-HTTP local or private-LAN MinIO. With `false` against any endpoint reachable over the public internet the access/secret key cross the network unencrypted
  - The bucket must already exist - knov doesn't create it. A wrong endpoint, bad credentials or a missing bucket surface as an error on `/system/backup` (and in the log for scheduled backups), not silently. Changing any `KNOV_BACKUP_S3_*` value requires a restart
  - One bucket+prefix belongs to a single knov instance. The event log (`log.json`) is rewritten whole on every backup and archive names collide across instances, so two instances sharing a bucket+prefix will lose log entries and can clobber each other's sets - give each its own bucket or a distinct prefix
  - Backups contain the full dataset, so on the bucket side enable server-side encryption and object versioning, and hand knov a key scoped to just get/put/list/delete under the prefix
- **Automatic backups** - off by default. Set `KNOV_BACKUP_AUTO_PROFILES` to one or more `name:cron:storages` entries, semicolon-separated, to create backups periodically in the background, on top of manual ones:
  - `cron` is a standard 5-field expression, e.g. `0 18 * * *` for daily at 18:00
  - `storages` is comma-separated and may be empty for the default backup set (metadata, chat, kanban, notifications, config, search)
  - e.g. `KNOV_BACKUP_AUTO_PROFILES=daily:0 0 * * *:metadata,chat,kanban,notifications,config;weekly-docs:0 0 * * 0:docs,media`
  - Each profile's due time tracks off that profile's own newest backup timestamp rather than a "ran today" flag, so a device that isn't running 24/7 still catches up as soon as it's next on past a missed slot
  - Requires a restart to take effect, like every other env var

## Logging

- `KNOV_LOG_LEVEL` (stdout) and `KNOV_LOG_FILE_LEVEL` (file) take effect immediately - no restart needed, unlike every other env var. The remaining `KNOV_LOG_*` vars are read once at startup.
- File logging writes to `logs/app.log` and rotates automatically
- For production use `info` or `warning` - `debug` is verbose

## favicon

To upload/use a favicon:
- use the theme settings (supported by the builtin theme)
- in the storage folder create a favicon folder and in there copy your favicon.ico, favicon.png or favicon.svg
