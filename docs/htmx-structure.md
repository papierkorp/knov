# HTMX Structure & Naming Conventions

## Core Principles

1. **Uniform ID/Class Structure**: All HTMX targets follow consistent naming
2. **Component-based**: Each component has a clear boundary and responsibility
3. **Progressive Enhancement**: Works without JavaScript, enhanced with HTMX
4. **URL State Management**: All navigation updates URL with `hx-push-url`

## Naming Convention

### IDs (HTMX Targets)
- `#app-main` - Main content area (entire page content)
- `#app-header` - Header component
- `#app-sidebar` - Sidebar (if present)
- `#component-{name}` - Reusable components (e.g., `#component-header`)
- `#page-{name}` - Page-specific containers (e.g., `#page-fileview`, `#page-dashboard`)
- `#section-{name}` - Page sections (e.g., `#section-metadata`, `#section-toc`)
- `#fragment-{name}` - Small dynamic fragments (e.g., `#fragment-breadcrumb`)

### Classes
- `.htmx-nav` - Navigation links that use HTMX
- `.htmx-indicator` - Loading indicators
- `.htmx-swapping` - Applied during swap
- `.page-container` - Wraps page content
- `.section-container` - Wraps sections
- `.component` - Base class for components

## Page Structure

```html
<body>
  <div id="app-wrapper">
    <header id="component-header"><!-- Always present --></header>
    <main id="app-main">
      <div id="page-{pagename}" class="page-container">
        <div id="section-{name}" class="section-container">
          <!-- Section content -->
        </div>
      </div>
    </main>
  </div>
</body>
```

## Navigation Patterns

### Full Page Navigation
```html
<a href="/path" 
   hx-get="/path" 
   hx-target="#app-main" 
   hx-push-url="true"
   hx-swap="innerHTML swap:0.2s"
   class="htmx-nav">Link</a>
```

### Partial Navigation (Same Page)
```html
<a href="/files/example.md"
   hx-get="/api/files/content/example.md"
   hx-target="#section-content"
   hx-swap="innerHTML swap:0.2s">File</a>
```

### Fragment Updates
```html
<div hx-get="/api/metadata/tags" 
     hx-trigger="load" 
     hx-target="#fragment-tags">
  Loading...
</div>
```

## API Response Structure

### Full Page Responses
Return complete page div with id:
```html
<div id="page-fileview" class="page-container" data-filepath="example.md">
  <div id="section-content">...</div>
  <div id="section-sidebar">...</div>
</div>
```

### Fragment Responses
Return just the content, no wrapper:
```html
<ul class="tags-list">
  <li><a href="/browse/tags/work">work</a></li>
</ul>
```

## Data Attributes

Use data attributes for state:
- `data-filepath` - Current file path
- `data-dashboard-id` - Dashboard ID
- `data-view-mode` - Current view mode
- `data-loading` - Loading state

## Example: File View Navigation

```html
<!-- Link in file list -->
<a href="/files/test.md"
   hx-get="/files/test.md"
   hx-target="#app-main"
   hx-push-url="true"
   hx-swap="innerHTML swap:0.2s"
   hx-indicator=".htmx-indicator"
   class="htmx-nav">
  test.md
</a>

<!-- Server returns -->
<div id="page-fileview" class="page-container" data-filepath="test.md">
  <div id="section-content" class="section-container">
    <article class="file-content"><!-- HTML content --></article>
  </div>
  <div id="section-sidebar" class="section-container">
    <!-- Sidebar with hx-get for dynamic parts -->
  </div>
</div>
```
