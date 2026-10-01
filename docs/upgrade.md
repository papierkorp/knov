<!-- upgrade notes for the next release: what changed, what the user has to do now, what is deprecated or removed (e.g. "## breaking changes", "## deprecated", "## removed" sections, no "# " headings). `make release` copies them into docs/releases/<version>.md and resets this file -->

## breaking changes

- api: `POST /api/files/export/zip` and `POST /api/files/export/markdown-converted` were removed. use `GET /api/exports/files` / `GET /api/exports/markdown` instead (the archive is streamed directly, no more building it in memory first)
- logging: the `dokuwiki-export` log key was removed, the bulk export logs (files, markdown, pdf) now go to the `export` log. an existing `dokuwiki-export.log` can be deleted
