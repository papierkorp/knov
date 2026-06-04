# Styling Guide

Reference for CSS class naming conventions, file structure, and theming rules across the project.

---

## 1. Core Principles

- **Classes for styling, IDs for function** — never style by ID; use classes instead
- **IDs only for HTMX swap targets and form label anchors**
- **Theme-agnostic class names** — classes describe structure/role, not visual appearance
- **Lowercase kebab-case everywhere** — no underscores, no camelCase
- **Flat child selectors** — `.section-metadata .label`, not `.section-metadata__label`

---

## 2. Prefix Rules

| Prefix | Purpose | Scope | Example |
|---|---|---|---|
| `page-` | Top-level page wrapper | One per route/template | `page-fileview`, `page-settings` |
| `layout-` | Structural layout regions | Shared across pages | `layout-sidebar`, `layout-header` |
| `section-` | Named content regions within a page | Page-scoped | `section-metadata`, `section-toc` |
| `component-` | Reusable UI components | Used anywhere | `component-search`, `component-table` |
| `widget-` | Dashboard widgets only | Dashboard pages | `widget-files`, `widget-filter` |
| *(none)* | Generic utility elements | Global | `btn`, `card`, `badge`, `status-ok` |

---

## 3. Naming Format

All classes use **lowercase kebab-case**:

```
page-file-edit       ✓
page-fileEdit        ✗
page_file_edit       ✗
pageFileEdit         ✗
```

Multi-word names separate every word with a hyphen:

```
page-browse-files    ✓
page-browsefiles     ✗
```

---

## 4. Modifiers

State and variants use a `--modifier` suffix on the base class:

```css
.btn--danger
.btn--small
.page-fileview--compact
.layout-sidebar--collapsed
.component-search--open
```

The modifier is always applied **in addition to** the base class:

```html
<button class="btn btn--danger">Delete</button>
<div class="page-fileview page-fileview--compact">...</div>
```

---

## 5. When to Use IDs

IDs are **only** permitted for:

1. **HTMX swap targets** — elements that receive `hx-target` or `hx-swap-oob`
2. **Form label anchors** — `<label for="...">` / `<input id="...">`

```html
<!-- OK: HTMX swap target -->
<div id="filter-results">...</div>
<div id="upload-status">...</div>

<!-- OK: form label anchor -->
<label for="media-file">File</label>
<input id="media-file" type="file">

<!-- NOT OK: styling hook -->
<div id="page-home">...</div>          <!-- use class="page-home" -->
<div id="component-sidebar">...</div>  <!-- use class="layout-sidebar" -->
```

---

## 6. CSS File Structure

### Global styles (shared across all themes)

Located in `static/css/`:

| File | Contents |
|---|---|
| `style.css` | Entry point — imports all other static CSS files |
| `base.css` | Resets, root variables, body defaults |
| `global.css` | Utility classes: `.btn`, `.card`, `.badge`, `.status-*` |
| `codehighlight.css` | Syntax highlighting (injected automatically) |
| `*editor.css` | Editor-specific styles (injected automatically) |

### Theme-specific styles

Located in `themes/<name>/style/`:

Each theme has its own folder with:

| File | Contents |
|---|---|
| `style.css` | Entry point — imports all sub-files |
| `base.css` | Theme-level overrides of variables and resets |
| `global.css` | Theme overrides of global utility classes |
| `<pagename>.css` | Page-scoped styles using `.page-<name>` selectors |
| `<componentname>.css` | Component styles using `.component-<name>` selectors |
| `layout.css` | Layout region styles using `.layout-*` selectors |
| `sidebar.css` | Sidebar styles using `.layout-sidebar`, `.section-*` |

### Selectors by file type

```css
/* global.css — generic utilities only */
.btn { ... }
.card { ... }
.badge { ... }
.status-ok { ... }

/* fileview.css — page-scoped */
.page-fileview { ... }
.page-fileview .section-metadata { ... }

/* sidebar.css — layout + sections */
.layout-sidebar { ... }
.section-metadata { ... }
.section-toc { ... }
.section-connections { ... }

/* search.css — component + page */
.component-search { ... }
.page-search { ... }
```

---

## 7. Full Class Reference

### Pages

```
page-home
page-fileview
page-file-editor
page-file-editor
page-file-dittable
page-files-overview
page-search
page-settings
page-admin
page-browse
page-browse-files
page-browse-metadata
page-dashboard
page-dashboard-editor
page-dashboard-editor
page-filter
page-media
page-media-overview
page-history
page-latest-changes
page-chat
page-playground
page-help
```

### Layout regions

```
layout-header
layout-sidebar
layout-main
layout-footer
layout-content
```

### Sections (within pages or sidebar)

```
section-metadata
section-toc
section-connections
section-para
section-version-history
section-actions
section-file-content
```

### Components

```
component-search
component-search-dropdown
component-table
component-table-controls
component-media-upload
component-media-preview
component-pagination
component-filter
```

### Widgets (dashboard only)

```
widget-files
widget-filter
widget-latest-changes
widget-search
```

### Generic utilities

```
btn
btn--danger
btn--small
btn--primary

card

badge
badge--info
badge--warning
badge--error

status-ok
status-error
status-warning
status-info

form-group
form-label
form-input
form-actions
```

---

## 8. Render Package Rules

HTML generated in the `render` package must:

- Use the class conventions above — no inline IDs for styling
- Use IDs only when the element is a known HTMX swap target
- Use `translation.SprintfForRequest` for every user-visible string
- Emit classes that are theme-agnostic (no color/visual names like `red-button`)

```go
// correct
fmt.Sprintf(`<div class="section-metadata">...</div>`)
fmt.Sprintf(`<div id="upload-status"></div>`)  // HTMX target

// incorrect
fmt.Sprintf(`<div id="component-sidebar-metadata">...</div>`)  // ID for styling
fmt.Sprintf(`<div class="blue-card">...</div>`)  // visual name
```

---

## 9. Template Rules

Each `.gohtml` template uses exactly **one** `page-*` wrapper:

```html
{{ define "content" }}
<div class="page-fileview">
    <div class="layout-main">
        <article class="section-file-content">
            ...
        </article>
        <aside class="layout-sidebar">
            <section class="section-metadata">...</section>
            <section class="section-toc">...</section>
        </aside>
    </div>
</div>
{{ end }}
```

---

## 10. Migration Checklist (per page)

When refactoring a page, work through this order:

1. `themes/builtin/<page>.gohtml` — replace ID wrappers with classes
2. `themes/rail/<page>.gohtml` — same changes
3. `render_<page>.go` — update any emitted IDs to classes (keep HTMX target IDs)
4. `themes/builtin/style/<page>.css` — rewrite selectors from `#page-*` to `.page-*`
5. `themes/rail/style/<page>.css` — same
6. Verify HTMX targets still have their IDs intact
