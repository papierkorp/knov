# HTMX Navigation Implementation Summary

## Overview
Implemented HTMX-powered navigation throughout the builtin theme with structured HTML responses and uniform class/id conventions.

## Key Changes

### 1. Base Template Structure (`themes/builtin/templates/base.html`)

**Updated:**
- Changed `#wrapper` to `#app-wrapper` (consistent naming)
- Changed `<main>` to `<main id="app-main">` (HTMX target for page navigation)
- Added loading indicator: `<div class="htmx-indicator" id="app-loading">`
- Added global HTMX event listeners:
  - `htmx:configRequest` - Adds custom headers
  - `htmx:afterSwap` - Updates page title
  - `htmx:beforeRequest` / `htmx:afterSettle` - Shows/hides loading indicator

**Navigation Links:**
All header navigation links now include HTMX attributes:
```html
<a href="/overview" 
   hx-get="/overview" 
   hx-target="#app-main" 
   hx-push-url="true" 
   hx-swap="innerHTML swap:0.2s"
   class="htmx-nav">Overview</a>
```

### 2. FileView Template (`themes/builtin/templates/fileview.html`)

**Structure:**
```html
<div id="page-fileview" class="page-container" data-filepath="..." data-page-title="...">
  <div id="section-fileview-detailed" class="section-container">
    <article id="section-content" class="file-content">...</article>
    <aside id="section-sidebar" class="file-sidebar">
      <section id="section-metadata">...</section>
      <section id="section-links">...</section>
      <section id="section-toc">...</section>
    </aside>
  </div>
</div>
```

**Fragment IDs:**
- `#fragment-metadata-collection`
- `#fragment-metadata-created`
- `#fragment-metadata-edited`
- `#fragment-metadata-tags`
- `#fragment-metadata-folders`
- `#fragment-links-parents`
- `#fragment-links-ancestors`
- `#fragment-links-children`
- `#fragment-links-outbound`
- `#fragment-links-inbound`

### 3. Server Changes (`internal/server/server.go`)

**New Functionality:**
- Detects HTMX requests via headers: `HX-Request: true` or `X-Requested-With: HTMX`
- For HTMX requests: calls `tm.RenderContent()` (returns only content div)
- For normal requests: calls `tm.RenderPage()` (returns full HTML page)

```go
isHTMX := r.Header.Get("HX-Request") == "true" || r.Header.Get("X-Requested-With") == "HTMX"

if isHTMX {
    tm.RenderContent(w, "fileview.html", data)
} else {
    tm.RenderPage(w, "fileview.html", data)
}
```

### 4. Theme Manager (`internal/thememanager/thememanager.go`)

**New Method:**
- `RenderContent(w io.Writer, page string, data interface{}) error`
- Executes only the `"content"` template (not `"base.html"`)
- Added to `IThemeManager` interface

**How it works:**
- `RenderPage()` → `ExecuteTemplate("base.html")` → Full page with header/footer
- `RenderContent()` → `ExecuteTemplate("content")` → Just the page content div

### 5. API Link Endpoints (`internal/server/api_links.go`)

**New Helper Function:**
```go
func createFileLink(linkPath, filename string) string {
    return fmt.Sprintf(
        `<a href="/files/%s" hx-get="/files/%s" hx-target="#app-main" hx-push-url="true" hx-swap="innerHTML swap:0.2s" class="htmx-nav" title="%s">%s</a>`,
        linkPath, linkPath, linkPath, filename,
    )
}
```

**Updated Endpoints:**
- `/api/links/parents`
- `/api/links/ancestors`
- `/api/links/kids`
- `/api/links/used`
- `/api/links/linkstohere`

All now return HTMX-enabled navigation links.

### 6. CSS (`themes/builtin/static/css/base.css`)

**Added:**
- `.htmx-indicator` - Loading bar at top of page
- `.htmx-indicator.active` - Shows loading animation
- `@keyframes htmx-loading` - Loading bar animation
- `@keyframes htmx-slide` - Sliding gradient effect
- `.htmx-swapping` / `.htmx-settling` - Transition effects

## Navigation Flow

### Example: User clicks a file link

1. **Initial State:** User on `/home`
2. **Click:** User clicks link to `/files/test.md`
3. **HTMX Request:** 
   - `GET /files/test.md`
   - Headers: `HX-Request: true`, `X-Requested-With: HTMX`
4. **Server Response:**
   - Detects HTMX request
   - Calls `tm.RenderContent()` 
   - Returns only: `<div id="page-fileview">...</div>`
5. **HTMX Swap:**
   - Replaces content of `#app-main`
   - Updates URL to `/files/test.md`
   - Triggers `afterSwap` event → updates page title
6. **Result:** Smooth page transition without full reload

## Benefits

✅ **Fast Navigation:** No full page reload, only content swaps  
✅ **Progressive Enhancement:** Works without JS (falls back to href)  
✅ **SEO Friendly:** Full HTML on initial load, HTMX for subsequent navigation  
✅ **Browser History:** `hx-push-url` maintains back/forward buttons  
✅ **Structured HTML:** Consistent IDs/classes for easy targeting  
✅ **Loading Feedback:** Visual indicator during navigation  
✅ **Smooth Transitions:** 0.2s swap animation  

## Testing Checklist

- [ ] Navigate from home to file view
- [ ] Click sidebar links (parents, children, etc.)
- [ ] Use header navigation (Overview, History, Settings)
- [ ] Test browser back/forward buttons
- [ ] Verify URL updates in address bar
- [ ] Check loading indicator appears/disappears
- [ ] Test with JS disabled (should fallback to normal links)
- [ ] Verify metadata fragments load correctly

## Future Enhancements

1. **Add HTMX to more pages:**
   - Dashboard
   - Settings  
   - Overview
   - Search results

2. **Optimize fragment loading:**
   - Add debouncing to prevent rapid requests
   - Implement caching for frequently accessed data

3. **Enhanced transitions:**
   - Add view transition API for smoother swaps
   - Custom animations per page type

4. **Error handling:**
   - Show user-friendly error messages
   - Retry logic for failed requests
