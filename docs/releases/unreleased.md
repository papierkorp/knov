# unreleased

_92 commits since last release_

## upgrading from v1.0.0 to next release

### breaking changes

#### kanban config moved from env vars to the settings page
- **what changed:** kanban boards, statuses, columns, tag colors, card styles, archive status, ancestor filter and foldersync are no longer read from env vars.
- **what you need to do:** re-enter your kanban configuration under Settings > Kanban.

#### auto-create tags moved from env var to the settings page
- **what changed:** `KNOV_AUTOCREATE_TAGS` is no longer read.
- **what you need to do:** re-enter your auto-create tags under Settings > Kanban.

#### allowedMimeTypes setting renamed to allowedMediaTypes
- **what changed:** a customized list is not migrated and falls back to the defaults.
- **what you need to do:** re-add your entries under Settings > Media Settings > Allowed Media Types.

#### link extraction and rewriting unified
- **what changed:** links are extracted by a new link walker, existing metadata still has the old link data.
- **what you need to do:** run a full metadata rebuild (Rebuild Metadata) once after upgrading.

#### tracker counter history moved to a new storage
- **what changed:** the counter history embedded in each tracker's config file is imported automatically into the new storage the first time that tracker is opened.
- **what you need to do:** back up your storage/config directory before upgrading, then open each existing tracker at least once to confirm the import ran before an automated backup rotation expires the pre-upgrade backup.

### removed
- `KNOV_KANBAN_STATUS`, `KNOV_KANBAN_COLUMNS`, `KNOV_KANBAN_TAG_COLORS`, `KNOV_KANBAN_CARD_STYLES`, `KNOV_KANBAN_ARCHIVE_STATUS`, `KNOV_KANBAN_ANCESTOR_ALLOWED_STATUS`, `KNOV_KANBAN_BOARDS`, `KNOV_KANBAN_FOLDERSYNC` - use Settings > Kanban
- `KNOV_AUTOCREATE_TAGS` - use Settings > Kanban

## changes
- inline toast for browse-list file delete instead of redirect
- add version range picker to release notes page
- add configurable columns and reset action to tracker counters
- paste in tabulator now creates columns/rows
- add table button to codemirror editor
- add settings menu to tableeditor
- support per-column alignment in table editor

## features
- new admin action find media files in doc folder and move to media folder
- add saved filter entries to books
- add switchable file views (rendered/raw, tracker statistics/counters)
- replace hide tags with hide files by tag
- add Android wrapper app (apk build)
- add hide tags besides hide paths
- add a open with another editor button to the built in theme info slideout
- add a date stamps setting for todo lists
- change tableeditor from handsontable to MIT licensed tabulator
- new tracker editor
- add KNOV_NOTIFY_MIN_LEVEL to mute notification toasts

## fixes
- sandbox every media response except pdf
- add the markdown extension to new files whose name contains a dot
- guard configeditor ids against path traversal
- apply browse hide scope to metadata counts by computing them live
- escape user content in kanban html rendering
- guard folder api against path traversal
- keep todo checkboxes correct across short code blocks and loose list items
- encode file path segments in file-edit links

