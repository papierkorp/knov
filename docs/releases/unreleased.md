# unreleased

_16 commits since last release_

## upgrading from v1.1.1 to next release

### breaking changes

- api: `POST /api/files/export/zip` and `POST /api/files/export/markdown-converted` were removed. use `GET /api/exports/files` / `GET /api/exports/markdown` instead (the archive is streamed directly, no more building it in memory first)
- logging: the `dokuwiki-export` log key was removed, the bulk export logs (files, markdown, pdf) now go to the `export` log. an existing `dokuwiki-export.log` can be deleted
- api: `POST /api/files/rename/{filepath}` no longer always answers with an `HX-Redirect` to the renamed file - it only redirects when the `HX-Current-URL` request header points at the renamed file (`POST /api/files/move-folder/{folderpath}` does the same for a file inside the moved folder), otherwise it answers with an `HX-Trigger` notify toast. scripts that relied on the redirect should read the new path from the response body instead
- themes: the dropdown menu classes `fp-menu-wrap`, `fp-menu-btn`, `fp-menu`, `fp-menu-item` and `fp-menu-item--danger` were renamed to `menu-wrap`, `menu-btn`, `menu`, `menu-item` and `menu-item--danger`. update custom css and custom themes that target the old names
- links: link paths are now percent-decoded exactly once everywhere (rendering, link metadata, rename and media relocation), and a renamed or relocated link is written percent-encoded (`[x](new%20name.md)`, `[[a%25b]]` for a file `a%b.md`) so it reads back unchanged. a hand-written link with a literal `%XX` in a filename (`[x](a%41.md)` for a file named `a%41.md`) now means the decoded name (`aA.md`) for link metadata and renames too, like it already did when rendering - write the `%` as `%25`
- markdown links: `.md` is only added to a link path without any extension (like wikilinks already do), so `[x](sub.index)` now opens `sub.index` and `[x](notes.txt)` opens `notes.txt`. a link to an extensionless note whose name contains a dot (`[x](v1.2 notes)`) has to be written with the extension (`[x](v1.2 notes.md)`)

## changes
- remove all selected rows/columns in table editor
- highlight the toc entry matching the url hash in the sidebar
- add optional setting to remove empty folders in the file sync cronjob

## features
- rename a kanban status everywhere from the settings page (tags, foldersync folders, card order, history)
- live table of contents in the codemirror editor

## fixes
- resolve link anchors written as heading text to the heading id

