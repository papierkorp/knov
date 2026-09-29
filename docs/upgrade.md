<!-- upgrade notes for the next release: what changed, what the user has to do now, what is deprecated or removed (e.g. "## breaking changes", "## deprecated", "## removed" sections, no "# " headings). `make release` copies them into docs/releases/<version>.md and resets this file -->

## breaking changes

### kanban config moved from env vars to the settings page
- **what changed:** kanban boards, statuses, columns, tag colors, card styles, archive status, ancestor filter and foldersync are no longer read from env vars.
- **what you need to do:** re-enter your kanban configuration under Settings > Kanban.

### auto-create tags moved from env var to the settings page
- **what changed:** `KNOV_AUTOCREATE_TAGS` is no longer read.
- **what you need to do:** re-enter your auto-create tags under Settings > Kanban.

### allowedMimeTypes setting renamed to allowedMediaTypes
- **what changed:** a customized list is not migrated and falls back to the defaults.
- **what you need to do:** re-add your entries under Settings > Media Settings > Allowed Media Types.

### link extraction and rewriting unified
- **what changed:** links are extracted by a new link walker, existing metadata still has the old link data.
- **what you need to do:** run a full metadata rebuild (Rebuild Metadata) once after upgrading.

### tracker counter history moved to a new storage
- **what changed:** the counter history embedded in each tracker's config file is imported automatically into the new storage the first time that tracker is opened.
- **what you need to do:** back up your storage/config directory before upgrading, then open each existing tracker at least once to confirm the import ran before an automated backup rotation expires the pre-upgrade backup.

## removed
- `KNOV_KANBAN_STATUS`, `KNOV_KANBAN_COLUMNS`, `KNOV_KANBAN_TAG_COLORS`, `KNOV_KANBAN_CARD_STYLES`, `KNOV_KANBAN_ARCHIVE_STATUS`, `KNOV_KANBAN_ANCESTOR_ALLOWED_STATUS`, `KNOV_KANBAN_BOARDS`, `KNOV_KANBAN_FOLDERSYNC` - use Settings > Kanban
- `KNOV_AUTOCREATE_TAGS` - use Settings > Kanban
