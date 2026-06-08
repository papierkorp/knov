# Configuration

A deeper look at each system. For the initial setup and key env variables see `quickstart.md`.  
All env variables go in your `.env` file — changes require a restart.

---

## Git & Conflict Handling

Knov uses git to version every file change automatically. You do not interact with git directly.

**Local only** — the default. No remote needed, full history available via the file history view.

**Remote sync** — set `KNOV_GIT_REMOTE` to enable. On every save knov commits and pushes in the background without blocking you. A background cronjob retries anything that failed (e.g. during a network hiccup).

**Multiple users on one remote:**
- Each user runs their own binary pointing to the same remote
- If two users edit different files — no problem, both changes apply
- If two users edit the **same file** at the same time:
  - the slower version is saved as `filename.conflict.YYYYMMDD-HHMMSS.md`
  - The file resets to the remote version
  - A warning notification appears and a diff banner shows above the file
  - You manually copy your changes back and save — no work is ever lost
  - Only one conflict copy is kept per file at a time

**Auth options:** SSH key (`KNOV_GIT_SSH_KEY`) or HTTPS with a personal access token (`KNOV_GIT_TOKEN`). Token takes priority over password if both are set.

---

## Kanban

The kanban board organises files into columns based on status tags.

**How files appear on a board:**
- A file must live inside a subfolder — the top-level folder name becomes the board (collection)
- Add one status tag to the file to place it in a column — e.g. `kb-status-inbox`
- Only one status tag per file is valid; if you add two the last one wins

**Configuring columns:**
- Default columns: `inbox`, `inprogress`, `blocked`, `archive`
- Change them with `KNOV_KANBAN_COLUMNS` (comma-separated)
- The tag prefix defaults to `kb-status` — change it with `KNOV_KANBAN_PREFIX`
- Tags that start with the prefix but are not in the allowed column list are rejected

**Card colours:**
- Non-status tags appear as chips on each card
- Give specific tags a colour with `KNOV_KANBAN_TAG_COLORS` — e.g. `urgent:red,user1:green`
- Any valid CSS colour name or hex value works

**Filtering on the board:**
- Quick filters (ancestor, tag, search) are always visible in the toolbar
- An advanced filter panel (same system as saved filters) can be opened with the filter button

---

## Themes & Appearance

**Switching themes:**
- Drop a theme folder into `themes/` and select it under **Settings => Theme**
- The builtin theme is always available as a fallback

**Per-user appearance settings** (no `.env` needed, saved in the UI):
- Dark mode, colour scheme, font family
- Which metadata fields show in the sidebar
- Custom CSS — applied on top of the active theme, survives theme switches

**Overwrite templates:**
- Create `themes/overwrite/` and place `.gohtml` files there
- Any template in that folder takes precedence over the active theme on every request — no restart needed
- Only put the templates you actually want to change there; everything else falls back normally
- as a base you can copy the template from your current theme

---

## Filters

Filters are saved queries that produce a live list of matching files.

- Create a filter via **New File => Filter**
- Configure criteria (field, operator, value) and logic (AND / OR)
- Saving a filter stores the config and immediately generates a paired index file showing the current results
- The index file updates automatically whenever metadata changes — you do not need to re-save the filter
- Filters are available as dashboard widgets and as browse targets

**Supported fields:** title, collection, tags, folders, editor type, created/edited date, PARA fields, ancestry, references and more — the field list in the filter editor is the authoritative list.

---

## File Auto-Tagging

Useful for kanban setups where every new file should land in a default column.

- `KNOV_AUTOCREATE_TAGS` — comma-separated list of tags applied to every newly created file
- `KNOV_AUTOCREATE_COLLECTIONS` — if set, auto-tagging only applies to files created in these collections; leave empty to apply everywhere

Example: `KNOV_AUTOCREATE_TAGS=kb-status-inbox` puts every new file straight into the inbox column.

---

## Metadata & Search

Knov tracks metadata (tags, collection, dates, relationships, PARA fields) for every file automatically. You do not configure this — it runs in the background.

**What you can influence:**
- tags, parent relationships and references set manually per file in the sidebar
- The metadata rebuild runs on a background cronjob and also after every save — you can trigger it manually from the admin page if something looks out of sync

**Search** is full-text and indexed in the background after each save. It covers file content as well as metadata fields.

---

## Notifications

Knov shows brief toast notifications for save confirmations, errors and git conflicts.

- `KNOV_NOTIFY_DURATION` — how long notifications stay visible in milliseconds (default: 3500)

Each theme supports a way to take a look at the last 100 notifications.

---

## Logging

- `KNOV_LOG_LEVEL` — controls verbosity (`debug`, `info`, `warning`, `error`)
- Logs rotate automatically; old log files are kept in `logs/`
- For production use `info` or `warning` — `debug` is verbose
