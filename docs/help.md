# Metadata Browsing

KNOV provides browsing capabilities for the following metadata fields on the overview page (`/browse/files`):

- **Tags**: Browse files by tags
- **Collections**: Browse files by collection
- **Folders**: Browse files by folder
- **Type**: Browse files by type (todo, fleeting, literature, moc, permanent, filter, journaling)
- **Priority**: Browse files by priority (low, medium, high)
- **Status**: Browse files by status (draft, published, archived)
- **PARA Organization**: Browse files by projects, areas, resources, and archive

Each browse link shows the count of files with that metadata value. Clicking a browse link displays all files with that specific metadata.

# Kanban

## Setup

- A file must be inside a **subfolder** to appear on a board (the folder name becomes the collection/board name)
- Add a kanban status tag to a file to put it on the board — e.g. `kb-status-inbox`
- Valid default statuses: `inbox`, `inprogress`, `blocked`, `archive` (can be configured by your admin)

## Using the Board

- Go to **`/kanban`** to pick a board (one per top-level folder)
- **Drag a card** to another column to change its status — saves automatically

## Filtering

- **Ancestors** dropdown — show only cards linked to a specific parent file
- **Tags** dropdown — filter by a specific tag
- **Search** — filters by title or file path as you type

## Tag Rules

- Only one kanban status tag per file — if you add two, the last one wins
- Tags starting with `kb-` but not in the allowed list are **rejected** with an error message in the editor
- Non-kanban tags (e.g. `project`, `urgent`) are shown as chips on the card and are not affected

# Git

Leave `KNOV_GIT_REMOTE` empty (the default) to enable local-only mode — local commits only, no push/pull, no auth required.

## Remote Sync

- Each user runs their own `knov` binary with separate settings/logs/themes/data
- All users share a common git remote (Gitea, GitHub, Gitlab, bare repo, etc.)
- On every file save, knov immediately commits and pushes (non-blocking, runs in the background)
- Before committing, knov fetches the remote and hard-resets to the remote HEAD to stay in sync
- The cronjob runs independently as a fallback: pulls at the start of each cycle, commits any files that were missed (e.g. saved during a remote timeout, or modified outside knov), and retries any pushes that failed on save

**Conflict handling**

- if two users edit different files => no conflict, both commits apply cleanly
- if two users edit the same file simultaneously:
  - knov fetches the remote — if the remote also changed this file:
    - your version is saved as `filename.conflict.YYYYMMDD-HHMMSS.md`
    - the file on disk is reset to the remote version (current HEAD)
    - a warning notification appears: `conflict in file.md — your version saved as file.conflict.md`
    - a banner appears above the file content with a link to the conflict copy and an inline diff
    - the conflict copy itself shows a banner indicating it is a conflict copy, also with an inline diff
    - you can then manually review, copy your changes back, and save again
  - only one conflict copy is kept at a time — a new conflict overwrites the previous one
- Fetch timeout (default 10s): if the remote is slow, knov skips the fetch and commits locally — it will sync on the next cronjob cycle


## Configuration

```env
KNOV_GIT_REMOTE=git@github.com:user/repo.git   # empty = local only
KNOV_GIT_SSH_KEY=/home/user/.ssh/id_rsa
KNOV_GIT_REMOTE_BRANCH=main
KNOV_GIT_AUTO_PUSH=true
KNOV_GIT_PUSH_TIMEOUT=10s

# HTTPS auth (token takes priority over password)
KNOV_GIT_USER=myuser
KNOV_GIT_TOKEN=ghp_xxxxx
# KNOV_GIT_PASSWORD=mypassword   # alternative to token

# SSH: point to your private key via KNOV_GIT_SSH_KEY
```
