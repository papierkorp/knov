# unreleased

_27 commits since last release_

## upgrading from v1.1.1 to next release

### breaking changes

- api: `POST /api/files/export/zip` and `POST /api/files/export/markdown-converted` were removed. use `GET /api/exports/files` / `GET /api/exports/markdown` instead (the archive is streamed directly, no more building it in memory first)
- logging: the `dokuwiki-export` log key was removed, the bulk export logs (files, markdown, pdf) now go to the `export` log. an existing `dokuwiki-export.log` can be deleted
- api: `POST /api/files/rename/{filepath}` no longer always answers with an `HX-Redirect` to the renamed file - it only redirects when the `HX-Current-URL` request header points at the renamed file (`POST /api/files/move-folder/{folderpath}` does the same for a file inside the moved folder), otherwise it answers with an `HX-Trigger` notify toast. scripts that relied on the redirect should read the new path from the response body instead
- themes: the dropdown menu classes `fp-menu-wrap`, `fp-menu-btn`, `fp-menu`, `fp-menu-item` and `fp-menu-item--danger` were renamed to `menu-wrap`, `menu-btn`, `menu`, `menu-item` and `menu-item--danger`. update custom css and custom themes that target the old names
- links: link paths are now percent-decoded exactly once everywhere (rendering, link metadata, rename and media relocation), and a renamed or relocated link is written percent-encoded (`[x](new%20name.md)`, `[[a%25b]]` for a file `a%b.md`) so it reads back unchanged. a hand-written link with a literal `%XX` in a filename (`[x](a%41.md)` for a file named `a%41.md`) now means the decoded name (`aA.md`) for link metadata and renames too, like it already did when rendering - write the `%` as `%25`
- markdown links: `.md` is only added to a link path without any extension (like wikilinks already do), so `[x](sub.index)` now opens `sub.index` and `[x](notes.txt)` opens `notes.txt`. a link to an extensionless note whose name contains a dot (`[x](v1.2 notes)`) has to be written with the extension (`[x](v1.2 notes.md)`)
- api: the `parents` of `POST /api/metadata/parents` (and other parent values) are no longer percent-decoded - send the plain file path (`docs/a b.md`), not an encoded url (`/files/docs/a%20b.md`). a `#`, `?` or `|` in a parent is now part of the filename instead of cutting it off
- markdown links: a link with any scheme (`mailto:`, `tel:`, `ns:page`) or a `//host` is now external and no longer routed to `/files/`, like it already was for link metadata - before only `://` links were. only `http:`, `https:` and `mailto:` links stay clickable, a link with any other scheme (`tel:`, `ns:page`, `obsidian://...`) is shown as plain text. a markdown link to a file whose name contains a ":" before the first "/" has to encode it (`[x](ns%3Apage.md)`) or use a wikilink (`[[ns:page]]`)
- index/book editor: a file entry is now typed and posted as the plain file path (`entries[][value]` of the index/book save api, `a b.md`) and stored url-encoded in the `[[...]]` link. saving the editor rewrites every file entry in that encoded form - a value sent already encoded (`a%20b.md`) is encoded again and points at a file literally named `a%20b.md`. send the plain path instead
- index/book editor: the section of a file entry has its own field (`entries[][section]`, the heading text or id) - a `#` or `|` in `entries[][value]` is now part of the file name, so `a.md#My Section` has to be sent as `a.md` plus section `My Section`. saving through the editor stores a section as its heading id (`#my-section`) and drops a hand-written `|alias` on an entry
- links: links inside inline code or fenced code blocks are no longer rendered as links (they already weren't counted as links). a markdown link to a `/files/` url without extension (`[x](/files/docs/a)`) now points at `a.md` like link metadata reads it - link a folder with a trailing `/`. a markdown link destination starting with an unclosed `<` (`[x](<a.md)`) is no link anymore, like in the rendered page. media relocation reads a markdown link without leading `/` (`![x](sub/a.png)`) from the docs root first, like it renders, and only falls back to the doc's folder. the dokuwiki import links `[[v1.2 notes]]` to the note `v1.2 notes.md` - only a known media extension makes a `[[...]]` link a media link
- links: an html entity in a markdown link or html src/href path (`[x](a&amp;b.md)`) now means the character it stands for (`a&b.md`) for link metadata and renames too, like it already did when rendering - a file with a literal `&` followed by an entity name in its name has to be linked with the `&` written as `%26`. a percent-encoded space at the start or end of a link path (`[x](%20a.md)`, `[[a.md%20]]`) is now part of the file name instead of being trimmed. an unclosed `[[wikilink` or one spanning a line break is no longer counted as a link or rendered
- links: quotes in a markdown link rewritten by rename or media relocation stay unencoded
- api: `POST /api/media/upload` reads `context_path` as the url path of the page the file is uploaded from (`/files/edit/<path>` or `/files/<path>`, percent-encoded like the browser's `location.pathname`) instead of the bare file path, and a page that isn't a saved file (`/files/new/...`) is answered with `400`. the media file now lands in the doc's real folder (`media/x (1)/pic.png`, before `media/x%20%281%29/pic.png`). a link the old upload inserted for such a folder now reads as the missing `media/x (1)/pic.png` - fix them with "Repair Broken Links" on the admin page, which suggests the media file the link points at. the response has a new `link` field, the ready-to-insert markdown link
- api: `GET /api/files/autocomplete`, `GET /api/media/autocomplete` and `GET /api/files/headers` take an optional `link=wiki|markdown` and then also return each suggestion as ready-to-insert link text (`link` in json, `data-link` on the html list item) - insert that instead of building the link from the plain path (`value` / `data-value`), which isn't encoded for a link
- files: new file and folder names can't contain `#`, `?`, `|`, `[`, `]` or `\` or start or end with a space anymore - creating, renaming or moving a file or folder, renaming media and saving a filter or tracker with such a name is answered with `400`. existing files with these characters (git sync, copied in by hand) keep working and can still be moved to another folder or renamed to a valid name - rename them if you want to link them without encoding

## changes
- remove all selected rows/columns in table editor
- highlight the toc entry matching the url hash in the sidebar
- add optional setting to remove empty folders in the file sync cronjob

## features
- rename a kanban status everywhere from the settings page (tags, foldersync folders, card order, history)
- live table of contents in the codemirror editor

## fixes
- serve media whose url a browser encodes differently from go
- resolve link anchors written as heading text to the heading id

