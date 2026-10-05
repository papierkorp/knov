# unreleased

_13 commits since last release_

## upgrading from v1.1.1 to next release

### breaking changes

- api: `POST /api/files/export/zip` and `POST /api/files/export/markdown-converted` were removed. use `GET /api/exports/files` / `GET /api/exports/markdown` instead (the archive is streamed directly, no more building it in memory first)
- logging: the `dokuwiki-export` log key was removed, the bulk export logs (files, markdown, pdf) now go to the `export` log. an existing `dokuwiki-export.log` can be deleted
- api: `POST /api/files/rename/{filepath}` no longer always answers with an `HX-Redirect` to the renamed file - it only redirects when the `HX-Current-URL` request header points at the renamed file (`POST /api/files/move-folder/{folderpath}` does the same for a file inside the moved folder), otherwise it answers with an `HX-Trigger` notify toast. scripts that relied on the redirect should read the new path from the response body instead
- themes: the dropdown menu classes `fp-menu-wrap`, `fp-menu-btn`, `fp-menu`, `fp-menu-item` and `fp-menu-item--danger` were renamed to `menu-wrap`, `menu-btn`, `menu`, `menu-item` and `menu-item--danger`. update custom css and custom themes that target the old names

## changes
- remove all selected rows/columns in table editor
- highlight the toc entry matching the url hash in the sidebar
- add optional setting to remove empty folders in the file sync cronjob

## features
- rename a kanban status everywhere from the settings page (tags, foldersync folders, card order, history)
- live table of contents in the codemirror editor

