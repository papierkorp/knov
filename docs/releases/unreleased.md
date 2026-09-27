# unreleased

_72 commits since last release_

## breaking changes
- unify link extraction and rewriting in one link walker, please make a manual metadata rebuild
- existing tracker counter history embedded in each tracker's config file is imported automatically into the new storage the first time that tracker is opened after upgrading (see importLegacyDays). Back up your storage/config directory before upgrading, and open each existing tracker at least once post-upgrade to confirm the migration ran before relying on any automated backup rotation to expire the pre-upgrade one.

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
- guard configeditor ids against path traversal
- apply browse hide scope to metadata counts by computing them live
- escape user content in kanban html rendering
- guard folder api against path traversal
- keep todo checkboxes correct across short code blocks and loose list items
- encode file path segments in file-edit links

