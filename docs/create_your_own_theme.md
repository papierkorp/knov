# Creating a Custom Theme

A theme is just a folder of HTML templates, CSS, and JavaScript — no Go knowledge needed. `themes/example/` is a minimal, working reference theme; building your own means copying it and reshaping the markup and styles. Jump to the [Quickstart](#quickstart) to start right away.

This section covers what the app enforces — a theme has to follow these rules to load and work.

## Structure & Loading

- `themes/<name>/` holds `theme.json`, one `.gohtml` file per page (every template in the example theme is required, none may be missing or empty), and any CSS/JS files you like. The folder name is the theme's identifier.
- Everything in the folder is served automatically under `/themes/<name>/` — no registration step.
- New theme folders are discovered at startup; a broken or incomplete theme is simply skipped with a log warning, never fatal. Switching between loaded themes (**Settings → Theme**) needs no restart.
- `themes/overwrite/` is reserved: a single template placed there overrides that template for whatever theme is active.

## Templates & Data

- Templates are plain HTML with `{{...}}` placeholders the server fills in. `base.gohtml` is the shared layout (head, navigation, scripts); every other template only defines a `content` block that gets rendered inside it. The comments in the example's `base.gohtml` explain every placeholder it must include.
- Every page receives a shared set of data: `.Title`, `.CurrentTheme`, `.ThemeSettings`, `.Language`, `.DateFormat`, `.Version`, `.FontStyleTag` (the user's font settings — must be rendered in `<head>`), and `.HeaderNavLinks` / `.MenuNavLinks` (the user-configured navigation). Pages add their own fields on top — the built-in theme (`themes/builtin/`) shows every field in real use and is the practical reference.
- Helper functions are available inside `{{...}}`: `T` translates a string, `urlQuery` / `urlPath` safely encode file paths for URLs. Again, `themes/builtin/` shows them in use.
- External `.js` files can't contain `{{...}}` placeholders — they are served as-is.

## Loading Data with htmx

The app is htmx-first: pages are mostly shells that load their content as HTML fragments via `hx-get` / `hx-post`. The example theme's `settings.gohtml` shows the pattern:

```html
<select name="theme" hx-get="/api/themes" hx-target="this" hx-swap="innerHTML" hx-trigger="load">
```

On page load, htmx requests `/api/themes` and the server answers with ready-made `<option>` elements, which htmx swaps into the select — no JavaScript written, no JSON parsed. Every endpoint works this way: it returns an HTML fragment when the request asks for HTML (htmx's default) and JSON otherwise — same URL, same data, so you can inspect any endpoint with your browser or `curl`.

- Mutating endpoints accept ordinary form data (`<form hx-post="...">` just works), not JSON.
- Success and error toasts are triggered by the server and rendered by an auto-injected script — your theme never handles them.
- Full endpoint reference: the Swagger UI at `/swagger/index.html` (see [docs/api.md](api.md)), which documents every route, its parameters, and responses.

## Included on Every Page

- Auto-injected into `<head>` / `<body>`: `static/css/defaults.css` (a fallback for every CSS variable — open it for the full list of colors like `--bg`, `--text`, `--primary` and fonts like `--font-body`) before your stylesheet, the user's custom CSS after it, and the app's core scripts and styles (notifications, editors, autocomplete, and similar) — don't add these yourself.
- Bundled and served offline from `/static/` — no CDNs: htmx, SortableJS, Font Awesome, and the editor libraries. You link the ones you use from `base.gohtml` yourself.
- System pages (`/system/*` — changelog, logs, and so on) render their fixed content inside your `base.gohtml`; style them via the `.system-page*` classes.

# Best Practices

Nothing here is enforced, but the built-in themes follow these conventions and your theme will integrate best if it does too:

- Never hardcode colors — use the CSS variables, and only override the ones your theme wants to change (`defaults.css` covers the rest).
- Keep global rules in `style.css`; scope page- or component-specific rules under ID selectors (`#page-*`, `#component-*`, `#view-*`).
- Dark mode is pure convention: keep the `darkMode` and `colorScheme` settings from the example's `theme.json`, mirror them as `data-*` attributes on `<body>`, and style against those attributes — that's the whole mechanism.
- Reference your own assets as `/themes/{{.CurrentTheme}}/...` instead of hardcoding the folder name, so a copied or renamed theme folder keeps loading its own files.
- Pass server values to external `.js` files via `data-*` attributes or a small inline config script, and keep the logic in the external file.
- Run user-facing text through the `T` template function so it gets translated.

# Theme Configuration

- `theme.json` describes your theme — `name`, `version`, `author`, `description` are all required. `themes/example/theme.json` is the reference.
- It can also declare your own options (toggles, dropdowns, text, numbers) under `themeSettings`. Declared options appear in the Settings UI automatically and are available in every template via `.ThemeSettings` — the example theme's `darkMode` and `colorScheme` are exactly this.

# Quickstart

```bash
cp -r themes/example themes/mytheme
```

1. Edit `theme.json`: set your own `name`, `version`, `author`, `description` — see [Theme Configuration](#theme-configuration).
2. Restart the app so the new folder is discovered, then activate it under **Settings → Theme**.
3. Read `base.gohtml` and `css/style.css` first — both are commented inline and explain everything a theme must render — see [Templates & Data](#templates--data).
4. Reshape the remaining page templates one by one, borrowing from `themes/builtin/` where you need to see real data and [htmx calls](#loading-data-with-htmx) in action, and following the [Best Practices](#best-practices).
