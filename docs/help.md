
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
