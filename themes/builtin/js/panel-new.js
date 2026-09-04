// theme: builtin — "new" rail snippet: static quick-links to create
// each content type. No per-instance state, so `body` just ignores its args.
// injected via x-html, so Alpine never binds @click here — close the panel
// with the plain global closePanel() instead (same pattern as panel-file.js).
railSnippetByID("new").body = () => `<div class="flyout-content" data-snippet="new">
  <a href="/dashboard/new" onclick="closePanel()">Dashboard</a>
  <a href="/files/new/codemirror" onclick="closePanel()">CodeMirror</a>
  <a href="/files/new/list" onclick="closePanel()">List</a>
  <a href="/files/new/todo" onclick="closePanel()">Todo</a>
  <a href="/files/new/filter" onclick="closePanel()">Filter</a>
  <a href="/files/new/index" onclick="closePanel()">Index</a>
  <a href="/files/new/book" onclick="closePanel()">Book</a>
</div>`;
