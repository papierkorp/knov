// theme: builtin-reworked — "new" rail snippet: static quick-links to create
// each content type. No per-instance state, so `body` just ignores its args.
railSnippetByID("new").body = () => `<div class="flyout-content" data-snippet="new">
  <a href="/dashboard/new">Dashboard</a>
  <a href="/files/new/codemirror">CodeMirror</a>
  <a href="/files/new/list">List</a>
  <a href="/files/new/todo">Todo</a>
  <a href="/files/new/filter">Filter</a>
  <a href="/files/new/index">Index</a>
</div>`;
